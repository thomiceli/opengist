package test

import (
	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	"testing"
)

func TestRegister(t *testing.T) {
	setup(t)
	s, err := newTestServer()
	require.NoError(t, err, "Failed to create test server")
	defer teardown(t, s)

	err = s.request("GET", "/", nil, 302)
	require.NoError(t, err)

	err = s.request("GET", "/register", nil, 200)
	require.NoError(t, err)

	user1 := db.UserDTO{Username: "thomas", Password: "thomas"}
	register(t, s, user1)

	user1db, err := db.GetUserById(1)
	require.NoError(t, err)
	require.Equal(t, user1.Username, user1db.Username)
	require.True(t, user1db.IsAdmin)

	err = s.request("GET", "/", nil, 200)
	require.NoError(t, err)

	s.sessionCookie = ""

	user2 := db.UserDTO{Username: "thomas", Password: "azeaze"}
	err = s.request("POST", "/register", user2, 200)
	require.Error(t, err)

	user3 := db.UserDTO{Username: "kaguya", Password: "kaguya"}
	register(t, s, user3)

	user3db, err := db.GetUserById(2)
	require.NoError(t, err)
	require.False(t, user3db.IsAdmin)

	s.sessionCookie = ""

	count, err := db.CountAll(db.User{})
	require.NoError(t, err)
	require.Equal(t, int64(2), count)
}

func TestLogin(t *testing.T) {
	setup(t)
	s, err := newTestServer()
	require.NoError(t, err, "Failed to create test server")
	defer teardown(t, s)

	err = s.request("GET", "/login", nil, 200)
	require.NoError(t, err)

	user1 := db.UserDTO{Username: "thomas", Password: "thomas"}
	register(t, s, user1)

	s.sessionCookie = ""

	login(t, s, user1)
	require.NotEmpty(t, s.sessionCookie)

	s.sessionCookie = ""

	user2 := db.UserDTO{Username: "thomas", Password: "azeaze"}
	user3 := db.UserDTO{Username: "azeaze", Password: ""}

	err = s.request("POST", "/login", user2, 302)
	require.Empty(t, s.sessionCookie)
	require.Error(t, err)

	err = s.request("POST", "/login", user3, 302)
	require.Empty(t, s.sessionCookie)
	require.Error(t, err)
}

func register(t *testing.T, s *testServer, user db.UserDTO) {
	err := s.request("POST", "/register", user, 302)
	require.NoError(t, err)
}

func login(t *testing.T, s *testServer, user db.UserDTO) {
	err := s.request("POST", "/login", user, 302)
	require.NoError(t, err)
}

type settingSet struct {
	key   string `form:"key"`
	value string `form:"value"`
}

func TestAnonymous(t *testing.T) {
	setup(t)
	s, err := newTestServer()
	require.NoError(t, err, "Failed to create test server")
	defer teardown(t, s)

	user := db.UserDTO{Username: "thomas", Password: "azeaze"}
	register(t, s, user)

	err = s.request("PUT", "/admin-panel/set-config", settingSet{"require-login", "1"}, 200)
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

	err = s.request("GET", "/all", nil, 200)
	require.NoError(t, err)

	cookie := s.sessionCookie
	s.sessionCookie = ""

	err = s.request("GET", "/all", nil, 302)
	require.NoError(t, err)

	// Should redirect to login if RequireLogin
	err = s.request("GET", "/"+gist1db.User.Username+"/"+gist1db.Uuid, nil, 302)
	require.NoError(t, err)

	s.sessionCookie = cookie

	err = s.request("PUT", "/admin-panel/set-config", settingSet{"allow-gists-without-login", "1"}, 200)
	require.NoError(t, err)

	s.sessionCookie = ""

	// Should return results
	err = s.request("GET", "/"+gist1db.User.Username+"/"+gist1db.Uuid, nil, 200)
	require.NoError(t, err)

}
