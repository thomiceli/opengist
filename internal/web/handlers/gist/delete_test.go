package gist_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

func TestDeleteGist(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	s.Register(t, "alice")

	t.Run("NoAuth", func(t *testing.T) {
		gistPath, _, username, identifier := s.CreateGist(t, "0")

		deleteURL := "/" + username + "/" + identifier + "/delete"
		s.Request(t, "POST", deleteURL, nil, 302)

		gistCheck, err := db.GetGist(username, identifier)
		require.NoError(t, err, "Gist should still exist in database")
		require.NotNil(t, gistCheck)

		_, err = os.Stat(gistPath)
		require.NoError(t, err, "Gist should still exist on filesystem")
	})

	t.Run("DeleteOwnGist", func(t *testing.T) {
		gistPath, _, username, identifier := s.CreateGist(t, "0")

		gistCheck, err := db.GetGist(username, identifier)
		require.NoError(t, err)
		require.NotNil(t, gistCheck)

		s.Login(t, "thomas")
		deleteURL := "/" + username + "/" + identifier + "/delete"
		s.Request(t, "POST", deleteURL, nil, 302)

		gistCheck, err = db.GetGist(username, identifier)
		require.Error(t, err, "Gist should be deleted from database")

		_, err = os.Stat(gistPath)
		require.Error(t, err, "Gist should not exist on filesystem after deletion")
		require.True(t, os.IsNotExist(err), "Filesystem should return 'not exist' error")
	})

	t.Run("DeleteOthersGist", func(t *testing.T) {
		gistPath, _, username, identifier := s.CreateGist(t, "0")

		s.Login(t, "alice")
		deleteURL := "/" + username + "/" + identifier + "/delete"
		s.Request(t, "POST", deleteURL, nil, 403)

		gistCheck, err := db.GetGist(username, identifier)
		require.NoError(t, err, "Gist should still exist in database")
		require.NotNil(t, gistCheck)

		_, err = os.Stat(gistPath)
		require.NoError(t, err, "Gist should still exist on filesystem")
	})

	t.Run("DeleteNonExistentGist", func(t *testing.T) {
		s.Login(t, "thomas")

		deleteURL := "/thomas/nonexistent-gist-12345/delete"
		s.Request(t, "POST", deleteURL, nil, 404)
	})
}
