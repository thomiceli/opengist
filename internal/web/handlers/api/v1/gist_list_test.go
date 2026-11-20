package v1_test

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/handlers/api/v1/types"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

// listGistsVisibilityFixture stands up the env shared by the ListGists
// visibility tests: a "caller" user with one gist of each visibility, plus an
// "other" user with one gist of each visibility. The API is enabled. The
// caller is left logged in so the test can mint a token if it needs one.
type listGistsVisibilityFixture struct {
	s                                *webtest.Server
	callerPub, callerUnl, callerPriv *db.Gist
	otherPub, otherUnl, otherPriv    *db.Gist
}

func setupListVisibility(t *testing.T) *listGistsVisibilityFixture {
	s := webtest.Setup(t)
	t.Cleanup(func() { webtest.Teardown(t) })

	// First-registered user is auto-admin in some flows - register an admin
	// stub so the actors below are plain users.
	s.Register(t, "admin")
	s.Logout()
	s.Register(t, "other")
	s.Register(t, "caller")

	// Other user: one of each visibility.
	_, otherPub, _, _ := s.CreateGistAs(t, "other", "0")
	_, otherUnl, _, _ := s.CreateGistAs(t, "other", "1")
	_, otherPriv, _, _ := s.CreateGistAs(t, "other", "2")

	// Caller: one of each visibility. Left logged in afterwards.
	_, callerPub, _, _ := s.CreateGistAs(t, "caller", "0")
	_, callerUnl, _, _ := s.CreateGistAs(t, "caller", "1")
	_, callerPriv, _, _ := s.CreateGistAs(t, "caller", "2")

	return &listGistsVisibilityFixture{
		s:          s,
		callerPub:  callerPub,
		callerUnl:  callerUnl,
		callerPriv: callerPriv,
		otherPub:   otherPub,
		otherUnl:   otherUnl,
		otherPriv:  otherPriv,
	}
}

// idSet returns the set of gist IDs seen in the response so tests can check
// membership without caring about order.
func idSet(arr []types.GistSimple) map[string]bool {
	out := make(map[string]bool, len(arr))
	for _, g := range arr {
		out[g.ID] = true
	}
	return out
}

// TestListGists_GistObjectShape verifies every field of a types.GistSimple coming
// back from /api/gists is populated as expected. HTTP git and the SSH server are
// enabled so the URL-bearing fields aren't empty.
func TestListGists_GistObjectShape(t *testing.T) {
	s := webtest.Setup(t)
	t.Cleanup(func() { webtest.Teardown(t) })
	s.Register(t, "admin")
	s.Logout()
	s.Register(t, "thomas")

	config.C.HttpGit = true
	config.C.SshGit = config.SshServerBuiltin
	config.C.SshExternalDomain = "gist.example.com"
	config.C.SshPort = "22"

	_, gist, _, _ := s.CreateGistAs(t, "thomas", "0")

	tok := s.CreateAccessToken(t, "shape", db.ReadPermission, db.ReadPermission)
	s.Logout()

	arr, _ := apiList[types.GistSimple](t, s, "/api/gists", tok, 200)
	require.Len(t, arr, 1)
	got := arr[0]

	// Identity + visibility.
	require.Equal(t, gist.Uuid, got.ID)
	require.Equal(t, "Test", got.Title) // CreateGistAs hardcodes title="Test"
	require.Equal(t, "", got.Description)
	require.Equal(t, "public", got.Visibility)
	require.True(t, got.Public)

	// Fork / counts default to zero on a fresh gist.
	require.Equal(t, 0, got.LikeCount)
	require.Equal(t, 0, got.ForkCount)

	// URLs (caller-derived from the request's host).
	require.Contains(t, got.HTMLUrl, "/thomas/"+gist.Identifier())
	require.Contains(t, got.CloneUrl, "/thomas/"+gist.Identifier()+".git")
	require.Equal(t, "gist.example.com:thomas/"+gist.Identifier()+".git", got.SSHUrl,
		"scp-style SSH URL when SshPort=22")

	// Timestamps populated.
	require.False(t, got.CreatedAt.IsZero())
	require.False(t, got.UpdatedAt.IsZero())

	// Topics default to no topics.
	require.Empty(t, got.Topics)
}

// TestListGists_Anonymous_OnlyPublicFromEveryone - caller passes no
// Authorization header. Should get every public gist regardless of owner, and
// no unlisted or private gist from anyone.
func TestListGists_Anonymous_OnlyPublicFromEveryone(t *testing.T) {
	f := setupListVisibility(t)
	f.s.Logout()

	arr, _ := apiList[types.GistSimple](t, f.s, "/api/gists?per_page=20", "", 200)

	require.Len(t, arr, 2, "expected 2 gists, got %d", len(arr))
	ids := idSet(arr)
	require.True(t, ids[f.callerPub.Uuid], "caller's PUBLIC gist must appear")
	require.True(t, ids[f.otherPub.Uuid], "other user's PUBLIC gist must appear")
	require.False(t, ids[f.callerUnl.Uuid], "caller's UNLISTED gist must NOT appear (anonymous)")
	require.False(t, ids[f.callerPriv.Uuid], "caller's PRIVATE gist must NOT appear (anonymous)")
	require.False(t, ids[f.otherUnl.Uuid], "other user's UNLISTED gist must NOT appear (anonymous)")
	require.False(t, ids[f.otherPriv.Uuid], "other user's PRIVATE gist must NOT appear (anonymous)")

	// Every returned gist must be public.
	for _, g := range arr {
		require.Equal(t, "public", g.Visibility, "non-public gist leaked into anonymous response: %+v", g)
	}
}

// TestListGists_ExcludesExpired - expired gists not yet deleted by the
// background job are left out of the list endpoints and their totals,
// including their owner's own list.
func TestListGists_ExcludesExpired(t *testing.T) {
	f := setupListVisibility(t)
	callerTok := f.s.CreateAccessToken(t, "list", db.ReadPermission, db.ReadPermission)
	f.s.Logout()

	expireGist(t, f.callerPub)
	expireGist(t, f.callerPriv)

	for _, c := range []struct {
		uri  string
		want []*db.Gist
	}{
		{"/api/gists/public", []*db.Gist{f.otherPub}},
		{"/api/gists", []*db.Gist{f.otherPub}},
		{"/api/users/caller/gists", nil},
	} {
		t.Run(c.uri, func(t *testing.T) {
			w, body := f.s.APIRequest(t, "GET", c.uri, "", nil, 200)
			var arr []types.GistSimple
			require.NoError(t, json.Unmarshal(body, &arr))
			ids := idSet(arr)
			require.Len(t, ids, len(c.want))
			for _, g := range c.want {
				require.True(t, ids[g.Uuid])
			}
			require.Equal(t, strconv.Itoa(len(c.want)), w.Header().Get("X-Total"))
		})
	}

	// The owner doesn't see them in their own list either.
	arr, _ := apiList[types.GistSimple](t, f.s, "/api/gists", callerTok, 200)
	ids := idSet(arr)
	require.Len(t, ids, 1)
	require.True(t, ids[f.callerUnl.Uuid])
}

// TestListGists_TokenWithoutGistRead_OnlyCallerPublic - caller authenticates
// with a token whose gist scope is NoPermission. Should get only their own
// public gists; their unlisted/private and everyone else's content stay
// hidden.
func TestListGists_TokenWithoutGistRead_OnlyCallerPublic(t *testing.T) {
	f := setupListVisibility(t)
	// Setup leaves caller logged in.
	tok := f.s.CreateAccessToken(t, "no-read", db.NoPermission, db.ReadPermission)
	f.s.Logout()

	arr, _ := apiList[types.GistSimple](t, f.s, "/api/gists?per_page=20", tok, 200)

	require.Len(t, arr, 1, "expected 1 gist, got %d", len(arr))
	ids := idSet(arr)
	require.True(t, ids[f.callerPub.Uuid], "caller's own PUBLIC gist must appear")
	require.False(t, ids[f.callerUnl.Uuid], "caller's UNLISTED gist hidden when token lacks gist:read")
	require.False(t, ids[f.callerPriv.Uuid], "caller's PRIVATE gist hidden when token lacks gist:read")
	require.False(t, ids[f.otherPub.Uuid], "other user's PUBLIC gist must NOT leak (use /gists/public)")
	require.False(t, ids[f.otherUnl.Uuid], "other user's UNLISTED gist must NOT appear")
	require.False(t, ids[f.otherPriv.Uuid], "other user's PRIVATE gist must NOT appear")

	for _, g := range arr {
		require.Equal(t, "public", g.Visibility, "non-public gist leaked into no-scope response: %+v", g)
		require.Equal(t, "caller", g.Owner.Login, "other user's gist leaked: %+v", g)
	}
}

// TestListGists_TokenWithGistRead_AllOwnRegardlessOfVisibility - caller has
// a token with gist:read. Should get every one of their own gists (public,
// unlisted, private) and nothing from other users.
func TestListGists_TokenWithGistRead_AllOwnRegardlessOfVisibility(t *testing.T) {
	f := setupListVisibility(t)
	tok := f.s.CreateAccessToken(t, "read", db.ReadPermission, db.ReadPermission)
	f.s.Logout()

	arr, _ := apiList[types.GistSimple](t, f.s, "/api/gists?per_page=20", tok, 200)

	require.Len(t, arr, 3, "expected 3 gists, got %d", len(arr))
	ids := idSet(arr)
	require.True(t, ids[f.callerPub.Uuid], "caller's PUBLIC gist must appear")
	require.True(t, ids[f.callerUnl.Uuid], "caller's UNLISTED gist must appear (token has gist:read)")
	require.True(t, ids[f.callerPriv.Uuid], "caller's PRIVATE gist must appear (token has gist:read)")
	require.False(t, ids[f.otherPub.Uuid], "other user's PUBLIC gist must NOT appear (use /gists/public)")
	require.False(t, ids[f.otherUnl.Uuid], "other user's UNLISTED gist must NOT appear")
	require.False(t, ids[f.otherPriv.Uuid], "other user's PRIVATE gist must NOT appear")

	for _, g := range arr {
		require.Equal(t, "caller", g.Owner.Login, "other user's gist leaked: %+v", g)
	}
}
