package v1_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

// apiError mirrors the JSON error envelope produced by context.HTTPError:
// {"message": "...", "status": <code>}.
type apiError struct {
	Message string `json:"message"`
	Status  int    `json:"status"`
}

func TestApiAuth_MissingToken(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)
	s.Register(t, "thomas")
	s.Login(t, "thomas")

	var body apiError
	s.APIRequestUnmarshal(t, "GET", "/api/user", "", nil, &body, 401)
	require.Equal(t, 401, body.Status)
	require.NotEmpty(t, body.Message)
}

func TestApiAuth_BearerAndTokenPrefix(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)
	s.Register(t, "thomas")
	s.Login(t, "thomas")
	tok := s.CreateAccessToken(t, "t", db.ReadPermission, db.ReadPermission)

	// Bearer
	s.APIRequest(t, "GET", "/api/user", tok, nil, 200)

	// Token prefix (legacy)
	req := newJSONReqWithAuth("GET", "/api/user", "Token "+tok)
	resp := s.RawRequest(t, req, 200)
	_ = json.NewDecoder(resp.Body).Decode(&map[string]interface{}{})
}

func TestApiAuth_ExpiredToken(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)
	s.Register(t, "thomas")
	s.Login(t, "thomas")
	tok := s.CreateAccessToken(t, "t", db.ReadPermission, db.ReadPermission)

	// Force the token to be expired.
	all, _ := db.GetAccessTokensByUserID(s.User().ID)
	require.Len(t, all, 1)
	all[0].ExpiresAt = 1
	require.NoError(t, db.SaveAccessTokenForTest(all[0]))

	var body apiError
	s.APIRequestUnmarshal(t, "GET", "/api/user", tok, nil, &body, 401)
	require.Equal(t, 401, body.Status)
	require.NotEmpty(t, body.Message)
}

func newJSONReqWithAuth(method, uri, authHeader string) *http.Request {
	req := httptest.NewRequest(method, uri, strings.NewReader(""))
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Accept", "application/json")
	return req
}

// TestApiScope_GistWriteInsufficient - a token without gist:write is rejected
// with 403 on a write endpoint. (Read endpoints are soft-scoped and return the
// public subset instead of 403, so the hard-scope check lives on a write route.)
func TestApiScope_GistWriteInsufficient(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)
	s.Register(t, "thomas")
	s.Login(t, "thomas")
	tok := s.CreateAccessToken(t, "no-write", db.ReadPermission, db.ReadPermission)

	var body apiError
	s.APIRequestUnmarshal(t, "POST", "/api/gists", tok, nil, &body, 403)
	require.Equal(t, 403, body.Status)
	require.NotEmpty(t, body.Message)
}

// TestApiScope_UserReadInsufficient - /user is a hard-scoped private resource,
// so a token lacking user:read gets 403 (not a reduced response).
func TestApiScope_UserReadInsufficient(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)
	s.Register(t, "thomas")
	s.Login(t, "thomas")
	tok := s.CreateAccessToken(t, "no-user", db.ReadPermission, db.NoPermission)

	var body apiError
	s.APIRequestUnmarshal(t, "GET", "/api/user", tok, nil, &body, 403)
	require.Equal(t, 403, body.Status)
	require.NotEmpty(t, body.Message)
}
