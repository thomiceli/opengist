package v1_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	v1 "github.com/thomiceli/opengist/internal/web/handlers/api/v1"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

// setupAPIUser registers "admin" (first user, auto-admin) + "thomas" (regular),
// logs in as thomas, creates a token with full gist + user scope, enables API.
func setupAPIUser(t *testing.T) (*webtest.Server, string) {
	s := webtest.Setup(t)
	t.Cleanup(func() { webtest.Teardown(t) })
	s.Register(t, "admin")
	s.Logout()
	s.Register(t, "thomas")
	s.Login(t, "thomas")
	tok := s.CreateAccessToken(t, "t", db.ReadWritePermission, db.ReadPermission)
	require.NoError(t, db.UpdateSetting(db.SettingApiEnabled, "1"))
	return s, tok
}

func TestListGists_Empty(t *testing.T) {
	s, tok := setupAPIUser(t)

	var resp v1.PaginatedGists
	s.APIRequestUnmarshal(t, "GET", "/api/v1/gists", tok, nil, &resp, 200)
	require.Equal(t, 1, resp.Page)
	require.Equal(t, 10, resp.PerPage)
	require.Equal(t, int64(0), resp.Total)
	require.Empty(t, resp.Data)
}

func TestListGists_Mine(t *testing.T) {
	s, tok := setupAPIUser(t)
	_, _, _, _ = s.CreateGistAs(t, "thomas", "0")
	_, _, _, _ = s.CreateGistAs(t, "thomas", "0")

	var resp v1.PaginatedGists
	s.APIRequestUnmarshal(t, "GET", "/api/v1/gists?per_page=5", tok, nil, &resp, 200)
	require.Equal(t, int64(2), resp.Total)
	require.Len(t, resp.Data, 2)
	require.Equal(t, "thomas", resp.Data[0].Owner.Username)
}

func TestCreateGist(t *testing.T) {
	s, tok := setupAPIUser(t)

	req := v1.CreateGistRequest{
		Title:       "Hello",
		Description: "from API",
		Visibility:  "public",
		Files: []v1.FileInput{
			{Filename: "a.txt", Content: "hello world"},
		},
	}
	var resp v1.GistDetail
	s.APIRequestUnmarshal(t, "POST", "/api/v1/gists", tok, req, &resp, 201)
	require.Equal(t, "Hello", resp.Title)
	require.Equal(t, "public", resp.Visibility)
	require.Len(t, resp.Files, 1)
	require.Equal(t, "a.txt", resp.Files[0].Filename)
	require.Equal(t, "hello world", resp.Files[0].Content)
}

func TestCreateGist_EmptyFiles(t *testing.T) {
	s, tok := setupAPIUser(t)
	req := v1.CreateGistRequest{Title: "x", Visibility: "public", Files: []v1.FileInput{}}
	var body map[string]string
	s.APIRequestUnmarshal(t, "POST", "/api/v1/gists", tok, req, &body, 400)
	require.Equal(t, "validation_failed", body["code"])
}

func TestCreateGist_NoWriteScope(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)
	s.Register(t, "admin")
	s.Logout()
	s.Register(t, "thomas")
	s.Login(t, "thomas")
	tok := s.CreateAccessToken(t, "ro", db.ReadPermission, db.NoPermission)
	require.NoError(t, db.UpdateSetting(db.SettingApiEnabled, "1"))

	req := v1.CreateGistRequest{Title: "x", Visibility: "public", Files: []v1.FileInput{{Filename: "a", Content: "b"}}}
	var body map[string]string
	s.APIRequestUnmarshal(t, "POST", "/api/v1/gists", tok, req, &body, 403)
	require.Equal(t, "forbidden", body["code"])
}

func TestGetGist_Public(t *testing.T) {
	s, tok := setupAPIUser(t)
	_, gist, _, _ := s.CreateGistAs(t, "thomas", "0")

	var resp v1.GistDetail
	s.APIRequestUnmarshal(t, "GET", "/api/v1/gists/"+gist.Uuid, tok, nil, &resp, 200)
	require.Equal(t, gist.Uuid, resp.UUID)
	require.NotEmpty(t, resp.Files)
}

func TestGetGist_PrivateOwner(t *testing.T) {
	s, tok := setupAPIUser(t)
	_, gist, _, _ := s.CreateGistAs(t, "thomas", "2")

	s.APIRequest(t, "GET", "/api/v1/gists/"+gist.Uuid, tok, nil, 200)
}

func TestGetGist_PrivateOther_404(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)
	s.Register(t, "admin")
	s.Logout()
	s.Register(t, "thomas")
	s.Register(t, "alice")
	s.Login(t, "thomas")
	_, gist, _, _ := s.CreateGistAs(t, "thomas", "2")

	s.Login(t, "alice")
	tok := s.CreateAccessToken(t, "t", db.ReadPermission, db.ReadPermission)
	require.NoError(t, db.UpdateSetting(db.SettingApiEnabled, "1"))

	var body map[string]string
	s.APIRequestUnmarshal(t, "GET", "/api/v1/gists/"+gist.Uuid, tok, nil, &body, 404)
	require.Equal(t, "not_found", body["code"])
}

func TestGetGist_NotFound(t *testing.T) {
	s, tok := setupAPIUser(t)
	var body map[string]string
	s.APIRequestUnmarshal(t, "GET", "/api/v1/gists/doesnotexist", tok, nil, &body, 404)
	require.Equal(t, "not_found", body["code"])
}

func TestUpdateGist_Title(t *testing.T) {
	s, tok := setupAPIUser(t)
	_, gist, _, _ := s.CreateGistAs(t, "thomas", "0")

	newTitle := "Renamed"
	req := v1.UpdateGistRequest{Title: &newTitle}
	var resp v1.GistDetail
	s.APIRequestUnmarshal(t, "PATCH", "/api/v1/gists/"+gist.Uuid, tok, req, &resp, 200)
	require.Equal(t, "Renamed", resp.Title)
}

func TestUpdateGist_ReplaceFiles(t *testing.T) {
	s, tok := setupAPIUser(t)
	_, gist, _, _ := s.CreateGistAs(t, "thomas", "0")

	files := []v1.FileInput{{Filename: "new.txt", Content: "fresh"}}
	req := v1.UpdateGistRequest{Files: &files}
	var resp v1.GistDetail
	s.APIRequestUnmarshal(t, "PATCH", "/api/v1/gists/"+gist.Uuid, tok, req, &resp, 200)
	require.Len(t, resp.Files, 1)
	require.Equal(t, "new.txt", resp.Files[0].Filename)
	require.Equal(t, "fresh", resp.Files[0].Content)
}

func TestUpdateGist_NotOwner_404(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)
	s.Register(t, "admin")
	s.Logout()
	s.Register(t, "thomas")
	s.Register(t, "alice")
	s.Login(t, "thomas")
	_, gist, _, _ := s.CreateGistAs(t, "thomas", "0")

	s.Login(t, "alice")
	tok := s.CreateAccessToken(t, "t", db.ReadWritePermission, db.NoPermission)
	require.NoError(t, db.UpdateSetting(db.SettingApiEnabled, "1"))

	newTitle := "hax"
	req := v1.UpdateGistRequest{Title: &newTitle}
	var body map[string]string
	s.APIRequestUnmarshal(t, "PATCH", "/api/v1/gists/"+gist.Uuid, tok, req, &body, 404)
	require.Equal(t, "not_found", body["code"])
}

func TestDeleteGist(t *testing.T) {
	s, tok := setupAPIUser(t)
	_, gist, _, _ := s.CreateGistAs(t, "thomas", "0")

	s.APIRequest(t, "DELETE", "/api/v1/gists/"+gist.Uuid, tok, nil, 204)
	s.APIRequest(t, "GET", "/api/v1/gists/"+gist.Uuid, tok, nil, 404)
}

func TestDeleteGist_NotOwner_404(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)
	s.Register(t, "admin")
	s.Logout()
	s.Register(t, "thomas")
	s.Register(t, "alice")
	s.Login(t, "thomas")
	_, gist, _, _ := s.CreateGistAs(t, "thomas", "0")

	s.Login(t, "alice")
	tok := s.CreateAccessToken(t, "t", db.ReadWritePermission, db.NoPermission)
	require.NoError(t, db.UpdateSetting(db.SettingApiEnabled, "1"))

	s.APIRequest(t, "DELETE", "/api/v1/gists/"+gist.Uuid, tok, nil, 404)
}

func TestListGists_PublicExcludesPrivate(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)
	s.Register(t, "admin")
	s.Logout()
	s.Register(t, "thomas")
	s.Register(t, "alice")
	s.Login(t, "thomas")
	_, _, _, _ = s.CreateGistAs(t, "thomas", "0") // public
	_, _, _, _ = s.CreateGistAs(t, "thomas", "2") // private

	s.Login(t, "alice")
	tok := s.CreateAccessToken(t, "t", db.ReadPermission, db.NoPermission)
	require.NoError(t, db.UpdateSetting(db.SettingApiEnabled, "1"))

	var resp v1.PaginatedGists
	s.APIRequestUnmarshal(t, "GET", "/api/v1/gists?visibility=public", tok, nil, &resp, 200)
	for _, g := range resp.Data {
		require.Equal(t, "public", g.Visibility, "private/unlisted must not appear in visibility=public list")
	}
}

func TestRawFile(t *testing.T) {
	s, tok := setupAPIUser(t)
	_, gist, _, _ := s.CreateGistAs(t, "thomas", "0")
	// CreateGistAs uses {name: file.txt, content: hello}

	body := s.APIRequest(t, "GET", "/api/v1/gists/"+gist.Uuid+"/files/file.txt/raw", tok, nil, 200)
	require.Equal(t, "hello", string(body))
}

func TestRawFile_NotFound(t *testing.T) {
	s, tok := setupAPIUser(t)
	_, gist, _, _ := s.CreateGistAs(t, "thomas", "0")

	var body map[string]string
	s.APIRequestUnmarshal(t, "GET", "/api/v1/gists/"+gist.Uuid+"/files/doesnotexist.txt/raw", tok, nil, &body, 404)
	require.Equal(t, "not_found", body["code"])
}
