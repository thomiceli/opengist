package gist

import (
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
	"github.com/thomiceli/opengist/internal/web/handlers"
)

func Revisions(ctx *context.Context) error {
	gist := ctx.GetData("gist").(*db.Gist)
	userName := gist.User.Username
	gistName := gist.Identifier()

	pageInt := handlers.GetPage(ctx)

	commits, err := gist.Log("HEAD", (pageInt-1)*10, 11)
	if err != nil {
		return ctx.ErrorRes(500, "Error fetching commits log", err)
	}

	if err := handlers.Paginate(ctx, commits, pageInt, 10, "commits", userName+"/"+gistName+"/revisions", 2, nil); err != nil {
		return ctx.ErrorRes(404, ctx.Tr("error.page-not-found"), nil)
	}

	ctx.SetData("page", "revisions")
	ctx.SetData("revision", "HEAD")
	ctx.SetData("htmlTitle", ctx.TrH("gist.revision-of", gist.Title))

	return ctx.Html("revisions.html")
}
