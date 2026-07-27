package validator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReservedUsernames(t *testing.T) {
	v := NewValidator()

	for _, name := range []string{
		"register",
		"login",
		"logout",
		"settings",
		"admin-panel",
		"all",
		"search",
	} {
		t.Run("allowed "+name, func(t *testing.T) {
			require.NoError(t, v.Var(name, "notreserved"))
		})
	}

	for _, name := range []string{
		"assets",
		"init",
		"healthcheck",
		"preview",
		"metrics",
		"mfa",
		"webauthn",
		"oauth",
	} {
		t.Run("reserved "+name, func(t *testing.T) {
			require.Error(t, v.Var(name, "notreserved"))
		})
	}
}

func TestUsernameRequiresAlphanumericCharacter(t *testing.T) {
	v := NewValidator()

	for _, name := range []string{"-", "---"} {
		require.Error(t, v.Var(name, "alphanumdash"))
	}
	for _, name := range []string{"-", "---", "_", "-_-"} {
		require.Error(t, v.Var(name, "alphanumdashunder"))
	}

	require.NoError(t, v.Var("user-name", "alphanumdash"))
	require.NoError(t, v.Var("user_name", "alphanumdashunder"))
}
