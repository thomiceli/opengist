package auth_test

import (
	"net/http/httptest"
	"net/url"
	"sync"
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

	// accountsNamed counts the accounts named exactly username, which the
	// database does not keep unique
	accountsNamed := func(username string) int {
		users, err := db.GetUsersByUsername(username)
		require.NoError(t, err)
		n := 0
		for _, user := range users {
			if user.Username == username {
				n++
			}
		}
		return n
	}

	t.Run("creates and logs in a new user", func(t *testing.T) {
		s.RequestWithHeaders(t, "GET", "/", nil, 200, map[string]string{
			"X-Auth-Request-User":  "alice",
			"X-Auth-Request-Email": "Alice@Example.com",
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
		for _, username := range []string{"al\u0130ce", "\u00e1lice", "alice\u200b"} {
			s.RequestWithHeaders(t, "GET", "/", nil, 403, asUser(username))
		}
	})

	t.Run("accounts with a lookalike name are not matched", func(t *testing.T) {
		// as LDAP may create them: "Kate" with a Kelvin sign, whose normalized
		// username is "kate", and "\u00E9lodie", equal to "elodie" in MySQL collations
		for lookalike, username := range map[string]string{"\u212Aate": "kate", "\u00E9lodie": "elodie"} {
			require.NoError(t, (&db.User{Username: lookalike}).Create())

			// the lookalike must not keep the account created by the first login
			// from being found by the next ones
			var id uint
			for i := 0; i < 3; i++ {
				s.RequestWithHeaders(t, "GET", "/", nil, 200, asUser(username))
				require.Equal(t, username, loggedUser().Username)
				if id == 0 {
					id = loggedUser().ID
				}
				require.Equal(t, id, loggedUser().ID)
			}
			require.Equal(t, 1, accountsNamed(username))
		}
	})

	t.Run("concurrent first logins create a single account", func(t *testing.T) {
		start := make(chan struct{})
		codes := make(chan int, 20)
		var wg sync.WaitGroup
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				codes <- s.RequestWithHeaders(t, "GET", "/", nil, 0, asUser("grace")).StatusCode
			}()
		}
		close(start)
		wg.Wait()
		close(codes)

		for code := range codes {
			require.Equal(t, 200, code)
		}
		require.Equal(t, 1, accountsNamed("grace"))
	})

	t.Run("repeated headers are not trusted", func(t *testing.T) {
		// a proxy adding its header after the one of the client
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Add("X-Auth-Request-User", "mallory")
		req.Header.Add("X-Auth-Request-User", "alice")
		s.RawRequest(t, req, 403)
		require.False(t, userExists("mallory"))

		req = httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Auth-Request-User", "heidi")
		req.Header.Add("X-Auth-Request-Email", "alice@example.com")
		req.Header.Add("X-Auth-Request-Email", "heidi@example.com")
		s.RawRequest(t, req, 200)
		heidi, err := db.GetUserByUsername("heidi")
		require.NoError(t, err)
		require.Empty(t, heidi.Email)
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

	t.Run("requests from other sites are only authenticated as navigations", func(t *testing.T) {
		resp := s.RequestWithHeaders(t, "POST", "/", url.Values{
			"title":   {"secret"},
			"name":    {"file.txt"},
			"content": {"hello"},
			"private": {"2"},
		}, 302, asUser("alice"))
		gist := resp.Header.Get("Location")

		fromOtherSite := func(mode, dest string) map[string]string {
			return map[string]string{
				"X-Auth-Request-User": "alice",
				"Sec-Fetch-Site":      "cross-site",
				"Sec-Fetch-Mode":      mode,
				"Sec-Fetch-Dest":      dest,
			}
		}

		// a <script> tag or an iframe on another site, which must not see the
		// private gist even if the proxy cookie is SameSite=None
		s.RequestWithHeaders(t, "GET", gist+".js", nil, 404, fromOtherSite("no-cors", "script"))
		s.RequestWithHeaders(t, "GET", gist, nil, 404, fromOtherSite("navigate", "iframe"))

		// following a link from another site
		s.RequestWithHeaders(t, "GET", gist, nil, 200, fromOtherSite("navigate", "document"))
		require.Equal(t, "alice", loggedUser().Username)

		s.RequestWithHeaders(t, "GET", gist+".js", nil, 200, asUser("alice"))
	})

	t.Run("not used by the API", func(t *testing.T) {
		s.RequestWithHeaders(t, "GET", "/api/gists/liked", nil, 401, asUser("alice"))
	})

	t.Run("LDAP does not create accounts", func(t *testing.T) {
		// nothing listens there: LDAP must not even be asked
		config.C.LDAPUrl = "ldap://127.0.0.1:1"
		defer func() { config.C.LDAPUrl = "" }()

		s.Request(t, "POST", "/-/login", db.UserDTO{Username: "ivan", Password: "ivan"}, 302)
		require.False(t, userExists("ivan"))
	})

	t.Run("disabled", func(t *testing.T) {
		config.C.ProxyAuthUserHeader = ""

		s.RequestWithHeaders(t, "GET", "/", nil, 302, asUser("alice"))
		require.Nil(t, loggedUser())

		s.RequestWithHeaders(t, "GET", "/", nil, 302, asUser("frank"))
		require.False(t, userExists("frank"))
	})
}
