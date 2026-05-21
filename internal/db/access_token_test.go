package db_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
)

func TestAccessTokenScopeUser(t *testing.T) {
	tok := &db.AccessToken{ScopeUser: db.ReadPermission}
	require.True(t, tok.HasUserReadPermission(), "ScopeUser=1 should grant user read")

	tokNone := &db.AccessToken{ScopeUser: 0}
	require.False(t, tokNone.HasUserReadPermission(), "ScopeUser=0 should not grant user read")
}

func TestAccessTokenDTOScopeUser(t *testing.T) {
	dto := &db.AccessTokenDTO{Name: "t", ScopeGist: 1, ScopeUser: 1}
	tok := dto.ToAccessToken()
	require.Equal(t, uint(1), tok.ScopeUser)
	require.Equal(t, uint(1), tok.ScopeGist)
}
