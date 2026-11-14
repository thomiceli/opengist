package gist

import (
	"errors"
	"strconv"

	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/i18n"
	"github.com/thomiceli/opengist/internal/validator"
	"github.com/thomiceli/opengist/internal/web/context"
	"gorm.io/gorm"
)

// CreateComment posts a comment on the gist as the logged-in user.
func CreateComment(ctx *context.Context) error {
	gist := ctx.GetData("gist").(*db.Gist)

	dto, ok := bindComment(ctx, gist)
	if !ok {
		return ctx.RedirectTo(gistUrl(gist) + "#comments")
	}

	comment := &db.GistComment{
		GistID:  gist.ID,
		UserID:  ctx.User.ID,
		Content: dto.Content,
	}
	if err := comment.Create(); err != nil {
		return ctx.ErrorRes(500, "Error creating comment", err)
	}

	return ctx.RedirectTo(commentUrl(gist, comment))
}

// EditComment changes the content of a comment. Only its author can.
func EditComment(ctx *context.Context) error {
	gist := ctx.GetData("gist").(*db.Gist)

	comment, err := getComment(ctx, gist)
	if err != nil {
		return err
	}
	if !comment.CanEdit(ctx.User) {
		return ctx.ErrorRes(403, "You don't have permission to edit this comment", nil)
	}

	dto, ok := bindComment(ctx, gist)
	if !ok {
		return ctx.RedirectTo(commentUrl(gist, comment))
	}

	comment.Content = dto.Content
	if err = comment.Update(); err != nil {
		return ctx.ErrorRes(500, "Error updating comment", err)
	}

	return ctx.RedirectTo(commentUrl(gist, comment))
}

// DeleteComment removes a comment. Its author, the gist owner and admins can.
func DeleteComment(ctx *context.Context) error {
	gist := ctx.GetData("gist").(*db.Gist)

	comment, err := getComment(ctx, gist)
	if err != nil {
		return err
	}
	if !comment.CanDelete(ctx.User, gist) {
		return ctx.ErrorRes(403, "You don't have permission to delete this comment", nil)
	}

	if err = comment.Delete(); err != nil {
		return ctx.ErrorRes(500, "Error deleting comment", err)
	}

	ctx.AddFlash(ctx.Tr("flash.gist.comment-deleted"), "success")
	return ctx.RedirectTo(gistUrl(gist) + "#comments")
}

// bindComment binds and validates the comment form. On a validation error, it
// adds the error as a flash message and returns false.
func bindComment(ctx *context.Context, gist *db.Gist) (*db.GistCommentDTO, bool) {
	dto := new(db.GistCommentDTO)
	if err := ctx.Bind(dto); err != nil {
		ctx.AddFlash(ctx.Tr("error.cannot-bind-data"), "error")
		return nil, false
	}
	dto.Normalize()
	if err := ctx.Validate(dto); err != nil {
		ctx.AddFlash(validator.ValidationMessages(&err, ctx.GetData("locale").(*i18n.Locale)), "error")
		return nil, false
	}
	return dto, true
}

// getComment loads the comment from the :id route param. It is looked up
// within the gist so that a comment can't be reached through another gist.
func getComment(ctx *context.Context, gist *db.Gist) (*db.GistComment, error) {
	commentId, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return nil, ctx.NotFound("Comment not found")
	}

	comment, err := db.GetGistComment(gist.ID, uint(commentId))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ctx.NotFound("Comment not found")
	} else if err != nil {
		return nil, ctx.ErrorRes(500, "Error fetching comment", err)
	}
	return comment, nil
}

func gistUrl(gist *db.Gist) string {
	return "/" + gist.User.Username + "/" + gist.Identifier()
}

func commentUrl(gist *db.Gist, comment *db.GistComment) string {
	return gistUrl(gist) + "#comment-" + strconv.FormatUint(uint64(comment.ID), 10)
}
