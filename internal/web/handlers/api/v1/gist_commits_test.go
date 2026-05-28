package v1_test

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/handlers/api/v1/types"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

// makeCommits creates a public gist and patches its file `count-1` times so
// the resulting gist has `count` commits total. Returns the gist id.
func makeCommits(t *testing.T, s *webtest.Server, tok string, count int) string {
	id := createGistViaAPI(t, s, tok, map[string]interface{}{
		"visibility": "public",
		"files":      fileMap{"a.txt": {"content": "v0"}},
	})
	for i := 1; i < count; i++ {
		body := fmt.Sprintf(`{"files": {"a.txt": {"content": "v%d"}}}`, i)
		s.APIRequest(t, "PATCH", "/api/v1/gists/"+id, tok, body, 200)
	}
	return id
}

// --- Visibility / access matrix ---

// TestListCommits_VisibilityAccess mirrors TestGetGist_VisibilityAccess for
// the commits endpoint: same lookup rules → same status codes.
func TestListCommits_VisibilityAccess(t *testing.T) {
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
			s.APIRequest(t, "GET", "/api/v1/gists/"+c.uuid+"/commits", c.tok, nil, c.want)
		})
	}
}

func TestListCommits_NotFound(t *testing.T) {
	s := setupGetGist(t)
	s.APIRequest(t, "GET", "/api/v1/gists/does-not-exist/commits", "", nil, 404)
}

// --- Response shape ---

// TestListCommits_Shape verifies what each commit object carries: identity
// (version), raw git author info, change_status from the shortstat, and a
// non-zero committed_at. A freshly created gist has one commit and no prior
// state, so additions > 0 and deletions == 0.
func TestListCommits_Shape(t *testing.T) {
	s := setupGetGist(t)
	_, gist, _, _ := s.CreateGistAs(t, "owner", "0")

	commits, _ := apiList[types.GistCommit](t, s, "/api/v1/gists/"+gist.Uuid+"/commits", "", 200)

	require.Len(t, commits, 1, "fresh gist must have exactly one commit")
	c := commits[0]

	require.NotEmpty(t, c.Version, "version (commit SHA) must be set")
	require.Len(t, c.Version, 40, "SHA-1 is 40 hex chars")
	require.False(t, c.CommittedAt.IsZero(), "committed_at must be populated")

	// Author is always populated from the raw commit metadata.
	require.NotEmpty(t, c.Author.Name, "author.name must come from the commit's git metadata")

	// change_status: one added file, no deletions, total == additions.
	require.Equal(t, 1, c.ChangeStatus.Files, "one file changed (the new file)")
	require.Greater(t, c.ChangeStatus.Additions, 0, "additions must be > 0 for an initial commit")
	require.Equal(t, 0, c.ChangeStatus.Deletions, "deletions must be 0 on initial commit")
	require.Equal(t, c.ChangeStatus.Additions+c.ChangeStatus.Deletions, c.ChangeStatus.Total,
		"total must equal additions + deletions")
}

// TestListCommits_UserResolutionByEmail verifies that when the gist author's
// email matches a registered Opengist user, the commit's `user` field is
// populated with that account (not just the raw author info).
func TestListCommits_UserResolutionByEmail(t *testing.T) {
	s := webtest.Setup(t)
	webtest.Teardown(t)
	s.Register(t, "admin")
	s.Logout()
	s.Register(t, "owner")

	// Set a real email on the owner *before* creating the gist so the
	// resulting git commit's author email matches.
	owner, err := db.GetUserByUsername("owner")
	require.NoError(t, err)
	owner.Email = "owner@example.com"
	require.NoError(t, owner.Update())

	_, gist, _, _ := s.CreateGistAs(t, "owner", "0")

	commits, _ := apiList[types.GistCommit](t, s, "/api/v1/gists/"+gist.Uuid+"/commits", "", 200)
	require.Len(t, commits, 1)

	c := commits[0]
	require.NotNil(t, c.User, "commit author email matches an account → user must be resolved")
	require.Equal(t, "owner", c.User.Login)
	// Author block still carries the raw git-side info (the canonical commit metadata).
	require.Equal(t, "owner@example.com", c.Author.Email)
}

// --- per_page ---

func TestListCommits_PerPage_LimitsResults(t *testing.T) {
	s, tok := setupCreateGist(t)
	id := makeCommits(t, s, tok, 5)

	commits, _ := apiList[types.GistCommit](t, s, "/api/v1/gists/"+id+"/commits?per_page=2", tok, 200)
	require.Len(t, commits, 2)
}

// TestListCommits_OmitsTotalHeaders - commits deliberately skip X-Total /
// X-Total-Pages (computing them would need an extra `git rev-list` call). The
// X-Page / X-Per-Page and Link headers still apply.
func TestListCommits_OmitsTotalHeaders(t *testing.T) {
	s, tok := setupCreateGist(t)
	id := makeCommits(t, s, tok, 3)

	w, _ := s.APIRequest(t, "GET", "/api/v1/gists/"+id+"/commits?per_page=2", tok, nil, 200)
	require.Empty(t, w.Header().Get("X-Total"), "commits must not emit X-Total")
	require.Empty(t, w.Header().Get("X-Total-Pages"), "commits must not emit X-Total-Pages")
	require.Equal(t, "1", w.Header().Get("X-Page"))
	require.Equal(t, "2", w.Header().Get("X-Per-Page"))
}

// --- page + Link header ---

func TestListCommits_Page_FirstPage_OnlyNext(t *testing.T) {
	s, tok := setupCreateGist(t)
	id := makeCommits(t, s, tok, 5)

	commits, rels := apiList[types.GistCommit](t, s, "/api/v1/gists/"+id+"/commits?per_page=2&page=1", tok, 200)
	require.Len(t, commits, 2)

	require.Contains(t, rels, "next", "first page must advertise rel=next when more rows exist")
	require.NotContains(t, rels, "prev", "first page must NOT advertise rel=prev")

	// next URL bumps page to 2 and preserves per_page.
	u, err := url.Parse(rels["next"])
	require.NoError(t, err)
	require.Equal(t, "2", u.Query().Get("page"))
	require.Equal(t, "2", u.Query().Get("per_page"))
}

func TestListCommits_Page_MiddlePage_PrevAndNext(t *testing.T) {
	s, tok := setupCreateGist(t)
	id := makeCommits(t, s, tok, 5)

	commits, rels := apiList[types.GistCommit](t, s, "/api/v1/gists/"+id+"/commits?per_page=2&page=2", tok, 200)
	require.Len(t, commits, 2)

	require.Contains(t, rels, "next", "middle page must advertise rel=next")
	require.Contains(t, rels, "prev", "middle page must advertise rel=prev")
}

func TestListCommits_Page_AcrossPagesNoDuplicates(t *testing.T) {
	s, tok := setupCreateGist(t)
	id := makeCommits(t, s, tok, 5)

	page1, _ := apiList[types.GistCommit](t, s, "/api/v1/gists/"+id+"/commits?per_page=2&page=1", tok, 200)
	page2, _ := apiList[types.GistCommit](t, s, "/api/v1/gists/"+id+"/commits?per_page=2&page=2", tok, 200)
	page3, _ := apiList[types.GistCommit](t, s, "/api/v1/gists/"+id+"/commits?per_page=2&page=3", tok, 200)

	seen := map[string]bool{}
	for _, c := range append(append(page1, page2...), page3...) {
		require.False(t, seen[c.Version], "commit %s appeared on more than one page", c.Version)
		seen[c.Version] = true
	}
	require.Len(t, seen, 5, "5 distinct commits across the three pages")
}

// TestListCommits_MultipleCommits_AfterPatch verifies that a subsequent
// PATCH produces a second commit and that the most recent commit appears
// first. The patch updates a file's content so deletions are also non-zero.
func TestListCommits_MultipleCommits_AfterPatch(t *testing.T) {
	s, tok := setupCreateGist(t)

	id := createGistViaAPI(t, s, tok, map[string]interface{}{
		"visibility": "public",
		"files":      fileMap{"a.txt": {"content": "alpha"}},
	})

	// PATCH the content → second commit.
	s.APIRequest(t, "PATCH", "/api/v1/gists/"+id, tok,
		`{"files": {"a.txt": {"content": "alpha v2"}}}`, 200)

	commits, _ := apiList[types.GistCommit](t, s, "/api/v1/gists/"+id+"/commits", tok, 200)

	require.Len(t, commits, 2, "create + PATCH = two commits")
	// git log is newest-first.
	require.True(t, !commits[0].CommittedAt.Before(commits[1].CommittedAt),
		"first entry must be at least as recent as the second")

	// The PATCH commit modifies an existing file → both additions and
	// deletions are non-zero.
	patchCommit := commits[0]
	require.Greater(t, patchCommit.ChangeStatus.Additions, 0)
	require.Greater(t, patchCommit.ChangeStatus.Deletions, 0)
}
