package v1_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/handlers/api/v1/types"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

// forkAs has `forker` fork the gist at /{parentUser}/{parentIdent} via the
// web /fork endpoint and returns the resulting db.Gist (looked up by the
// redirect Location). Caller is logged out on return.
func forkAs(t *testing.T, s *webtest.Server, forker, parentUser, parentIdent string) *db.Gist {
	s.Login(t, forker)
	resp := s.Request(t, "POST", "/"+parentUser+"/"+parentIdent+"/fork", nil, 302)
	loc := resp.Header.Get("Location")
	parts := strings.Split(strings.TrimPrefix(loc, "/"), "/")
	require.Len(t, parts, 2, "fork redirect must be /{user}/{ident}, got %q", loc)
	s.Logout()
	fork, err := db.GetGist(forker, parts[1])
	require.NoError(t, err)
	return fork
}

// setVisibility flips a gist's visibility in the DB directly - saves
// chaining PATCHes through the API just to set up test fixtures.
func setVisibility(t *testing.T, g *db.Gist, v db.Visibility) {
	g.Private = v
	require.NoError(t, g.Update())
}

// --- Visibility / access matrix on the parent gist ---

// TestListForks_VisibilityAccess mirrors the matrix for /commits: the access
// rule comes from lookupGistByUUID, so the response codes must be identical.
func TestListForks_VisibilityAccess(t *testing.T) {
	s := setupGetGist(t)

	_, gPub, _, _ := s.CreateGistAs(t, "owner", "0")
	_, gUnl, _, _ := s.CreateGistAs(t, "owner", "1")
	_, gPriv, _, _ := s.CreateGistAs(t, "owner", "2")

	ownerTok := apiTokenFor(t, s, "owner", db.ReadPermission)
	ownerNoScope := apiTokenFor(t, s, "owner", db.NoPermission)
	otherTok := apiTokenFor(t, s, "other", db.ReadPermission)

	cases := []struct {
		name string
		uuid string
		tok  string
		want int
	}{
		// Public: readable by anyone.
		{"public/anonymous", gPub.Uuid, "", 200},
		{"public/no-scope owner", gPub.Uuid, ownerNoScope, 200},
		{"public/scoped owner", gPub.Uuid, ownerTok, 200},
		{"public/scoped other", gPub.Uuid, otherTok, 200},

		// Unlisted: URL-shareable.
		{"unlisted/anonymous", gUnl.Uuid, "", 200},
		{"unlisted/scoped other", gUnl.Uuid, otherTok, 200},

		// Private: owner + gist:read only.
		{"private/anonymous", gPriv.Uuid, "", 404},
		{"private/no-scope owner", gPriv.Uuid, ownerNoScope, 404},
		{"private/scoped other", gPriv.Uuid, otherTok, 404},
		{"private/scoped owner", gPriv.Uuid, ownerTok, 200},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s.APIRequest(t, "GET", "/api/gists/"+c.uuid+"/forks", c.tok, nil, c.want)
		})
	}
}

func TestListForks_NotFound(t *testing.T) {
	s := setupGetGist(t)
	s.APIRequest(t, "GET", "/api/gists/does-not-exist/forks", "", nil, 404)
}

// --- Visibility filter on the forks list itself ---

// forksFixture sets up a public parent and three forks: alice's = public,
// bob's = unlisted, charlie's = private. Returned alongside everyone's
// tokens so tests can assert what each viewer sees.
type forksFixture struct {
	s                               *webtest.Server
	parent                          *db.Gist
	aliceFork, bobFork, charlieFork *db.Gist
	aliceTok, bobTok, charlieTok    string
}

func setupForksVisibility(t *testing.T) *forksFixture {
	s := webtest.Setup(t)
	t.Cleanup(func() { webtest.Teardown(t) })
	s.Register(t, "admin")
	s.Logout()
	s.Register(t, "parentowner")
	s.Register(t, "alice")
	s.Register(t, "bob")
	s.Register(t, "charlie")

	_, parent, parentUser, parentIdent := s.CreateGistAs(t, "parentowner", "0")

	// Each user forks; fork inherits the parent's public visibility.
	aliceFork := forkAs(t, s, "alice", parentUser, parentIdent)
	bobFork := forkAs(t, s, "bob", parentUser, parentIdent)
	charlieFork := forkAs(t, s, "charlie", parentUser, parentIdent)

	// Diversify visibilities on the forks. alice stays public.
	setVisibility(t, bobFork, db.UnlistedVisibility)
	setVisibility(t, charlieFork, db.PrivateVisibility)

	return &forksFixture{
		s:           s,
		parent:      parent,
		aliceFork:   aliceFork,
		bobFork:     bobFork,
		charlieFork: charlieFork,
		aliceTok:    apiTokenFor(t, s, "alice", db.ReadPermission),
		bobTok:      apiTokenFor(t, s, "bob", db.ReadPermission),
		charlieTok:  apiTokenFor(t, s, "charlie", db.ReadPermission),
	}
}

// idSetSimple returns the set of ids from a []GistSimple slice.
func idSetSimple(arr []types.GistSimple) map[string]bool {
	out := make(map[string]bool, len(arr))
	for _, g := range arr {
		out[g.ID] = true
	}
	return out
}

// TestListForks_Anonymous_OnlyPublicForks - an unauthenticated caller sees
// only public forks; unlisted and private forks stay hidden regardless of
// who owns them.
func TestListForks_Anonymous_OnlyPublicForks(t *testing.T) {
	f := setupForksVisibility(t)

	arr, _ := apiList[types.GistSimple](t, f.s, "/api/gists/"+f.parent.Uuid+"/forks?per_page=20", "", 200)

	ids := idSetSimple(arr)
	require.True(t, ids[f.aliceFork.Uuid], "alice's PUBLIC fork must appear")
	require.False(t, ids[f.bobFork.Uuid], "bob's UNLISTED fork must NOT appear for anonymous")
	require.False(t, ids[f.charlieFork.Uuid], "charlie's PRIVATE fork must NOT appear for anonymous")
	for _, g := range arr {
		require.Equal(t, "public", g.Visibility,
			"non-public fork leaked into anonymous response: %+v", g)
	}
}

// TestListForks_AuthenticatedSeesOwnUnlistedFork - bob's token must
// surface his own unlisted fork (in addition to public forks), but not
// other users' non-public forks.
func TestListForks_AuthenticatedSeesOwnUnlistedFork(t *testing.T) {
	f := setupForksVisibility(t)

	arr, _ := apiList[types.GistSimple](t, f.s, "/api/gists/"+f.parent.Uuid+"/forks?per_page=20", f.bobTok, 200)

	ids := idSetSimple(arr)
	require.True(t, ids[f.aliceFork.Uuid], "alice's PUBLIC fork must appear")
	require.True(t, ids[f.bobFork.Uuid], "bob's own UNLISTED fork must appear when bob is the caller")
	require.False(t, ids[f.charlieFork.Uuid], "charlie's PRIVATE fork must NOT leak to bob")
}

// TestListForks_AuthenticatedSeesOwnPrivateFork - same idea, swapped to
// charlie's token + private fork.
func TestListForks_AuthenticatedSeesOwnPrivateFork(t *testing.T) {
	f := setupForksVisibility(t)

	arr, _ := apiList[types.GistSimple](t, f.s, "/api/gists/"+f.parent.Uuid+"/forks?per_page=20", f.charlieTok, 200)

	ids := idSetSimple(arr)
	require.True(t, ids[f.aliceFork.Uuid], "alice's PUBLIC fork must appear")
	require.False(t, ids[f.bobFork.Uuid], "bob's UNLISTED fork must NOT leak to charlie")
	require.True(t, ids[f.charlieFork.Uuid], "charlie's own PRIVATE fork must appear when charlie is the caller")
}

// TestListForks_ScopedThirdParty_OnlyPublic - alice owns only the public
// fork in this fixture, so even with a scoped token she sees just her own
// public fork; bob's unlisted and charlie's private stay hidden.
func TestListForks_ScopedThirdParty_OnlyPublic(t *testing.T) {
	f := setupForksVisibility(t)

	arr, _ := apiList[types.GistSimple](t, f.s, "/api/gists/"+f.parent.Uuid+"/forks?per_page=20", f.aliceTok, 200)

	ids := idSetSimple(arr)
	require.True(t, ids[f.aliceFork.Uuid], "alice's PUBLIC fork must appear")
	require.False(t, ids[f.bobFork.Uuid], "bob's UNLISTED fork must NOT leak to alice")
	require.False(t, ids[f.charlieFork.Uuid], "charlie's PRIVATE fork must NOT leak to alice")
}

// =========================================================================
// GET /api/gists/forked - ListForkedGists (the caller's forks)
// =========================================================================

// forkedListFixture: parent_owner owns a public parent gist; the caller
// ("caller") has forked it once. "stranger" has also forked the parent
// independently to check we don't leak other users' forks. Caller also has
// a non-fork gist (regular) which must NOT appear in /gists/forked.
type forkedListFixture struct {
	s            *webtest.Server
	parent       *db.Gist
	callerFork   *db.Gist
	strangerFork *db.Gist
	regularGist  *db.Gist
}

func setupForkedList(t *testing.T) *forkedListFixture {
	s := webtest.Setup(t)
	t.Cleanup(func() { webtest.Teardown(t) })
	s.Register(t, "admin")
	s.Logout()
	s.Register(t, "parentowner")
	s.Register(t, "caller")
	s.Register(t, "stranger")

	_, parent, parentUser, parentIdent := s.CreateGistAs(t, "parentowner", "0")
	callerFork := forkAs(t, s, "caller", parentUser, parentIdent)
	strangerFork := forkAs(t, s, "stranger", parentUser, parentIdent)

	_, regular, _, _ := s.CreateGistAs(t, "caller", "0") // not a fork

	return &forkedListFixture{
		s:            s,
		parent:       parent,
		callerFork:   callerFork,
		strangerFork: strangerFork,
		regularGist:  regular,
	}
}

func TestListForkedGists_NoAuth(t *testing.T) {
	s := setupGetGist(t)
	s.APIRequest(t, "GET", "/api/gists/forked", "", nil, 401)
}

func TestListForkedGists_EmptyWhenNoForks(t *testing.T) {
	s := webtest.Setup(t)
	t.Cleanup(func() { webtest.Teardown(t) })
	s.Register(t, "admin")
	s.Logout()
	s.Register(t, "thomas")

	tok := apiTokenFor(t, s, "thomas", db.ReadPermission)

	arr, _ := apiList[types.GistSimple](t, s, "/api/gists/forked", tok, 200)
	require.Empty(t, arr)
}

// TestListForkedGists_ReturnsOnlyCallersForks - caller's token returns only
// their fork; stranger's fork stays hidden, and the caller's non-fork
// regular gist must NOT appear (the endpoint is forks-only).
func TestListForkedGists_ReturnsOnlyCallersForks(t *testing.T) {
	f := setupForkedList(t)
	callerTok := apiTokenFor(t, f.s, "caller", db.ReadPermission)

	arr, _ := apiList[types.GistSimple](t, f.s, "/api/gists/forked?per_page=20", callerTok, 200)

	ids := idSetSimple(arr)
	require.True(t, ids[f.callerFork.Uuid], "caller's fork must appear")
	require.False(t, ids[f.strangerFork.Uuid], "stranger's fork must NOT appear in caller's /forked")
	require.False(t, ids[f.regularGist.Uuid], "caller's regular (non-fork) gist must NOT appear")
}

// TestListForkedGists_TokenWithGistRead_IncludesOwnPrivateFork - with
// gist:read, the caller's private/unlisted forks come through too.
func TestListForkedGists_TokenWithGistRead_IncludesOwnPrivateFork(t *testing.T) {
	f := setupForkedList(t)
	setVisibility(t, f.callerFork, db.PrivateVisibility)
	callerTok := apiTokenFor(t, f.s, "caller", db.ReadPermission)

	arr, _ := apiList[types.GistSimple](t, f.s, "/api/gists/forked?per_page=20", callerTok, 200)

	ids := idSetSimple(arr)
	require.True(t, ids[f.callerFork.Uuid],
		"caller's own PRIVATE fork must appear when token has gist:read")
}

// TestListForkedGists_TokenWithoutGistRead_OnlyPublicForks - token without
// gist:read drops to public-only: a private fork the caller owns is hidden
// (same soft-scope rule as /gists and /gists/liked).
func TestListForkedGists_TokenWithoutGistRead_OnlyPublicForks(t *testing.T) {
	f := setupForkedList(t)
	setVisibility(t, f.callerFork, db.UnlistedVisibility)
	noScopeTok := apiTokenFor(t, f.s, "caller", db.NoPermission)

	arr, _ := apiList[types.GistSimple](t, f.s, "/api/gists/forked?per_page=20", noScopeTok, 200)

	ids := idSetSimple(arr)
	require.False(t, ids[f.callerFork.Uuid],
		"caller's UNLISTED fork must be hidden when token lacks gist:read")
	for _, g := range arr {
		require.Equal(t, "public", g.Visibility,
			"non-public fork leaked into no-scope response: %+v", g)
	}
}

// --- Shape ---

func TestListForks_Shape(t *testing.T) {
	s := setupGetGist(t)
	_, parent, parentUser, parentIdent := s.CreateGistAs(t, "owner", "0")
	fork := forkAs(t, s, "other", parentUser, parentIdent)

	arr, _ := apiList[types.GistSimple](t, s, "/api/gists/"+parent.Uuid+"/forks", "", 200)

	require.Len(t, arr, 1)
	got := arr[0]
	require.Equal(t, fork.Uuid, got.ID)
	require.Equal(t, "other", got.Owner.Login)
	require.Equal(t, "public", got.Visibility)
	require.True(t, got.Public)
	require.False(t, got.CreatedAt.IsZero())
}

// =========================================================================
// POST /api/gists/:uuid/forks
// =========================================================================

// --- Auth / scope ---

func TestForkGist_NoAuth(t *testing.T) {
	s := setupGetGist(t)
	_, parent, _, _ := s.CreateGistAs(t, "owner", "0")

	// apiRequireAuth on the route → 401 before the handler runs.
	s.APIRequest(t, "POST", "/api/gists/"+parent.Uuid+"/forks", "", nil, 401)
}

func TestForkGist_TokenWithoutWriteScope_403(t *testing.T) {
	s := setupGetGist(t)
	_, parent, _, _ := s.CreateGistAs(t, "owner", "0")

	// Read-only token can read but can't fork (creates a new gist).
	roTok := apiTokenFor(t, s, "other", db.ReadPermission)
	s.APIRequest(t, "POST", "/api/gists/"+parent.Uuid+"/forks", roTok, nil, 403)
}

// --- Visibility / access matrix on the parent ---

// TestForkGist_VisibilityAccess focuses on the lookup-side rule. A caller
// without a token (anonymous) is already rejected by apiRequireAuth above;
// this matrix uses a write-scoped token to drive the handler logic:
//   - Public/unlisted parent owned by someone else → 201 (forkable).
//   - Private parent owned by someone else → 404 (hidden by lookup).
//   - Any parent owned by the caller → 422 (self-fork).
func TestForkGist_VisibilityAccess(t *testing.T) {
	s := setupGetGist(t)

	_, gPub, _, _ := s.CreateGistAs(t, "owner", "0")
	_, gUnl, _, _ := s.CreateGistAs(t, "owner", "1")
	_, gPriv, _, _ := s.CreateGistAs(t, "owner", "2")

	ownerTok := apiTokenFor(t, s, "owner", db.ReadWritePermission)
	otherTok := apiTokenFor(t, s, "other", db.ReadWritePermission)

	cases := []struct {
		name string
		uuid string
		tok  string
		want int
	}{
		// Non-owner forks public/unlisted → 201.
		{"public/other", gPub.Uuid, otherTok, 201},
		{"unlisted/other", gUnl.Uuid, otherTok, 201},

		// Non-owner forks private → 404 (lookup hides existence).
		{"private/other", gPriv.Uuid, otherTok, 404},

		// Owner tries to fork own gists → 422 (self-fork rule).
		{"public/self", gPub.Uuid, ownerTok, 422},
		{"unlisted/self", gUnl.Uuid, ownerTok, 422},
		{"private/self", gPriv.Uuid, ownerTok, 422},

		// Unknown gist → 404.
		{"missing", "doesnotexist", otherTok, 404},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s.APIRequest(t, "POST", "/api/gists/"+c.uuid+"/forks", c.tok, nil, c.want)
		})
	}
}

// --- Success path: response shape + side effects ---

func TestForkGist_Success_ResponseAndState(t *testing.T) {
	s := setupGetGist(t)
	_, parent, _, _ := s.CreateGistAs(t, "owner", "0")
	otherTok := apiTokenFor(t, s, "other", db.ReadWritePermission)

	w, body := s.APIRequest(t, "POST", "/api/gists/"+parent.Uuid+"/forks", otherTok, nil, 201)

	var resp types.GistSimple
	require.NoError(t, json.Unmarshal(body, &resp), "body: %s", string(body))

	// Owner of the fork is the caller.
	require.Equal(t, "other", resp.Owner.Login)
	// Visibility inherited from the parent.
	require.Equal(t, "public", resp.Visibility)
	require.True(t, resp.Public)
	// Title carried over.
	require.Equal(t, parent.Title, resp.Title)

	// Location header points to the new fork.
	loc := w.Header().Get("Location")
	require.NotEmpty(t, loc)
	require.Contains(t, loc, "/api/gists/"+resp.ID)

	// DB-side: fork row exists with ForkedID, parent's NbForks bumped.
	fork, err := db.GetGistByUUID(resp.ID)
	require.NoError(t, err)
	require.Equal(t, parent.ID, fork.ForkedID, "fork.ForkedID must point at parent")

	parentReloaded, err := db.GetGistByUUID(parent.Uuid)
	require.NoError(t, err)
	require.Equal(t, 1, parentReloaded.NbForks, "parent's NbForks must increment")
}

// TestForkGist_VisibilityInheritedFromUnlistedParent - confirms forks of
// unlisted parents are also unlisted (and of private - private).
func TestForkGist_VisibilityInheritedFromUnlistedParent(t *testing.T) {
	s := setupGetGist(t)
	_, parent, _, _ := s.CreateGistAs(t, "owner", "1") // unlisted
	otherTok := apiTokenFor(t, s, "other", db.ReadWritePermission)

	_, body := s.APIRequest(t, "POST", "/api/gists/"+parent.Uuid+"/forks", otherTok, nil, 201)
	var resp types.GistSimple
	require.NoError(t, json.Unmarshal(body, &resp))
	require.Equal(t, "unlisted", resp.Visibility)
	require.False(t, resp.Public)
}

// --- Idempotency: forking the same gist twice ---

// TestForkGist_AlreadyForked_200 - second fork attempt by the same user must
// NOT create another fork. The handler returns 200 with the existing fork in
// the body (vs 201 for a fresh fork) and a Location header pointing at it.
func TestForkGist_AlreadyForked_200(t *testing.T) {
	s := setupGetGist(t)
	_, parent, _, _ := s.CreateGistAs(t, "owner", "0")
	otherTok := apiTokenFor(t, s, "other", db.ReadWritePermission)

	// First fork → 201.
	_, firstBody := s.APIRequest(t, "POST", "/api/gists/"+parent.Uuid+"/forks", otherTok, nil, 201)
	var first types.GistSimple
	require.NoError(t, json.Unmarshal(firstBody, &first))

	// Second attempt → 200 with the existing fork echoed back, Location →
	// existing fork.
	w, secondBody := s.APIRequest(t, "POST", "/api/gists/"+parent.Uuid+"/forks", otherTok, nil, 200)
	var second types.GistSimple
	require.NoError(t, json.Unmarshal(secondBody, &second))
	require.Equal(t, first.ID, second.ID, "the existing fork must be returned, not a new one")
	require.Contains(t, w.Header().Get("Location"), "/api/gists/"+first.ID,
		"Location must point at the existing fork")

	// And the parent's NbForks must not double-count.
	parentReloaded, err := db.GetGistByUUID(parent.Uuid)
	require.NoError(t, err)
	require.Equal(t, 1, parentReloaded.NbForks, "duplicate fork attempt must not bump NbForks")
}
