package gist

import (
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/i18n"
	"github.com/thomiceli/opengist/internal/validator"
	"github.com/thomiceli/opengist/internal/web/context"
)

// GistSettings renders the per-gist settings page (visibility, archive, delete).
// It is owner-only (gated by the writePermission middleware). The actions
// themselves POST to the existing /visibility, /archive and /delete routes.
func GistSettings(ctx *context.Context) error {
	gist := ctx.GetData("gist").(*db.Gist)

	ctx.SetData("page", "settings")
	ctx.SetData("htmlTitle", gist.Title)
	return ctx.Html("gist_settings.html")
}

// EditMetadata updates a gist's title, URL path, description and topics without
// touching its files. Owner-only and blocked on archived gists (router guards).
func EditMetadata(ctx *context.Context) error {
	gist := ctx.GetData("gist").(*db.Gist)

	dto := new(db.GistMetadataDTO)
	if err := ctx.Bind(dto); err != nil {
		return ctx.ErrorRes(400, ctx.Tr("error.cannot-bind-data"), err)
	}
	if err := ctx.Validate(dto); err != nil {
		ctx.AddFlash(validator.ValidationMessages(&err, ctx.GetData("locale").(*i18n.Locale)), "error")
		return ctx.RedirectTo("/" + gist.User.Username + "/" + gist.Identifier() + "/settings")
	}

	dto.ToExistingGist(gist)
	if err := gist.Update(); err != nil {
		return ctx.ErrorRes(500, "Error updating this gist", err)
	}
	gist.AddInIndex()

	ctx.AddFlash(ctx.Tr("flash.gist.updated"), "success")
	return ctx.RedirectTo("/" + gist.User.Username + "/" + gist.Identifier())
}
