package auth

import (
	"errors"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/auth/ldap"
	passwordpkg "github.com/thomiceli/opengist/internal/auth/password"
	"github.com/thomiceli/opengist/internal/db"
	"gorm.io/gorm"
)

type AuthError struct {
	message string
}

func (e AuthError) Error() string {
	return e.message
}

func TryAuthentication(username, password string) (*db.User, error) {
	user, err := db.GetUserByUsername(username)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Error().Err(err).Msgf("Cannot get user by username %s", username)
			return nil, err
		}
	}

	if user.Password != "" {
		return tryDbLogin(user, password)
	} else {
		if ldap.Enabled() {
			return tryLdapLogin(username, password)
		}
		return nil, AuthError{"no authentication method available"}
	}
}

// TryAuthenticationWithAccessToken attempts to authenticate using a plain-text access token.
// It verifies the token belongs to the given username, is not expired, and has the required scope/permission.
func TryAuthenticationWithAccessToken(username, plainToken string, scope, permission uint) (*db.User, *db.AccessToken, error) {
	if !strings.HasPrefix(plainToken, "og_") {
		return nil, nil, AuthError{"not an access token"}
	}

	token, err := db.GetAccessTokenByToken(plainToken)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, AuthError{"invalid access token"}
		}
		log.Error().Err(err).Msg("Cannot get access token")
		return nil, nil, err
	}

	if token.IsExpired() {
		return nil, nil, AuthError{"access token is expired"}
	}

	if token.User.Username != username {
		return nil, nil, AuthError{"access token does not belong to user"}
	}

	if !token.CheckForPermission(scope, permission) {
		return nil, nil, AuthError{"access token does not have required permission"}
	}

	return &token.User, token, nil
}

func tryDbLogin(user *db.User, password string) (*db.User, error) {
	if ok, err := passwordpkg.VerifyPassword(password, user.Password); !ok {
		if err != nil {
			log.Error().Err(err).Msg("Password verification failed")
			return nil, err
		}
		return nil, AuthError{"invalid password"}
	}

	return user, nil
}

func tryLdapLogin(username, password string) (user *db.User, err error) {
	ok, err := ldap.Authenticate(username, password)
	if err != nil {
		log.Error().Err(err).Msg("LDAP authentication failed")
		return nil, err
	}

	if !ok {
		return nil, AuthError{"invalid LDAP credentials"}
	}

	if user, err = db.GetUserByUsername(username); err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Error().Err(err).Msgf("Cannot get user by username %s", username)
			return nil, err
		}
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		user = &db.User{
			Username: username,
		}
		if err = user.Create(); err != nil {
			log.Warn().Err(err).Msg("Cannot create user after LDAP authentication")
			return nil, err
		}

		return user, nil
	}

	return user, nil
}
