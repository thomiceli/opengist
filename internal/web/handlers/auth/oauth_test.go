package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
	"time"

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
		require.Equal(t, "alice-id", user.OIDCID)
		updatedInvitation, err := db.GetInvitationByID(invitation.ID)
		require.NoError(t, err)
		require.Equal(t, uint(1), updatedInvitation.NbUsed)
	})
}
