package server_test

import (
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/test"
	"github.com/thomiceli/opengist/templates"
)

// /assets renders the .html templates from $OPENGIST_HOME/custom, and only
// those: the built-in pages, layouts and partials are never rendered there.
func TestAssetsOnlyRenderCustomPages(t *testing.T) {
	s := test.Setup(t)
	defer test.Teardown(t)

	customPages := map[string]string{
		"legal.html": "Custom legal page",
		// same file name as a built-in page
		"search.html": "Custom search page",
	}
	for name, text := range customPages {
		content := `{{ template "header" . }}<p>` + text + `</p>{{ template "footer" . }}`
		require.NoError(t, os.WriteFile(filepath.Join(config.GetHomeDir(), "custom", name), []byte(content), 0644))
	}
	// custom templates are parsed when the server starts
	s.Restart()

	var builtin []string
	for _, pattern := range []string{"pages/*.html", "layouts/*.html", "partials/*.html"} {
		paths, err := fs.Glob(templates.Files, pattern)
		require.NoError(t, err)
		for _, p := range paths {
			if _, ok := customPages[path.Base(p)]; !ok {
				builtin = append(builtin, path.Base(p))
			}
		}
	}
	require.Contains(t, builtin, "admin_config.html")

	s.Register(t, "thomas")
	s.Login(t, "thomas")
	s.Request(t, "PUT", "/-/admin-panel/set-config", url.Values{"key": {db.SettingRequireLogin}, "value": {"1"}}, 200)

	t.Run("BuiltinTemplatesAdminUser", func(t *testing.T) {
		s.Login(t, "thomas")
		for _, name := range builtin {
			s.Request(t, "GET", "/assets/"+name, nil, 404)
		}
	})

	t.Run("BuiltinTemplatesNoUser", func(t *testing.T) {
		s.Logout()
		for _, name := range builtin {
			s.Request(t, "GET", "/assets/"+name, nil, 404)
		}
	})

	t.Run("InvalidNames", func(t *testing.T) {
		s.Logout()
		for _, name := range []string{"missing.html", "custom/legal.html", "../custom/legal.html", "sub/../legal.html"} {
			s.Request(t, "GET", "/assets/"+name, nil, 404)
		}
	})

	// like the other files in $OPENGIST_HOME/custom, custom pages do not
	// require login, as they are linked from the login page
	t.Run("CustomPagesNoUser", func(t *testing.T) {
		s.Logout()
		for name, text := range customPages {
			resp := s.Request(t, "GET", "/assets/"+name, nil, 200)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Contains(t, string(body), text)
			require.Contains(t, string(body), "<html", "custom page should be rendered with the site layout")
		}
	})
}
