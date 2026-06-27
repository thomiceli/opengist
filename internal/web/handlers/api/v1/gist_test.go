package v1_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/handlers/api/v1/types"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

// fullGist is a typed shim that decodes the GET /gists/:uuid response. The
// real types.Gist has ForkOf as interface{}; here we pin it to a pointer so
// assertions stay sane.
type fullGist struct {
	types.GistSimple
	ForkOf    *types.GistSimple         `json:"fork_of"`
	Forks     []types.GistSimple        `json:"forks"`
	Files     map[string]types.GistFile `json:"files"`
	Truncated bool                      `json:"truncated"`
}

// setupGetGist registers an admin stub + owner + other, enables the API, and
// logs out. Caller is responsible for session/token setup.
func setupGetGist(t *testing.T) *webtest.Server {
	s := webtest.Setup(t)
	t.Cleanup(func() { webtest.Teardown(t) })
	s.Register(t, "admin")
	s.Logout()
	s.Register(t, "owner")
	s.Register(t, "other")

	s.Logout()
	return s
}

// apiTokenFor logs in as `user`, mints a token with the given gist scope (user
// scope fixed to read) and logs out. Use NoPermission for "no-scope" tokens.
func apiTokenFor(t *testing.T, s *webtest.Server, user string, gistScope uint) string {
	s.Login(t, user)
	tok := s.CreateAccessToken(t, user+"-tok", gistScope, db.ReadPermission)
	s.Logout()
	return tok
}

// likeTokenFor returns a token for `user` carrying the user:write scope that
// the PUT /like|/star toggle requires, plus gist:read so private-gist
// visibility checks still pass.
func likeTokenFor(t *testing.T, s *webtest.Server, user string) string {
	s.Login(t, user)
	tok := s.CreateAccessToken(t, user+"-like-tok", db.ReadPermission, db.ReadWritePermission)
	s.Logout()
	return tok
}

// createGistViaAPI posts a body to /api/gists and returns the resulting
// gist's id. Useful for multi-file / large-content fixtures that can't be
// built through the web form helper.
func createGistViaAPI(t *testing.T, s *webtest.Server, tok string, body interface{}) string {
	_, raw := s.APIRequest(t, "POST", "/api/gists", tok, body, 201)
	var resp struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(raw, &resp))
	require.NotEmpty(t, resp.ID)
	return resp.ID
}

// --- Visibility / access matrix ---

func TestGetGist_VisibilityAccess(t *testing.T) {
	s := setupGetGist(t)

	_, gistPub, _, _ := s.CreateGistAs(t, "owner", "0")
	_, gistUnl, _, _ := s.CreateGistAs(t, "owner", "1")
	_, gistPriv, _, _ := s.CreateGistAs(t, "owner", "2")

	ownerTok := apiTokenFor(t, s, "owner", db.ReadPermission)
	ownerNoScope := apiTokenFor(t, s, "owner", db.NoPermission)
	otherTok := apiTokenFor(t, s, "other", db.ReadPermission)

	type tc struct {
		name string
		uuid string
		tok  string
		want int
	}
	cases := []tc{
		// Public gist - readable by everyone.
		{"public/anonymous", gistPub.Uuid, "", 200},
		{"public/no-scope owner", gistPub.Uuid, ownerNoScope, 200},
		{"public/scoped owner", gistPub.Uuid, ownerTok, 200},
		{"public/scoped other", gistPub.Uuid, otherTok, 200},

		// Unlisted gist - URL-shareable: readable by anyone with the UUID.
		{"unlisted/anonymous", gistUnl.Uuid, "", 200},
		{"unlisted/no-scope owner", gistUnl.Uuid, ownerNoScope, 200},
		{"unlisted/scoped owner", gistUnl.Uuid, ownerTok, 200},
		{"unlisted/scoped other", gistUnl.Uuid, otherTok, 200},

		// Private gist - owner with gist:read only.
		{"private/anonymous", gistPriv.Uuid, "", 404},
		{"private/no-scope owner", gistPriv.Uuid, ownerNoScope, 404},
		{"private/scoped other (non-owner)", gistPriv.Uuid, otherTok, 404},
		{"private/scoped owner", gistPriv.Uuid, ownerTok, 200},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s.APIRequest(t, "GET", "/api/gists/"+c.uuid, c.tok, nil, c.want)
		})
	}
}

func TestGetGist_NotFound(t *testing.T) {
	s := setupGetGist(t)
	s.APIRequest(t, "GET", "/api/gists/does-not-exist", "", nil, 404)
}

// --- Response structure ---

func TestGetGist_ResponseShape(t *testing.T) {
	s := setupGetGist(t)

	// Enable both git transports so the URL-bearing fields aren't empty.
	prevHTTP, prevSSH := config.C.HttpGit, config.C.SshGit
	prevDomain, prevPort := config.C.SshExternalDomain, config.C.SshPort
	t.Cleanup(func() {
		config.C.HttpGit, config.C.SshGit = prevHTTP, prevSSH
		config.C.SshExternalDomain, config.C.SshPort = prevDomain, prevPort
	})
	config.C.HttpGit = true
	config.C.SshGit = config.SshServerBuiltin
	config.C.SshExternalDomain = "gist.example.com"
	config.C.SshPort = "22"

	_, gist, _, _ := s.CreateGistAs(t, "owner", "0")

	_, raw := s.APIRequest(t, "GET", "/api/gists/"+gist.Uuid, "", nil, 200)
	var got fullGist
	require.NoError(t, json.Unmarshal(raw, &got))

	// Identity + visibility.
	require.Equal(t, gist.Uuid, got.ID)
	require.Equal(t, "Test", got.Title) // CreateGistAs hardcodes title="Test"
	require.Equal(t, "", got.Description)
	require.Equal(t, "public", got.Visibility)
	require.True(t, got.Public)

	// Counters default to zero for a fresh gist.
	require.Equal(t, 0, got.LikeCount)
	require.Equal(t, 0, got.ForkCount)

	// Owner block.
	require.Equal(t, "owner", got.Owner.Login)
	require.Equal(t, "User", got.Owner.Type)
	require.NotZero(t, got.Owner.ID)

	// URLs (derived from the request's host).
	require.Contains(t, got.HTMLUrl, "/owner/"+gist.Identifier())
	require.Contains(t, got.CloneUrl, "/owner/"+gist.Identifier()+".git")
	require.Equal(t, "gist.example.com:owner/"+gist.Identifier()+".git", got.SSHUrl,
		"scp-style SSH URL when SshPort=22")

	// Timestamps populated.
	require.False(t, got.CreatedAt.IsZero())
	require.False(t, got.UpdatedAt.IsZero())

	// Topics default to empty.
	require.Empty(t, got.Topics)

	// Files map populated with file.txt (CreateGistAs hardcodes that).
	require.NotEmpty(t, got.Files)
	require.Contains(t, got.Files, "file.txt")
	f := got.Files["file.txt"]
	require.Equal(t, "file.txt", f.Filename)
	require.Equal(t, "hello", f.Content)
	require.False(t, f.Truncated)
	require.NotEmpty(t, f.Encoding)

	// Gist-level fork/truncate defaults.
	require.Nil(t, got.ForkOf, "non-forked gist must have fork_of=null")
	require.Empty(t, got.Forks)
	require.False(t, got.Truncated)
}

// --- ForkOf ---

func TestGetGist_ForkOfPopulated(t *testing.T) {
	s := setupGetGist(t)

	_, parent, parentUser, parentIdent := s.CreateGistAs(t, "owner", "0")

	// "other" forks the gist via the web /fork endpoint. The redirect Location
	// gives us /{forker}/{newIdentifier}.
	s.Login(t, "other")
	resp := s.Request(t, "POST", "/"+parentUser+"/"+parentIdent+"/fork", nil, 302)
	loc := resp.Header.Get("Location")
	parts := strings.Split(strings.TrimPrefix(loc, "/"), "/")
	require.Len(t, parts, 2, "fork redirect must be /{user}/{ident}, got %q", loc)
	forkIdent := parts[1]
	s.Logout()

	// Look the fork up to grab its UUID (Identifier == UUID when no custom URL).
	forkGist, err := db.GetGist("other", forkIdent)
	require.NoError(t, err)

	// GET the fork - fork_of must point at the parent.
	_, raw := s.APIRequest(t, "GET", "/api/gists/"+forkGist.Uuid, "", nil, 200)
	var got fullGist
	require.NoError(t, json.Unmarshal(raw, &got))

	require.NotNil(t, got.ForkOf, "fork_of must be populated for a forked gist")
	require.Equal(t, parent.Uuid, got.ForkOf.ID, "fork_of.id must point at the parent gist")
	require.Equal(t, "owner", got.ForkOf.Owner.Login, "fork_of.owner must be the parent's owner")

	// Sanity: getting the parent reflects the bumped fork count and nil fork_of.
	_, parentRaw := s.APIRequest(t, "GET", "/api/gists/"+parent.Uuid, "", nil, 200)
	var parentGot fullGist
	require.NoError(t, json.Unmarshal(parentRaw, &parentGot))
	require.Nil(t, parentGot.ForkOf, "parent gist itself is not a fork")
	require.Equal(t, 1, parentGot.ForkCount, "parent's fork_count must reflect the new fork")
}

// --- Truncation: too many files ---

func TestGetGist_GistTruncatedWhenManyFiles(t *testing.T) {
	s := setupGetGist(t)
	tok := apiTokenFor(t, s, "owner", db.ReadWritePermission)

	// CatFileBatch caps the file listing at 50 entries - push 51 to flip the
	// gist-level Truncated flag.
	files := map[string]map[string]string{}
	for i := 0; i < 51; i++ {
		files[fmt.Sprintf("file%02d.txt", i)] = map[string]string{"content": fmt.Sprintf("body %d", i)}
	}
	id := createGistViaAPI(t, s, tok, map[string]interface{}{
		"title":      "many files",
		"visibility": "public",
		"files":      files,
	})

	_, raw := s.APIRequest(t, "GET", "/api/gists/"+id, "", nil, 200)
	var got fullGist
	require.NoError(t, json.Unmarshal(raw, &got))

	require.True(t, got.Truncated, "Truncated must be true when the gist has more than 50 files")
	require.LessOrEqual(t, len(got.Files), 50, "Files map must be capped at 50 entries when truncated")
}

// --- Truncation: file content too large ---

func TestGetGist_FileContentTruncatedWhenLarge(t *testing.T) {
	s := setupGetGist(t)
	tok := apiTokenFor(t, s, "owner", db.ReadWritePermission)

	// truncateLimit in internal/git is 2<<18 (512 KiB). A 600 KiB file forces
	// per-file truncation while keeping the gist-level flag false (1 file).
	const truncateLimit = 2 << 18
	bigContent := strings.Repeat("a", truncateLimit+1024)

	id := createGistViaAPI(t, s, tok, map[string]interface{}{
		"title":      "big file",
		"visibility": "public",
		"files": map[string]map[string]string{
			"big.txt": {"content": bigContent},
		},
	})

	_, raw := s.APIRequest(t, "GET", "/api/gists/"+id, "", nil, 200)
	var got fullGist
	require.NoError(t, json.Unmarshal(raw, &got))

	require.False(t, got.Truncated, "gist-level Truncated stays false for a single file")
	require.Contains(t, got.Files, "big.txt")
	f := got.Files["big.txt"]
	require.True(t, f.Truncated, "per-file Truncated must be true for >512 KiB content")
	require.LessOrEqual(t, len(f.Content), truncateLimit,
		"truncated content length must be at most truncateLimit (got %d)", len(f.Content))
}
