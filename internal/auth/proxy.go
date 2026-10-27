package auth

import (
	"crypto/md5"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/validator"
	"gorm.io/gorm"
)

var (
	ErrProxyAuthSignupDisabled  = AuthError{"signup is disabled"}
	ErrProxyAuthInvalidUsername = AuthError{"invalid username provided by the authentication proxy"}
)

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

	username := strings.TrimSpace(r.Header.Get(config.C.ProxyAuthUserHeader))
	if username == "" {
		return nil, nil
	}

	// RemoteAddr is the TCP peer, unlike X-Forwarded-For which any client can set
	if !isTrustedProxy(r.RemoteAddr) {
		log.Warn().Str("header", config.C.ProxyAuthUserHeader).Msg("Ignoring proxy authentication header sent from untrusted address " + r.RemoteAddr)
		return nil, nil
	}

	user, err := db.GetUserByUsername(username)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Error().Err(err).Msgf("Cannot get user by username %s", username)
		return nil, err
	}

	if !allowSignup {
		return nil, ErrProxyAuthSignupDisabled
	}

	if err = validator.NewValidator().Var(username, "max=24,alphanumdash,notreserved"); err != nil {
		log.Warn().Msgf("Cannot create user from proxy authentication header: invalid username %q", username)
		return nil, ErrProxyAuthInvalidUsername
	}

	user = &db.User{
		Username: username,
	}

	if config.C.ProxyAuthEmailHeader != "" {
		email := strings.TrimSpace(r.Header.Get(config.C.ProxyAuthEmailHeader))
		if email != "" && validator.NewValidator().Var(email, "email") == nil {
			user.Email = email
			user.MD5Hash = fmt.Sprintf("%x", md5.Sum([]byte(strings.ToLower(email))))
		}
	}

	if err = user.Create(); err != nil {
		// a concurrent request for the same new user may have created it first
		if db.IsUniqueConstraintViolation(err) {
			return db.GetUserByUsername(username)
		}
		log.Warn().Err(err).Msg("Cannot create user after proxy authentication")
		return nil, err
	}

	if user.ID == 1 {
		if err = user.SetAdmin(); err != nil {
			return nil, err
		}
	}

	return user, nil
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
