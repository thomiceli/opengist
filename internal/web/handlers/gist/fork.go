package gist

import (
	"errors"
	"github.com/google/uuid"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
	"github.com/thomiceli/opengist/internal/web/handlers"
	"gorm.io/gorm"
	"strings"
)

func Fork(ctx *context.Context) error {
	gist := ctx.GetData("gist").(*db.Gist)
	currentUser := ctx.User

	alreadyForked, err := gist.GetForkParent(currentUser)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return ctx.ErrorRes(500, "Error checking if gist is already forked", err)
	}

	if gist.User.ID == currentUser.ID {
		ctx.AddFlash(ctx.Tr("flash.gist.fork-own-gist"), "error")
		return ctx.RedirectTo("/" + gist.User.Username + "/" + gist.Identifier())
	}

	if alreadyForked.ID != 0 {
		return ctx.RedirectTo("/" + alreadyForked.User.Username + "/" + alreadyForked.Identifier())
	}

	uuidGist, err := uuid.NewRandom()
	if err != nil {
		return ctx.ErrorRes(500, "Error creating an UUID", err)
	}

	newGist := &db.Gist{
		Uuid:            strings.Replace(uuidGist.String(), "-", "", -1),
		Title:           gist.Title,
		Preview:         gist.Preview,
		PreviewFilename: gist.PreviewFilename,
		Description:     gist.Description,
		Private:         gist.Private,
		UserID:          currentUser.ID,
		ForkedID:        gist.ID,
		NbFiles:         gist.NbFiles,
	}

	if err = newGist.CreateForked(); err != nil {
		return ctx.ErrorRes(500, "Error forking the gist in database", err)
	}

	if err = gist.ForkClone(currentUser.Username, newGist.Uuid); err != nil {
		return ctx.ErrorRes(500, "Error cloning the repository while forking", err)
	}
	if err = gist.IncrementForkCount(); err != nil {
		return ctx.ErrorRes(500, "Error incrementing the fork count", err)
	}

	ctx.AddFlash(ctx.Tr("flash.gist.forked"), "success")

	return ctx.RedirectTo("/" + currentUser.Username + "/" + newGist.Identifier())
}

func Forks(ctx *context.Context) error {
	gist := ctx.GetData("gist").(*db.Gist)
	pageInt := handlers.GetPage(ctx)

	currentUser := ctx.User
	var fromUserID uint = 0
	if currentUser != nil {
		fromUserID = currentUser.ID
	}

	forks, err := gist.GetForks(fromUserID, pageInt-1)
	if err != nil {
		return ctx.ErrorRes(500, "Error getting users who liked this gist", err)
	}

	if err = handlers.Paginate(ctx, forks, pageInt, 30, "forks", gist.User.Username+"/"+gist.Identifier()+"/forks", 2); err != nil {
		return ctx.ErrorRes(404, ctx.Tr("error.page-not-found"), nil)
	}

	ctx.SetData("htmlTitle", ctx.TrH("gist.forks.for", gist.Title))
	ctx.SetData("revision", "HEAD")
	return ctx.Html("forks.html")
}
