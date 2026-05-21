package v1_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	v1 "github.com/thomiceli/opengist/internal/web/handlers/api/v1"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

func TestGetUser(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)
	// admin is the first user (ID=1), thomas is a regular user
	s.Register(t, "admin")
	s.Register(t, "thomas")
	s.Login(t, "thomas")
	tok := s.CreateAccessToken(t, "t", db.ReadPermission, db.ReadPermission)
	require.NoError(t, db.UpdateSetting(db.SettingApiEnabled, "1"))

	var resp v1.UserResponse
	s.APIRequestUnmarshal(t, "GET", "/api/v1/user", tok, nil, &resp, 200)
	require.Equal(t, "thomas", resp.Username)
	require.NotZero(t, resp.ID)
	require.False(t, resp.IsAdmin)
	require.False(t, resp.CreatedAt.IsZero())
}
