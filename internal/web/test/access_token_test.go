package test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
)

func TestAccessTokensCRUD(t *testing.T) {
	s := Setup(t)
	defer Teardown(t, s)

	// Register and login
	user1 := db.UserDTO{Username: "thomas", Password: "thomas"}
	register(t, s, user1)

	// Access tokens page requires login
	s.sessionCookie = ""
	err := s.Request("GET", "/settings/access-tokens", nil, 302)
	require.NoError(t, err)

	login(t, s, user1)

	// Access tokens page
	err = s.Request("GET", "/settings/access-tokens", nil, 200)
	require.NoError(t, err)

	// Create a token with read permission
	tokenDTO := db.AccessTokenDTO{
		Name:      "test-token",
		ScopeGist: db.ReadPermission,
	}
	err = s.Request("POST", "/settings/access-tokens", tokenDTO, 302)
	require.NoError(t, err)

	// Verify token was created in database
	tokens, err := db.GetAccessTokensByUserID(1)
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	require.Equal(t, "test-token", tokens[0].Name)
	require.Equal(t, uint(db.ReadPermission), tokens[0].ScopeGist)
	require.Equal(t, int64(0), tokens[0].ExpiresAt)

	// Create another token with expiration
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	tokenDTO2 := db.AccessTokenDTO{
		Name:      "expiring-token",
		ScopeGist: db.ReadWritePermission,
		ExpiresAt: tomorrow,
	}
	err = s.Request("POST", "/settings/access-tokens", tokenDTO2, 302)
	require.NoError(t, err)

	tokens, err = db.GetAccessTokensByUserID(1)
	require.NoError(t, err)
	require.Len(t, tokens, 2)

	// Delete the first token
	err = s.Request("DELETE", "/settings/access-tokens/1", nil, 302)
	require.NoError(t, err)

	tokens, err = db.GetAccessTokensByUserID(1)
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	require.Equal(t, "expiring-token", tokens[0].Name)
}

func TestAccessTokenPrivateGistAccess(t *testing.T) {
	s := Setup(t)
	defer Teardown(t, s)

	// Register user and create a private gist
	user1 := db.UserDTO{Username: "thomas", Password: "thomas"}
	register(t, s, user1)

	gist1 := db.GistDTO{
		Title:       "private-gist",
		Description: "my private gist",
		VisibilityDTO: db.VisibilityDTO{
			Private: db.PrivateVisibility,
		},
		Name:    []string{"secret.txt"},
		Content: []string{"secret content"},
	}
	err := s.Request("POST", "/", gist1, 302)
	require.NoError(t, err)

	gist1db, err := db.GetGistByID("1")
	require.NoError(t, err)

	// Create access token with read permission
	token := &db.AccessToken{
		Name:      "read-token",
		UserID:    1,
		ScopeGist: db.ReadPermission,
	}
	plainToken, err := token.GenerateToken()
	require.NoError(t, err)
	err = token.Create()
	require.NoError(t, err)

	// Clear session - simulate unauthenticated request
	s.sessionCookie = ""

	// Without token, private gist should return 404
	err = s.Request("GET", "/thomas/"+gist1db.Uuid, nil, 404)
	require.NoError(t, err)

	// With valid token, private gist should be accessible
	headers := map[string]string{"Authorization": "Token " + plainToken}
	err = s.RequestWithHeaders("GET", "/thomas/"+gist1db.Uuid, nil, 200, headers)
	require.NoError(t, err)

	// Raw content should also be accessible with token
	err = s.RequestWithHeaders("GET", "/thomas/"+gist1db.Uuid+"/raw/HEAD/secret.txt", nil, 200, headers)
	require.NoError(t, err)

	// JSON endpoint should also be accessible with token
	err = s.RequestWithHeaders("GET", "/thomas/"+gist1db.Uuid+".json", nil, 200, headers)
	require.NoError(t, err)

	// Invalid token should not work
	invalidHeaders := map[string]string{"Authorization": "Token invalid_token"}
	err = s.RequestWithHeaders("GET", "/thomas/"+gist1db.Uuid, nil, 404, invalidHeaders)
	require.NoError(t, err)
}

func TestAccessTokenPermissions(t *testing.T) {
	s := Setup(t)
	defer Teardown(t, s)

	// Register user and create a private gist
	user1 := db.UserDTO{Username: "thomas", Password: "thomas"}
	register(t, s, user1)

	gist1 := db.GistDTO{
		Title:       "private-gist",
		Description: "my private gist",
		VisibilityDTO: db.VisibilityDTO{
			Private: db.PrivateVisibility,
		},
		Name:    []string{"file.txt"},
		Content: []string{"content"},
	}
	err := s.Request("POST", "/", gist1, 302)
	require.NoError(t, err)

	gist1db, err := db.GetGistByID("1")
	require.NoError(t, err)

	// Create token with NO permission
	noPermToken := &db.AccessToken{
		Name:      "no-perm-token",
		UserID:    1,
		ScopeGist: db.NoPermission,
	}
	noPermPlain, err := noPermToken.GenerateToken()
	require.NoError(t, err)
	err = noPermToken.Create()
	require.NoError(t, err)

	// Create token with READ permission
	readToken := &db.AccessToken{
		Name:      "read-token",
		UserID:    1,
		ScopeGist: db.ReadPermission,
	}
	readPlain, err := readToken.GenerateToken()
	require.NoError(t, err)
	err = readToken.Create()
	require.NoError(t, err)

	s.sessionCookie = ""

	// No permission token should not grant access
	noPermHeaders := map[string]string{"Authorization": "Token " + noPermPlain}
	err = s.RequestWithHeaders("GET", "/thomas/"+gist1db.Uuid, nil, 404, noPermHeaders)
	require.NoError(t, err)

	// Read permission token should grant access
	readHeaders := map[string]string{"Authorization": "Token " + readPlain}
	err = s.RequestWithHeaders("GET", "/thomas/"+gist1db.Uuid, nil, 200, readHeaders)
	require.NoError(t, err)
}

func TestAccessTokenExpiration(t *testing.T) {
	s := Setup(t)
	defer Teardown(t, s)

	// Register user and create a private gist
	user1 := db.UserDTO{Username: "thomas", Password: "thomas"}
	register(t, s, user1)

	gist1 := db.GistDTO{
		Title:       "private-gist",
		Description: "my private gist",
		VisibilityDTO: db.VisibilityDTO{
			Private: db.PrivateVisibility,
		},
		Name:    []string{"file.txt"},
		Content: []string{"content"},
	}
	err := s.Request("POST", "/", gist1, 302)
	require.NoError(t, err)

	gist1db, err := db.GetGistByID("1")
	require.NoError(t, err)

	// Create an expired token
	expiredToken := &db.AccessToken{
		Name:      "expired-token",
		UserID:    1,
		ScopeGist: db.ReadPermission,
		ExpiresAt: time.Now().Add(-24 * time.Hour).Unix(), // Expired yesterday
	}
	expiredPlain, err := expiredToken.GenerateToken()
	require.NoError(t, err)
	err = expiredToken.Create()
	require.NoError(t, err)

	// Create a valid (non-expired) token
	validToken := &db.AccessToken{
		Name:      "valid-token",
		UserID:    1,
		ScopeGist: db.ReadPermission,
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(), // Expires tomorrow
	}
	validPlain, err := validToken.GenerateToken()
	require.NoError(t, err)
	err = validToken.Create()
	require.NoError(t, err)

	s.sessionCookie = ""

	// Expired token should not grant access
	expiredHeaders := map[string]string{"Authorization": "Token " + expiredPlain}
	err = s.RequestWithHeaders("GET", "/thomas/"+gist1db.Uuid, nil, 404, expiredHeaders)
	require.NoError(t, err)

	// Valid token should grant access
	validHeaders := map[string]string{"Authorization": "Token " + validPlain}
	err = s.RequestWithHeaders("GET", "/thomas/"+gist1db.Uuid, nil, 200, validHeaders)
	require.NoError(t, err)
}

func TestAccessTokenWrongUser(t *testing.T) {
	s := Setup(t)
	defer Teardown(t, s)

	// Register two users
	user1 := db.UserDTO{Username: "thomas", Password: "thomas"}
	register(t, s, user1)

	// Create a private gist for user1
	gist1 := db.GistDTO{
		Title:       "thomas-private-gist",
		Description: "thomas private gist",
		VisibilityDTO: db.VisibilityDTO{
			Private: db.PrivateVisibility,
		},
		Name:    []string{"file.txt"},
		Content: []string{"content"},
	}
	err := s.Request("POST", "/", gist1, 302)
	require.NoError(t, err)

	gist1db, err := db.GetGistByID("1")
	require.NoError(t, err)

	s.sessionCookie = ""
	user2 := db.UserDTO{Username: "kaguya", Password: "kaguya"}
	register(t, s, user2)

	// Create token for user2
	user2Token := &db.AccessToken{
		Name:      "kaguya-token",
		UserID:    2,
		ScopeGist: db.ReadPermission,
	}
	user2Plain, err := user2Token.GenerateToken()
	require.NoError(t, err)
	err = user2Token.Create()
	require.NoError(t, err)

	s.sessionCookie = ""

	// User2's token should NOT grant access to user1's private gist
	user2Headers := map[string]string{"Authorization": "Token " + user2Plain}
	err = s.RequestWithHeaders("GET", "/thomas/"+gist1db.Uuid, nil, 404, user2Headers)
	require.NoError(t, err)

	// Create token for user1
	user1Token := &db.AccessToken{
		Name:      "thomas-token",
		UserID:    1,
		ScopeGist: db.ReadPermission,
	}
	user1Plain, err := user1Token.GenerateToken()
	require.NoError(t, err)
	err = user1Token.Create()
	require.NoError(t, err)

	// User1's token SHOULD grant access to user1's private gist
	user1Headers := map[string]string{"Authorization": "Token " + user1Plain}
	err = s.RequestWithHeaders("GET", "/thomas/"+gist1db.Uuid, nil, 200, user1Headers)
	require.NoError(t, err)
}

func TestAccessTokenLastUsedUpdate(t *testing.T) {
	s := Setup(t)
	defer Teardown(t, s)

	// Register user and create a private gist
	user1 := db.UserDTO{Username: "thomas", Password: "thomas"}
	register(t, s, user1)

	gist1 := db.GistDTO{
		Title:       "private-gist",
		Description: "my private gist",
		VisibilityDTO: db.VisibilityDTO{
			Private: db.PrivateVisibility,
		},
		Name:    []string{"file.txt"},
		Content: []string{"content"},
	}
	err := s.Request("POST", "/", gist1, 302)
	require.NoError(t, err)

	gist1db, err := db.GetGistByID("1")
	require.NoError(t, err)

	// Create token
	token := &db.AccessToken{
		Name:      "test-token",
		UserID:    1,
		ScopeGist: db.ReadPermission,
	}
	plainToken, err := token.GenerateToken()
	require.NoError(t, err)
	err = token.Create()
	require.NoError(t, err)

	// Verify LastUsedAt is 0 initially
	tokenFromDB, err := db.GetAccessTokenByID(token.ID)
	require.NoError(t, err)
	require.Equal(t, int64(0), tokenFromDB.LastUsedAt)

	s.sessionCookie = ""

	// Use the token
	headers := map[string]string{"Authorization": "Token " + plainToken}
	err = s.RequestWithHeaders("GET", "/thomas/"+gist1db.Uuid, nil, 200, headers)
	require.NoError(t, err)

	// Verify LastUsedAt was updated
	tokenFromDB, err = db.GetAccessTokenByID(token.ID)
	require.NoError(t, err)
	require.NotEqual(t, int64(0), tokenFromDB.LastUsedAt)
}
