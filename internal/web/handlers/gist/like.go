package gist

import (
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
	"github.com/thomiceli/opengist/internal/web/handlers"
)

func Like(ctx *context.Context) error {
	gist := ctx.GetData("gist").(*db.Gist)
	currentUser := ctx.User

	hasLiked, err := currentUser.HasLiked(gist)
	if err != nil {
		return ctx.ErrorRes(500, "Error checking if user has liked a gist", err)
	}

	if hasLiked {
		err = gist.RemoveUserLike(ctx.User)
	} else {
		err = gist.AppendUserLike(ctx.User)
	}

	if err != nil {
		return ctx.ErrorRes(500, "Error liking/dislking this gist", err)
	}

	redirectTo := "/" + gist.User.Username + "/" + gist.Identifier()
	if r := ctx.QueryParam("redirecturl"); r != "" {
		redirectTo = r
	}
	return ctx.RedirectTo(redirectTo)
}

func Likes(ctx *context.Context) error {
	gist := ctx.GetData("gist").(*db.Gist)

	pageInt := handlers.GetPage(ctx)

	likers, err := gist.GetUsersLikes(pageInt - 1)
	if err != nil {
		return ctx.ErrorRes(500, "Error getting users who liked this gist", err)
	}

	if err = handlers.Paginate(ctx, likers, pageInt, 30, "likers", gist.User.Username+"/"+gist.Identifier()+"/likes", 1); err != nil {
		return ctx.ErrorRes(404, ctx.Tr("error.page-not-found"), nil)
	}

	ctx.SetData("htmlTitle", ctx.TrH("gist.likes.for", gist.Title))
	ctx.SetData("revision", "HEAD")
	return ctx.Html("likes.html")
}
