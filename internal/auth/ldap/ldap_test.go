package ldap

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/config"
)

func setupConfig(t *testing.T) {
	require.NoError(t, config.InitConfig("", io.Discard))
	// Nothing listens there, so any connection attempt fails
	config.C.LDAPUrl = "ldap://127.0.0.1:1"
	config.C.LDAPSearchFilter = "(&(objectCategory=Person)(sAMAccountName=%s))"
}

func TestSearchFilterEscapesUsername(t *testing.T) {
	setupConfig(t)

	for username, expected := range map[string]string{
		"thomas":                  `(&(objectCategory=Person)(sAMAccountName=thomas))`,
		"first.last":              `(&(objectCategory=Person)(sAMAccountName=first.last))`,
		"*":                       `(&(objectCategory=Person)(sAMAccountName=\2a))`,
		"th*":                     `(&(objectCategory=Person)(sAMAccountName=th\2a))`,
		"x*)(sAMAccountName=jdoe": `(&(objectCategory=Person)(sAMAccountName=x\2a\29\28sAMAccountName=jdoe))`,
		`domain\user`:             `(&(objectCategory=Person)(sAMAccountName=domain\5cuser))`,
	} {
		t.Run(username, func(t *testing.T) {
			require.Equal(t, expected, searchFilter(username))
		})
	}
}

func TestAuthenticateEmptyPassword(t *testing.T) {
	setupConfig(t)

	ok, err := Authenticate("thomas", "")
	require.NoError(t, err)
	require.False(t, ok)
}
