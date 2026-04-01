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
