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

func TestApiAuth_MissingToken(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)
	s.Register(t, "thomas")
	s.Login(t, "thomas")
	require.NoError(t, db.UpdateSetting(db.SettingApiEnabled, "1"))

	var body map[string]string
	s.APIRequestUnmarshal(t, "GET", "/api/v1/user", "", nil, &body, 401)
	require.Equal(t, "unauthorized", body["code"])
}

func TestApiAuth_ApiDisabled(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)
	s.Register(t, "thomas")
	s.Login(t, "thomas")
	tok := s.CreateAccessToken(t, "t", db.ReadPermission, db.ReadPermission)
	require.NoError(t, db.UpdateSetting(db.SettingApiEnabled, "0"))

	var body map[string]string
	s.APIRequestUnmarshal(t, "GET", "/api/v1/user", tok, nil, &body, 503)
	require.Equal(t, "api_disabled", body["code"])
	require.Contains(t, body["hint"], "/admin-panel/configuration")
}

func TestApiAuth_BearerAndTokenPrefix(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)
	s.Register(t, "thomas")
	s.Login(t, "thomas")
	tok := s.CreateAccessToken(t, "t", db.ReadPermission, db.ReadPermission)
	require.NoError(t, db.UpdateSetting(db.SettingApiEnabled, "1"))

	// Bearer
	s.APIRequest(t, "GET", "/api/v1/user", tok, nil, 200)

	// Token prefix (legacy)
	req := newJSONReqWithAuth("GET", "/api/v1/user", "Token "+tok)
	resp := s.RawRequest(t, req, 200)
	_ = json.NewDecoder(resp.Body).Decode(&map[string]interface{}{})
}

func TestApiAuth_ExpiredToken(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)
	s.Register(t, "thomas")
	s.Login(t, "thomas")
	tok := s.CreateAccessToken(t, "t", db.ReadPermission, db.ReadPermission)
	require.NoError(t, db.UpdateSetting(db.SettingApiEnabled, "1"))

	// Force the token to be expired.
	all, _ := db.GetAccessTokensByUserID(s.User().ID)
	require.Len(t, all, 1)
	all[0].ExpiresAt = 1
	require.NoError(t, db.SaveAccessTokenForTest(all[0]))

	var body map[string]string
	s.APIRequestUnmarshal(t, "GET", "/api/v1/user", tok, nil, &body, 401)
	require.Equal(t, "unauthorized", body["code"])
}

func newJSONReqWithAuth(method, uri, authHeader string) *http.Request {
	req := httptest.NewRequest(method, uri, strings.NewReader(""))
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Accept", "application/json")
	return req
}

func TestApiScope_GistReadInsufficient(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)
	s.Register(t, "thomas")
	s.Login(t, "thomas")
	tok := s.CreateAccessToken(t, "no-gist", db.NoPermission, db.ReadPermission)
	require.NoError(t, db.UpdateSetting(db.SettingApiEnabled, "1"))

	var body map[string]string
	s.APIRequestUnmarshal(t, "GET", "/api/v1/gists", tok, nil, &body, 403)
	require.Equal(t, "forbidden", body["code"])
}

func TestApiScope_UserReadInsufficient(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)
	s.Register(t, "thomas")
	s.Login(t, "thomas")
	tok := s.CreateAccessToken(t, "no-user", db.ReadPermission, db.NoPermission)
	require.NoError(t, db.UpdateSetting(db.SettingApiEnabled, "1"))

	var body map[string]string
	s.APIRequestUnmarshal(t, "GET", "/api/v1/user", tok, nil, &body, 403)
	require.Equal(t, "forbidden", body["code"])
}
