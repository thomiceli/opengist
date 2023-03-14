package web

import (
	"github.com/labstack/echo/v4"
	"opengist/internal/config"
	"opengist/internal/git"
	"opengist/internal/models"
	"runtime"
)

func adminIndex(ctx echo.Context) error {
	setData(ctx, "title", "Admin panel")
	setData(ctx, "adminHeaderPage", "index")

	setData(ctx, "opengistVersion", config.OpengistVersion)
	setData(ctx, "goVersion", runtime.Version())
	gitVersion, err := git.GetGitVersion()
	if err != nil {
		return errorRes(500, "Cannot get git version", err)
	}
	setData(ctx, "gitVersion", gitVersion)

	countUsers, err := models.CountAll(&models.User{})
	if err != nil {
		return errorRes(500, "Cannot count users", err)
	}
	setData(ctx, "countUsers", countUsers)

	countGists, err := models.CountAll(&models.Gist{})
	if err != nil {
		return errorRes(500, "Cannot count gists", err)
	}
	setData(ctx, "countGists", countGists)

	countKeys, err := models.CountAll(&models.SSHKey{})
	if err != nil {
		return errorRes(500, "Cannot count SSH keys", err)
	}
	setData(ctx, "countKeys", countKeys)

	return html(ctx, "admin_index.html")
}

func adminUsers(ctx echo.Context) error {
	setData(ctx, "title", "Users")
	setData(ctx, "adminHeaderPage", "users")
	pageInt := getPage(ctx)

	var data []*models.User
	var err error
	if data, err = models.GetAllUsers(pageInt - 1); err != nil {
		return errorRes(500, "Cannot get users", err)
	}

	if err = paginate(ctx, data, pageInt, 10, "data", "admin/users"); err != nil {
		return errorRes(404, "Page not found", nil)
	}

	return html(ctx, "admin_users.html")
}

func adminGists(ctx echo.Context) error {
	setData(ctx, "title", "Users")
	setData(ctx, "adminHeaderPage", "gists")
	pageInt := getPage(ctx)

	var data []*models.Gist
	var err error
	if data, err = models.GetAllGists(pageInt - 1); err != nil {
		return errorRes(500, "Cannot get gists", err)
	}

	if err = paginate(ctx, data, pageInt, 10, "data", "admin/gists"); err != nil {
		return errorRes(404, "Page not found", nil)
	}

	return html(ctx, "admin_gists.html")
}

func adminUserDelete(ctx echo.Context) error {
	if err := models.DeleteUserByID(ctx.Param("user")); err != nil {
		return errorRes(500, "Cannot delete this user", err)
	}

	addFlash(ctx, "User has been deleted", "success")
	return redirect(ctx, "/admin/users")
}

func adminGistDelete(ctx echo.Context) error {
	gist, err := models.GetGistByID(ctx.Param("gist"))
	if err != nil {
		return errorRes(500, "Cannot retrieve gist", err)
	}

	if err = git.DeleteRepository(gist.User.Username, gist.Uuid); err != nil {
		return errorRes(500, "Cannot delete the repository", err)
	}

	if err = models.DeleteGist(gist); err != nil {
		return errorRes(500, "Cannot delete this gist", err)
	}

	addFlash(ctx, "Gist has been deleted", "success")
	return redirect(ctx, "/admin/gists")

}
