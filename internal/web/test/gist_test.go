package test

import (
	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"testing"
)

func TestGists(t *testing.T) {
	setup(t)
	s, err := newTestServer()
	require.NoError(t, err, "Failed to create test server")
	defer teardown(t, s)

	err = s.request("GET", "/", nil, 302)
	require.NoError(t, err)

	user1 := db.UserDTO{Username: "thomas", Password: "thomas"}
	register(t, s, user1)

	err = s.request("GET", "/all", nil, 200)
	require.NoError(t, err)

	gist1 := db.GistDTO{
		Title:       "gist1",
		Description: "my first gist",
		Private:     0,
		Name:        []string{"gist1.txt", "gist2.txt", "gist3.txt"},
		Content:     []string{"yeah", "yeah\ncool", "yeah\ncool gist actually"},
	}
	err = s.request("POST", "/", gist1, 302)
	require.NoError(t, err)

	gist1db, err := db.GetGistByID("1")
	require.NoError(t, err)
	require.Equal(t, gist1.Title, gist1db.Title)
	require.Equal(t, gist1.Description, gist1db.Description)
	require.Regexp(t, "[a-f0-9]{32}", gist1db.Uuid)

	gist1files, err := git.GetFilesOfRepository(gist1db.User.Username, gist1db.Uuid, "HEAD")
	require.NoError(t, err)
	require.Equal(t, len(gist1.Name), len(gist1files))

	gist1fileContent, _, err := git.GetFileContent(gist1db.User.Username, gist1db.Uuid, "HEAD", gist1.Name[0], false)
	require.NoError(t, err)
	require.Equal(t, gist1.Content[0], gist1fileContent)

	gist2 := db.GistDTO{
		Title:       "gist2",
		Description: "my second gist",
		Private:     0,
		Name:        []string{"", "gist2.txt", "gist3.txt"},
		Content:     []string{"", "yeah\ncool", "yeah\ncool gist actually"},
	}
	err = s.request("POST", "/", gist2, 200)
	require.NoError(t, err)

	gist3 := db.GistDTO{
		Title:       "gist3",
		Description: "my third gist",
		Private:     0,
		Name:        []string{""},
		Content:     []string{"yeah"},
	}
	err = s.request("POST", "/", gist3, 302)
	require.NoError(t, err)

	gist3db, err := db.GetGistByID("2")
	require.NoError(t, err)

	gist3files, err := git.GetFilesOfRepository(gist3db.User.Username, gist3db.Uuid, "HEAD")
	require.NoError(t, err)
	require.Equal(t, "gistfile1.txt", gist3files[0])
}

func TestVisibility(t *testing.T) {
	setup(t)
	s, err := newTestServer()
	require.NoError(t, err, "Failed to create test server")
	defer teardown(t, s)

	user1 := db.UserDTO{Username: "thomas", Password: "thomas"}
	register(t, s, user1)

	gist1 := db.GistDTO{
		Title:       "gist1",
		Description: "my first gist",
		Private:     1,
		Name:        []string{""},
		Content:     []string{"yeah"},
	}
	err = s.request("POST", "/", gist1, 302)
	require.NoError(t, err)

	gist1db, err := db.GetGistByID("1")
	require.NoError(t, err)
	require.Equal(t, 1, gist1db.Private)

	err = s.request("POST", "/"+gist1db.User.Username+"/"+gist1db.Uuid+"/visibility", nil, 302)
	require.NoError(t, err)
	gist1db, err = db.GetGistByID("1")
	require.NoError(t, err)
	require.Equal(t, 2, gist1db.Private)

	err = s.request("POST", "/"+gist1db.User.Username+"/"+gist1db.Uuid+"/visibility", nil, 302)
	require.NoError(t, err)
	gist1db, err = db.GetGistByID("1")
	require.NoError(t, err)
	require.Equal(t, 0, gist1db.Private)

	err = s.request("POST", "/"+gist1db.User.Username+"/"+gist1db.Uuid+"/visibility", nil, 302)
	require.NoError(t, err)
	gist1db, err = db.GetGistByID("1")
	require.NoError(t, err)
	require.Equal(t, 1, gist1db.Private)
}
