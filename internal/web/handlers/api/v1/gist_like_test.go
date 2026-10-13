package v1_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/handlers/api/v1/types"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

// likedFixture stands up the env shared by the ListLikedGists tests:
// "caller" and "other" each own one gist of every visibility, and "caller"
// stars all six. The API is enabled. Logged out at the end so requests go
// through the token-auth path.
type likedFixture struct {
	s                                *webtest.Server
	callerPub, callerUnl, callerPriv *db.Gist
	otherPub, otherUnl, otherPriv    *db.Gist
}

func setupLiked(t *testing.T) *likedFixture {
	s := webtest.Setup(t)
	t.Cleanup(func() { webtest.Teardown(t) })

	s.Register(t, "admin")
	s.Logout()
	s.Register(t, "other")
	s.Register(t, "caller")

	_, otherPub, _, _ := s.CreateGistAs(t, "other", "0")
	_, otherUnl, _, _ := s.CreateGistAs(t, "other", "1")
	_, otherPriv, _, _ := s.CreateGistAs(t, "other", "2")
	_, callerPub, _, _ := s.CreateGistAs(t, "caller", "0")
	_, callerUnl, _, _ := s.CreateGistAs(t, "caller", "1")
	_, callerPriv, _, _ := s.CreateGistAs(t, "caller", "2")

	// "caller" stars all 6 gists. Stars are inserted directly via the DB
	// helper so the test doesn't depend on the (potentially scope-gated)
	// HTTP /like endpoint.
	caller, err := db.GetUserByUsername("caller")
	require.NoError(t, err)
	for _, g := range []*db.Gist{otherPub, otherUnl, otherPriv, callerPub, callerUnl, callerPriv} {
		require.NoError(t, g.AppendUserLike(caller))
	}

	return &likedFixture{
		s:          s,
		callerPub:  callerPub,
		callerUnl:  callerUnl,
		callerPriv: callerPriv,
		otherPub:   otherPub,
		otherUnl:   otherUnl,
		otherPriv:  otherPriv,
	}
}

func TestListLikedGists_NoAuth(t *testing.T) {
	s := webtest.Setup(t)
	t.Cleanup(func() { webtest.Teardown(t) })
	s.Register(t, "thomas")

	s.Logout()

	// apiRequireAuth on the route rejects anonymous callers.
	s.APIRequest(t, "GET", "/api/gists/liked", "", nil, 401)
}

func TestListLikedGists_EmptyWhenNoStars(t *testing.T) {
	s := webtest.Setup(t)
	t.Cleanup(func() { webtest.Teardown(t) })
	s.Register(t, "admin")
	s.Logout()
	s.Register(t, "thomas")
	s.Login(t, "thomas")

	tok := s.CreateAccessToken(t, "tok", db.ReadPermission, db.ReadPermission)
	s.Logout()

	arr, _ := apiList[types.GistSimple](t, s, "/api/gists/liked", tok, 200)
	require.Empty(t, arr)
}

// TestListLikedGists_TokenWithGistRead_AllAllowed - token has gist:read, so
// the caller sees the visibility-allowed subset of every gist they liked:
// public from anyone, plus their own unlisted/private. Other users'
// unlisted/private (which they shouldn't really be able to see anyway) stay
// hidden.
func TestListLikedGists_TokenWithGistRead_AllAllowed(t *testing.T) {
	f := setupLiked(t)
	f.s.Login(t, "caller")
	tok := f.s.CreateAccessToken(t, "read", db.ReadPermission, db.ReadPermission)
	f.s.Logout()

	arr, _ := apiList[types.GistSimple](t, f.s, "/api/gists/liked?per_page=20", tok, 200)

	ids := idSet(arr)
	require.True(t, ids[f.callerPub.Uuid], "caller's own PUBLIC liked gist visible")
	require.True(t, ids[f.callerUnl.Uuid], "caller's own UNLISTED liked gist visible (they own it)")
	require.True(t, ids[f.callerPriv.Uuid], "caller's own PRIVATE liked gist visible (they own it)")
	require.True(t, ids[f.otherPub.Uuid], "other user's PUBLIC liked gist visible")
	require.False(t, ids[f.otherUnl.Uuid], "other user's UNLISTED liked gist NOT visible")
	require.False(t, ids[f.otherPriv.Uuid], "other user's PRIVATE liked gist NOT visible")
}

// =========================================================================
// GET /api/gists/:uuid/like - CheckLike
// =========================================================================

// likeGist directly inserts a like row for `username` on `g`, avoiding the
// HTTP /like endpoint (which is scope-gated and would complicate the
// fixture).
func likeGist(t *testing.T, g *db.Gist, username string) {
	u, err := db.GetUserByUsername(username)
	require.NoError(t, err)
	require.NoError(t, g.AppendUserLike(u))
}

// TestCheckLike_NoAuth - anonymous calls hit apiRequireAuth and 401 before
// the handler ever sees them.
func TestCheckLike_NoAuth(t *testing.T) {
	s := setupGetGist(t)
	_, gist, _, _ := s.CreateGistAs(t, "owner", "0")

	s.APIRequest(t, "GET", "/api/gists/"+gist.Uuid+"/like", "", nil, 401)
}

// TestCheckLike_StatusCodes is the headline matrix: combinations of (gist
// visibility, caller is owner?, gist liked by caller?) → expected code.
// Several cases collapse on the "hidden private gist" rule (always 404),
// which doubles as a check that like existence doesn't leak past the
// visibility filter.
func TestCheckLike_StatusCodes(t *testing.T) {
	s := setupGetGist(t)

	// gists owned by "owner" with different visibilities and pre-baked like
	// state for "other". The private-not-owned case uses a separate gist
	// without a like (since we expect 404 regardless).
	_, gPubLiked, _, _ := s.CreateGistAs(t, "owner", "0")
	likeGist(t, gPubLiked, "other")
	_, gPubUnliked, _, _ := s.CreateGistAs(t, "owner", "0")

	_, gUnlLiked, _, _ := s.CreateGistAs(t, "owner", "1")
	likeGist(t, gUnlLiked, "other")

	_, gPrivOwnLiked, _, _ := s.CreateGistAs(t, "owner", "2")
	likeGist(t, gPrivOwnLiked, "owner")
	_, gPrivOwnUnliked, _, _ := s.CreateGistAs(t, "owner", "2")
	_, gPrivHidden, _, _ := s.CreateGistAs(t, "owner", "2")

	ownerTok := apiTokenFor(t, s, "owner", db.ReadPermission)
	otherTok := apiTokenFor(t, s, "other", db.ReadPermission)

	cases := []struct {
		name string
		uuid string
		tok  string
		want int
	}{
		// Visible gist + caller has liked → 204.
		{"public/other/liked", gPubLiked.Uuid, otherTok, 204},
		{"unlisted/other/liked", gUnlLiked.Uuid, otherTok, 204},
		{"private/owner/liked", gPrivOwnLiked.Uuid, ownerTok, 204},

		// Visible gist + caller hasn't liked → 404.
		{"public/other/not liked", gPubUnliked.Uuid, otherTok, 404},
		{"private/owner/not liked", gPrivOwnUnliked.Uuid, ownerTok, 404},

		// Hidden private gist → 404 (visibility check kicks in before the
		// like check; existence stays hidden).
		{"private/other (hidden)", gPrivHidden.Uuid, otherTok, 404},

		// Unknown UUID → 404.
		{"missing", "does-not-exist", otherTok, 404},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s.APIRequest(t, "GET", "/api/gists/"+c.uuid+"/like", c.tok, nil, c.want)
		})
	}
}

// TestCheckLike_204HasEmptyBody - RFC 7230: 2xx with No Content body. We
// don't carry the gist in the response; just verify it's empty.
func TestCheckLike_204HasEmptyBody(t *testing.T) {
	s := setupGetGist(t)
	_, gist, _, _ := s.CreateGistAs(t, "owner", "0")
	likeGist(t, gist, "other")
	otherTok := apiTokenFor(t, s, "other", db.ReadPermission)

	_, body := s.APIRequest(t, "GET", "/api/gists/"+gist.Uuid+"/like", otherTok, nil, 204)
	require.Empty(t, body, "204 No Content responses must carry an empty body")
}

// =========================================================================
// PUT /api/gists/:uuid/like - ToggleLike
// =========================================================================

func TestToggleLike_NoAuth(t *testing.T) {
	s := setupGetGist(t)
	_, gist, _, _ := s.CreateGistAs(t, "owner", "0")

	s.APIRequest(t, "PUT", "/api/gists/"+gist.Uuid+"/like", "", nil, 401)
}

// TestToggleLike_LikesIfUnliked - first PUT on a never-liked gist adds the
// like. Verifies the response code, NbLikes increment, and HasLiked state.
func TestToggleLike_LikesIfUnliked(t *testing.T) {
	s := setupGetGist(t)
	_, gist, _, _ := s.CreateGistAs(t, "owner", "0")
	otherTok := likeTokenFor(t, s, "other")

	_, body := s.APIRequest(t, "PUT", "/api/gists/"+gist.Uuid+"/like", otherTok, nil, 204)
	require.Empty(t, body, "204 must have empty body")

	reloaded, err := db.GetGistByUUID(gist.Uuid)
	require.NoError(t, err)
	require.Equal(t, 1, reloaded.NbLikes, "NbLikes must increment to 1")

	other, err := db.GetUserByUsername("other")
	require.NoError(t, err)
	liked, err := other.HasLiked(reloaded)
	require.NoError(t, err)
	require.True(t, liked, "other must now have liked the gist")
}

// TestToggleLike_UnlikesIfLiked - PUT on an already-liked gist removes the
// like. Decrements NbLikes back down to 0.
func TestToggleLike_UnlikesIfLiked(t *testing.T) {
	s := setupGetGist(t)
	_, gist, _, _ := s.CreateGistAs(t, "owner", "0")
	likeGist(t, gist, "other")
	otherTok := likeTokenFor(t, s, "other")

	// AppendUserLike bumps NbLikes to 1; confirm precondition.
	reloaded, err := db.GetGistByUUID(gist.Uuid)
	require.NoError(t, err)
	require.Equal(t, 1, reloaded.NbLikes, "fixture precondition: NbLikes=1 after likeGist")

	s.APIRequest(t, "PUT", "/api/gists/"+gist.Uuid+"/like", otherTok, nil, 204)

	reloaded, err = db.GetGistByUUID(gist.Uuid)
	require.NoError(t, err)
	require.Equal(t, 0, reloaded.NbLikes, "NbLikes must decrement back to 0")

	other, err := db.GetUserByUsername("other")
	require.NoError(t, err)
	liked, err := other.HasLiked(reloaded)
	require.NoError(t, err)
	require.False(t, liked, "other must no longer have liked the gist")
}

// TestToggleLike_FullCycle - PUT twice on a fresh gist returns to the
// initial state (no like, NbLikes=0). Also verifies that toggling is
// genuinely idempotent across the pair.
func TestToggleLike_FullCycle(t *testing.T) {
	s := setupGetGist(t)
	_, gist, _, _ := s.CreateGistAs(t, "owner", "0")
	otherTok := likeTokenFor(t, s, "other")

	url := "/api/gists/" + gist.Uuid + "/like"
	s.APIRequest(t, "PUT", url, otherTok, nil, 204) // like
	s.APIRequest(t, "PUT", url, otherTok, nil, 204) // unlike

	reloaded, err := db.GetGistByUUID(gist.Uuid)
	require.NoError(t, err)
	require.Equal(t, 0, reloaded.NbLikes, "two PUTs must net to no change in NbLikes")

	// And CheckLike now agrees: no like, 404.
	s.APIRequest(t, "GET", url, otherTok, nil, 404)
}

// TestToggleLike_HiddenPrivateGist_404 - same visibility rule as CheckLike.
// A private gist not owned by the caller is invisible, so the toggle just
// 404s without leaking existence or mutating state.
func TestToggleLike_HiddenPrivateGist_404(t *testing.T) {
	s := setupGetGist(t)
	_, gist, _, _ := s.CreateGistAs(t, "owner", "2") // private
	otherTok := likeTokenFor(t, s, "other")

	s.APIRequest(t, "PUT", "/api/gists/"+gist.Uuid+"/like", otherTok, nil, 404)

	// State unchanged.
	reloaded, err := db.GetGistByUUID(gist.Uuid)
	require.NoError(t, err)
	require.Equal(t, 0, reloaded.NbLikes, "failed PUT must NOT mutate NbLikes")
}

func TestToggleLike_NotFound(t *testing.T) {
	s := setupGetGist(t)
	otherTok := likeTokenFor(t, s, "other")

	s.APIRequest(t, "PUT", "/api/gists/does-not-exist/like", otherTok, nil, 404)
}

// TestToggleLike_TokenWithoutUserWrite_403 - the toggle mutates the caller's
// own like state, so a token lacking the user:write scope is rejected with
// 403 before the handler runs, and no like state is mutated.
func TestToggleLike_TokenWithoutUserWrite_403(t *testing.T) {
	s := setupGetGist(t)
	_, gist, _, _ := s.CreateGistAs(t, "owner", "0")

	// gist:read but only user:read - enough to read, not to toggle a like.
	s.Login(t, "other")
	tok := s.CreateAccessToken(t, "no-user-write", db.ReadPermission, db.ReadPermission)
	s.Logout()

	s.APIRequest(t, "PUT", "/api/gists/"+gist.Uuid+"/like", tok, nil, 403)

	reloaded, err := db.GetGistByUUID(gist.Uuid)
	require.NoError(t, err)
	require.Equal(t, 0, reloaded.NbLikes, "rejected PUT must NOT mutate NbLikes")
}

// TestListLikedGists_TokenWithoutGistRead_OnlyPublic - token lacks
// gist:read, so the caller only sees public gists they liked. Their own
// unlisted/private liked gists are hidden even though they own them.
func TestListLikedGists_TokenWithoutGistRead_OnlyPublic(t *testing.T) {
	f := setupLiked(t)
	f.s.Login(t, "caller")
	tok := f.s.CreateAccessToken(t, "no-read", db.NoPermission, db.ReadPermission)
	f.s.Logout()

	arr, _ := apiList[types.GistSimple](t, f.s, "/api/gists/liked?per_page=20", tok, 200)

	ids := idSet(arr)
	require.True(t, ids[f.callerPub.Uuid], "caller's own PUBLIC liked gist visible")
	require.True(t, ids[f.otherPub.Uuid], "other user's PUBLIC liked gist visible")
	require.False(t, ids[f.callerUnl.Uuid], "own UNLISTED hidden without gist:read")
	require.False(t, ids[f.callerPriv.Uuid], "own PRIVATE hidden without gist:read")
	require.False(t, ids[f.otherUnl.Uuid], "other's UNLISTED hidden")
	require.False(t, ids[f.otherPriv.Uuid], "other's PRIVATE hidden")

	for _, g := range arr {
		require.Equal(t, "public", g.Visibility, "non-public gist leaked into no-scope liked response: %+v", g)
	}
}
