package gist_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

func TestVisibility(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	s.Register(t, "alice")

	t.Run("ChangeVisibility", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Login(t, "thomas")
		s.Request(t, "POST", "/"+username+"/"+identifier+"/visibility", url.Values{
			"private": {"2"},
		}, 302)

		gist, err := db.GetGist(username, identifier)
		require.NoError(t, err)
		require.Equal(t, db.PrivateVisibility, gist.Private)
	})

	t.Run("ChangeToUnlisted", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Login(t, "thomas")
		s.Request(t, "POST", "/"+username+"/"+identifier+"/visibility", url.Values{
			"private": {"1"},
		}, 302)

		gist, err := db.GetGist(username, identifier)
		require.NoError(t, err)
		require.Equal(t, db.UnlistedVisibility, gist.Private)
	})

	t.Run("OtherUserCannotChange", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Login(t, "alice")
		s.Request(t, "POST", "/"+username+"/"+identifier+"/visibility", nil, 403)

		gist, err := db.GetGist(username, identifier)
		require.NoError(t, err)
		require.Equal(t, db.PublicVisibility, gist.Private)
	})

	t.Run("NoAuth", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Logout()
		s.Request(t, "POST", "/"+username+"/"+identifier+"/visibility", nil, 302)

		gist, err := db.GetGist(username, identifier)
		require.NoError(t, err)
		require.Equal(t, db.PublicVisibility, gist.Private)
	})
}

func TestArchive(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	s.Register(t, "alice")

	t.Run("OwnerCanArchiveAndUnarchive", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Login(t, "thomas")
		s.Request(t, "POST", "/"+username+"/"+identifier+"/archive", nil, 302)

		gist, err := db.GetGist(username, identifier)
		require.NoError(t, err)
		require.True(t, gist.Archived)

		// Toggling again unarchives the gist.
		s.Request(t, "POST", "/"+username+"/"+identifier+"/archive", nil, 302)

		gist, err = db.GetGist(username, identifier)
		require.NoError(t, err)
		require.False(t, gist.Archived)
	})

	t.Run("OtherUserCannotArchive", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Login(t, "alice")
		s.Request(t, "POST", "/"+username+"/"+identifier+"/archive", nil, 403)

		gist, err := db.GetGist(username, identifier)
		require.NoError(t, err)
		require.False(t, gist.Archived)
	})

	t.Run("CannotEditArchivedGist", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Login(t, "thomas")
		s.Request(t, "POST", "/"+username+"/"+identifier+"/archive", nil, 302)

		// Both the edit page and the edit submission are blocked while archived.
		s.Request(t, "GET", "/"+username+"/"+identifier+"/edit", nil, 403)
		s.Request(t, "POST", "/"+username+"/"+identifier+"/edit", url.Values{
			"title":   {"Changed"},
			"name":    {"file.txt"},
			"content": {"changed content"},
		}, 403)

		// The checkbox toggle is a write path too, so it must be blocked.
		s.Request(t, "PUT", "/"+username+"/"+identifier+"/checkbox", url.Values{
			"file":     {"file.txt"},
			"checkbox": {"0"},
		}, 403)
	})

	t.Run("CanEditAfterUnarchive", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Login(t, "thomas")
		// Archive then unarchive.
		s.Request(t, "POST", "/"+username+"/"+identifier+"/archive", nil, 302)
		s.Request(t, "POST", "/"+username+"/"+identifier+"/archive", nil, 302)

		// Editing works again once the gist is no longer archived.
		s.Request(t, "GET", "/"+username+"/"+identifier+"/edit", nil, 200)
		s.Request(t, "POST", "/"+username+"/"+identifier+"/edit", url.Values{
			"title":   {"Changed"},
			"name":    {"file.txt"},
			"content": {"changed content"},
		}, 302)
	})
}
