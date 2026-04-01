package gist_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

func TestFork(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	s.Register(t, "alice")

	t.Run("Fork", func(t *testing.T) {
		_, gist, username, identifier := s.CreateGist(t, "0")
		s.Login(t, "alice")

		resp := s.Request(t, "POST", "/"+username+"/"+identifier+"/fork", nil, 302)

		forkedGist, err := db.GetGistByID("2")
		require.NoError(t, err)
		require.Equal(t, "alice", forkedGist.User.Username)
		require.Equal(t, gist.Title, forkedGist.Title)
		require.Equal(t, gist.Description, forkedGist.Description)
		require.Equal(t, gist.Private, forkedGist.Private)
		require.Equal(t, gist.ID, forkedGist.ForkedID)

		forkedFiles, err := forkedGist.Files("HEAD", false)
		require.NoError(t, err)

		gistFiles, err := gist.Files("HEAD", false)
		require.NoError(t, err)

		for i, file := range gistFiles {
			require.Equal(t, file.Filename, forkedFiles[i].Filename)
			require.Equal(t, file.Content, forkedFiles[i].Content)
		}

		require.Equal(t, "/alice/"+forkedGist.Identifier(), resp.Header.Get("Location"))

		original, err := db.GetGistByID("1")
		require.NoError(t, err)
		require.Equal(t, 1, original.NbForks)

		forks, err := original.GetForks(2, 0)
		require.NoError(t, err)
		require.Len(t, forks, 1)
		require.Equal(t, forkedGist.ID, forks[0].ID)

		forkedGists, err := db.GetAllGistsForkedByUser(2, 2, 0, "created", "asc")
		require.NoError(t, err)
		require.Len(t, forkedGists, 1)
		require.Equal(t, forkedGist.ID, forkedGists[0].ID)
	})

	t.Run("OwnGist", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")
		s.Login(t, "thomas")

		s.Request(t, "POST", "/"+username+"/"+identifier+"/fork", nil, 302)

		original, err := db.GetGist(username, identifier)
		require.NoError(t, err)
		require.Equal(t, 0, original.NbForks)
	})

	t.Run("AlreadyForked", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")
		s.Login(t, "alice")

		firstResp := s.Request(t, "POST", "/"+username+"/"+identifier+"/fork", nil, 302)
		forkLocation := firstResp.Header.Get("Location")

		secondResp := s.Request(t, "POST", "/"+username+"/"+identifier+"/fork", nil, 302)
		require.Equal(t, forkLocation, secondResp.Header.Get("Location"))

		original, err := db.GetGist(username, identifier)
		require.NoError(t, err)
		require.Equal(t, 1, original.NbForks)
	})

	t.Run("NoAuth", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Request(t, "POST", "/"+username+"/"+identifier+"/fork", nil, 302)

		original, err := db.GetGist(username, identifier)
		require.NoError(t, err)
		require.Equal(t, 0, original.NbForks)
	})

	t.Run("PrivateGist", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "2")
		s.Login(t, "alice")

		s.Request(t, "POST", "/"+username+"/"+identifier+"/fork", nil, 404)
	})
}
