package v1_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/handlers/api/v1/types"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

// userGistsFixture stands up the env shared by the ListUserGists visibility
// tests: a "target" user with one gist of each visibility, plus an "other"
// user used as a separate viewer.
type userGistsFixture struct {
	s                                *webtest.Server
	targetPub, targetUnl, targetPriv *db.Gist
}

func setupListUserGists(t *testing.T) *userGistsFixture {
	s := webtest.Setup(t)
	t.Cleanup(func() { webtest.Teardown(t) })
	s.Register(t, "admin")
	s.Logout()
	s.Register(t, "target")
	s.Register(t, "other")

	_, targetPub, _, _ := s.CreateGistAs(t, "target", "0")
	_, targetUnl, _, _ := s.CreateGistAs(t, "target", "1")
	_, targetPriv, _, _ := s.CreateGistAs(t, "target", "2")

	return &userGistsFixture{
		s:          s,
		targetPub:  targetPub,
		targetUnl:  targetUnl,
		targetPriv: targetPriv,
	}
}

// TestListUserGists_UnknownUser_404 - the lookup-side rule: a username that
// doesn't exist returns 404 (not an empty array). Kept separate because it
// doesn't need the per-visibility fixture.
func TestListUserGists_UnknownUser_404(t *testing.T) {
	s := webtest.Setup(t)
	t.Cleanup(func() { webtest.Teardown(t) })
	s.Register(t, "thomas")

	s.APIRequest(t, "GET", "/api/users/nobody/gists", "", nil, 404)
}

// userLikedFixture sets up a "target" user who has liked one public + one
// unlisted + one private gist (all owned by target so the visibility filter
// is exercisable). Plus an "other" user used as a separate viewer.
type userLikedFixture struct {
	s                                *webtest.Server
	targetPub, targetUnl, targetPriv *db.Gist
}

func setupUserLiked(t *testing.T) *userLikedFixture {
	s := webtest.Setup(t)
	t.Cleanup(func() { webtest.Teardown(t) })
	s.Register(t, "admin")
	s.Logout()
	s.Register(t, "target")
	s.Register(t, "other")

	_, pub, _, _ := s.CreateGistAs(t, "target", "0")
	_, unl, _, _ := s.CreateGistAs(t, "target", "1")
	_, priv, _, _ := s.CreateGistAs(t, "target", "2")

	// target likes all three of their own gists. The visibility filter
	// (`private = 0 OR user_id = currentUserId`) then decides per-caller
	// whether the unlisted/private rows surface.
	for _, g := range []*db.Gist{pub, unl, priv} {
		likeGist(t, g, "target")
	}
	return &userLikedFixture{s: s, targetPub: pub, targetUnl: unl, targetPriv: priv}
}

// TestListUserLikedGists_Visibility - same visibility matrix as
// ListUserGists, applied to the liked-by-:username view.
func TestListUserLikedGists_Visibility(t *testing.T) {
	f := setupUserLiked(t)

	otherTok := apiTokenFor(t, f.s, "other", db.ReadPermission)
	selfScopedTok := apiTokenFor(t, f.s, "target", db.ReadPermission)
	selfNoScopeTok := apiTokenFor(t, f.s, "target", db.NoPermission)

	cases := []struct {
		name    string
		tok     string
		seePub  bool
		seeUnl  bool
		seePriv bool
	}{
		{"anonymous", "", true, false, false},
		{"other user (scoped)", otherTok, true, false, false},
		{"self (gist:read)", selfScopedTok, true, true, true},
		{"self (no scope)", selfNoScopeTok, true, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			arr, _ := apiList[types.GistSimple](t, f.s, "/api/users/target/liked?per_page=20", c.tok, 200)

			ids := idSetSimple(arr)
			require.Equal(t, c.seePub, ids[f.targetPub.Uuid], "PUBLIC visibility expectation")
			require.Equal(t, c.seeUnl, ids[f.targetUnl.Uuid], "UNLISTED visibility expectation")
			require.Equal(t, c.seePriv, ids[f.targetPriv.Uuid], "PRIVATE visibility expectation")
		})
	}
}

func TestListUserLikedGists_UnknownUser_404(t *testing.T) {
	s := webtest.Setup(t)
	t.Cleanup(func() { webtest.Teardown(t) })
	s.Register(t, "thomas")

	s.APIRequest(t, "GET", "/api/users/nobody/liked", "", nil, 404)
}

// userForkedFixture sets up parentowner → public parent. target forks the
// parent (so a fork owned by target exists). Then target's fork is left
// public, but we also create a second fork by target on a different parent
// (also public) and flip its visibility to unlisted / private to exercise
// the per-row visibility filter.
type userForkedFixture struct {
	s                                *webtest.Server
	targetPub, targetUnl, targetPriv *db.Gist
}

func setupUserForked(t *testing.T) *userForkedFixture {
	s := webtest.Setup(t)
	t.Cleanup(func() { webtest.Teardown(t) })
	s.Register(t, "admin")
	s.Logout()
	s.Register(t, "parentowner")
	s.Register(t, "target")
	s.Register(t, "other")

	// Three independent parents so target can create three distinct forks
	// (forking the same parent twice would be idempotent).
	_, _, p1user, p1ident := s.CreateGistAs(t, "parentowner", "0")
	_, _, p2user, p2ident := s.CreateGistAs(t, "parentowner", "0")
	_, _, p3user, p3ident := s.CreateGistAs(t, "parentowner", "0")

	pubFork := forkAs(t, s, "target", p1user, p1ident)
	unlFork := forkAs(t, s, "target", p2user, p2ident)
	privFork := forkAs(t, s, "target", p3user, p3ident)

	setVisibility(t, unlFork, db.UnlistedVisibility)
	setVisibility(t, privFork, db.PrivateVisibility)

	return &userForkedFixture{s: s, targetPub: pubFork, targetUnl: unlFork, targetPriv: privFork}
}

// TestListUserForkedGists_Visibility - same matrix as the liked one.
func TestListUserForkedGists_Visibility(t *testing.T) {
	f := setupUserForked(t)

	otherTok := apiTokenFor(t, f.s, "other", db.ReadPermission)
	selfScopedTok := apiTokenFor(t, f.s, "target", db.ReadPermission)
	selfNoScopeTok := apiTokenFor(t, f.s, "target", db.NoPermission)

	cases := []struct {
		name    string
		tok     string
		seePub  bool
		seeUnl  bool
		seePriv bool
	}{
		{"anonymous", "", true, false, false},
		{"other user (scoped)", otherTok, true, false, false},
		{"self (gist:read)", selfScopedTok, true, true, true},
		{"self (no scope)", selfNoScopeTok, true, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			arr, _ := apiList[types.GistSimple](t, f.s, "/api/users/target/forked?per_page=20", c.tok, 200)

			ids := idSetSimple(arr)
			require.Equal(t, c.seePub, ids[f.targetPub.Uuid], "PUBLIC fork visibility expectation")
			require.Equal(t, c.seeUnl, ids[f.targetUnl.Uuid], "UNLISTED fork visibility expectation")
			require.Equal(t, c.seePriv, ids[f.targetPriv.Uuid], "PRIVATE fork visibility expectation")
		})
	}
}

func TestListUserForkedGists_UnknownUser_404(t *testing.T) {
	s := webtest.Setup(t)
	t.Cleanup(func() { webtest.Teardown(t) })
	s.Register(t, "thomas")

	s.APIRequest(t, "GET", "/api/users/nobody/forked", "", nil, 404)
}

// TestListUserGists_Visibility - table-driven matrix of caller types ×
// expected visibility of target's three gists.
//
//   - Anonymous and other users see only public.
//   - The target user with gist:read sees everything (public + own
//     unlisted + own private).
//   - The target user without gist:read soft-degrades to public-only,
//     same as anonymous.
func TestListUserGists_Visibility(t *testing.T) {
	f := setupListUserGists(t)

	otherTok := apiTokenFor(t, f.s, "other", db.ReadPermission)
	selfScopedTok := apiTokenFor(t, f.s, "target", db.ReadPermission)
	selfNoScopeTok := apiTokenFor(t, f.s, "target", db.NoPermission)

	cases := []struct {
		name    string
		tok     string
		seePub  bool
		seeUnl  bool
		seePriv bool
	}{
		{"anonymous", "", true, false, false},
		{"other user (scoped)", otherTok, true, false, false},
		{"self (gist:read)", selfScopedTok, true, true, true},
		{"self (no scope)", selfNoScopeTok, true, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			arr, _ := apiList[types.GistSimple](t, f.s, "/api/users/target/gists?per_page=20", c.tok, 200)

			ids := idSetSimple(arr)
			require.Equal(t, c.seePub, ids[f.targetPub.Uuid], "PUBLIC visibility expectation")
			require.Equal(t, c.seeUnl, ids[f.targetUnl.Uuid], "UNLISTED visibility expectation")
			require.Equal(t, c.seePriv, ids[f.targetPriv.Uuid], "PRIVATE visibility expectation")
		})
	}
}
