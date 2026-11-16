package auth_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/test"
)

func TestProxyAuth(t *testing.T) {
	s := test.Setup(t)
	defer test.Teardown(t)

	config.C.ProxyAuthUserHeader = "X-Auth-Request-User"
	config.C.ProxyAuthEmailHeader = "X-Auth-Request-Email"
	// requests built by the test server come from 192.0.2.1
	config.C.ProxyAuthTrustedCIDRs = "192.0.2.0/24"

	asUser := func(username string) map[string]string {
		return map[string]string{"X-Auth-Request-User": username}
	}

	loggedUser := func() *db.User {
		v, _ := s.ContextData("userLogged")
		user, _ := v.(*db.User)
		return user
	}

	userExists := func(username string) bool {
		exists, err := db.UserExists(username)
		require.NoError(t, err)
		return exists
	}

	t.Run("creates and logs in a new user", func(t *testing.T) {
		s.RequestWithHeaders(t, "GET", "/", nil, 200, map[string]string{
			"X-Auth-Request-User":  "alice",
			"X-Auth-Request-Email": "alice@example.com",
		})
		require.NotNil(t, loggedUser())
		require.Equal(t, "alice", loggedUser().Username)

		user, err := db.GetUserByUsername("alice")
		require.NoError(t, err)
		require.Equal(t, "alice@example.com", user.Email)
		require.NotEmpty(t, user.MD5Hash)
		require.True(t, user.IsAdmin, "first user should be admin")
	})

	t.Run("logs in an existing user", func(t *testing.T) {
		// registration is disabled while proxy authentication is enabled
		config.C.ProxyAuthUserHeader = ""
		s.Register(t, "bob")
		s.Logout()
		config.C.ProxyAuthUserHeader = "X-Auth-Request-User"

		s.RequestWithHeaders(t, "GET", "/", nil, 200, asUser("Bob"))
		require.Equal(t, "bob", loggedUser().Username)
		require.False(t, loggedUser().IsAdmin)
	})

	t.Run("header takes precedence over the session", func(t *testing.T) {
		s.Login(t, "bob")
		defer s.Logout()

		s.RequestWithHeaders(t, "GET", "/", nil, 200, asUser("alice"))
		require.Equal(t, "alice", loggedUser().Username)
	})

	t.Run("no header", func(t *testing.T) {
		s.RequestWithHeaders(t, "GET", "/", nil, 302, nil)
		require.Nil(t, loggedUser())
	})

	t.Run("ignores the header from an untrusted address", func(t *testing.T) {
		config.C.ProxyAuthTrustedCIDRs = "10.0.0.0/8, ::1"
		defer func() { config.C.ProxyAuthTrustedCIDRs = "192.0.2.0/24" }()

		s.RequestWithHeaders(t, "GET", "/", nil, 302, asUser("alice"))
		require.Nil(t, loggedUser())

		s.RequestWithHeaders(t, "GET", "/", nil, 302, asUser("carol"))
		require.False(t, userExists("carol"))
	})

	t.Run("signup disabled", func(t *testing.T) {
		require.NoError(t, db.UpdateSetting(db.SettingDisableSignup, "1"))
		defer func() { require.NoError(t, db.UpdateSetting(db.SettingDisableSignup, "0")) }()

		s.RequestWithHeaders(t, "GET", "/", nil, 403, asUser("dave"))
		require.False(t, userExists("dave"))

		// assets of the error page are still served
		resp := s.RequestWithHeaders(t, "GET", "/assets/does-not-exist.css", nil, 0, asUser("dave"))
		require.NotEqual(t, 403, resp.StatusCode)

		// existing users can still log in
		s.RequestWithHeaders(t, "GET", "/", nil, 200, asUser("alice"))
		require.Equal(t, "alice", loggedUser().Username)
	})

	t.Run("invalid username", func(t *testing.T) {
		s.RequestWithHeaders(t, "GET", "/", nil, 403, asUser("eve@example.com"))
		require.False(t, userExists("eve@example.com"))

		s.RequestWithHeaders(t, "GET", "/", nil, 403, asUser("api"))
		require.False(t, userExists("api"))
	})

	t.Run("unicode lookalikes do not match an existing user", func(t *testing.T) {
		// "alİce" lowercases to "alice" in Go, "álice" equals it in MySQL collations
		for _, username := range []string{"alİce", "álice", "alice​"} {
			s.RequestWithHeaders(t, "GET", "/", nil, 403, asUser(username))
		}
	})

	t.Run("local registration is disabled", func(t *testing.T) {
		invitation := &db.Invitation{ExpiresAt: time.Now().Add(time.Hour).Unix(), NbMax: 1}
		require.NoError(t, invitation.Create())

		for _, uri := range []string{"/-/register", "/-/register?code=" + invitation.Code} {
			s.Request(t, "GET", uri, nil, 200)
			disableSignup, _ := s.ContextData("disableSignup")
			require.Equal(t, true, disableSignup)

			s.Request(t, "POST", uri, db.UserDTO{Username: "mallory", Password: "mallory"}, 403)
			require.False(t, userExists("mallory"))
		}
	})

	t.Run("username cannot be changed", func(t *testing.T) {
		s.RequestWithHeaders(t, "GET", "/-/settings", nil, 200, asUser("alice"))

		resp := s.RequestWithHeaders(t, "PUT", "/-/settings/username", url.Values{"username": {"mallory"}}, 302, asUser("alice"))
		require.Equal(t, "/-/settings", resp.Header.Get("Location"))
		require.True(t, userExists("alice"))
		require.False(t, userExists("mallory"))

		alice, err := db.GetUserByUsername("alice")
		require.NoError(t, err)
		token := &db.AccessToken{Name: "rename", UserID: alice.ID, ScopeUser: db.ReadWritePermission}
		plainToken, err := token.GenerateToken()
		require.NoError(t, err)
		require.NoError(t, token.Create())

		_, body := s.APIRequest(t, "PATCH", "/api/user", plainToken, map[string]string{"username": "mallory"}, 403)
		require.Contains(t, string(body), "authentication proxy")
		require.False(t, userExists("mallory"))

		// the email can still be changed
		s.APIRequest(t, "PATCH", "/api/user", plainToken, map[string]string{"email": "alice@corp.example"}, 200)
	})

	t.Run("not used by the API", func(t *testing.T) {
		s.RequestWithHeaders(t, "GET", "/api/gists/liked", nil, 401, asUser("alice"))
	})

	t.Run("disabled", func(t *testing.T) {
		config.C.ProxyAuthUserHeader = ""

		s.RequestWithHeaders(t, "GET", "/", nil, 302, asUser("alice"))
		require.Nil(t, loggedUser())

		s.RequestWithHeaders(t, "GET", "/", nil, 302, asUser("frank"))
		require.False(t, userExists("frank"))
	})
}
