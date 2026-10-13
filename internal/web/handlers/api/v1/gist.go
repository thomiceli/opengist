package v1

import (
	"regexp"

	"gorm.io/gorm"

	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
)

// reCommitSHA matches a partial or full git SHA-1 - hex only, 4–40 chars.
// Used at the API boundary to keep user input from reaching `git log` as
// something that could parse as an option (e.g. "--all", "-p"). git itself
// has no clean "end of options" marker on the log command, so the cheapest
// robust defense is to forbid anything non-hex up front.
var reCommitSHA = regexp.MustCompile(`^[0-9a-fA-F]{4,40}$`)

// lookupGistByUUID fetches a gist by UUID and enforces visibility. The caller
// is treated as "able to see private gists" only when they're the owner AND
// their token holds gist:read (or there's no token at all, in which case the
// gist isn't private to start with). Returns gorm.ErrRecordNotFound for the
// hidden case so the handler can map it to a 404 without leaking existence.
func lookupGistByUUID(ctx *context.Context, uuid string) (*db.Gist, error) {
	g, err := db.GetGistByUUID(uuid)
	if err != nil {
		return nil, err
	}
	if g.Private == db.PrivateVisibility {
		tok, _ := ctx.GetData("accessToken").(*db.AccessToken)
		isOwnerWithScope := ctx.User != nil &&
			ctx.User.ID == g.UserID &&
			tok != nil && tok.HasGistReadPermission()
		if !isOwnerWithScope {
			return nil, gorm.ErrRecordNotFound
		}
	}
	return g, nil
}

// GetGist handles GET /api/gists/:uuid.
// Public and unlisted gists are readable by anyone (including anonymous
// callers), matching the rest of the API's soft-scope rule. Private gists
// only resolve for their owner with a
// gist:read token - every other caller sees a 404 (existence hidden).
func GetGist(ctx *context.Context) error {
	g, err := lookupGistByUUID(ctx, ctx.Param("uuid"))
	if err != nil {
		return ctx.ErrorJson(404, "Gist not found", nil)
	}
	resp, err := g.ToAPI(apiBaseURL(ctx), "HEAD")
	if err != nil {
		return ctx.ErrorJson(500, "failed to serialize gist", err)
	}
	return ctx.JSON(200, resp)
}

// GetGistRevision handles GET /api/gists/:uuid/:sha.
// Same shape as GetGist, but returns the gist as it stood at the given
// commit SHA instead of HEAD. Visibility rules are identical (lookup-side
// check). An unknown revision surfaces as 404 - same code as the not-found
// case, so we don't leak which commits exist on the gist.
func GetGistRevision(ctx *context.Context) error {
	// Gist visibility check first - if the caller can't even see the gist,
	// they get "Gist not found" rather than a misleading "Revision not
	// found" or 400.
	g, err := lookupGistByUUID(ctx, ctx.Param("uuid"))
	if err != nil {
		return ctx.ErrorJson(404, "Gist not found", nil)
	}

	sha := ctx.Param("sha")
	if !reCommitSHA.MatchString(sha) {
		// Malformed input → 400. Also keeps `--all`-style values from ever
		// reaching git as a flag.
		return ctx.ErrorJson(400, "Invalid revision format", nil)
	}

	resp, err := g.ToAPI(apiBaseURL(ctx), sha)
	if err != nil {
		// SHA was well-formed but doesn't resolve in the gist's repo.
		return ctx.ErrorJson(404, "Revision not found", nil)
	}
	return ctx.JSON(200, resp)
}
