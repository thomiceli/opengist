package admin_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

func TestAdminActions(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)
	urls := []string{
		"/admin-panel/sync-fs",
		"/admin-panel/sync-db",
		"/admin-panel/gc-repos",
		"/admin-panel/sync-previews",
		"/admin-panel/reset-hooks",
		"/admin-panel/index-gists",
		"/admin-panel/sync-languages",
	}

	s.Register(t, "thomas")
	s.Register(t, "nonadmin")

	t.Run("NoUser", func(t *testing.T) {
		for _, url := range urls {
			s.Request(t, "POST", url, nil, 404)
		}
	})

	t.Run("AdminUser", func(t *testing.T) {
		s.Login(t, "thomas")
		for _, url := range urls {
			resp := s.Request(t, "POST", url, nil, 302)
			require.Equal(t, "/admin-panel", resp.Header.Get("Location"))
		}
	})

	t.Run("NonAdminUser", func(t *testing.T) {
		s.Login(t, "nonadmin")
		for _, url := range urls {
			s.Request(t, "POST", url, nil, 404)
		}
	})
}
