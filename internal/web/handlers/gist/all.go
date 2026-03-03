package gist

import (
	"errors"
	"slices"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/index"
	"github.com/thomiceli/opengist/internal/render"
	"github.com/thomiceli/opengist/internal/web/context"
	"github.com/thomiceli/opengist/internal/web/handlers"
	"gorm.io/gorm"
)

func AllGists(ctx *context.Context) error {
	var err error
	var urlPage string

	fromUserStr := ctx.Param("user")
	userLogged := ctx.User
	pageInt := handlers.GetPage(ctx)

	sort := "created"
	sortText := ctx.TrH("gist.list.sort-by-created")
	order := "desc"
	orderText := ctx.TrH("gist.list.order-by-desc")

	if ctx.QueryParam("sort") == "updated" {
		sort = "updated"
		sortText = ctx.TrH("gist.list.sort-by-updated")
	}

	if ctx.QueryParam("order") == "asc" {
		order = "asc"
		orderText = ctx.TrH("gist.list.order-by-asc")
	}

	pagination := &handlers.PaginationParams{
		Sort:  sort,
		Order: order,
	}

	ctx.SetData("sort", sortText)
	ctx.SetData("order", orderText)

	var gists []*db.Gist
	var currentUserId uint
	if userLogged != nil {
		currentUserId = userLogged.ID
	} else {
		currentUserId = 0
	}

	mode := ctx.GetData("mode")
	if fromUserStr == "" {
		switch mode {
		case "search":
			ctx.SetData("htmlTitle", ctx.TrH("gist.list.search-results"))
			ctx.SetData("searchQuery", ctx.QueryParam("q"))
			pagination.Query = ctx.QueryParam("q")
			urlPage = "search"
			gists, err = db.GetAllGistsFromSearch(currentUserId, ctx.QueryParam("q"), pageInt-1, sort, order, "")
		case "topics":
			ctx.SetData("htmlTitle", ctx.TrH("gist.list.topic-results-topic", ctx.Param("topic")))
			ctx.SetData("topic", ctx.Param("topic"))
			urlPage = "topics/" + ctx.Param("topic")
			gists, err = db.GetAllGistsFromSearch(currentUserId, "", pageInt-1, sort, order, ctx.Param("topic"))
		case "all":
			ctx.SetData("htmlTitle", ctx.TrH("gist.list.all"))
			urlPage = "all"
			gists, err = db.GetAllGistsForCurrentUser(currentUserId, pageInt-1, sort, order)
		}
	} else {
		var fromUser *db.User
		var count int64

		fromUser, err = db.GetUserByUsername(fromUserStr)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ctx.NotFound("User not found")
			}
			return ctx.ErrorRes(500, "Error fetching user", err)
		}
		ctx.SetData("fromUser", fromUser)

		if countFromUser, err := db.CountAllGistsFromUser(fromUser.ID, currentUserId); err != nil {
			return ctx.ErrorRes(500, "Error counting gists", err)
		} else {
			ctx.SetData("countFromUser", countFromUser)
		}

		if countLiked, err := db.CountAllGistsLikedByUser(fromUser.ID, currentUserId); err != nil {
			return ctx.ErrorRes(500, "Error counting liked gists", err)
		} else {
			ctx.SetData("countLiked", countLiked)
		}

		if countForked, err := db.CountAllGistsForkedByUser(fromUser.ID, currentUserId); err != nil {
			return ctx.ErrorRes(500, "Error counting forked gists", err)
		} else {
			ctx.SetData("countForked", countForked)
		}

		switch mode {
		case "liked":
			urlPage = fromUserStr + "/liked"
			ctx.SetData("htmlTitle", ctx.TrH("gist.list.all-liked-by", fromUserStr))
			gists, err = db.GetAllGistsLikedByUser(fromUser.ID, currentUserId, pageInt-1, sort, order)
		case "forked":
			urlPage = fromUserStr + "/forked"
			ctx.SetData("htmlTitle", ctx.TrH("gist.list.all-forked-by", fromUserStr))
			gists, err = db.GetAllGistsForkedByUser(fromUser.ID, currentUserId, pageInt-1, sort, order)
		case "fromUser":
			urlPage = fromUserStr

			if languages, err := db.GetGistLanguagesForUser(fromUser.ID, currentUserId); err != nil {
				return ctx.ErrorRes(500, "Error fetching languages", err)
			} else {
				ctx.SetData("languages", languages)
			}
			title := ctx.QueryParam("title")
			language := ctx.QueryParam("language")
			visibility := ctx.QueryParam("visibility")
			topicsStr := ctx.QueryParam("topics")
			topics := strings.Fields(topicsStr)
			if len(topics) > 10 {
				topics = topics[:10]
			}
			slices.Sort(topics)
			topics = slices.Compact(topics)
			pagination.Title = title
			pagination.Language = language
			pagination.Visibility = visibility
			pagination.Topics = topicsStr

			ctx.SetData("title", title)
			ctx.SetData("language", language)
			ctx.SetData("visibility", visibility)
			ctx.SetData("topics", topicsStr)
			ctx.SetData("htmlTitle", ctx.TrH("gist.list.all-from", fromUserStr))
			gists, count, err = db.GetAllGistsFromUser(fromUser.ID, currentUserId, title, language, visibility, topics, pageInt-1, sort, order)
			ctx.SetData("countFromUser", count)
		}
	}
	if err != nil {
		return ctx.ErrorRes(500, "Error fetching gists", err)
	}

	renderedGists := make([]*render.RenderedGist, 0, len(gists))
	for _, gist := range gists {
		rendered, err := render.HighlightGistPreview(gist)
		if err != nil {
			log.Error().Err(err).Msg("Error rendering gist preview for " + gist.Identifier() + " - " + gist.PreviewFilename)
		}
		renderedGists = append(renderedGists, &rendered)
	}

	if err = handlers.Paginate(ctx, renderedGists, pageInt, 10, "gists", urlPage, 2, pagination); err != nil {
		return ctx.ErrorRes(404, ctx.Tr("error.page-not-found"), nil)
	}

	return ctx.Html("all.html")
}

// Search handles the search page for gists.
//
// It takes a query parameter "q" which is a search query in the format:
// "user:username title:title description:description filename:filename language:language topic:topic"
//
// It also takes a page parameter "page" which is the page number to display.
//
// It returns an error if the search query is invalid or if the page number is invalid.
//
// It returns the search results as a list of rendered gists, along with the total number of results, the languages found, and the search query.
//
// The search results are paginated, with 10 results per page.
func Search(ctx *context.Context) error {
	var err error

	pagination := &handlers.PaginationParams{
		Query: ctx.QueryParam("q"),
	}

	metadata := handlers.ParseSearchQueryStr(ctx.QueryParam("q"))
	pageInt := handlers.GetPage(ctx)

	var currentUserId uint
	userLogged := ctx.User
	if userLogged != nil {
		currentUserId = userLogged.ID
	} else {
		currentUserId = 0
	}

	// Search gists in the index and fetch the gists IDs from the database
	gistsIds, nbHits, langs, err := index.SearchGists(index.SearchGistMetadata{
		Username:    metadata["user"],
		Title:       metadata["title"],
		Description: metadata["description"],
		Filename:    metadata["filename"],
		Extension:   metadata["extension"],
		Language:    metadata["language"],
		Topic:       metadata["topic"],
		Content:     metadata["content"],
		All:         metadata["all"],
	}, currentUserId, pageInt)
	if err != nil {
		return ctx.ErrorRes(500, "Error searching gists", err)
	}

	gists, err := db.GetAllGistsByIds(gistsIds)
	if err != nil {
		return ctx.ErrorRes(500, "Error fetching gists", err)
	}

	renderedGists := make([]*render.RenderedGist, 0, len(gists))
	for _, gist := range gists {
		rendered, err := render.HighlightGistPreview(gist)
		if err != nil {
			log.Error().Err(err).Msg("Error rendering gist preview for " + gist.Identifier() + " - " + gist.PreviewFilename)
		}
		renderedGists = append(renderedGists, &rendered)
	}

	if err = handlers.Paginate(ctx, renderedGists, pageInt, 10, "gists", "search", 2, pagination); err != nil {
		return ctx.ErrorRes(404, ctx.Tr("error.page-not-found"), nil)
	}

	ctx.SetData("htmlTitle", ctx.TrH("gist.list.search-results"))
	ctx.SetData("nbHits", nbHits)
	ctx.SetData("langs", langs)
	ctx.SetData("searchQuery", ctx.QueryParam("q"))
	return ctx.Html("search.html")
}
