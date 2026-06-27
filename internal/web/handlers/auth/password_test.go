package auth_test

import (
	"net/url"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

func TestRegisterPage(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)
	s.Register(t, "thomas")

	t.Run("Form", func(t *testing.T) {
		s.Request(t, "GET", "/register", nil, 200)
		s.TestCtxData(t, echo.Map{
			"isLoginPage":   false,
			"disableForm":   false,
			"disableSignup": false,
		})
	})

	t.Run("FormDisabled", func(t *testing.T) {
		s.Login(t, "thomas")
		s.Request(t, "PUT", "/admin-panel/set-config", url.Values{"key": {"disable-signup"}, "value": {"1"}}, 200)
		s.Logout()

		s.Request(t, "GET", "/register", nil, 200)
		s.TestCtxData(t, echo.Map{
			"disableSignup": true,
		})
	})

	t.Run("FormDisabledWithInviteCode", func(t *testing.T) {
		s.Login(t, "thomas")
		s.Request(t, "PUT", "/admin-panel/set-config", url.Values{"key": {"disable-signup"}, "value": {"1"}}, 200)

		s.Request(t, "POST", "/admin-panel/invitations", url.Values{
			"nbMax":         {"10"},
			"expiredAtUnix": {""},
		}, 302)

		invitation, err := db.GetInvitationByID(1)
		require.NoError(t, err)

		s.Logout()

		s.Request(t, "GET", "/register", nil, 200)
		s.TestCtxData(t, echo.Map{
			"disableSignup": true,
		})
		s.Request(t, "GET", "/register?code="+invitation.Code, nil, 200)
		s.TestCtxData(t, echo.Map{
			"disableSignup": false,
		})
	})
}

func TestProcessRegister(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	s.Register(t, "alice")

	t.Run("Register", func(t *testing.T) {
		user, err := db.GetUserByUsername("thomas")
		require.NoError(t, err)
		require.True(t, user.IsAdmin)
		s.Logout()

		s.Request(t, "POST", "/register", db.UserDTO{Username: "seconduser", Password: "password123"}, 302)
		user, err = db.GetUserByUsername("seconduser")
		require.NoError(t, err)
		require.False(t, user.IsAdmin)
		s.Logout()
	})

	t.Run("DuplicateUsername", func(t *testing.T) {
		s.Request(t, "POST", "/register", db.UserDTO{Username: "useraaa", Password: "password123"}, 302)
		s.Logout()
		s.Request(t, "POST", "/register", db.UserDTO{Username: "useraaa", Password: "password456"}, 200)
		s.Logout()
	})

	t.Run("InvalidUsername", func(t *testing.T) {
		s.Request(t, "POST", "/register", db.UserDTO{Username: "", Password: "password123"}, 200)
		s.Request(t, "POST", "/register", db.UserDTO{Username: "aze@", Password: "password123"}, 200)
	})

	t.Run("EmptyPassword", func(t *testing.T) {
		s.Request(t, "POST", "/register", db.UserDTO{Username: "newuser", Password: ""}, 200)
	})

	t.Run("RegisterDisabled", func(t *testing.T) {
		s.Login(t, "thomas")
		s.Request(t, "PUT", "/admin-panel/set-config", url.Values{"key": {"disable-signup"}, "value": {"1"}}, 200)
		s.Logout()

		s.Request(t, "POST", "/register", db.UserDTO{Username: "blocked", Password: "password123"}, 403)

		exists, err := db.UserExists("blocked")
		require.NoError(t, err)
		require.False(t, exists)
	})

	t.Run("RegisterWithInvitationCode", func(t *testing.T) {
		s.Login(t, "thomas")
		s.Request(t, "PUT", "/admin-panel/set-config", url.Values{"key": {"disable-signup"}, "value": {"1"}}, 200)
		s.Request(t, "POST", "/admin-panel/invitations", url.Values{
			"nbMax":         {"10"},
			"expiredAtUnix": {""},
		}, 302)
		s.Logout()

		invitations, err := db.GetAllInvitations()
		require.NoError(t, err)
		require.NotEmpty(t, invitations)
		invitation := invitations[len(invitations)-1]

		s.Logout()

		s.Request(t, "POST", "/register?code="+invitation.Code, db.UserDTO{Username: "inviteduser", Password: "password123"}, 302)

		user, err := db.GetUserByUsername("inviteduser")
		require.NoError(t, err)
		require.Equal(t, "inviteduser", user.Username)

		updatedInvitation, err := db.GetInvitationByID(invitation.ID)
		require.NoError(t, err)
		require.Equal(t, uint(1), updatedInvitation.NbUsed)
	})
}

func TestLoginPage(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)
	s.Register(t, "thomas")

	t.Run("Form", func(t *testing.T) {
		s.Request(t, "GET", "/login", nil, 200)
		s.TestCtxData(t, echo.Map{
			"isLoginPage": true,
			"disableForm": false,
		})
	})

	t.Run("FormDisabled", func(t *testing.T) {
		s.Login(t, "thomas")
		s.Request(t, "PUT", "/admin-panel/set-config", url.Values{"key": {"disable-login-form"}, "value": {"1"}}, 200)
		s.Logout()

		s.Request(t, "GET", "/login", nil, 200)
		s.TestCtxData(t, echo.Map{
			"disableForm": true,
		})
	})
}

func TestProcessLogin(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")

	t.Run("ValidCredentials", func(t *testing.T) {
		resp := s.Request(t, "POST", "/login", db.UserDTO{Username: "thomas", Password: "thomas"}, 302)
		require.Equal(t, "/", resp.Header.Get("Location"))
		require.NotEmpty(t, s.SessionCookie)
		require.Equal(t, "thomas", s.User().Username)

		s.Logout()
	})

	t.Run("InvalidPassword", func(t *testing.T) {
		resp := s.Request(t, "POST", "/login", db.UserDTO{Username: "thomas", Password: "wrongpassword"}, 302)
		require.Equal(t, "/login", resp.Header.Get("Location"))
		require.Nil(t, s.User())
	})

	t.Run("NonExistentUser", func(t *testing.T) {
		resp := s.Request(t, "POST", "/login", db.UserDTO{Username: "nonexistent", Password: "password"}, 302)
		require.Equal(t, "/login", resp.Header.Get("Location"))
		require.Nil(t, s.User())
	})

	t.Run("EmptyCredentials", func(t *testing.T) {
		s.Request(t, "POST", "/login", db.UserDTO{Username: "", Password: ""}, 302)
		require.Nil(t, s.User())
	})

	t.Run("LoginFormDisabled", func(t *testing.T) {
		s.Login(t, "thomas")
		s.Request(t, "PUT", "/admin-panel/set-config", url.Values{"key": {"disable-login-form"}, "value": {"1"}}, 200)
		s.Logout()

		s.Request(t, "POST", "/login", db.UserDTO{Username: "thomas", Password: "thomas"}, 403)
		require.Nil(t, s.User())
	})
}

func TestLogout(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")

	t.Run("LogoutRedirects", func(t *testing.T) {
		s.Login(t, "thomas")
		require.Equal(t, "thomas", s.User().Username)

		resp := s.Request(t, "GET", "/logout", nil, 302)
		require.Equal(t, "/all", resp.Header.Get("Location"))
		require.Nil(t, s.User())
		s.Request(t, "GET", "/", nil, 302)
	})
}
