package auth_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/oauth2-proxy/mockoidc"
	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/test"
)

type oidcUser struct {
	*mockoidc.MockUser
}

func (u *oidcUser) Userinfo(scope []string) ([]byte, error) {
	data, err := u.MockUser.Userinfo(scope)
	if err != nil {
		return nil, err
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(data, &claims); err != nil {
		return nil, err
	}
	claims["sub"] = u.Subject
	return json.Marshal(claims)
}

func TestOIDCLoginPKCE(t *testing.T) {
	m, err := mockoidc.Run()
	require.NoError(t, err, "could not start mock OIDC server")
	defer func() { _ = m.Shutdown() }()

	s := test.Setup(t)
	defer test.Teardown(t)

	config.C.OIDCProviderName = "mock"
	config.C.OIDCClientKey = m.ClientID
	config.C.OIDCSecret = m.ClientSecret
	config.C.OIDCDiscoveryUrl = m.DiscoveryEndpoint()
	config.C.OIDCGroupClaimName = "groups"

	base := s.StartHttpServer(t)

	login := func(t *testing.T, invitationCode string, tamper func(*url.URL)) (*http.Response, *url.URL, *http.Client) {
		t.Helper()

		m.QueueUser(&oidcUser{MockUser: &mockoidc.MockUser{
			Subject:           t.Name(),
			Email:             "alice@example.com",
			PreferredUsername: "alice",
			EmailVerified:     true,
		}})

		jar, err := cookiejar.New(nil)
		require.NoError(t, err)

		var authorizeURL *url.URL
		client := &http.Client{
			Jar: jar,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if strings.HasPrefix(req.URL.String(), m.AuthorizationEndpoint()) {
					if tamper != nil {
						tamper(req.URL)
					}
					captured := *req.URL
					authorizeURL = &captured
				}
				if len(via) >= 15 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		}

		if invitationCode != "" {
			resp, err := client.Get(base + "/-/register?code=" + url.QueryEscape(invitationCode))
			require.NoError(t, err)
			_ = resp.Body.Close()
		}

		resp, err := client.Get(base + "/oauth/openid-connect")
		require.NoError(t, err)
		return resp, authorizeURL, client
	}

	t.Run("valid challenge completes login", func(t *testing.T) {
		resp, authorizeURL, _ := login(t, "", nil)
		defer resp.Body.Close()

		require.NotNil(t, authorizeURL, "no redirect to the OIDC authorization endpoint was observed")
		require.NotEmpty(t, authorizeURL.Query().Get("code_challenge"), "code_challenge missing from the authorization request")
		require.Equal(t, "S256", authorizeURL.Query().Get("code_challenge_method"))

		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "/oauth/register", resp.Request.URL.Path)
	})

	t.Run("mismatched challenge fails the token exchange", func(t *testing.T) {
		resp, _, _ := login(t, "", func(u *url.URL) {
			q := u.Query()
			q.Set("code_challenge", "this-does-not-match-the-stored-verifier")
			u.RawQuery = q.Encode()
		})
		defer resp.Body.Close()

		require.Equal(t, "/-/login", resp.Request.URL.Path)
	})

	assertRegistrationDenied := func(t *testing.T, client *http.Client, username string) {
		t.Helper()
		baseURL, err := url.Parse(base)
		require.NoError(t, err)
		var csrfToken string
		for _, cookie := range client.Jar.Cookies(baseURL) {
			if cookie.Name == "_csrf" {
				csrfToken = cookie.Value
			}
		}
		require.NotEmpty(t, csrfToken)

		// Inspect the initial response, including any body written after a redirect.
		client.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
		resp, err := client.PostForm(base+"/oauth/register", url.Values{
			"_csrf":    {csrfToken},
			"username": {username},
			"email":    {"alice@example.com"},
		})
		require.NoError(t, err)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusFound, resp.StatusCode)
		require.Equal(t, "/-/login", resp.Header.Get("Location"))
		exists, err := db.UserExists(username)
		require.NoError(t, err)
		require.False(t, exists, "denied registration must not create an account")

		resp, err = client.Get(base + "/oauth/register")
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		require.NoError(t, err)
		require.Equal(t, http.StatusFound, resp.StatusCode)
		require.Equal(t, "/-/login", resp.Header.Get("Location"))
		require.Empty(t, body, "denied registration must not render the form")
	}

	t.Run("disabled signup rejects registration without an invitation", func(t *testing.T) {
		require.NoError(t, db.UpdateSetting(db.SettingDisableSignup, "1"))
		resp, _, client := login(t, "", nil)
		_ = resp.Body.Close()
		require.Equal(t, "/-/login", resp.Request.URL.Path)
		assertRegistrationDenied(t, client, "uninvited")
	})

	t.Run("disabled signup rejects an invitation invalidated after OAuth", func(t *testing.T) {
		for _, tc := range []struct {
			name       string
			invalidate func(*db.Invitation) error
		}{
			{"expired", func(invitation *db.Invitation) error {
				invitation.ExpiresAt = time.Now().Add(-time.Hour).Unix()
				return invitation.Update()
			}},
			{"exhausted", func(invitation *db.Invitation) error {
				return invitation.Use()
			}},
			{"deleted", func(invitation *db.Invitation) error {
				return invitation.Delete()
			}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				require.NoError(t, db.UpdateSetting(db.SettingDisableSignup, "1"))
				invitation := &db.Invitation{ExpiresAt: time.Now().Add(time.Hour).Unix(), NbMax: 1}
				require.NoError(t, invitation.Create())
				resp, _, client := login(t, invitation.Code, nil)
				_ = resp.Body.Close()
				require.Equal(t, "/oauth/register", resp.Request.URL.Path)
				require.NoError(t, tc.invalidate(invitation))
				assertRegistrationDenied(t, client, "blocked"+tc.name)
			})
		}
	})

	t.Run("invitation permits registration when signup is disabled", func(t *testing.T) {
		require.NoError(t, db.UpdateSetting(db.SettingDisableSignup, "1"))
		blockedResp, _, _ := login(t, "", nil)
		_ = blockedResp.Body.Close()
		require.Equal(t, "/-/login", blockedResp.Request.URL.Path)

		invitation := &db.Invitation{ExpiresAt: time.Now().Add(time.Hour).Unix(), NbMax: 1}
		require.NoError(t, invitation.Create())

		resp, _, client := login(t, invitation.Code, nil)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "/oauth/register", resp.Request.URL.Path)

		baseURL, err := url.Parse(base)
		require.NoError(t, err)
		var csrfToken string
		for _, cookie := range client.Jar.Cookies(baseURL) {
			if cookie.Name == "_csrf" {
				csrfToken = cookie.Value
			}
		}
		require.NotEmpty(t, csrfToken)

		registrationResp, err := client.PostForm(base+"/oauth/register", url.Values{
			"_csrf":    {csrfToken},
			"username": {"invited"},
			"email":    {"alice@example.com"},
		})
		require.NoError(t, err)
		defer registrationResp.Body.Close()
		require.Equal(t, "/", registrationResp.Request.URL.Path)

		user, err := db.GetUserByUsername("invited")
		require.NoError(t, err)
		require.Equal(t, t.Name(), user.OIDCID)
		updatedInvitation, err := db.GetInvitationByID(invitation.ID)
		require.NoError(t, err)
		require.Equal(t, uint(1), updatedInvitation.NbUsed)
	})

	// A user who is still logged in and signs in through the provider again
	// reaches the callback with a session, which is the same path used to link a
	// new account. The identity it finds is their own, and that must not be
	// reported as belonging to somebody else.
	//
	// This lives here rather than in its own test function because the gothic
	// session store is initialised once per process (gothicStoreOnce), against
	// the first test's home directory. A second test.Setup in this package gets
	// a new temp dir that the store never picks up, so its OIDC flow fails on a
	// missing session file.
	t.Run("re-authenticating an already linked account is not a conflict", func(t *testing.T) {
		require.NoError(t, db.UpdateSetting(db.SettingDisableSignup, "0"))
		queueUser := func() {
			m.QueueUser(&oidcUser{MockUser: &mockoidc.MockUser{
				Subject:           "bob-id",
				Email:             "bob@example.com",
				PreferredUsername: "bob",
				EmailVerified:     true,
			}})
		}

		jar, err := cookiejar.New(nil)
		require.NoError(t, err)

		client := &http.Client{
			Jar: jar,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 15 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		}

		// first sign-in: no account exists yet, so the registration form is shown
		queueUser()
		resp, err := client.Get(base + "/oauth/openid-connect")
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, "/oauth/register", resp.Request.URL.Path)

		csrf := regexp.MustCompile(`name="_csrf" value="([^"]+)"`).FindStringSubmatch(string(body))
		require.Len(t, csrf, 2, "could not find the CSRF token in the registration form")

		// completing registration links the OIDC identity and logs the user in
		resp, err = client.PostForm(base+"/oauth/register", url.Values{
			"username": {"bob"},
			"email":    {"bob@example.com"},
			"_csrf":    {csrf[1]},
		})
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, "/", resp.Request.URL.Path, "registration did not complete")

		// sign in again through the provider while the session is still active
		queueUser()
		resp, err = client.Get(base + "/oauth/openid-connect")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, "/", resp.Request.URL.Path,
			"re-authenticating as the already-linked user must not be treated as a conflicting link attempt")
	})

	// Setting a password logs out the user's other sessions; a later login
	// through the provider must still be kept.
	t.Run("login after a password change is kept", func(t *testing.T) {
		require.NoError(t, db.UpdateSetting(db.SettingDisableSignup, "0"))
		queueUser := func() {
			m.QueueUser(&oidcUser{MockUser: &mockoidc.MockUser{
				Subject:           "carol-id",
				Email:             "carol@example.com",
				PreferredUsername: "carol",
				EmailVerified:     true,
			}})
		}
		newClient := func() *http.Client {
			jar, err := cookiejar.New(nil)
			require.NoError(t, err)
			return &http.Client{Jar: jar}
		}
		getCsrf := func(client *http.Client, path string) (*http.Response, string) {
			resp, err := client.Get(base + path)
			require.NoError(t, err)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			csrf := regexp.MustCompile(`name="_csrf" value="([^"]+)"`).FindStringSubmatch(string(body))
			require.Len(t, csrf, 2, "could not find the CSRF token on %s", path)
			return resp, csrf[1]
		}

		client := newClient()
		queueUser()
		resp, csrf := getCsrf(client, "/oauth/openid-connect")
		require.Equal(t, "/oauth/register", resp.Request.URL.Path)
		resp, err := client.PostForm(base+"/oauth/register", url.Values{
			"username": {"carol"},
			"email":    {"carol@example.com"},
			"_csrf":    {csrf},
		})
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, "/", resp.Request.URL.Path, "registration did not complete")

		_, csrf = getCsrf(client, "/-/settings/authentication")
		resp, err = client.PostForm(base+"/-/settings/password", url.Values{
			"_method":  {"PUT"},
			"password": {"carolpassword"},
			"_csrf":    {csrf},
		})
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, "/-/settings", resp.Request.URL.Path, "the session that set the password must stay logged in")

		user, err := db.GetUserByUsername("carol")
		require.NoError(t, err)
		require.Equal(t, uint(1), user.SessionVersion)

		// log in through the provider from another browser
		client = newClient()
		queueUser()
		resp, err = client.Get(base + "/oauth/openid-connect")
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, "/", resp.Request.URL.Path)

		resp, err = client.Get(base + "/-/settings")
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "/-/settings", resp.Request.URL.Path, "the login through the provider must be kept")
	})
}

// crossSiteNavigation holds the fetch metadata headers a browser sends when a
// page on another site navigates to a URL.
var crossSiteNavigation = map[string]string{
	"Sec-Fetch-Site": "cross-site",
	"Sec-Fetch-Mode": "navigate",
	"Sec-Fetch-Dest": "document",
}

// newClient returns an HTTP client that keeps cookies and does not follow
// redirects. Like an older browser it sends no Sec-Fetch-Site header, so its
// unsafe requests have to carry the CSRF token.
func newClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// send makes a request, with form values and headers when given, and returns
// the response along with its body.
func send(t *testing.T, client *http.Client, method, target string, form url.Values, headers map[string]string) (*http.Response, string) {
	t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequest(method, target, body)
	require.NoError(t, err)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, string(b)
}

// csrfToken returns the CSRF token of the forms rendered on a page.
func csrfToken(t *testing.T, client *http.Client, page string) string {
	t.Helper()
	_, body := send(t, client, http.MethodGet, page, nil, nil)
	token := regexp.MustCompile(`name="_csrf" value="([^"]+)"`).FindStringSubmatch(body)
	require.Len(t, token, 2, "no CSRF token found on %s", page)
	return token[1]
}

// loginClient returns a client logged in as a user whose password is its
// username.
func loginClient(t *testing.T, base, username string) *http.Client {
	t.Helper()
	client := newClient(t)
	resp, _ := send(t, client, http.MethodPost, base+"/-/login", url.Values{
		"username": {username},
		"password": {username},
		"_csrf":    {csrfToken(t, client, base+"/-/login")},
	}, nil)
	require.Equal(t, http.StatusFound, resp.StatusCode)
	require.Equal(t, "/", resp.Header.Get("Location"))
	return client
}

func isLoggedIn(t *testing.T, client *http.Client, base string) bool {
	t.Helper()
	resp, _ := send(t, client, http.MethodGet, base+"/", nil, nil)
	return resp.StatusCode == http.StatusOK
}

func TestOauthUnlink(t *testing.T) {
	s := test.Setup(t)
	defer test.Teardown(t)

	config.C.GithubClientKey = "github-key"
	config.C.GithubSecret = "github-secret"
	config.C.OIDCClientKey = "oidc-key"
	config.C.OIDCSecret = "oidc-secret"
	config.C.OIDCDiscoveryUrl = "http://oidc.invalid/.well-known/openid-configuration"

	s.Register(t, "thomas")
	base := s.StartHttpServer(t)
	client := loginClient(t, base, "thomas")
	settingsPage := base + "/-/settings/authentication"

	getUser := func(t *testing.T) *db.User {
		t.Helper()
		user, err := db.GetUserByUsername("thomas")
		require.NoError(t, err)
		return user
	}
	passwordHash := getUser(t).Password

	type loginMethods struct {
		password, passkey, github, gitlab, ldap, loginFormDisabled bool
	}
	// setLoginMethods makes the user an account linked to OpenID Connect, with
	// only the given other ways to log in.
	setLoginMethods := func(t *testing.T, m loginMethods) {
		t.Helper()
		user := getUser(t)
		user.Password, user.GithubID, user.GitlabID, user.OIDCID = "", "", "", "oidc-id"
		if m.password {
			user.Password = passwordHash
		}
		if m.github {
			user.GithubID = "github-id"
		}
		if m.gitlab {
			user.GitlabID = "gitlab-id"
		}
		require.NoError(t, user.Update())

		if m.passkey {
			passkey, err := db.CreateFromCrendential(user.ID, "passkey", &webauthn.Credential{ID: []byte(t.Name())})
			require.NoError(t, err)
			t.Cleanup(func() { _ = passkey.Delete() })
		}

		disableLoginForm := "0"
		if m.loginFormDisabled {
			disableLoginForm = "1"
		}
		require.NoError(t, db.UpdateSetting(db.SettingDisableLoginForm, disableLoginForm))

		config.C.LDAPUrl = ""
		if m.ldap {
			config.C.LDAPUrl = "ldap://ldap.invalid"
		}
	}

	t.Run("GET does not unlink", func(t *testing.T) {
		setLoginMethods(t, loginMethods{password: true, github: true})

		resp, _ := send(t, client, http.MethodGet, base+"/oauth/github/unlink", nil, crossSiteNavigation)
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
		require.Equal(t, "github-id", getUser(t).GithubID)
	})

	t.Run("POST without a CSRF token is rejected", func(t *testing.T) {
		setLoginMethods(t, loginMethods{password: true, github: true})
		csrfToken(t, client, settingsPage) // sets the CSRF cookie

		for _, headers := range []map[string]string{nil, crossSiteNavigation} {
			resp, _ := send(t, client, http.MethodPost, base+"/oauth/github/unlink", url.Values{}, headers)
			require.GreaterOrEqual(t, resp.StatusCode, http.StatusBadRequest)
			require.Equal(t, "github-id", getUser(t).GithubID)
		}
	})

	t.Run("POST with a CSRF token unlinks", func(t *testing.T) {
		setLoginMethods(t, loginMethods{password: true, github: true})

		_, page := send(t, client, http.MethodGet, settingsPage, nil, nil)
		require.True(t, strings.Contains(page, `action="/oauth/github/unlink"`), "the settings page has no unlink form")

		resp, _ := send(t, client, http.MethodPost, base+"/oauth/github/unlink", url.Values{
			"_csrf": {csrfToken(t, client, settingsPage)},
		}, nil)
		require.Equal(t, http.StatusFound, resp.StatusCode)
		require.Equal(t, "/-/settings", resp.Header.Get("Location"))
		require.Empty(t, getUser(t).GithubID)
	})

	t.Run("only login method is not unlinked", func(t *testing.T) {
		for _, tc := range []struct {
			name    string
			methods loginMethods
			allowed bool
		}{
			{"with a password", loginMethods{password: true}, true},
			{"with no other login method", loginMethods{}, false},
			{"with a password and the login form disabled", loginMethods{password: true, loginFormDisabled: true}, false},
			{"with a passkey", loginMethods{passkey: true}, true},
			{"with a passkey and the login form disabled", loginMethods{passkey: true, loginFormDisabled: true}, false},
			{"with LDAP enabled but no password", loginMethods{ldap: true}, false},
			{"with another linked provider", loginMethods{github: true}, true},
			{"with another linked provider and the login form disabled", loginMethods{github: true, loginFormDisabled: true}, true},
			{"with another linked provider that is not configured", loginMethods{gitlab: true}, false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				setLoginMethods(t, tc.methods)

				resp, _ := send(t, client, http.MethodPost, base+"/oauth/openid-connect/unlink", url.Values{
					"_csrf": {csrfToken(t, client, settingsPage)},
				}, nil)
				require.Equal(t, http.StatusFound, resp.StatusCode)

				if tc.allowed {
					require.Equal(t, "/-/settings", resp.Header.Get("Location"))
					require.Empty(t, getUser(t).OIDCID)
					return
				}

				require.Equal(t, "/-/settings/authentication", resp.Header.Get("Location"))
				require.Equal(t, "oidc-id", getUser(t).OIDCID)
				_, page := send(t, client, http.MethodGet, settingsPage, nil, nil)
				require.True(t, strings.Contains(page, "This account cannot be unlinked as it is your only way to log in"),
					"the settings page does not show why the account was not unlinked")
			})
		}
	})
}
