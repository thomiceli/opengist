package v1

import (
	"time"

	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
	"github.com/thomiceli/opengist/internal/web/handlers/api/v1/types"
)

// gistFetcher is the per-mode db query used by listGistsCommon. Different
// list modes (anonymous, scoped token, unscoped token, /public) all share
// pagination + Link-header plumbing; only the underlying SQL differs. It
// returns the page of gists plus the total count matching the query (across
// all pages) for the response's pagination metadata.
type gistFetcher func(since *time.Time, offset, limit, perPage int, sort, order string) ([]*db.Gist, int64, error)

// listGistsCommon handles the shared pagination + Link-header plumbing for
// the list endpoints. The caller supplies a gistFetcher that runs the right
// query for the current mode.
func listGistsCommon(ctx *context.Context, fetch gistFetcher) error {
	page := parsePage(ctx)
	perPage := parsePerPage(ctx)
	since, err := parseSince(ctx)
	if err != nil {
		return ctx.ErrorJson(400, "GistSimple not found", nil)
	}

	// Fetch one extra row as the peek-next sentinel: if we get back perPage+1
	// rows, there's another page.
	gists, total, err := fetch(since, page-1, perPage+1, perPage, "updated", "desc")
	if err != nil {
		return ctx.ErrorJson(500, "failed to list gists", err)
	}

	hasMore := len(gists) > perPage
	if hasMore {
		gists = gists[:perPage]
	}

	baseURL := apiBaseURL(ctx)
	out := make([]types.GistSimple, 0, len(gists))
	for _, g := range gists {
		out = append(out, g.ToAPISimple(baseURL))
	}

	writePaginationHeaders(ctx, baseURL, page, perPage, hasMore, &total)
	return ctx.JSON(200, out)
}

// ListGists handles GET /api/v1/gists.
// Returns a JSON array; pagination is signaled via the X-* and Link response
// headers. Scope-gated visibility of the caller's own gists; other users' gists
// are never returned here (use /gists/public for that).
//   - Token with gist:read → caller's own gists in every visibility (public,
//     unlisted, private).
//   - Token without gist:read → only the caller's own public gists.
//   - Anonymous → all public gists from everyone (matches /gists/public so
//     unauthenticated GET /gists is still useful).
//
// Supports `since` (RFC 3339) and `page` query params.
func ListGists(ctx *context.Context) error {
	if ctx.User != nil {
		uid := ctx.User.ID
		tok, _ := ctx.GetData("accessToken").(*db.AccessToken)
		if tok != nil && tok.HasGistReadPermission() {
			return listGistsCommon(ctx, func(since *time.Time, offset, limit, perPage int, sort, order string) ([]*db.Gist, int64, error) {
				gists, err := db.GetAllGistsOfUser(uid, since, offset, sort, order, limit, perPage)
				if err != nil {
					return nil, 0, err
				}
				total, err := db.CountAllGistsOfUser(uid, since)
				return gists, total, err
			})
		}
		return listGistsCommon(ctx, func(since *time.Time, offset, limit, perPage int, sort, order string) ([]*db.Gist, int64, error) {
			gists, err := db.GetAllPublicGistsOfUser(uid, since, offset, sort, order, limit, perPage)
			if err != nil {
				return nil, 0, err
			}
			total, err := db.CountAllPublicGistsOfUser(uid, since)
			return gists, total, err
		})
	}
	return listGistsCommon(ctx, func(since *time.Time, offset, limit, perPage int, sort, order string) ([]*db.Gist, int64, error) {
		gists, err := db.GetAllGistsForCurrentUser(0, since, offset, sort, order, limit, perPage)
		if err != nil {
			return nil, 0, err
		}
		total, err := db.CountAllGistsForCurrentUser(0, since)
		return gists, total, err
	})
}

// ListPublicGists handles GET /api/v1/gists/public.
// Returns only public gists regardless of the caller's auth state.
func ListPublicGists(ctx *context.Context) error {
	return listGistsCommon(ctx, func(since *time.Time, offset, limit, perPage int, sort, order string) ([]*db.Gist, int64, error) {
		gists, err := db.GetAllGistsForCurrentUser(0, since, offset, sort, order, limit, perPage)
		if err != nil {
			return nil, 0, err
		}
		total, err := db.CountAllGistsForCurrentUser(0, since)
		return gists, total, err
	})
}
