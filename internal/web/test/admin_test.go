package test

import (
	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestAdminPages(t *testing.T) {
	s := Setup(t)
	defer Teardown(t, s)
	urls := []string{
		"/admin-panel",
		"/admin-panel/users",
		"/admin-panel/gists",
		"/admin-panel/invitations",
		"/admin-panel/configuration",
	}

	for _, url := range urls {
		err := s.Request("GET", url, nil, 404)
		require.NoError(t, err)
	}

	user1 := db.UserDTO{Username: "admin", Password: "admin"}
	register(t, s, user1)
	login(t, s, user1)
	for _, url := range urls {
		err := s.Request("GET", url, nil, 200)
		require.NoError(t, err)
	}

	user2 := db.UserDTO{Username: "nonadmin", Password: "nonadmin"}
	register(t, s, user2)
	login(t, s, user2)
	for _, url := range urls {
		err := s.Request("GET", url, nil, 404)
		require.NoError(t, err)
	}
}

func TestSetConfig(t *testing.T) {
	s := Setup(t)
	defer Teardown(t, s)
	settings := []string{
		db.SettingDisableSignup,
		db.SettingRequireLogin,
		db.SettingAllowGistsWithoutLogin,
		db.SettingDisableLoginForm,
		db.SettingDisableGravatar,
	}

	user1 := db.UserDTO{Username: "admin", Password: "admin"}
	register(t, s, user1)
	login(t, s, user1)

	for _, setting := range settings {
		val, err := db.GetSetting(setting)
		require.NoError(t, err)
		require.Equal(t, "0", val)

		err = s.Request("PUT", "/admin-panel/set-config", settingSet{setting, "1"}, 200)
		require.NoError(t, err)

		val, err = db.GetSetting(setting)
		require.NoError(t, err)
		require.Equal(t, "1", val)

		err = s.Request("PUT", "/admin-panel/set-config", settingSet{setting, "0"}, 200)
		require.NoError(t, err)

		val, err = db.GetSetting(setting)
		require.NoError(t, err)
		require.Equal(t, "0", val)
	}
}

func TestPagination(t *testing.T) {
	s := Setup(t)
	defer Teardown(t, s)

	user1 := db.UserDTO{Username: "admin", Password: "admin"}
	register(t, s, user1)
	for i := 0; i < 11; i++ {
		user := db.UserDTO{Username: "user" + strconv.Itoa(i), Password: "user" + strconv.Itoa(i)}
		register(t, s, user)
	}

	login(t, s, user1)

	err := s.Request("GET", "/admin-panel/users", nil, 200)
	require.NoError(t, err)

	err = s.Request("GET", "/admin-panel/users?page=2", nil, 200)
	require.NoError(t, err)

	err = s.Request("GET", "/admin-panel/users?page=3", nil, 404)
	require.NoError(t, err)

	err = s.Request("GET", "/admin-panel/users?page=0", nil, 200)
	require.NoError(t, err)

	err = s.Request("GET", "/admin-panel/users?page=-1", nil, 200)
	require.NoError(t, err)

	err = s.Request("GET", "/admin-panel/users?page=a", nil, 200)
	require.NoError(t, err)
}

func TestAdminUser(t *testing.T) {
	s := Setup(t)
	defer Teardown(t, s)

	user1 := db.UserDTO{Username: "admin", Password: "admin"}
	user2 := db.UserDTO{Username: "nonadmin", Password: "nonadmin"}
	register(t, s, user1)
	register(t, s, user2)

	login(t, s, user2)

	gist1 := db.GistDTO{
		Title: "gist",
		VisibilityDTO: db.VisibilityDTO{
			Private: 0,
		},
		Name:    []string{"gist1.txt"},
		Content: []string{"yeah"},
		Topics:  "",
	}
	err := s.Request("POST", "/", gist1, 302)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(config.GetHomeDir(), git.ReposDirectory, user2.Username))
	require.NoError(t, err)

	count, err := db.CountAll(db.User{})
	require.NoError(t, err)
	require.Equal(t, int64(2), count)

	login(t, s, user1)

	err = s.Request("POST", "/admin-panel/users/2/delete", nil, 302)
	require.NoError(t, err)

	count, err = db.CountAll(db.User{})
	require.NoError(t, err)
	require.Equal(t, int64(1), count)

	_, err = os.Stat(filepath.Join(config.GetHomeDir(), git.ReposDirectory, user2.Username))
	require.Error(t, err)
}

func TestAdminGist(t *testing.T) {
	s := Setup(t)
	defer Teardown(t, s)

	user1 := db.UserDTO{Username: "admin", Password: "admin"}
	register(t, s, user1)
	login(t, s, user1)

	gist1 := db.GistDTO{
		Title: "gist",
		VisibilityDTO: db.VisibilityDTO{
			Private: 0,
		},
		Name:    []string{"gist1.txt"},
		Content: []string{"yeah"},
		Topics:  "",
	}
	err := s.Request("POST", "/", gist1, 302)
	require.NoError(t, err)

	count, err := db.CountAll(db.Gist{})
	require.NoError(t, err)
	require.Equal(t, int64(1), count)

	gist1Db, err := db.GetGistByID("1")
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(config.GetHomeDir(), git.ReposDirectory, user1.Username, gist1Db.Identifier()))
	require.NoError(t, err)

	err = s.Request("POST", "/admin-panel/gists/1/delete", nil, 302)
	require.NoError(t, err)

	count, err = db.CountAll(db.Gist{})
	require.NoError(t, err)
	require.Equal(t, int64(0), count)

	_, err = os.Stat(filepath.Join(config.GetHomeDir(), git.ReposDirectory, user1.Username, gist1Db.Identifier()))
	require.Error(t, err)
}

func TestAdminInvitation(t *testing.T) {
	s := Setup(t)
	defer Teardown(t, s)

	user1 := db.UserDTO{Username: "admin", Password: "admin"}
	register(t, s, user1)
	login(t, s, user1)

	err := s.Request("POST", "/admin-panel/invitations", invitationAdmin{
		nbMax:         "",
		expiredAtUnix: "",
	}, 302)
	require.NoError(t, err)
	invitation1, err := db.GetInvitationByID(1)
	require.NoError(t, err)
	require.Equal(t, uint(1), invitation1.ID)
	require.Equal(t, uint(0), invitation1.NbUsed)
	require.Equal(t, uint(10), invitation1.NbMax)
	require.InDelta(t, time.Now().Unix()+604800, invitation1.ExpiresAt, 10)

	err = s.Request("POST", "/admin-panel/invitations", invitationAdmin{
		nbMax:         "aa",
		expiredAtUnix: "1735722000",
	}, 302)
	require.NoError(t, err)
	invitation2, err := db.GetInvitationByID(2)
	require.NoError(t, err)
	require.Equal(t, invitation2, &db.Invitation{
		ID:        2,
		Code:      invitation2.Code,
		ExpiresAt: time.Unix(1735722000, 0).Unix(),
		NbUsed:    0,
		NbMax:     10,
	})

	err = s.Request("POST", "/admin-panel/invitations", invitationAdmin{
		nbMax:         "20",
		expiredAtUnix: "1735722000",
	}, 302)
	require.NoError(t, err)
	invitation3, err := db.GetInvitationByID(3)
	require.NoError(t, err)
	require.Equal(t, invitation3, &db.Invitation{
		ID:        3,
		Code:      invitation3.Code,
		ExpiresAt: time.Unix(1735722000, 0).Unix(),
		NbUsed:    0,
		NbMax:     20,
	})

	count, err := db.CountAll(db.Invitation{})
	require.NoError(t, err)
	require.Equal(t, int64(3), count)

	err = s.Request("POST", "/admin-panel/invitations/1/delete", nil, 302)
	require.NoError(t, err)

	count, err = db.CountAll(db.Invitation{})
	require.NoError(t, err)
	require.Equal(t, int64(2), count)
}
