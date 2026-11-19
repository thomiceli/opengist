package v1

import (
	"errors"
	"strconv"

	"gorm.io/gorm"

	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
	"github.com/thomiceli/opengist/internal/web/handlers/api/v1/types"
)

// ListComments handles GET /api/gists/:uuid/comments.
// Returns the comments posted on the gist, oldest first. Same visibility
// rules as /:uuid - public/unlisted readable by anyone, private only
// resolves for its owner with a gist:read token. Supports `page` and
// `per_page` (default 30, capped at 100); pagination is signaled via the
// Link header.
func ListComments(ctx *context.Context) error {
	g, err := lookupGistByUUID(ctx, ctx.Param("uuid"))
	if err != nil {
		return ctx.ErrorJson(404, "Gist not found", nil)
	}

	page := parsePage(ctx)
	perPage := parsePerPage(ctx)

	// perPage+1 is the peek-next sentinel for the Link header.
	comments, err := g.GetCommentsPage(page-1, perPage+1, perPage)
	if err != nil {
		return ctx.ErrorJson(500, "failed to list comments", err)
	}
	hasMore := len(comments) > perPage
	if hasMore {
		comments = comments[:perPage]
	}

	total, err := g.CountComments()
	if err != nil {
		return ctx.ErrorJson(500, "failed to count comments", err)
	}

	baseURL := apiBaseURL(ctx)
	out := make([]types.GistComment, 0, len(comments))
	for _, c := range comments {
		out = append(out, c.ToAPI(baseURL, g))
	}

	writePaginationHeaders(ctx, baseURL, page, perPage, hasMore, &total)
	return ctx.JSON(200, out)
}

// GetComment handles GET /api/gists/:uuid/comments/:id.
// Same visibility rules as ListComments. A comment id that doesn't belong to
// the gist is a 404.
func GetComment(ctx *context.Context) error {
	g, err := lookupGistByUUID(ctx, ctx.Param("uuid"))
	if err != nil {
		return ctx.ErrorJson(404, "Gist not found", nil)
	}
	c, err := lookupComment(ctx, g)
	if err != nil {
		return err
	}
	return ctx.JSON(200, c.ToAPI(apiBaseURL(ctx), g))
}

// CreateComment handles POST /api/gists/:uuid/comments.
// Posts a comment as the authenticated caller on any gist they can see.
// Body: {"body": "<markdown>"}. Returns 201 with the comment and a
// `Location` header; an empty or too long body → 422, an archived gist → 403.
func CreateComment(ctx *context.Context) error {
	g, err := lookupGistByUUID(ctx, ctx.Param("uuid"))
	if err != nil {
		return ctx.ErrorJson(404, "Gist not found", nil)
	}
	if g.Archived {
		return ctx.ErrorJson(403, "This gist is archived and is read-only", nil)
	}

	dto, err := bindCommentInput(ctx)
	if err != nil {
		return err
	}

	c := &db.GistComment{
		GistID:  g.ID,
		UserID:  ctx.User.ID,
		Content: dto.Content,
	}
	if err := c.Create(); err != nil {
		return ctx.ErrorJson(500, "failed to create comment", err)
	}
	c.User = *ctx.User

	baseURL := apiBaseURL(ctx)
	resp := c.ToAPI(baseURL, g)
	ctx.Response().Header().Set("Location", resp.URL)
	return ctx.JSON(201, resp)
}

// UpdateComment handles PATCH /api/gists/:uuid/comments/:id.
// Replaces the comment's body. Only the comment's author can edit it (403
// otherwise, the comment being visible); an archived gist → 403, an empty or
// too long body → 422.
func UpdateComment(ctx *context.Context) error {
	g, err := lookupGistByUUID(ctx, ctx.Param("uuid"))
	if err != nil {
		return ctx.ErrorJson(404, "Gist not found", nil)
	}
	c, err := lookupComment(ctx, g)
	if err != nil {
		return err
	}
	if !c.CanEdit(ctx.User) {
		return ctx.ErrorJson(403, "You are not the author of this comment", nil)
	}
	if g.Archived {
		return ctx.ErrorJson(403, "This gist is archived and is read-only", nil)
	}

	dto, err := bindCommentInput(ctx)
	if err != nil {
		return err
	}

	c.Content = dto.Content
	if err := c.Update(); err != nil {
		return ctx.ErrorJson(500, "failed to update comment", err)
	}
	return ctx.JSON(200, c.ToAPI(apiBaseURL(ctx), g))
}

// DeleteComment handles DELETE /api/gists/:uuid/comments/:id.
// The comment's author, the gist's owner and admins can delete it, archived
// gist or not (403 for anyone else). Returns 204 No Content.
func DeleteComment(ctx *context.Context) error {
	g, err := lookupGistByUUID(ctx, ctx.Param("uuid"))
	if err != nil {
		return ctx.ErrorJson(404, "Gist not found", nil)
	}
	c, err := lookupComment(ctx, g)
	if err != nil {
		return err
	}
	if !c.CanDelete(ctx.User, g) {
		return ctx.ErrorJson(403, "You are not allowed to delete this comment", nil)
	}

	if err := c.Delete(); err != nil {
		return ctx.ErrorJson(500, "failed to delete comment", err)
	}
	return ctx.NoContent(204)
}

// lookupComment fetches the comment from the :id route param, scoped to the
// gist g. Returns an HTTP-ready error envelope (404 for a malformed id or a
// comment of another gist).
func lookupComment(ctx *context.Context, g *db.Gist) (*db.GistComment, error) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 0)
	if err != nil {
		return nil, ctx.ErrorJson(404, "Comment not found", nil)
	}
	c, err := db.GetGistComment(g.ID, uint(id))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ctx.ErrorJson(404, "Comment not found", nil)
	} else if err != nil {
		return nil, ctx.ErrorJson(500, "failed to fetch comment", err)
	}
	return c, nil
}

// bindCommentInput binds the JSON body and runs it through the same
// normalization and validation as the web form. Returns an HTTP-ready error
// envelope on failure (422).
func bindCommentInput(ctx *context.Context) (*db.GistCommentDTO, error) {
	var req types.GistCommentInput
	if err := ctx.Bind(&req); err != nil {
		return nil, ctx.ErrorJson(422, "could not bind data", nil)
	}
	dto := &db.GistCommentDTO{Content: req.Body}
	dto.Normalize()
	if err := ctx.Validate(dto); err != nil {
		return nil, ctx.ErrorJson(422, err.Error(), nil)
	}
	return dto, nil
}
