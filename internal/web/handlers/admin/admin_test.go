package admin_test

import (
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

func TestAdminPages(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	urls := []string{
		"/admin-panel",
		"/admin-panel/users",
		"/admin-panel/gists",
		"/admin-panel/invitations",
		"/admin-panel/configuration",
	}

	s.Register(t, "thomas")
	s.Register(t, "nonadmin")

	t.Run("NoUser", func(t *testing.T) {
		for _, url := range urls {
			s.Request(t, "GET", url, nil, 404)
		}
	})

	t.Run("AdminUser", func(t *testing.T) {
		s.Login(t, "thomas")
		for _, url := range urls {
			s.Request(t, "GET", url, nil, 200)
		}
	})

	t.Run("NonAdminUser", func(t *testing.T) {
		s.Login(t, "nonadmin")
		for _, url := range urls {
			s.Request(t, "GET", url, nil, 404)
		}
	})
}

func TestAdminSetConfig(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	settings := []string{
		db.SettingDisableSignup,
		db.SettingRequireLogin,
		db.SettingAllowGistsWithoutLogin,
		db.SettingDisableLoginForm,
		db.SettingDisableGravatar,
	}

	s.Register(t, "thomas")
	s.Register(t, "nonadmin")

	t.Run("NoUser", func(t *testing.T) {
		s.Request(t, "PUT", "/admin-panel/set-config", url.Values{"key": {db.SettingDisableSignup}, "value": {"1"}}, 404)
	})

	t.Run("NonAdminUser", func(t *testing.T) {
		s.Login(t, "nonadmin")
		s.Request(t, "PUT", "/admin-panel/set-config", url.Values{"key": {db.SettingDisableSignup}, "value": {"1"}}, 404)
	})

	t.Run("AdminUser", func(t *testing.T) {
		s.Login(t, "thomas")

		for _, setting := range settings {
			val, err := db.GetSetting(setting)
			require.NoError(t, err)
			require.Equal(t, "0", val)

			s.Request(t, "PUT", "/admin-panel/set-config", url.Values{"key": {setting}, "value": {"1"}}, 200)

			val, err = db.GetSetting(setting)
			require.NoError(t, err)
			require.Equal(t, "1", val)

			s.Request(t, "PUT", "/admin-panel/set-config", url.Values{"key": {setting}, "value": {"0"}}, 200)

			val, err = db.GetSetting(setting)
			require.NoError(t, err)
			require.Equal(t, "0", val)
		}
	})
}

func TestAdminPagination(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	for i := 0; i < 11; i++ {
		s.Register(t, "user"+strconv.Itoa(i))
	}

	t.Run("Pagination", func(t *testing.T) {
		s.Login(t, "thomas")

		s.Request(t, "GET", "/admin-panel/users", nil, 200)
		s.Request(t, "GET", "/admin-panel/users?page=2", nil, 200)
		s.Request(t, "GET", "/admin-panel/users?page=3", nil, 404)
		s.Request(t, "GET", "/admin-panel/users?page=0", nil, 200)
		s.Request(t, "GET", "/admin-panel/users?page=-1", nil, 200)
		s.Request(t, "GET", "/admin-panel/users?page=a", nil, 200)
	})
}

func TestAdminUserOperations(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	s.Register(t, "nonadmin")

	t.Run("DeleteUser", func(t *testing.T) {
		s.Login(t, "nonadmin")

		gist1 := db.GistDTO{
			Title: "gist",
			VisibilityDTO: db.VisibilityDTO{
				Private: 0,
			},
			Name:    []string{"gist1.txt"},
			Content: []string{"yeah"},
			Topics:  "",
		}
		s.Request(t, "POST", "/", gist1, 302)

		_, err := os.Stat(filepath.Join(config.GetHomeDir(), git.ReposDirectory, "nonadmin"))
		require.NoError(t, err)

		count, err := db.CountAll(db.User{})
		require.NoError(t, err)
		require.Equal(t, int64(2), count)

		s.Request(t, "POST", "/admin-panel/users/2/delete", nil, 404)

		s.Login(t, "thomas")

		s.Request(t, "POST", "/admin-panel/users/2/delete", nil, 302)

		count, err = db.CountAll(db.User{})
		require.NoError(t, err)
		require.Equal(t, int64(1), count)

		_, err = os.Stat(filepath.Join(config.GetHomeDir(), git.ReposDirectory, "nonadmin"))
		require.Error(t, err)
	})
}

func TestAdminGistOperations(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	s.Register(t, "nonadmin")

	t.Run("DeleteGist", func(t *testing.T) {
		s.Login(t, "nonadmin")
		gist1 := db.GistDTO{
			Title: "gist",
			VisibilityDTO: db.VisibilityDTO{
				Private: 0,
			},
			Name:    []string{"gist1.txt"},
			Content: []string{"yeah"},
			Topics:  "",
		}
		s.Request(t, "POST", "/", gist1, 302)

		count, err := db.CountAll(db.Gist{})
		require.NoError(t, err)
		require.Equal(t, int64(1), count)

		gist1Db, err := db.GetGistByID("1")
		require.NoError(t, err)

		_, err = os.Stat(filepath.Join(config.GetHomeDir(), git.ReposDirectory, "nonadmin", gist1Db.Identifier()))
		require.NoError(t, err)

		s.Request(t, "POST", "/admin-panel/gists/1/delete", nil, 404)

		s.Login(t, "thomas")

		s.Request(t, "POST", "/admin-panel/gists/1/delete", nil, 302)

		count, err = db.CountAll(db.Gist{})
		require.NoError(t, err)
		require.Equal(t, int64(0), count)

		_, err = os.Stat(filepath.Join(config.GetHomeDir(), git.ReposDirectory, "nonadmin", gist1Db.Identifier()))
		require.Error(t, err)
	})
}

func TestAdminInvitationOperations(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	s.Register(t, "nonadmin")

	t.Run("Invitation", func(t *testing.T) {
		s.Login(t, "thomas")

		s.Request(t, "POST", "/admin-panel/invitations", url.Values{
			"nbMax":         {""},
			"expiredAtUnix": {""},
		}, 302)
		invitation1, err := db.GetInvitationByID(1)
		require.NoError(t, err)
		require.Equal(t, uint(1), invitation1.ID)
		require.Equal(t, uint(0), invitation1.NbUsed)
		require.Equal(t, uint(10), invitation1.NbMax)
		require.InDelta(t, time.Now().Unix()+604800, invitation1.ExpiresAt, 10)

		s.Request(t, "POST", "/admin-panel/invitations", url.Values{
			"nbMax":         {"aa"},
			"expiredAtUnix": {"1735722000"},
		}, 302)
		invitation2, err := db.GetInvitationByID(2)
		require.NoError(t, err)
		require.Equal(t, invitation2, &db.Invitation{
			ID:        2,
			Code:      invitation2.Code,
			ExpiresAt: time.Unix(1735722000, 0).Unix(),
			NbUsed:    0,
			NbMax:     10,
		})

		s.Request(t, "POST", "/admin-panel/invitations", url.Values{
			"nbMax":         {"20"},
			"expiredAtUnix": {"1735722000"},
		}, 302)
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

		s.Request(t, "POST", "/admin-panel/invitations/1/delete", nil, 302)

		count, err = db.CountAll(db.Invitation{})
		require.NoError(t, err)
		require.Equal(t, int64(2), count)
	})
}
