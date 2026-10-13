package v1

import (
	"time"

	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
)

// ListLikedGists handles GET /api/gists/liked.
// Lists gists the authenticated user has liked. Auth is mandatory (the route
// uses apiRequireAuth) but the
// gist:read scope is soft-checked here so a token without it degrades to the
// public subset of liked gists rather than 403ing.
//
//   - Token with gist:read → every liked gist the caller is allowed to see
//     (public + caller's own private/unlisted).
//   - Token without gist:read → only the public gists the caller has liked.
//
// Supports the same `page`, `per_page`, and `since` (RFC 3339) query params
// as the other list endpoints; pagination is signaled via the Link header.
func ListLikedGists(ctx *context.Context) error {
	uid := ctx.User.ID
	tok, _ := ctx.GetData("accessToken").(*db.AccessToken)
	if tok != nil && tok.HasGistReadPermission() {
		return listGistsCommon(ctx, func(since *time.Time, offset, limit, perPage int, sort, order string) ([]*db.Gist, int64, error) {
			gists, err := db.GetAllGistsLikedByUser(uid, uid, since, offset, sort, order, limit, perPage)
			if err != nil {
				return nil, 0, err
			}
			total, err := db.CountAllGistsLikedByUserSince(uid, uid, since)
			return gists, total, err
		})
	}
	// currentUserId=0 collapses the visibility OR-clause (`private=0 OR
	// user_id=0`) to just `private=0`, leaving public-only stars.
	return listGistsCommon(ctx, func(since *time.Time, offset, limit, perPage int, sort, order string) ([]*db.Gist, int64, error) {
		gists, err := db.GetAllGistsLikedByUser(uid, 0, since, offset, sort, order, limit, perPage)
		if err != nil {
			return nil, 0, err
		}
		total, err := db.CountAllGistsLikedByUserSince(uid, 0, since)
		return gists, total, err
	})
}

// ToggleLike handles PUT /api/gists/:uuid/like.
// Idempotent toggle: if the authenticated user has liked the gist, it
// removes the like; otherwise it adds one. Either way the response is 204
// No Content. 404 when the gist isn't visible to the caller (same
// existence-hiding rule as the rest of the API). Mutates the caller's own
// like state, so the route requires the user:write scope.
func ToggleLike(ctx *context.Context) error {
	g, err := lookupGistByUUID(ctx, ctx.Param("uuid"))
	if err != nil {
		return ctx.NoContent(404)
	}
	liked, err := ctx.User.HasLiked(g)
	if err != nil {
		return ctx.ErrorJson(500, "failed to check like state", err)
	}
	if liked {
		if err := g.RemoveUserLike(ctx.User); err != nil {
			return ctx.ErrorJson(500, "failed to unlike", err)
		}
	} else {
		if err := g.AppendUserLike(ctx.User); err != nil {
			return ctx.ErrorJson(500, "failed to like", err)
		}
	}
	return ctx.NoContent(204)
}

// CheckLike handles GET /api/gists/:uuid/like.
// Returns 204 No Content if the authenticated user has liked the gist, 404
// otherwise. The gist's visibility is enforced first via lookupGistByUUID -
// a hidden private gist returns 404 just like the rest of the API, without
// revealing whether the caller had ever liked it.
func CheckLike(ctx *context.Context) error {
	g, err := lookupGistByUUID(ctx, ctx.Param("uuid"))
	if err != nil {
		return ctx.NoContent(404)
	}
	liked, err := ctx.User.HasLiked(g)
	if err != nil {
		return ctx.ErrorJson(500, "failed to check like", err)
	}
	if !liked {
		return ctx.NoContent(404)
	}
	return ctx.NoContent(204)
}
