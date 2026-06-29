package git

import (
	"encoding/base64"
	"errors"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/auth"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
)

// authOrFail authenticates the given credentials. The credential is tried as a
// password (DB or LDAP) first, then as an access token with the required scope
// and permission — this lets SSO/passwordless users authenticate over Git HTTP.
// On success it returns the user. On failure it writes the appropriate HTTP
// response and returns a nil user, so callers stop with:
//
//	user, err := authOrFail(...)
//	if user == nil {
//		return err
//	}
//
// `err` is nil for an already-written invalid-credentials response and a
// renderable error for an internal authentication failure.
func authOrFail(ctx *context.Context, username, password string, scope, permission uint, invalidCode int, invalidMsg string) (*db.User, error) {
	user, err := auth.TryAuthentication(username, password)
	if err == nil {
		return user, nil
	}

	var authErr auth.AuthError
	if !errors.As(err, &authErr) {
		return nil, ctx.ErrorRes(500, "Authentication system error", nil)
	}

	// Fall back to access token authentication.
	user, token, tokenErr := auth.TryAuthenticationWithAccessToken(username, password, scope, permission)
	if tokenErr != nil {
		log.Warn().Msg("Invalid HTTP authentication attempt from " + ctx.RealIP())
		return nil, ctx.PlainText(invalidCode, invalidMsg)
	}
	_ = token.UpdateLastUsed()

	return user, nil
}

func basicAuth(ctx *context.Context) error {
	ctx.Response().Header().Set("WWW-Authenticate", `Basic realm="."`)
	return ctx.PlainText(401, "Requires authentication")
}

func parseAuthHeader(ctx *context.Context) (string, string, error) {
	authHeader := ctx.Request().Header.Get("Authorization")
	if authHeader == "" {
		return "", "", errors.New("no auth header")
	}

	authFields := strings.Fields(authHeader)
	if len(authFields) != 2 || authFields[0] != "Basic" {
		return "", "", errors.New("invalid auth header")
	}

	authUsername, authPassword, err := basicAuthDecode(authFields[1])
	if err != nil {
		log.Error().Err(err).Msg("Cannot decode basic auth header")
		return "", "", err
	}

	return authUsername, authPassword, nil
}

func basicAuthDecode(encoded string) (string, string, error) {
	s, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", "", err
	}

	auth := strings.SplitN(string(s), ":", 2)
	return auth[0], auth[1], nil
}
