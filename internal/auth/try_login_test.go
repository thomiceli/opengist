package auth

import (
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/config"
)

func TestLdapLoginInvalidUsername(t *testing.T) {
	require.NoError(t, config.InitConfig("", io.Discard))
	// Not a supported scheme, so connecting fails right away with a non-authentication error
	config.C.LDAPUrl = "unsupported://localhost"
	config.C.LDAPSearchFilter = "(uid=%s)"

	for _, username := range []string{".", "..", "thomas/../../other", `..\other`, "thomas/gist1"} {
		t.Run(username, func(t *testing.T) {
			_, err := tryLdapLogin(username, "password")
			var authErr AuthError
			require.True(t, errors.As(err, &authErr), "unexpected error: %v", err)
		})
	}

	// These are only checked when creating the user, after authenticating against the directory
	for _, username := range []string{"jane@corp.example", "josé", "first.last"} {
		t.Run(username, func(t *testing.T) {
			_, err := tryLdapLogin(username, "password")
			var authErr AuthError
			require.Error(t, err)
			require.False(t, errors.As(err, &authErr), "unexpected error: %v", err)
		})
	}
}
