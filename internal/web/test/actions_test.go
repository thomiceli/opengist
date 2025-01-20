package test

import (
	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	"testing"
)

func TestAdminActions(t *testing.T) {
	s := Setup(t)
	defer Teardown(t, s)
	urls := []string{
		"/admin-panel/sync-fs",
		"/admin-panel/sync-db",
		"/admin-panel/gc-repos",
		"/admin-panel/sync-previews",
		"/admin-panel/reset-hooks",
		"/admin-panel/index-gists",
	}

	for _, url := range urls {
		err := s.Request("POST", url, nil, 404)
		require.NoError(t, err)
	}

	user1 := db.UserDTO{Username: "admin", Password: "admin"}
	register(t, s, user1)
	login(t, s, user1)
	for _, url := range urls {
		err := s.Request("POST", url, nil, 302)
		require.NoError(t, err)
	}

	user2 := db.UserDTO{Username: "nonadmin", Password: "nonadmin"}
	register(t, s, user2)
	login(t, s, user2)
	for _, url := range urls {
		err := s.Request("POST", url, nil, 404)
		require.NoError(t, err)
	}
}
