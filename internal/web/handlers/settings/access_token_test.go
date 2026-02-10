package settings_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

func TestAccessTokensCRUD(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")

	t.Run("RequiresAuth", func(t *testing.T) {
		s.Logout()
		s.Request(t, "GET", "/settings/access-tokens", nil, 302)
	})

	t.Run("AccessTokensPage", func(t *testing.T) {
		s.Login(t, "thomas")
		s.Request(t, "GET", "/settings/access-tokens", nil, 200)
	})

	t.Run("CreateReadToken", func(t *testing.T) {
		s.Login(t, "thomas")
		s.Request(t, "POST", "/settings/access-tokens", db.AccessTokenDTO{
			Name:      "test-token",
			ScopeGist: db.ReadPermission,
		}, 302)

		tokens, err := db.GetAccessTokensByUserID(1)
		require.NoError(t, err)
		require.Len(t, tokens, 1)
		require.Equal(t, "test-token", tokens[0].Name)
		require.Equal(t, uint(db.ReadPermission), tokens[0].ScopeGist)
		require.Equal(t, int64(0), tokens[0].ExpiresAt)
	})

	t.Run("CreateExpiringToken", func(t *testing.T) {
		s.Login(t, "thomas")
		tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
		s.Request(t, "POST", "/settings/access-tokens", db.AccessTokenDTO{
			Name:      "expiring-token",
			ScopeGist: db.ReadWritePermission,
			ExpiresAt: tomorrow,
		}, 302)

		tokens, err := db.GetAccessTokensByUserID(1)
		require.NoError(t, err)
		require.Len(t, tokens, 2)
	})

	t.Run("DeleteToken", func(t *testing.T) {
		s.Login(t, "thomas")
		s.Request(t, "DELETE", "/settings/access-tokens/1", nil, 302)

		tokens, err := db.GetAccessTokensByUserID(1)
		require.NoError(t, err)
		require.Len(t, tokens, 1)
		require.Equal(t, "expiring-token", tokens[0].Name)
	})
}

func TestAccessTokenPrivateGistAccess(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	_, _, user, identifier := s.CreateGist(t, "2")

	// Create access token with read permission
	token := &db.AccessToken{
		Name:      "read-token",
		UserID:    1,
		ScopeGist: db.ReadPermission,
	}
	plainToken, err := token.GenerateToken()
	require.NoError(t, err)
	require.NoError(t, token.Create())

	s.Logout()
	headers := map[string]string{"Authorization": "Token " + plainToken}

	t.Run("NoTokenReturns404", func(t *testing.T) {
		s.Request(t, "GET", "/"+user+"/"+identifier, nil, 404)
	})

	t.Run("ValidTokenGrantsAccess", func(t *testing.T) {
		s.RequestWithHeaders(t, "GET", "/"+user+"/"+identifier, nil, 200, headers)
	})

	t.Run("RawContentAccessible", func(t *testing.T) {
		s.RequestWithHeaders(t, "GET", "/"+user+"/"+identifier+"/raw/HEAD/file.txt", nil, 200, headers)
	})

	t.Run("JSONEndpointAccessible", func(t *testing.T) {
		s.RequestWithHeaders(t, "GET", "/"+user+"/"+identifier+".json", nil, 200, headers)
	})

	t.Run("InvalidTokenReturns404", func(t *testing.T) {
		s.RequestWithHeaders(t, "GET", "/"+user+"/"+identifier, nil, 404, map[string]string{
			"Authorization": "Token invalid_token",
		})
	})
}

func TestAccessTokenPermissions(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	_, _, user, identifier := s.CreateGist(t, "2")

	// Create token with NO permission
	noPermToken := &db.AccessToken{
		Name:      "no-perm-token",
		UserID:    1,
		ScopeGist: db.NoPermission,
	}
	noPermPlain, err := noPermToken.GenerateToken()
	require.NoError(t, err)
	require.NoError(t, noPermToken.Create())

	// Create token with READ permission
	readToken := &db.AccessToken{
		Name:      "read-token",
		UserID:    1,
		ScopeGist: db.ReadPermission,
	}
	readPlain, err := readToken.GenerateToken()
	require.NoError(t, err)
	require.NoError(t, readToken.Create())

	s.Logout()

	t.Run("NoPermissionDenied", func(t *testing.T) {
		s.RequestWithHeaders(t, "GET", "/"+user+"/"+identifier, nil, 404, map[string]string{
			"Authorization": "Token " + noPermPlain,
		})
	})

	t.Run("ReadPermissionGranted", func(t *testing.T) {
		s.RequestWithHeaders(t, "GET", "/"+user+"/"+identifier, nil, 200, map[string]string{
			"Authorization": "Token " + readPlain,
		})
	})
}

func TestAccessTokenExpiration(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	_, _, user, identifier := s.CreateGist(t, "2")

	// Create an expired token
	expiredToken := &db.AccessToken{
		Name:      "expired-token",
		UserID:    1,
		ScopeGist: db.ReadPermission,
		ExpiresAt: time.Now().Add(-24 * time.Hour).Unix(),
	}
	expiredPlain, err := expiredToken.GenerateToken()
	require.NoError(t, err)
	require.NoError(t, expiredToken.Create())

	// Create a valid token
	validToken := &db.AccessToken{
		Name:      "valid-token",
		UserID:    1,
		ScopeGist: db.ReadPermission,
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
	}
	validPlain, err := validToken.GenerateToken()
	require.NoError(t, err)
	require.NoError(t, validToken.Create())

	s.Logout()

	t.Run("ExpiredTokenDenied", func(t *testing.T) {
		s.RequestWithHeaders(t, "GET", "/"+user+"/"+identifier, nil, 404, map[string]string{
			"Authorization": "Token " + expiredPlain,
		})
	})

	t.Run("ValidTokenGranted", func(t *testing.T) {
		s.RequestWithHeaders(t, "GET", "/"+user+"/"+identifier, nil, 200, map[string]string{
			"Authorization": "Token " + validPlain,
		})
	})
}

func TestAccessTokenWrongUser(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	s.Register(t, "kaguya")

	_, _, user, identifier := s.CreateGist(t, "2")

	// Create token for kaguya
	kaguyaToken := &db.AccessToken{
		Name:      "kaguya-token",
		UserID:    2,
		ScopeGist: db.ReadPermission,
	}
	kaguyaPlain, err := kaguyaToken.GenerateToken()
	require.NoError(t, err)
	require.NoError(t, kaguyaToken.Create())

	// Create token for thomas
	thomasToken := &db.AccessToken{
		Name:      "thomas-token",
		UserID:    1,
		ScopeGist: db.ReadPermission,
	}
	thomasPlain, err := thomasToken.GenerateToken()
	require.NoError(t, err)
	require.NoError(t, thomasToken.Create())

	s.Logout()

	t.Run("OtherUserTokenDenied", func(t *testing.T) {
		s.RequestWithHeaders(t, "GET", "/"+user+"/"+identifier, nil, 404, map[string]string{
			"Authorization": "Token " + kaguyaPlain,
		})
	})

	t.Run("OwnerTokenGranted", func(t *testing.T) {
		s.RequestWithHeaders(t, "GET", "/"+user+"/"+identifier, nil, 200, map[string]string{
			"Authorization": "Token " + thomasPlain,
		})
	})
}

func TestAccessTokenLastUsedUpdate(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	_, _, user, identifier := s.CreateGist(t, "2")

	token := &db.AccessToken{
		Name:      "test-token",
		UserID:    1,
		ScopeGist: db.ReadPermission,
	}
	plainToken, err := token.GenerateToken()
	require.NoError(t, err)
	require.NoError(t, token.Create())

	// Verify LastUsedAt is 0 initially
	tokenFromDB, err := db.GetAccessTokenByID(token.ID)
	require.NoError(t, err)
	require.Equal(t, int64(0), tokenFromDB.LastUsedAt)

	s.Logout()

	// Use the token
	s.RequestWithHeaders(t, "GET", "/"+user+"/"+identifier, nil, 200, map[string]string{
		"Authorization": "Token " + plainToken,
	})

	// Verify LastUsedAt was updated
	tokenFromDB, err = db.GetAccessTokenByID(token.ID)
	require.NoError(t, err)
	require.NotEqual(t, int64(0), tokenFromDB.LastUsedAt)
}

func TestAccessTokenWithRequireLogin(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	_, _, user1, identifier1 := s.CreateGist(t, "2")

	s.Login(t, "thomas")
	_, _, user2, identifier2 := s.CreateGist(t, "0")

	s.Login(t, "thomas")
	token := &db.AccessToken{
		Name:      "read-token",
		UserID:    1,
		ScopeGist: db.ReadPermission,
	}
	plainToken, err := token.GenerateToken()
	require.NoError(t, err)
	require.NoError(t, token.Create())

	s.Request(t, "PUT", "/admin-panel/set-config", url.Values{"key": {db.SettingRequireLogin}, "value": {"1"}}, 200)
	s.Logout()

	headers := map[string]string{"Authorization": "Token " + plainToken}

	t.Run("UnauthenticatedRedirects", func(t *testing.T) {
		s.Request(t, "GET", "/"+user1+"/"+identifier1, nil, 302)
		s.Request(t, "GET", "/"+user2+"/"+identifier2, nil, 302)
	})

	t.Run("ValidTokenGrantsAccess", func(t *testing.T) {
		s.RequestWithHeaders(t, "GET", "/"+user1+"/"+identifier1, nil, 200, headers)
		s.RequestWithHeaders(t, "GET", "/"+user2+"/"+identifier2, nil, 200, headers)
		s.RequestWithHeaders(t, "GET", "/"+user1+"/"+identifier1+"/raw/HEAD/file.txt", nil, 200, headers)
	})

	t.Run("InvalidTokenRedirects", func(t *testing.T) {
		s.RequestWithHeaders(t, "GET", "/"+user1+"/"+identifier1, nil, 302, map[string]string{
			"Authorization": "Token invalid_token",
		})
	})

	t.Run("NoPermTokenRedirects", func(t *testing.T) {
		noPermToken := &db.AccessToken{
			Name:      "no-perm-token",
			UserID:    1,
			ScopeGist: db.NoPermission,
		}
		noPermPlain, err := noPermToken.GenerateToken()
		require.NoError(t, err)
		require.NoError(t, noPermToken.Create())

		s.RequestWithHeaders(t, "GET", "/"+user1+"/"+identifier1, nil, 302, map[string]string{
			"Authorization": "Token " + noPermPlain,
		})
	})
}
