package gist_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

func TestLike(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	s.Register(t, "alice")

	t.Run("Like", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Login(t, "alice")
		resp := s.Request(t, "POST", "/"+username+"/"+identifier+"/like", nil, 302)
		require.Equal(t, "/"+username+"/"+identifier, resp.Header.Get("Location"))

		s.Login(t, "thomas")
		s.Request(t, "POST", "/"+username+"/"+identifier+"/like", nil, 302)

		gist, err := db.GetGist(username, identifier)
		require.NoError(t, err)
		require.Equal(t, 2, gist.NbLikes)

		likers, err := gist.GetUsersLikes(0)
		require.NoError(t, err)
		require.Len(t, likers, 2)
	})

	t.Run("Unlike", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Login(t, "alice")
		s.Request(t, "POST", "/"+username+"/"+identifier+"/like", nil, 302)

		s.Request(t, "POST", "/"+username+"/"+identifier+"/like", nil, 302)

		gist, err := db.GetGist(username, identifier)
		require.NoError(t, err)
		require.Equal(t, 0, gist.NbLikes)

		likers, err := gist.GetUsersLikes(0)
		require.NoError(t, err)
		require.Len(t, likers, 0)
	})

	t.Run("NoAuth", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Logout()
		s.Request(t, "POST", "/"+username+"/"+identifier+"/like", nil, 302)

		gist, err := db.GetGist(username, identifier)
		require.NoError(t, err)
		require.Equal(t, 0, gist.NbLikes)
	})

	t.Run("PrivateGist", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "2")

		s.Login(t, "alice")
		s.Request(t, "POST", "/"+username+"/"+identifier+"/like", nil, 404)
	})
}

func TestLikes(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	s.Register(t, "alice")

	t.Run("Likes", func(t *testing.T) {
		_, gist, username, identifier := s.CreateGist(t, "0")

		s.Login(t, "thomas")
		s.Request(t, "POST", "/"+username+"/"+identifier+"/like", nil, 302)
		s.Login(t, "alice")
		s.Request(t, "POST", "/"+username+"/"+identifier+"/like", nil, 302)

		s.Request(t, "GET", "/"+username+"/"+identifier+"/likes", nil, 200)

		users, err := gist.GetUsersLikes(0)
		require.NoError(t, err)
		require.Len(t, users, 2)
		require.Equal(t, "thomas", users[0].Username)
		require.Equal(t, "alice", users[1].Username)
	})
}
