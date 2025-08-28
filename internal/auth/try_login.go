package auth

import (
	"errors"

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
