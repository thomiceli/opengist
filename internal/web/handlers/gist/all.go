package gist

import (
	"errors"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/index"
	"github.com/thomiceli/opengist/internal/render"
	"github.com/thomiceli/opengist/internal/web/context"
	"github.com/thomiceli/opengist/internal/web/handlers"
	"gorm.io/gorm"
	"html/template"
	"regexp"
	"strings"
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

	ctx.SetData("sort", sortText)
	ctx.SetData("order", orderText)

	var gists []*db.Gist
	var currentUserId uint
	if userLogged != nil {
		currentUserId = userLogged.ID
	} else {
		currentUserId = 0
	}

	if fromUserStr == "" {
		urlctx := ctx.Request().URL.Path
		if strings.HasSuffix(urlctx, "search") {
			ctx.SetData("htmlTitle", ctx.TrH("gist.list.search-results"))
			ctx.SetData("mode", "search")
			ctx.SetData("searchQuery", ctx.QueryParam("q"))
			ctx.SetData("searchQueryUrl", template.URL("&q="+ctx.QueryParam("q")))
			urlPage = "search"
			gists, err = db.GetAllGistsFromSearch(currentUserId, ctx.QueryParam("q"), pageInt-1, sort, order)
		} else if strings.HasSuffix(urlctx, "all") {
			ctx.SetData("htmlTitle", ctx.TrH("gist.list.all"))
			ctx.SetData("mode", "all")
			urlPage = "all"
			gists, err = db.GetAllGistsForCurrentUser(currentUserId, pageInt-1, sort, order)
		}
	} else {
		liked := false
		forked := false

		liked, err = regexp.MatchString(`/[^/]*/liked`, ctx.Request().URL.Path)
		if err != nil {
			return ctx.ErrorRes(500, "Error matching regexp", err)
		}

		forked, err = regexp.MatchString(`/[^/]*/forked`, ctx.Request().URL.Path)
		if err != nil {
			return ctx.ErrorRes(500, "Error matching regexp", err)
		}

		var fromUser *db.User

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

		if liked {
			urlPage = fromUserStr + "/liked"
			ctx.SetData("htmlTitle", ctx.TrH("gist.list.all-liked-by", fromUserStr))
			ctx.SetData("mode", "liked")
			gists, err = db.GetAllGistsLikedByUser(fromUser.ID, currentUserId, pageInt-1, sort, order)
		} else if forked {
			urlPage = fromUserStr + "/forked"
			ctx.SetData("htmlTitle", ctx.TrH("gist.list.all-forked-by", fromUserStr))
			ctx.SetData("mode", "forked")
			gists, err = db.GetAllGistsForkedByUser(fromUser.ID, currentUserId, pageInt-1, sort, order)
		} else {
			urlPage = fromUserStr
			ctx.SetData("htmlTitle", ctx.TrH("gist.list.all-from", fromUserStr))
			ctx.SetData("mode", "fromUser")
			gists, err = db.GetAllGistsFromUser(fromUser.ID, currentUserId, pageInt-1, sort, order)
		}
	}

	renderedGists := make([]*render.RenderedGist, 0, len(gists))
	for _, gist := range gists {
		rendered, err := render.HighlightGistPreview(gist)
		if err != nil {
			log.Error().Err(err).Msg("Error rendering gist preview for " + gist.Identifier() + " - " + gist.PreviewFilename)
		}
		renderedGists = append(renderedGists, &rendered)
	}

	if err != nil {
		return ctx.ErrorRes(500, "Error fetching gists", err)
	}

	if err = handlers.Paginate(ctx, renderedGists, pageInt, 10, "gists", fromUserStr, 2, "&sort="+sort+"&order="+order); err != nil {
		return ctx.ErrorRes(404, ctx.Tr("error.page-not-found"), nil)
	}

	ctx.SetData("urlPage", urlPage)
	return ctx.Html("all.html")
}

func Search(ctx *context.Context) error {
	var err error

	content, meta := handlers.ParseSearchQueryStr(ctx.QueryParam("q"))
	pageInt := handlers.GetPage(ctx)

	var currentUserId uint
	userLogged := ctx.User
	if userLogged != nil {
		currentUserId = userLogged.ID
	} else {
		currentUserId = 0
	}

	var visibleGistsIds []uint
	visibleGistsIds, err = db.GetAllGistsVisibleByUser(currentUserId)
	if err != nil {
		return ctx.ErrorRes(500, "Error fetching gists", err)
	}

	gistsIds, nbHits, langs, err := index.SearchGists(content, index.SearchGistMetadata{
		Username:  meta["user"],
		Title:     meta["title"],
		Filename:  meta["filename"],
		Extension: meta["extension"],
		Language:  meta["language"],
	}, visibleGistsIds, pageInt)
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

	if pageInt > 1 && len(renderedGists) != 0 {
		ctx.SetData("prevPage", pageInt-1)
	}
	if 10*pageInt < int(nbHits) {
		ctx.SetData("nextPage", pageInt+1)
	}
	ctx.SetData("prevLabel", ctx.TrH("pagination.previous"))
	ctx.SetData("nextLabel", ctx.TrH("pagination.next"))
	ctx.SetData("urlPage", "search")
	ctx.SetData("urlParams", template.URL("&q="+ctx.QueryParam("q")))
	ctx.SetData("htmlTitle", ctx.TrH("gist.list.search-results"))
	ctx.SetData("nbHits", nbHits)
	ctx.SetData("gists", renderedGists)
	ctx.SetData("langs", langs)
	ctx.SetData("searchQuery", ctx.QueryParam("q"))
	return ctx.Html("search.html")
}
