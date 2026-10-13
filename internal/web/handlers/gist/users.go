package gist

import (
	"strings"

	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
	"github.com/thomiceli/opengist/internal/web/handlers"
)

// Users lists all users for the explore "Users" page, each with their avatar,
// join date and number of visible gists. The list is paginated to 10 users per
// page and sortable by username or join date, ascending or descending.
func Users(ctx *context.Context) error {
	var currentUserId uint
	if ctx.User != nil {
		currentUserId = ctx.User.ID
	}

	pageInt := handlers.GetPage(ctx)

	// Resolve the sort field and order, defaulting to username ascending.
	sort := "username"
	sortColumn := "username_normalized"
	if ctx.QueryParam("sort") == "joined" {
		sort = "joined"
		sortColumn = "created_at"
	}

	order := "asc"
	if ctx.QueryParam("order") == "desc" {
		order = "desc"
	}

	query := strings.TrimSpace(ctx.QueryParam("q"))

	users, err := db.GetUsersWithGistCounts(currentUserId, query, pageInt-1, 11, 10, sortColumn, order)
	if err != nil {
		return ctx.ErrorRes(500, "Error fetching users", err)
	}

	pagination := &handlers.PaginationParams{
		Sort:  sort,
		Order: order,
		Query: query,
	}

	if err = handlers.Paginate(ctx, users, pageInt, 10, "users", "-/users", 1, pagination); err != nil {
		return ctx.ErrorRes(404, ctx.Tr("error.page-not-found"), nil)
	}

	ctx.SetData("sort", sort)
	ctx.SetData("order", order)
	ctx.SetData("searchQuery", query)
	ctx.SetData("currentPage", "users")
	ctx.SetData("htmlTitle", ctx.TrH("gist.list.users"))
	return ctx.Html("explore_users.html")
}
