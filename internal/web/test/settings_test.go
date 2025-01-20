package test

import (
	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	"testing"
)

func TestSettingsPage(t *testing.T) {
	s := Setup(t)
	defer Teardown(t, s)

	err := s.Request("GET", "/settings", nil, 302)
	require.NoError(t, err)

	user1 := db.UserDTO{Username: "thomas", Password: "thomas"}
	register(t, s, user1)
	login(t, s, user1)

	err = s.Request("GET", "/settings", nil, 200)
	require.NoError(t, err)
}
