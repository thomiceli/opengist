package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/auth/ldap"
	passwordpkg "github.com/thomiceli/opengist/internal/auth/password"
	"github.com/thomiceli/opengist/internal/config"
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
	}

	if ldap.Enabled() {
		return tryLdapLogin(username, password)
	}

	// Try GitHub token authentication if GitHub OAuth is configured
	if config.C.GithubClientKey != "" {
		// If the user was found by username and has a GithubID, try token auth directly
		if user.GithubID != "" {
			return tryGithubTokenLogin(user, password)
		}
		// If username lookup failed, try authenticating the token first,
		// then match by GitHub user ID
		return tryGithubTokenLoginByToken(password)
	}

	return nil, AuthError{"no authentication method available"}
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

func tryGithubTokenLogin(user *db.User, token string) (*db.User, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create GitHub API request: %w", err)
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot reach GitHub API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, AuthError{"invalid GitHub token"}
	}

	var ghUser struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ghUser); err != nil {
		return nil, fmt.Errorf("cannot decode GitHub API response: %w", err)
	}

	if strconv.FormatInt(ghUser.ID, 10) != user.GithubID {
		return nil, AuthError{"GitHub token does not match user"}
	}

	return user, nil
}

// tryGithubTokenLoginByToken validates a GitHub token and looks up the Opengist user
// by GitHub user ID, regardless of what username was provided.
func tryGithubTokenLoginByToken(token string) (*db.User, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create GitHub API request: %w", err)
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot reach GitHub API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, AuthError{"invalid GitHub token"}
	}

	var ghUser struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ghUser); err != nil {
		return nil, fmt.Errorf("cannot decode GitHub API response: %w", err)
	}

	ghID := strconv.FormatInt(ghUser.ID, 10)
	user, err := db.GetUserByProvider(ghID, "github")
	if err != nil {
		return nil, AuthError{"no Opengist user linked to this GitHub account"}
	}

	return user, nil
}
