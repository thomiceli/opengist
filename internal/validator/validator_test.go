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

func TestLdapUsername(t *testing.T) {
	v := NewValidator()

	for _, name := range []string{
		"thomas", "first.last", "first_last", "first-last", ".hidden",
		"jane@corp.example", "jane+tag@corp.example", "o'brien@corp.example",
		"josé", "Müller", "Łukasz", "张伟",
		// e followed by U+0301 combining acute accent
		"jose\xcc\x81",
	} {
		require.NoError(t, v.Var(name, "ldapusername"), "%q", name)
	}

	for _, name := range []string{
		"", " ", "\t", ".", "..", "...", "thomas.", "-", "_", "@", "-_.@",
		"a/b", "../a", `a\b`, "a:b", "a?b", "a#b", "a%41",
		"a b", " thomas", "thomas ", "a\tb", "a\nb", "a\x00b", "a\x7fb", "a\xffb",
		// U+0085 next line, U+00A0 no-break space, U+3000 ideographic space,
		// U+200B zero width space and U+202E right-to-left override
		"a\xc2\x85b", "a\xc2\xa0b", "a\xe3\x80\x80b", "a\xe2\x80\x8bb", "a\xe2\x80\xaeb",
	} {
		require.Error(t, v.Var(name, "ldapusername"), "%q", name)
	}
}
