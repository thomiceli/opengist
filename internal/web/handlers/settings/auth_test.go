package settings_test

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"
	ogtotp "github.com/thomiceli/opengist/internal/auth/totp"
	"github.com/thomiceli/opengist/internal/db"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

// sessionCookie returns the value of the session cookie set by the response,
// or an empty string if it does not set one.
func sessionCookie(resp *http.Response) string {
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "session" {
			return cookie.Value
		}
	}
	return ""
}

func TestPasswordChangeLogsOutOtherSessions(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")

	// two sessions of the same user, as if logged in from two browsers
	s.Login(t, "thomas")
	sessionA := s.SessionCookie
	s.Logout()
	s.Login(t, "thomas")
	sessionB := s.SessionCookie

	s.SessionCookie = sessionA
	resp := s.Request(t, "PUT", "/-/settings/password", url.Values{"password": {"newpassword"}}, 302)
	newSessionA := sessionCookie(resp)

	// the other session is logged out
	s.SessionCookie = sessionB
	s.Request(t, "GET", "/-/settings", nil, 302)
	require.Nil(t, s.User())

	// the session that changed the password stays logged in, under a new cookie
	require.NotEmpty(t, newSessionA)
	s.SessionCookie = newSessionA
	s.Request(t, "GET", "/-/settings", nil, 200)
	require.Equal(t, "thomas", s.User().Username)

	// so that a copy of its previous cookie is logged out as well
	s.SessionCookie = sessionA
	s.Request(t, "GET", "/-/settings", nil, 302)
	require.Nil(t, s.User())

	// only the new password logs in, and the new session is kept
	s.Logout()
	s.Request(t, "POST", "/-/login", db.UserDTO{Username: "thomas", Password: "thomas"}, 302)
	require.Nil(t, s.User())
	s.Request(t, "POST", "/-/login", db.UserDTO{Username: "thomas", Password: "newpassword"}, 302)
	s.Request(t, "GET", "/-/settings", nil, 200)
	require.Equal(t, "thomas", s.User().Username)
}

func TestPasswordChangeKeepsTotpLogin(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	s.Login(t, "thomas")
	s.Request(t, "PUT", "/-/settings/password", url.Values{"password": {"newpassword"}}, 302)

	user, err := db.GetUserByUsername("thomas")
	require.NoError(t, err)
	require.Equal(t, uint(1), user.SessionVersion)

	secret, _, _, err := ogtotp.GenerateQRCode(user.Username, "opengist.io", nil)
	require.NoError(t, err)
	userTotp := &db.TOTP{UserID: user.ID}
	require.NoError(t, userTotp.StoreSecret(secret))
	require.NoError(t, userTotp.Create())

	// a login completed with TOTP after the password change is kept
	s.Logout()
	resp := s.Request(t, "POST", "/-/login", db.UserDTO{Username: "thomas", Password: "newpassword"}, 302)
	require.Equal(t, "/mfa", resp.Header.Get("Location"))

	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)
	s.Request(t, "POST", "/mfa/totp/assertion", url.Values{"code": {code}}, 302)
	s.Request(t, "GET", "/-/settings", nil, 200)
	require.Equal(t, "thomas", s.User().Username)
}
