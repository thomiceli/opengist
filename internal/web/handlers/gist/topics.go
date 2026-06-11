package gist

import (
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
	"github.com/thomiceli/opengist/internal/web/handlers"
)

// Topics lists topics in use, ordered from most to least used, each linking to
// the gists tagged with it. The list is not sortable and is paginated to 20
// topics per page.
func Topics(ctx *context.Context) error {
	var currentUserId uint
	if ctx.User != nil {
		currentUserId = ctx.User.ID
	}

	pageInt := handlers.GetPage(ctx)

	topics, err := db.GetTopicsWithCount(currentUserId, pageInt-1, 21, 20)
	if err != nil {
		return ctx.ErrorRes(500, "Error fetching topics", err)
	}

	if err = handlers.Paginate(ctx, topics, pageInt, 20, "topics", "/-/topics", 1, nil); err != nil {
		return ctx.ErrorRes(404, ctx.Tr("error.page-not-found"), nil)
	}

	ctx.SetData("currentPage", "topics")
	ctx.SetData("htmlTitle", ctx.TrH("gist.list.topics"))
	return ctx.Html("topics.html")
}
