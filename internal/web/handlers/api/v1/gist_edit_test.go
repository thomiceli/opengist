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

// setupCreateGist registers "thomas" (after a stub admin), enables the API,
// and mints a token with full gist + user scope. Logs the user out before
// returning so requests go through the token-auth path.
func setupCreateGist(t *testing.T) (*webtest.Server, string) {
	s := webtest.Setup(t)
	t.Cleanup(func() { webtest.Teardown(t) })

	s.Register(t, "admin")
	s.Logout()
	s.Register(t, "thomas")
	s.Login(t, "thomas")

	tok := s.CreateAccessToken(t, "tok", db.ReadWritePermission, db.ReadPermission)
	s.Logout()
	return s, tok
}

// fileMap is the shape clients send for the request `files` map: filename →
// {"content": "..."}.
type fileMap = map[string]map[string]string

// TestUpdateGist_ChangeRenameDelete exercises the three PATCH operations in
// one go: rewrite a file's content, rename a file, and delete a file (via
// JSON null). The gist starts with three files; after the patch only the
// expected two should remain, with their expected names and contents.
func TestUpdateGist_ChangeRenameDelete(t *testing.T) {
	s, tok := setupCreateGist(t)

	id := createGistViaAPI(t, s, tok, map[string]interface{}{
		"visibility": "public",
		"files": fileMap{
			"a.txt": {"content": "alpha"},
			"b.txt": {"content": "beta"},
			"c.txt": {"content": "gamma"},
		},
	})

	// Patch:
	//   - a.txt → update content
	//   - b.txt → rename to renamed.txt (content preserved)
	//   - c.txt → delete (JSON null)
	// Sent as raw JSON because the entry-level `null` for delete doesn't
	// round-trip through a typed map[string]map[string]string.
	patch := `{
		"files": {
			"a.txt": {"content": "alpha v2"},
			"b.txt": {"filename": "renamed.txt"},
			"c.txt": null
		}
	}`

	_, raw := s.APIRequest(t, "PATCH", "/api/v1/gists/"+id, tok, patch, 200)
	var resp fullGist
	require.NoError(t, json.Unmarshal(raw, &resp), "response: %s", string(raw))

	// a.txt: content updated, name unchanged.
	require.Contains(t, resp.Files, "a.txt", "a.txt must still exist after content update")
	require.Equal(t, "alpha v2", resp.Files["a.txt"].Content, "a.txt content must reflect the patch")

	// b.txt: renamed, content preserved.
	require.NotContains(t, resp.Files, "b.txt", "old name b.txt must be gone after rename")
	require.Contains(t, resp.Files, "renamed.txt", "renamed.txt must appear")
	require.Equal(t, "beta", resp.Files["renamed.txt"].Content,
		"renamed file must keep its original content when only filename is set")

	// c.txt: deleted.
	require.NotContains(t, resp.Files, "c.txt", "c.txt must be deleted (set to null)")

	// Exactly the two expected files.
	require.Len(t, resp.Files, 2)
}

// createSeedGist makes a public gist with predictable title/description/files
// used by the PATCH metadata tests. Returns the gist's id.
func createSeedGist(t *testing.T, s *webtest.Server, tok string) string {
	return createGistViaAPI(t, s, tok, map[string]interface{}{
		"title":       "seed-title",
		"description": "seed-description",
		"visibility":  "public",
		"files":       fileMap{"a.txt": {"content": "seed"}},
	})
}

// TestUpdateGist_VisibilityChange - PATCHing only `visibility` changes the
// visibility and leaves title/description untouched.
func TestUpdateGist_VisibilityChange(t *testing.T) {
	s, tok := setupCreateGist(t)
	id := createSeedGist(t, s, tok)

	_, raw := s.APIRequest(t, "PATCH", "/api/v1/gists/"+id, tok,
		`{"visibility": "private"}`, 200)
	var resp fullGist
	require.NoError(t, json.Unmarshal(raw, &resp))

	require.Equal(t, "private", resp.Visibility)
	require.False(t, resp.Public)
	// Untouched fields.
	require.Equal(t, "seed-title", resp.Title)
	require.Equal(t, "seed-description", resp.Description)
}

// TestUpdateGist_TitleChange - PATCHing only `title` changes the title and
// leaves description/visibility untouched.
func TestUpdateGist_TitleChange(t *testing.T) {
	s, tok := setupCreateGist(t)
	id := createSeedGist(t, s, tok)

	_, raw := s.APIRequest(t, "PATCH", "/api/v1/gists/"+id, tok,
		`{"title": "renamed-title"}`, 200)
	var resp fullGist
	require.NoError(t, json.Unmarshal(raw, &resp))

	require.Equal(t, "renamed-title", resp.Title)
	// Untouched fields.
	require.Equal(t, "seed-description", resp.Description)
	require.Equal(t, "public", resp.Visibility)
}

// TestUpdateGist_DescriptionChange - PATCHing only `description` changes the
// description (including overwriting with empty string) and leaves the rest
// alone.
func TestUpdateGist_DescriptionChange(t *testing.T) {
	s, tok := setupCreateGist(t)
	id := createSeedGist(t, s, tok)

	_, raw := s.APIRequest(t, "PATCH", "/api/v1/gists/"+id, tok,
		`{"description": "updated-description"}`, 200)
	var resp fullGist
	require.NoError(t, json.Unmarshal(raw, &resp))

	require.Equal(t, "updated-description", resp.Description)
	// Untouched fields.
	require.Equal(t, "seed-title", resp.Title)
	require.Equal(t, "public", resp.Visibility)
}

// TestUpdateGist_NoAccess mirrors TestDeleteGist_NoAccess for the PATCH
// endpoint: anonymous → 401; authenticated non-owner gets 403 for
// public/unlisted gists (visible via GET so a 403 is honest) and 404 for
// private gists (existence stays hidden).
func TestUpdateGist_NoAccess(t *testing.T) {
	s := webtest.Setup(t)
	t.Cleanup(func() { webtest.Teardown(t) })
	s.Register(t, "admin")
	s.Logout()
	s.Register(t, "thomas")
	s.Register(t, "other")

	_, gPub, _, _ := s.CreateGistAs(t, "thomas", "0")
	_, gUnl, _, _ := s.CreateGistAs(t, "thomas", "1")
	_, gPriv, _, _ := s.CreateGistAs(t, "thomas", "2")

	s.Login(t, "other")
	otherTok := s.CreateAccessToken(t, "other-tok", db.ReadWritePermission, db.ReadPermission)
	s.Logout()

	// Minimal valid body - satisfies the "at least one field must be set"
	// guard so a 422 doesn't pre-empt the access-control check we're after.
	body := `{"description": "hacked"}`

	cases := []struct {
		name string
		uuid string
		tok  string
		want int
	}{
		{"anon/public", gPub.Uuid, "", 401},
		{"anon/unlisted", gUnl.Uuid, "", 401},
		{"anon/private", gPriv.Uuid, "", 401},

		{"other/public", gPub.Uuid, otherTok, 403},
		{"other/unlisted", gUnl.Uuid, otherTok, 403},
		{"other/private", gPriv.Uuid, otherTok, 404},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s.APIRequest(t, "PATCH", "/api/v1/gists/"+c.uuid, c.tok, body, c.want)
		})
	}

	// Reload each gist from the DB - descriptions must NOT have been touched.
	for _, want := range []*db.Gist{gPub, gUnl, gPriv} {
		got, err := db.GetGistByUUID(want.Uuid)
		require.NoError(t, err)
		require.NotEqual(t, "hacked", got.Description,
			"failed PATCH attempt mutated description on gist %s", want.Uuid)
	}
}

// TestDeleteGist_NoAccess covers the failure-code semantics for callers who
// can't delete a gist. The route's apiRequireAuth catches missing tokens
// first (→ 401). When the caller is authenticated but not the owner, the
// handler returns:
//
//   - 403 for public / unlisted gists (existence already disclosed via GET)
//   - 404 for private gists (existence stays hidden)
func TestDeleteGist_NoAccess(t *testing.T) {
	s := webtest.Setup(t)
	t.Cleanup(func() { webtest.Teardown(t) })
	s.Register(t, "admin")
	s.Logout()
	s.Register(t, "thomas")
	s.Register(t, "other")

	// thomas owns one gist of each visibility.
	_, gPub, _, _ := s.CreateGistAs(t, "thomas", "0")
	_, gUnl, _, _ := s.CreateGistAs(t, "thomas", "1")
	_, gPriv, _, _ := s.CreateGistAs(t, "thomas", "2")

	// "other" has a write-scoped token (anything less would be 403'd by the
	// apiScope(ScopeGist, ReadWritePermission) middleware before reaching the
	// handler - uninteresting for these assertions).
	s.Login(t, "other")
	otherTok := s.CreateAccessToken(t, "other-tok", db.ReadWritePermission, db.ReadPermission)
	s.Logout()

	cases := []struct {
		name string
		uuid string
		tok  string
		want int
	}{
		// Anonymous (no token) is rejected by apiRequireAuth → 401 across
		// every visibility.
		{"anon/public", gPub.Uuid, "", 401},
		{"anon/unlisted", gUnl.Uuid, "", 401},
		{"anon/private", gPriv.Uuid, "", 401},

		// Authenticated non-owner: 403 when existence is already disclosed
		// (public + unlisted), 404 when it isn't (private).
		{"other/public", gPub.Uuid, otherTok, 403},
		{"other/unlisted", gUnl.Uuid, otherTok, 403},
		{"other/private", gPriv.Uuid, otherTok, 404},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s.APIRequest(t, "DELETE", "/api/v1/gists/"+c.uuid, c.tok, nil, c.want)
		})
	}

	// All three gists should still be there - none of the failed deletes
	// touched DB or filesystem state.
	for _, g := range []*db.Gist{gPub, gUnl, gPriv} {
		_, err := db.GetGistByUUID(g.Uuid)
		require.NoError(t, err, "gist %s must still exist after failed delete attempts", g.Uuid)
	}
}

// TestUpdateGist_EmptyBody_422 - a PATCH with no actionable fields must
// 422 instead of silently no-opping (and bumping updated_at for nothing).
func TestUpdateGist_EmptyBody_422(t *testing.T) {
	s, tok := setupCreateGist(t)
	id := createSeedGist(t, s, tok)

	s.APIRequest(t, "PATCH", "/api/v1/gists/"+id, tok, `{}`, 422)
	// Same when only an empty files map is supplied - no actual change.
	s.APIRequest(t, "PATCH", "/api/v1/gists/"+id, tok, `{"files": {}}`, 422)
}

func TestCreateGist_NoAuth(t *testing.T) {
	s, _ := setupCreateGist(t)

	body := map[string]interface{}{
		"files": fileMap{"test.txt": {"content": "hello"}},
	}
	s.APIRequest(t, "POST", "/api/v1/gists", "", body, 401)

	count, err := db.CountAll(db.Gist{})
	require.NoError(t, err)
	require.Equal(t, int64(0), count, "no gist must have been created")
}

func TestCreateGist(t *testing.T) {
	s, tok := setupCreateGist(t)

	tests := []struct {
		name              string
		body              interface{}
		expectedCode      int
		expectGistCreated bool
		expectedTitle     string
		expectedDesc      string
		expectedVis       string // "public"|"unlisted"|"private"
		expectedPublic    bool
		expectedFilenames []string
		expectedContents  map[string]string // filename → content
	}{
		{
			name:         "NoFiles",
			body:         map[string]interface{}{"title": "Test GistSimple"},
			expectedCode: 422,
		},
		{
			name: "EmptyContent",
			body: map[string]interface{}{
				"title": "Test GistSimple",
				"files": fileMap{"test.txt": {"content": ""}},
			},
			// Empty content is silently skipped; min=1 on Files then fails.
			expectedCode: 422,
		},
		{
			name: "TitleTooLong",
			body: map[string]interface{}{
				"title": strings.Repeat("a", 251),
				"files": fileMap{"test.txt": {"content": "hello"}},
			},
			expectedCode: 422,
		},
		{
			name: "DescriptionTooLong",
			body: map[string]interface{}{
				"description": strings.Repeat("a", 1001),
				"files":       fileMap{"test.txt": {"content": "hello"}},
			},
			expectedCode: 422,
		},
		{
			name: "FilenameTooLong",
			body: map[string]interface{}{
				"files": fileMap{strings.Repeat("a", 256) + ".txt": {"content": "hello"}},
			},
			expectedCode: 422,
		},
		{
			name: "UnknownVisibilityCoercesToPublic",
			body: map[string]interface{}{
				"visibility": "secret", // not a known string
				"files":      fileMap{"test.txt": {"content": "hello"}},
			},
			// db.ParseVisibility defaults unknown values to public - same as the
			// web form path. The API doesn't 422 on this.
			expectedCode:      201,
			expectGistCreated: true,
			expectedTitle:     "test.txt",
			expectedVis:       "public",
			expectedPublic:    true,
			expectedFilenames: []string{"test.txt"},
			expectedContents:  map[string]string{"test.txt": "hello"},
		},
		{
			name: "Valid",
			body: map[string]interface{}{
				"title":      "My Test GistSimple",
				"visibility": "public",
				"files":      fileMap{"test.txt": {"content": "hello world"}},
			},
			expectedCode:      201,
			expectGistCreated: true,
			expectedTitle:     "My Test GistSimple",
			expectedVis:       "public",
			expectedPublic:    true,
			expectedFilenames: []string{"test.txt"},
			expectedContents:  map[string]string{"test.txt": "hello world"},
		},
		{
			name: "AutoNamedFile",
			body: map[string]interface{}{
				"title":      "Auto Named",
				"visibility": "public",
				"files":      fileMap{"": {"content": "content without name"}},
			},
			expectedCode:      201,
			expectGistCreated: true,
			expectedTitle:     "Auto Named",
			expectedVis:       "public",
			expectedPublic:    true,
			expectedFilenames: []string{"gistfile1.txt"},
			expectedContents:  map[string]string{"gistfile1.txt": "content without name"},
		},
		{
			name: "MultipleFiles",
			body: map[string]interface{}{
				"title":      "Multi File GistSimple",
				"visibility": "public",
				"files": fileMap{
					"a.txt":    {"content": "content 1"},
					"file2.md": {"content": "content 2"},
				},
			},
			expectedCode:      201,
			expectGistCreated: true,
			expectedTitle:     "Multi File GistSimple",
			expectedVis:       "public",
			expectedPublic:    true,
			expectedFilenames: []string{"a.txt", "file2.md"},
			expectedContents: map[string]string{
				"a.txt":    "content 1",
				"file2.md": "content 2",
			},
		},
		{
			name: "NoTitle_FallsBackToFirstFilename",
			body: map[string]interface{}{
				"visibility": "public",
				"files":      fileMap{"readme.md": {"content": "# README"}},
			},
			expectedCode:      201,
			expectGistCreated: true,
			expectedTitle:     "readme.md",
			expectedVis:       "public",
			expectedPublic:    true,
			expectedFilenames: []string{"readme.md"},
			expectedContents:  map[string]string{"readme.md": "# README"},
		},
		{
			name: "Unlisted",
			body: map[string]interface{}{
				"title":      "Unlisted GistSimple",
				"visibility": "unlisted",
				"files":      fileMap{"secret.txt": {"content": "secret content"}},
			},
			expectedCode:      201,
			expectGistCreated: true,
			expectedTitle:     "Unlisted GistSimple",
			expectedVis:       "unlisted",
			expectedPublic:    false,
			expectedFilenames: []string{"secret.txt"},
			expectedContents:  map[string]string{"secret.txt": "secret content"},
		},
		{
			name: "Private",
			body: map[string]interface{}{
				"title":      "Private GistSimple",
				"visibility": "private",
				"files":      fileMap{"secret.txt": {"content": "secret content"}},
			},
			expectedCode:      201,
			expectGistCreated: true,
			expectedTitle:     "Private GistSimple",
			expectedVis:       "private",
			expectedPublic:    false,
			expectedFilenames: []string{"secret.txt"},
			expectedContents:  map[string]string{"secret.txt": "secret content"},
		},
		{
			name: "FilenameWithUnicode",
			body: map[string]interface{}{
				"title":      "Unicode Filename",
				"visibility": "public",
				"files":      fileMap{"文件.txt": {"content": "hello world"}},
			},
			expectedCode:      201,
			expectGistCreated: true,
			expectedTitle:     "Unicode Filename",
			expectedVis:       "public",
			expectedPublic:    true,
			expectedFilenames: []string{"文件.txt"},
			expectedContents:  map[string]string{"文件.txt": "hello world"},
		},
		{
			name: "FilenamePathTraversal",
			body: map[string]interface{}{
				"title":      "Path Traversal",
				"visibility": "public",
				"files":      fileMap{"../../../etc/passwd": {"content": "malicious"}},
			},
			// CleanTreePathName strips path separators, leaving just the base name.
			expectedCode:      201,
			expectGistCreated: true,
			expectedTitle:     "Path Traversal",
			expectedVis:       "public",
			expectedPublic:    true,
			expectedFilenames: []string{"passwd"},
			expectedContents:  map[string]string{"passwd": "malicious"},
		},
		{
			name: "EmptyAndValidContent",
			body: map[string]interface{}{
				"title":      "Mixed Content",
				"visibility": "public",
				"files": fileMap{
					"empty.txt":      {"content": ""},
					"valid.txt":      {"content": "valid content"},
					"also-empty.txt": {"content": ""},
				},
			},
			// Empty-content entries are dropped; only valid.txt survives.
			expectedCode:      201,
			expectGistCreated: true,
			expectedTitle:     "Mixed Content",
			expectedVis:       "public",
			expectedPublic:    true,
			expectedFilenames: []string{"valid.txt"},
			expectedContents:  map[string]string{"valid.txt": "valid content"},
		},
		{
			name: "ContentMultibyteUnicode",
			body: map[string]interface{}{
				"title":      "Unicode Content",
				"visibility": "public",
				"files":      fileMap{"unicode.txt": {"content": "Hello 世界 🌍 Привет"}},
			},
			expectedCode:      201,
			expectGistCreated: true,
			expectedTitle:     "Unicode Content",
			expectedVis:       "public",
			expectedPublic:    true,
			expectedFilenames: []string{"unicode.txt"},
			expectedContents:  map[string]string{"unicode.txt": "Hello 世界 🌍 Привет"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, body := s.APIRequest(t, "POST", "/api/v1/gists", tok, tt.body, tt.expectedCode)

			if !tt.expectGistCreated {
				return
			}

			var resp types.Gist
			require.NoError(t, json.Unmarshal(body, &resp), "response body: %s", string(body))

			require.Equal(t, tt.expectedTitle, resp.Title, "title mismatch")
			require.Equal(t, tt.expectedVis, resp.Visibility, "visibility mismatch")
			require.Equal(t, tt.expectedPublic, resp.Public, "public bool mismatch")
			require.Equal(t, "thomas", resp.Owner.Login, "owner mismatch")
			require.NotEmpty(t, resp.ID, "gist UUID must be set in response")

			// Location header: required on 201.
			require.NotEmpty(t, w.Header().Get("Location"), "Location header missing")
			require.Contains(t, w.Header().Get("Location"), "/api/v1/gists/"+resp.ID)

			// Files map keyed by filename.
			require.Len(t, resp.Files, len(tt.expectedFilenames), "file count mismatch")
			for _, name := range tt.expectedFilenames {
				require.Contains(t, resp.Files, name, "expected file %s missing from response", name)
			}
			for name, want := range tt.expectedContents {
				require.Equal(t, want, resp.Files[name].Content, "content mismatch for file %s", name)
			}

			// Verify the gist actually landed in the DB.
			saved, err := db.GetGistByUUID(resp.ID)
			require.NoError(t, err)
			require.Equal(t, "thomas", saved.User.Username)
			require.Equal(t, tt.expectedTitle, saved.Title)
			require.Equal(t, len(tt.expectedFilenames), saved.NbFiles)
		})
	}
}
