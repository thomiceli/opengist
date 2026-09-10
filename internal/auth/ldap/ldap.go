package ldap

import (
	"fmt"
	"github.com/go-ldap/ldap/v3"
	"github.com/thomiceli/opengist/internal/config"
)

func Enabled() bool {
	return config.C.LDAPUrl != ""
}

// bindService binds the connection as the configured service account, or anonymously
// when no bind DN is configured, for LDAP servers allowing anonymous queries.
func bindService(l *ldap.Conn) error {
	if config.C.LDAPBindDn == "" {
		return l.UnauthenticatedBind("")
	}
	return l.Bind(config.C.LDAPBindDn, config.C.LDAPBindCredentials)
}

// Authenticate attempts to authenticate a user against the configured LDAP instance.
func Authenticate(username, password string) (bool, error) {
	l, err := ldap.DialURL(config.C.LDAPUrl)
	if err != nil {
		return false, fmt.Errorf("unable to connect to URI: %v", config.C.LDAPUrl)
	}
	defer func(l *ldap.Conn) {
		_ = l.Close()
	}(l)

	// First bind with a read only user
	err = bindService(l)
	if err != nil {
		return false, err
	}

	searchFilter := fmt.Sprintf(config.C.LDAPSearchFilter, username)
	searchRequest := ldap.NewSearchRequest(
		config.C.LDAPSearchBase,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		searchFilter,
		[]string{"dn"},
		nil,
	)

	sr, err := l.Search(searchRequest)
	if err != nil {
		return false, err
	}

	if len(sr.Entries) != 1 {
		return false, nil
	}

	// Bind as the user to verify their password
	err = l.Bind(sr.Entries[0].DN, password)
	if err != nil {
		return false, nil
	}

	// Rebind as the read only user for any further queries
	err = bindService(l)
	if err != nil {
		return false, err
	}

	return true, nil
}
