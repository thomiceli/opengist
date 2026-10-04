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

	"github.com/oauth2-proxy/mockoidc"
	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/config"
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

	login := func(t *testing.T, tamper func(*url.URL)) (*http.Response, *url.URL) {
		t.Helper()

		m.QueueUser(&oidcUser{MockUser: &mockoidc.MockUser{
			Subject:           "alice-id",
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

		resp, err := client.Get(base + "/oauth/openid-connect")
		require.NoError(t, err)
		return resp, authorizeURL
	}

	t.Run("valid challenge completes login", func(t *testing.T) {
		resp, authorizeURL := login(t, nil)
		defer resp.Body.Close()

		require.NotNil(t, authorizeURL, "no redirect to the OIDC authorization endpoint was observed")
		require.NotEmpty(t, authorizeURL.Query().Get("code_challenge"), "code_challenge missing from the authorization request")
		require.Equal(t, "S256", authorizeURL.Query().Get("code_challenge_method"))

		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "/oauth/register", resp.Request.URL.Path)
	})

	t.Run("mismatched challenge fails the token exchange", func(t *testing.T) {
		resp, _ := login(t, func(u *url.URL) {
			q := u.Query()
			q.Set("code_challenge", "this-does-not-match-the-stored-verifier")
			u.RawQuery = q.Encode()
		})
		defer resp.Body.Close()

		require.Equal(t, "/-/login", resp.Request.URL.Path)
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
}
