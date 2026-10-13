package git

import (
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
)

// handlePull authorizes a clone/pull of an existing gist before serving it.
func handlePull(ctx *context.Context, route *gitRoute, gist *db.Gist, gistExists bool, username, password string) error {
	log.Debug().Msg("Detected git pull operation")
	if !gistExists {
		log.Debug().Str("authUsername", username).Msg("Pulling unknown gist")
		return ctx.PlainText(404, "Check your credentials or make sure you have access to the Gist")
	}

	// For a non-private gist (reached here only when unauthenticated access is
	// disabled) any valid account may pull; for a private gist the password is
	// checked against the gist owner.
	userToCheck := gist.User.Username
	if gist.Private != db.PrivateVisibility {
		log.Debug().Str("authUsername", username).Msg("Pulling non-private gist with authenticated access")
		userToCheck = username
	} else {
		log.Debug().Str("authUsername", username).Str("gistOwner", gist.User.Username).Msg("Pulling private gist")
	}

	if user, err := authOrFail(ctx, userToCheck, password, db.ScopeGist, db.ReadPermission, 404, "Check your credentials or make sure you have access to the Gist"); user == nil {
		return err
	}

	log.Debug().Str("authUsername", username).Msg("Pulling gist")
	return route.handler(ctx)
}

// handlePush authorizes a push to an existing gist, or creates a new gist when
// the user pushes to /<user>/<name>.
func handlePush(ctx *context.Context, route *gitRoute, gist *db.Gist, gistExists bool, username, password string) error {
	log.Debug().Msg("Detected git push operation")

	if gistExists {
		log.Debug().Str("authUsername", username).Str("gistOwner", gist.User.Username).Msg("Pushing to existing gist")
		if user, err := authOrFail(ctx, gist.User.Username, password, db.ScopeGist, db.ReadWritePermission, 404, "Check your credentials or make sure you have access to the Gist"); user == nil {
			return err
		}

		if gist.Archived {
			log.Debug().Str("authUsername", username).Msg("Pushing to archived gist")
			return ctx.PlainText(403, "This gist is archived and is read-only")
		}

		log.Debug().Str("authUsername", username).Msg("Pushing gist")
		return route.handler(ctx)
	}

	// The gist does not exist: the user creates it by pushing to /<user>/<name>.
	log.Debug().Str("authUsername", username).Msg("Creating new gist by pushing")
	user, err := authOrFail(ctx, username, password, db.ScopeGist, db.ReadWritePermission, 404, "Check your credentials or make sure you have access to the Gist")
	if user == nil {
		return err
	}

	urlPath := ctx.Request().URL.Path
	pathParts := strings.Split(strings.Trim(urlPath, "/"), "/")
	if pathParts[0] != username || len(pathParts) != 4 {
		log.Debug().Str("authUsername", username).Any("path", pathParts).Msg("Invalid URL format for push operation")
		return ctx.PlainText(401, "Invalid URL format for push operation")
	}

	log.Debug().Str("authUsername", username).Msg("Valid URL format for push operation")
	gist, err = createGist(user, pathParts[1])
	if err != nil {
		return ctx.ErrorRes(500, "Cannot create gist", err)
	}
	log.Debug().Str("authUsername", username).Str("url", urlPath).Msg("Gist created")

	setGistContext(ctx, gist)
	return route.handler(ctx)
}
