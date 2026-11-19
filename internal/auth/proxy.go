package auth

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"net/netip"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/validator"
)

var (
	ErrProxyAuthSignupDisabled  = AuthError{"signup is disabled"}
	ErrProxyAuthInvalidUsername = AuthError{"invalid username provided by the authentication proxy"}
	ErrProxyAuthRepeatedHeader  = AuthError{"the authentication proxy header is set more than once"}
)

// proxySignupMu serializes the accounts created by proxy authentication: the
// database does not always enforce unique usernames, and the first requests of
// a new user can arrive together.
var proxySignupMu sync.Mutex

func ProxyAuthEnabled() bool {
	return config.C.ProxyAuthUserHeader != ""
}

// TryProxyAuthentication returns the user named in the proxy-auth user header
// of a request sent by a trusted reverse proxy, creating the account if it does
// not exist yet and allowSignup is true.
// It returns no user and no error when the request carries no identity to trust.
func TryProxyAuthentication(r *http.Request, allowSignup bool) (*db.User, error) {
	if !ProxyAuthEnabled() {
		return nil, nil
	}

	values := r.Header.Values(config.C.ProxyAuthUserHeader)
	if len(values) == 0 || (len(values) == 1 && strings.TrimSpace(values[0]) == "") {
		return nil, nil
	}

	// RemoteAddr is the TCP peer, unlike X-Forwarded-For which any client can set
	if !isTrustedProxy(r.RemoteAddr) {
		log.Warn().Str("header", config.C.ProxyAuthUserHeader).Msg("Ignoring proxy authentication header sent from untrusted address " + r.RemoteAddr)
		return nil, nil
	}

	// a proxy adding its header next to the one sent by the client, instead of
	// replacing it, would let the client pick the user with the first value
	if len(values) > 1 {
		log.Warn().Str("header", config.C.ProxyAuthUserHeader).Msg("Refusing proxy authentication: the header is set more than once")
		return nil, ErrProxyAuthRepeatedHeader
	}
	username := strings.TrimSpace(values[0])

	// Opengist usernames are ASCII; checking it before the case-insensitive
	// lookup keeps another name from matching an account through Unicode case
	// folding, e.g. "admİn" for "admin"
	if !isPrintableASCII(username) {
		log.Warn().Msgf("Refusing proxy authentication: invalid username %q", username)
		return nil, ErrProxyAuthInvalidUsername
	}

	if user, err := getUserByProxyUsername(username); user != nil || err != nil {
		return user, err
	}

	if !allowSignup {
		return nil, ErrProxyAuthSignupDisabled
	}

	if err := validator.NewValidator().Var(username, "max=24,alphanumdash,notreserved"); err != nil {
		log.Warn().Msgf("Cannot create user from proxy authentication header: invalid username %q", username)
		return nil, ErrProxyAuthInvalidUsername
	}

	proxySignupMu.Lock()
	defer proxySignupMu.Unlock()

	// a concurrent request may have created the account while this one waited
	if user, err := getUserByProxyUsername(username); user != nil || err != nil {
		return user, err
	}

	user := &db.User{
		Username: username,
	}

	if config.C.ProxyAuthEmailHeader != "" {
		// as for the username, a repeated header may carry a value from the client
		if emails := r.Header.Values(config.C.ProxyAuthEmailHeader); len(emails) == 1 {
			email := strings.ToLower(strings.TrimSpace(emails[0]))
			if email != "" && validator.NewValidator().Var(email, "email") == nil {
				user.Email = email
				user.MD5Hash = fmt.Sprintf("%x", md5.Sum([]byte(email)))
			}
		}
	}

	if err := user.Create(); err != nil {
		// with a unique index on usernames, as in older databases, another
		// process using the database may have created the account first
		if db.IsUniqueConstraintViolation(err) {
			if user, err := getUserByProxyUsername(username); user != nil || err != nil {
				return user, err
			}
			return nil, ErrProxyAuthInvalidUsername
		}
		log.Warn().Err(err).Msg("Cannot create user after proxy authentication")
		return nil, err
	}

	if user.ID == 1 {
		if err := user.SetAdmin(); err != nil {
			return nil, err
		}
	}

	return user, nil
}

// getUserByProxyUsername returns the oldest account named username, an ASCII
// string, ignoring ASCII case only, or no user and no error if there is none.
// The database lookup alone is looser: its collation may ignore accents
// ("élodie" for "elodie"), and the normalized username it compares lowercases a
// Kelvin sign to "k", so it can also return accounts created by LDAP with a
// non-ASCII name, which must neither match nor hide the account they look like.
func getUserByProxyUsername(username string) (*db.User, error) {
	users, err := db.GetUsersByUsername(username)
	if err != nil {
		log.Error().Err(err).Msgf("Cannot get user by username %s", username)
		return nil, err
	}
	for _, user := range users {
		// with both names ASCII, EqualFold only folds ASCII case
		if isPrintableASCII(user.Username) && strings.EqualFold(user.Username, username) {
			return user, nil
		}
	}
	return nil, nil
}

func isPrintableASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '!' || s[i] > '~' {
			return false
		}
	}
	return true
}

func isTrustedProxy(remoteAddr string) bool {
	addrPort, err := netip.ParseAddrPort(remoteAddr)
	if err != nil {
		// e.g. a Unix socket peer, which has no address
		return false
	}
	addr := addrPort.Addr().Unmap().WithZone("")

	networks, err := config.C.ProxyAuthTrustedNetworks()
	if err != nil {
		log.Error().Err(err).Msg("Cannot parse proxy-auth.trusted-cidrs")
		return false
	}

	for _, network := range networks {
		if network.Contains(addr) {
			return true
		}
	}
	return false
}
