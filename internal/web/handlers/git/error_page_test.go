package git_test

import (
	"encoding/base64"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

// TestGitRouteErrorPageOmitsGist checks that an error page served for a Git
// HTTP URL does not render the gist, whatever its visibility: for instance the
// 404 page a browser gets when opening a clone URL. Git clients are still
// served from the same URLs.
func TestGitRouteErrorPageOmitsGist(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	baseUrl := s.StartHttpServer(t)

	s.Register(t, "thomas")
	s.Login(t, "thomas")

	type markedGist struct {
		label, id, title, content string
	}

	// Each gist gets a unique title and content (and no description, so the
	// content preview would be used as the page description).
	createGist := func(label, visibility string) markedGist {
		g := markedGist{
			label:   label,
			title:   "title-" + label + "-7f3a9c",
			content: "content-" + label + "-2d8e4b",
		}
		resp := s.Request(t, "POST", "/", url.Values{
			"title":   {g.title},
			"name":    {"file.txt"},
			"content": {g.content},
			"private": {visibility},
		}, 302)
		parts := strings.Split(strings.TrimPrefix(resp.Header.Get("Location"), "/"), "/")
		require.Len(t, parts, 2)
		g.id = parts[1]
		return g
	}

	public := createGist("public", "0")
	unlisted := createGist("unlisted", "1")
	private := createGist("private", "2")
	expired := createGist("expired", "0")

	expiredGist, err := db.GetGist("thomas", expired.id)
	require.NoError(t, err)
	expiredGist.ExpiresAt = time.Now().Add(-time.Hour).Unix()
	require.NoError(t, expiredGist.UpdateNoTimestamps())
	s.Request(t, "GET", "/thomas/"+expired.id, nil, 404) // already hidden from its own page

	browser := map[string]string{"User-Agent": "Mozilla/5.0", "Accept": "text/html"}
	gitClient := map[string]string{"User-Agent": "git/2.43.0"}

	tests := []struct {
		name     string
		settings map[string]string
	}{
		{name: "Default"},
		{name: "RequireLogin", settings: map[string]string{db.SettingRequireLogin: "1"}},
		{name: "AllowGistsWithoutLogin", settings: map[string]string{db.SettingRequireLogin: "1", db.SettingAllowGistsWithoutLogin: "1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s.Login(t, "thomas")
			for k, v := range tt.settings {
				s.Request(t, "PUT", "/-/admin-panel/set-config", url.Values{"key": {k}, "value": {v}}, 200)
			}
			s.Logout()

			for _, g := range []markedGist{public, unlisted, private, expired} {
				t.Run(g.label, func(t *testing.T) {
					requests := []struct {
						uri     string
						headers map[string]string
					}{
						{"/thomas/" + g.id + "/x", browser},
						{"/thomas/" + g.id + ".git/info/refs", browser},
						{"/thomas/" + g.id + "/x", gitClient},
					}

					for _, r := range requests {
						resp := s.RequestWithHeaders(t, "GET", r.uri, nil, 404, r.headers)
						body, err := io.ReadAll(resp.Body)
						require.NoError(t, err)

						require.Falsef(t, strings.Contains(string(body), g.title), "404 page for %s must not render the gist title", r.uri)
						require.Falsef(t, strings.Contains(string(body), g.content), "404 page for %s must not render the gist content", r.uri)
					}
				})
			}

			s.Login(t, "thomas")
			for k := range tt.settings {
				s.Request(t, "PUT", "/-/admin-panel/set-config", url.Values{"key": {k}, "value": {"0"}}, 200)
			}
		})
	}

	// Git clients are still served the gists they may clone.
	t.Run("Clone", func(t *testing.T) {
		s.Logout()

		dest := t.TempDir()
		require.NoError(t, gitClone(baseUrl, "", "thomas", public.id, dest))
		_, err := os.Stat(filepath.Join(dest, "file.txt"))
		require.NoError(t, err)

		dest = t.TempDir()
		require.NoError(t, gitClone(baseUrl, "thomas:thomas", "thomas", private.id, dest))
		_, err = os.Stat(filepath.Join(dest, "file.txt"))
		require.NoError(t, err)
	})

	// The error page rendered when the credentials cannot be checked does not
	// render the gist either. This breaks the owner's login, so it runs last.
	t.Run("AuthenticationError", func(t *testing.T) {
		owner, err := db.GetUserByUsername("thomas")
		require.NoError(t, err)
		owner.Password = "not-a-password-hash"
		require.NoError(t, owner.Update())

		resp := s.RequestWithHeaders(t, "GET", "/thomas/"+private.id+".git/info/refs?service=git-upload-pack", nil, 500, map[string]string{
			"User-Agent":    "git/2.43.0",
			"Accept":        "text/html",
			"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte("thomas:thomas")),
		})
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		require.False(t, strings.Contains(string(body), private.title), "error page must not render the gist title")
		require.False(t, strings.Contains(string(body), private.content), "error page must not render the gist content")
	})
}
