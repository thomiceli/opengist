package v1

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
	"github.com/thomiceli/opengist/internal/web/handlers/api/v1/types"
)

// ListForkedGists handles GET /api/gists/forked.
// Lists gists the authenticated user has forked. Auth is mandatory (the
// route uses apiRequireAuth) but the gist:read scope is soft-checked here
// so a token without it degrades to the public subset of forked gists
// rather than 403ing.
//
//   - Token with gist:read → every forked gist the caller is allowed to
//     see (public + caller's own private/unlisted forks).
//   - Token without gist:read → only the public gists the caller has
//     forked.
//
// Supports the same `page`, `per_page`, and `since` (RFC 3339) query params
// as the other list endpoints; pagination is signaled via the Link header.
func ListForkedGists(ctx *context.Context) error {
	uid := ctx.User.ID
	tok, _ := ctx.GetData("accessToken").(*db.AccessToken)
	if tok != nil && tok.HasGistReadPermission() {
		return listGistsCommon(ctx, func(since *time.Time, offset, limit, perPage int, sort, order string) ([]*db.Gist, int64, error) {
			gists, err := db.GetAllGistsForkedByUser(uid, uid, since, offset, sort, order, limit, perPage)
			if err != nil {
				return nil, 0, err
			}
			total, err := db.CountAllGistsForkedByUserSince(uid, uid, since)
			return gists, total, err
		})
	}
	// currentUserId=0 collapses the visibility OR-clause (`private=0 OR
	// user_id=0`) to just `private=0`, leaving public-only forks.
	return listGistsCommon(ctx, func(since *time.Time, offset, limit, perPage int, sort, order string) ([]*db.Gist, int64, error) {
		gists, err := db.GetAllGistsForkedByUser(uid, 0, since, offset, sort, order, limit, perPage)
		if err != nil {
			return nil, 0, err
		}
		total, err := db.CountAllGistsForkedByUserSince(uid, 0, since)
		return gists, total, err
	})
}

// ListForks handles GET /api/gists/:uuid/forks.
// Returns the gists that fork
// the targeted gist as a list of GistSimple. Same visibility rules as
// /:uuid (and /:uuid/commits) - public/unlisted readable by anyone,
// private only resolves for its owner with a gist:read token. Supports
// `page` and `per_page` (default 30, capped at 100); pagination is
// signaled via the Link header.
func ListForks(ctx *context.Context) error {
	g, err := lookupGistByUUID(ctx, ctx.Param("uuid"))
	if err != nil {
		return ctx.ErrorJson(404, "Gist not found", nil)
	}

	page := parsePage(ctx)
	perPage := parsePerPage(ctx)

	// Visibility-filter forks to what the caller can see: their own
	// private/unlisted forks are included alongside any public fork. callerID
	// = 0 (anonymous) trims to public-only, matching the gist endpoints.
	var callerID uint
	if ctx.User != nil {
		callerID = ctx.User.ID
	}

	// perPage+1 is the peek-next sentinel for the Link header.
	forks, err := g.GetForks(callerID, page-1, perPage+1, perPage)
	if err != nil {
		return ctx.ErrorJson(500, "failed to list forks", err)
	}
	hasMore := len(forks) > perPage
	if hasMore {
		forks = forks[:perPage]
	}

	total, err := g.CountForks(callerID)
	if err != nil {
		return ctx.ErrorJson(500, "failed to count forks", err)
	}

	baseURL := apiBaseURL(ctx)
	out := make([]types.GistSimple, 0, len(forks))
	for _, f := range forks {
		out = append(out, f.ToAPISimple(baseURL))
	}

	writePaginationHeaders(ctx, baseURL, page, perPage, hasMore, &total)
	return ctx.JSON(200, out)
}

// ForkGist handles POST /api/gists/:uuid/forks.
// The authenticated caller
// gets a new gist owned by them whose content is a clone of the parent,
// with `forked_id` pointing back. Visibility (public/unlisted/private) is
// inherited from the parent. Returns 201 with the new fork's GistSimple
// and a `Location` header pointing to the new gist's API URL.
//
// Rules:
//   - The parent's visibility decides whether the caller can see (and
//     therefore fork) it; the same lookup rule as /gists/:uuid applies.
//   - Forking your own gist is rejected with 422 - matches the web flow.
//   - Forking the same parent twice is idempotent: returns 200 with the
//     existing fork (vs 201 for a newly created one), plus a `Location` header.
func ForkGist(ctx *context.Context) error {
	parent, err := lookupGistByUUID(ctx, ctx.Param("uuid"))
	if err != nil {
		return ctx.ErrorJson(404, "Gist not found", nil)
	}
	user := ctx.User

	if parent.UserID == user.ID {
		return ctx.ErrorJson(422, "cannot fork your own gist", nil)
	}

	// Has the caller already forked this gist? (GetForkParent is misnamed -
	// it actually returns the caller's fork of `parent`.) Return the existing
	// fork idempotently so retries are safe.
	existing, err := parent.GetForkParent(user)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return ctx.ErrorJson(500, "failed to check existing fork", err)
	}
	if existing.ID != 0 {
		// Already forked → return the existing fork idempotently with 200 (vs
		// 201 for a freshly created one). Reload so User/Forked.User are
		// preloaded for ToAPISimple.
		saved, err := db.GetGistByID(strconv.FormatUint(uint64(existing.ID), 10))
		if err != nil {
			return ctx.ErrorJson(500, "failed to reload existing fork", err)
		}
		baseURL := apiBaseURL(ctx)
		ctx.Response().Header().Set("Location", baseURL+"/api/gists/"+saved.Uuid)
		return ctx.JSON(200, saved.ToAPISimple(baseURL))
	}

	id, err := uuid.NewRandom()
	if err != nil {
		return ctx.ErrorJson(500, "uuid generation failed", err)
	}
	newGist := &db.Gist{
		Uuid:            strings.ReplaceAll(id.String(), "-", ""),
		Title:           parent.Title,
		Preview:         parent.Preview,
		PreviewFilename: parent.PreviewFilename,
		Description:     parent.Description,
		Private:         parent.Private,
		UserID:          user.ID,
		ForkedID:        parent.ID,
		NbFiles:         parent.NbFiles,
		Topics:          parent.Topics,
	}

	if err := newGist.CreateForked(); err != nil {
		return ctx.ErrorJson(500, "failed to create fork in database", err)
	}
	if err := parent.ForkClone(user.Username, newGist.Uuid); err != nil {
		return ctx.ErrorJson(500, "failed to clone repository", err)
	}
	if err := parent.IncrementForkCount(); err != nil {
		return ctx.ErrorJson(500, "failed to increment fork count", err)
	}

	// Reload so User/Forked.User are preloaded for ToAPISimple.
	saved, err := db.GetGistByID(strconv.FormatUint(uint64(newGist.ID), 10))
	if err != nil {
		return ctx.ErrorJson(500, "failed to reload new fork", err)
	}

	baseURL := apiBaseURL(ctx)
	resp := saved.ToAPISimple(baseURL)
	ctx.Response().Header().Set("Location", baseURL+"/api/gists/"+saved.Uuid)
	return ctx.JSON(201, resp)
}
