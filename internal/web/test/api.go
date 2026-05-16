package test

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
)

// APIRequest sends a JSON request to /api/v1/* and asserts the status code.
// body: nil, raw JSON string, or any serializable struct/map.
// token: empty disables Authorization; otherwise sent as "Bearer <token>".
// Returns the response body bytes (already drained).
func (s *Server) APIRequest(t *testing.T, method, uri, token string, body interface{}, expectedCode int) []byte {
	var bodyReader *bytes.Reader
	switch v := body.(type) {
	case nil:
		bodyReader = bytes.NewReader(nil)
	case string:
		bodyReader = bytes.NewReader([]byte(v))
	default:
		buf, err := json.Marshal(v)
		require.NoError(t, err, "failed to marshal body")
		bodyReader = bytes.NewReader(buf)
	}

	req := httptest.NewRequest(method, uri, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	s.server.ServeHTTP(w, req)
	if expectedCode != 0 {
		require.Equalf(t, expectedCode, w.Code,
			"unexpected status for %s %s: got %d, body=%s",
			method, uri, w.Code, strings.TrimSpace(w.Body.String()))
	}
	return w.Body.Bytes()
}

// APIRequestUnmarshal calls APIRequest and unmarshals the body into out (if non-nil).
func (s *Server) APIRequestUnmarshal(t *testing.T, method, uri, token string, body, out interface{}, expectedCode int) {
	raw := s.APIRequest(t, method, uri, token, body, expectedCode)
	if out != nil && len(raw) > 0 {
		require.NoErrorf(t, json.Unmarshal(raw, out),
			"failed to unmarshal response: %s", string(raw))
	}
}

// CreateAccessToken creates an access token for the currently logged-in user
// and returns the plain token. The caller must be logged in via s.Login(...).
func (s *Server) CreateAccessToken(t *testing.T, name string, scopeGist, scopeUser uint) string {
	u := s.User()
	require.NotNil(t, u, "must be logged in to create a token")
	tok := &db.AccessToken{
		Name:      name,
		UserID:    u.ID,
		ScopeGist: scopeGist,
		ScopeUser: scopeUser,
	}
	plain, err := tok.GenerateToken()
	require.NoError(t, err)
	require.NoError(t, tok.Create())
	require.NotEmpty(t, plain)
	require.NotZero(t, tok.ID)
	return plain
}
