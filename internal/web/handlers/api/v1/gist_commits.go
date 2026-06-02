package v1

import (
	"github.com/thomiceli/opengist/internal/web/context"
	"github.com/thomiceli/opengist/internal/web/handlers/api/v1/types"
)

// ListCommits handles GET /api/v1/gists/:uuid/commits.
// Each commit's author is
// resolved to an Opengist user via db.Gist.Log's bulk email lookup (see
// db.GistCommit) so the API and the web revisions page share the same
// resolution. The per-commit shape comes from db.GistCommit.ToAPI, which
// also powers the 10-most-recent embed in the gist-detail response.
// Public/unlisted gists are readable by anyone; private gists only resolve
// for their owner with a gist:read token. Supports `page` and `per_page`
// (default 30, capped at 100); pagination is signaled via the Link header.
func ListCommits(ctx *context.Context) error {
	g, err := lookupGistByUUID(ctx, ctx.Param("uuid"))
	if err != nil {
		return ctx.ErrorJson(404, "Gist not found", nil)
	}

	page := parsePage(ctx)
	perPage := parsePerPage(ctx)

	// Fetch one extra row as the peek-next sentinel: a slice of perPage+1
	// tells us there's another page worth fetching.
	commits, err := g.Log("HEAD", (page-1)*perPage, perPage+1)
	if err != nil {
		return ctx.ErrorJson(500, "failed to read commit log", err)
	}
	hasMore := len(commits) > perPage
	if hasMore {
		commits = commits[:perPage]
	}

	baseURL := apiBaseURL(ctx)
	out := make([]types.GistCommit, 0, len(commits))
	for _, c := range commits {
		out = append(out, c.ToAPI())
	}

	// total is omitted (nil) for commits: computing it would need an extra
	// `git rev-list --count` subprocess per request, which isn't worth it.
	writePaginationHeaders(ctx, baseURL, page, perPage, hasMore, nil)
	return ctx.JSON(200, out)
}
