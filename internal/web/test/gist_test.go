package test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
)

func TestGists(t *testing.T) {
	s := Setup(t)
	defer Teardown(t, s)

	err := s.Request("GET", "/", nil, 302)
	require.NoError(t, err)

	user1 := db.UserDTO{Username: "thomas", Password: "thomas"}
	register(t, s, user1)

	err = s.Request("GET", "/all", nil, 200)
	require.NoError(t, err)

	err = s.Request("POST", "/", nil, 400)
	require.NoError(t, err)

	gist1 := db.GistDTO{
		Title:       "gist1",
		Description: "my first gist",
		VisibilityDTO: db.VisibilityDTO{
			Private: 0,
		},
		Name:    []string{"gist1.txt", "gist2.txt", "gist3.txt"},
		Content: []string{"yeah", "yeah\ncool", "yeah\ncool gist actually"},
		Topics:  "",
	}
	err = s.Request("POST", "/", gist1, 302)
	require.NoError(t, err)

	gist1db, err := db.GetGistByID("1")
	require.NoError(t, err)
	require.Equal(t, uint(1), gist1db.ID)
	require.Equal(t, gist1.Title, gist1db.Title)
	require.Equal(t, gist1.Description, gist1db.Description)
	require.Regexp(t, "[a-f0-9]{32}", gist1db.Uuid)
	require.Equal(t, user1.Username, gist1db.User.Username)

	err = s.Request("GET", "/"+gist1db.User.Username+"/"+gist1db.Uuid, nil, 200)
	require.NoError(t, err)

	gist1files, err := git.GetFilesOfRepository(gist1db.User.Username, gist1db.Uuid, "HEAD")
	require.NoError(t, err)
	require.Equal(t, 3, len(gist1files))

	gist1fileContent, _, err := git.GetFileContent(gist1db.User.Username, gist1db.Uuid, "HEAD", gist1.Name[0], false)
	require.NoError(t, err)
	require.Equal(t, gist1.Content[0], gist1fileContent)

	gist2 := db.GistDTO{
		Title:       "gist2",
		Description: "my second gist",
		VisibilityDTO: db.VisibilityDTO{
			Private: 0,
		},
		Name:    []string{"", "gist2.txt", "gist3.txt"},
		Content: []string{"", "yeah\ncool", "yeah\ncool gist actually"},
		Topics:  "",
	}
	err = s.Request("POST", "/", gist2, 400)
	require.NoError(t, err)

	gist3 := db.GistDTO{
		Title:       "gist3",
		Description: "my third gist",
		VisibilityDTO: db.VisibilityDTO{
			Private: 0,
		},
		Name:    []string{""},
		Content: []string{"yeah"},
		Topics:  "",
	}
	err = s.Request("POST", "/", gist3, 302)
	require.NoError(t, err)

	gist3db, err := db.GetGistByID("2")
	require.NoError(t, err)

	gist3files, err := git.GetFilesOfRepository(gist3db.User.Username, gist3db.Uuid, "HEAD")
	require.NoError(t, err)
	require.Equal(t, "gistfile1.txt", gist3files[0])

	err = s.Request("POST", "/"+gist1db.User.Username+"/"+gist1db.Uuid+"/edit", nil, 400)
	require.NoError(t, err)

	gist1.Name = []string{"gist1.txt"}
	gist1.Content = []string{"only want one gist"}

	err = s.Request("POST", "/"+gist1db.User.Username+"/"+gist1db.Uuid+"/edit", gist1, 302)
	require.NoError(t, err)

	gist1files, err = git.GetFilesOfRepository(gist1db.User.Username, gist1db.Uuid, "HEAD")
	require.NoError(t, err)
	require.Equal(t, 1, len(gist1files))

	err = s.Request("POST", "/"+gist1db.User.Username+"/"+gist1db.Uuid+"/delete", nil, 302)
	require.NoError(t, err)
}

func TestVisibility(t *testing.T) {
	s := Setup(t)
	defer Teardown(t, s)

	user1 := db.UserDTO{Username: "thomas", Password: "thomas"}
	register(t, s, user1)

	gist1 := db.GistDTO{
		Title:       "gist1",
		Description: "my first gist",
		VisibilityDTO: db.VisibilityDTO{
			Private: db.UnlistedVisibility,
		},
		Name:    []string{""},
		Content: []string{"yeah"},
		Topics:  "",
	}
	err := s.Request("POST", "/", gist1, 302)
	require.NoError(t, err)

	gist1db, err := db.GetGistByID("1")
	require.NoError(t, err)
	require.Equal(t, db.UnlistedVisibility, gist1db.Private)

	err = s.Request("POST", "/"+gist1db.User.Username+"/"+gist1db.Uuid+"/visibility", db.VisibilityDTO{Private: db.PrivateVisibility}, 302)
	require.NoError(t, err)
	gist1db, err = db.GetGistByID("1")
	require.NoError(t, err)
	require.Equal(t, db.PrivateVisibility, gist1db.Private)

	err = s.Request("POST", "/"+gist1db.User.Username+"/"+gist1db.Uuid+"/visibility", db.VisibilityDTO{Private: db.PublicVisibility}, 302)
	require.NoError(t, err)
	gist1db, err = db.GetGistByID("1")
	require.NoError(t, err)
	require.Equal(t, db.PublicVisibility, gist1db.Private)

	err = s.Request("POST", "/"+gist1db.User.Username+"/"+gist1db.Uuid+"/visibility", db.VisibilityDTO{Private: db.UnlistedVisibility}, 302)
	require.NoError(t, err)
	gist1db, err = db.GetGistByID("1")
	require.NoError(t, err)
	require.Equal(t, db.UnlistedVisibility, gist1db.Private)
}

func TestLikeFork(t *testing.T) {
	s := Setup(t)
	defer Teardown(t, s)

	user1 := db.UserDTO{Username: "thomas", Password: "thomas"}
	register(t, s, user1)

	gist1 := db.GistDTO{
		Title:       "gist1",
		Description: "my first gist",
		VisibilityDTO: db.VisibilityDTO{
			Private: 1,
		},
		Name:    []string{""},
		Content: []string{"yeah"},
		Topics:  "",
	}
	err := s.Request("POST", "/", gist1, 302)
	require.NoError(t, err)

	s.sessionCookie = ""

	user2 := db.UserDTO{Username: "kaguya", Password: "kaguya"}
	register(t, s, user2)

	gist1db, err := db.GetGistByID("1")
	require.NoError(t, err)
	require.Equal(t, 0, gist1db.NbLikes)
	likeCount, err := db.CountAll(db.Like{})
	require.NoError(t, err)
	require.Equal(t, int64(0), likeCount)

	err = s.Request("POST", "/"+gist1db.User.Username+"/"+gist1db.Uuid+"/like", nil, 302)
	require.NoError(t, err)
	gist1db, err = db.GetGistByID("1")
	require.NoError(t, err)
	require.Equal(t, 1, gist1db.NbLikes)
	likeCount, err = db.CountAll(db.Like{})
	require.NoError(t, err)
	require.Equal(t, int64(1), likeCount)

	err = s.Request("POST", "/"+gist1db.User.Username+"/"+gist1db.Uuid+"/like", nil, 302)
	require.NoError(t, err)
	gist1db, err = db.GetGistByID("1")
	require.NoError(t, err)
	require.Equal(t, 0, gist1db.NbLikes)
	likeCount, err = db.CountAll(db.Like{})
	require.NoError(t, err)
	require.Equal(t, int64(0), likeCount)

	err = s.Request("POST", "/"+gist1db.User.Username+"/"+gist1db.Uuid+"/fork", nil, 302)
	require.NoError(t, err)
	gist2db, err := db.GetGistByID("2")
	require.NoError(t, err)
	require.Equal(t, gist1db.Title, gist2db.Title)
	require.Equal(t, gist1db.Description, gist2db.Description)
	require.Equal(t, gist1db.Private, gist2db.Private)
	require.Equal(t, user2.Username, gist2db.User.Username)
}

func TestCustomUrl(t *testing.T) {
	s := Setup(t)
	defer Teardown(t, s)

	user1 := db.UserDTO{Username: "thomas", Password: "thomas"}
	register(t, s, user1)

	gist1 := db.GistDTO{
		Title:       "gist1",
		URL:         "my-gist",
		Description: "my first gist",
		VisibilityDTO: db.VisibilityDTO{
			Private: 0,
		},
		Name:    []string{"gist1.txt", "gist2.txt", "gist3.txt"},
		Content: []string{"yeah", "yeah\ncool", "yeah\ncool gist actually"},
		Topics:  "",
	}
	err := s.Request("POST", "/", gist1, 302)
	require.NoError(t, err)

	gist1db, err := db.GetGistByID("1")
	require.NoError(t, err)
	require.Equal(t, uint(1), gist1db.ID)
	require.Equal(t, gist1.Title, gist1db.Title)
	require.Equal(t, gist1.Description, gist1db.Description)
	require.Regexp(t, "[a-f0-9]{32}", gist1db.Uuid)
	require.Equal(t, gist1.URL, gist1db.URL)
	require.Equal(t, user1.Username, gist1db.User.Username)

	gist1dbUuid, err := db.GetGist(user1.Username, gist1db.Uuid)
	require.NoError(t, err)
	require.Equal(t, gist1db, gist1dbUuid)

	gist1dbUrl, err := db.GetGist(user1.Username, gist1.URL)
	require.NoError(t, err)
	require.Equal(t, gist1db, gist1dbUrl)

	require.Equal(t, gist1.URL, gist1db.Identifier())

	gist2 := db.GistDTO{
		Title:       "gist2",
		Description: "my second gist",
		VisibilityDTO: db.VisibilityDTO{
			Private: 0,
		},
		Name:    []string{"gist1.txt", "gist2.txt", "gist3.txt"},
		Content: []string{"yeah", "yeah\ncool", "yeah\ncool gist actually"},
		Topics:  "",
	}
	err = s.Request("POST", "/", gist2, 302)
	require.NoError(t, err)

	gist2db, err := db.GetGistByID("2")
	require.NoError(t, err)

	require.Equal(t, gist2db.Uuid, gist2db.Identifier())
	require.NotEqual(t, gist2db.URL, gist2db.Identifier())
}

func TestTopics(t *testing.T) {
	s := Setup(t)
	defer Teardown(t, s)

	user1 := db.UserDTO{Username: "thomas", Password: "thomas"}
	register(t, s, user1)

	gist1 := db.GistDTO{
		Title:       "gist1",
		URL:         "my-gist",
		Description: "my first gist",
		VisibilityDTO: db.VisibilityDTO{
			Private: 0,
		},
		Name:    []string{"gist1.txt", "gist2.txt", "gist3.txt"},
		Content: []string{"yeah", "yeah\ncool", "yeah\ncool gist actually"},
		Topics:  "topic1 topic2 topic3",
	}
	err := s.Request("POST", "/", gist1, 302)
	require.NoError(t, err)

	gist1db, err := db.GetGistByID("1")
	require.NoError(t, err)

	require.Equal(t, []db.GistTopic{
		{GistID: 1, Topic: "topic1"},
		{GistID: 1, Topic: "topic2"},
		{GistID: 1, Topic: "topic3"},
	}, gist1db.Topics)

	gist2 := db.GistDTO{
		Title:       "gist2",
		URL:         "my-gist",
		Description: "my second gist",
		VisibilityDTO: db.VisibilityDTO{
			Private: 0,
		},
		Name:    []string{"gist1.txt", "gist2.txt", "gist3.txt"},
		Content: []string{"yeah", "yeah\ncool", "yeah\ncool gist actually"},
		Topics:  "topic1 topic2 topic3 topic2 topic4 topic1",
	}
	err = s.Request("POST", "/", gist2, 302)
	require.NoError(t, err)

	gist2db, err := db.GetGistByID("2")
	require.NoError(t, err)
	require.Equal(t, []db.GistTopic{
		{GistID: 2, Topic: "topic1"},
		{GistID: 2, Topic: "topic2"},
		{GistID: 2, Topic: "topic3"},
		{GistID: 2, Topic: "topic4"},
	}, gist2db.Topics)

	gist3 := db.GistDTO{
		Title:       "gist3",
		URL:         "my-gist",
		Description: "my third gist",
		VisibilityDTO: db.VisibilityDTO{
			Private: 0,
		},
		Name:    []string{"gist1.txt", "gist2.txt", "gist3.txt"},
		Content: []string{"yeah", "yeah\ncool", "yeah\ncool gist actually"},
		Topics:  "topic1 topic2 topic3 topic4 topic5 topic6 topic7 topic8 topic9 topic10 topic11",
	}
	err = s.Request("POST", "/", gist3, 400)
	require.NoError(t, err)

	gist3.Topics = "topictoolongggggggggggggggggggggggggggggggggggggggg"
	err = s.Request("POST", "/", gist3, 400)
	require.NoError(t, err)
}
