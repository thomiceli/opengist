package test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
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
