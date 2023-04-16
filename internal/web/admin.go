package web

import (
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"opengist/internal/config"
	"opengist/internal/git"
	"opengist/internal/models"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

var (
	syncReposFromFS = false
	syncReposFromDB = false
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

	setData(ctx, "syncReposFromFS", syncReposFromFS)
	setData(ctx, "syncReposFromDB", syncReposFromDB)
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

	if err = paginate(ctx, data, pageInt, 10, "data", "admin-panel/users", 1); err != nil {
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

	if err = paginate(ctx, data, pageInt, 10, "data", "admin-panel/gists", 1); err != nil {
		return errorRes(404, "Page not found", nil)
	}

	return html(ctx, "admin_gists.html")
}

func adminUserDelete(ctx echo.Context) error {
	userId, _ := strconv.ParseUint(ctx.Param("user"), 10, 64)
	user, err := models.GetUserById(uint(userId))
	if err != nil {
		return errorRes(500, "Cannot retrieve user", err)
	}

	if err := user.Delete(); err != nil {
		return errorRes(500, "Cannot delete this user", err)
	}

	addFlash(ctx, "User has been deleted", "success")
	return redirect(ctx, "/admin-panel/users")
}

func adminGistDelete(ctx echo.Context) error {
	gist, err := models.GetGistByID(ctx.Param("gist"))
	if err != nil {
		return errorRes(500, "Cannot retrieve gist", err)
	}

	if err = gist.DeleteRepository(); err != nil {
		return errorRes(500, "Cannot delete the repository", err)
	}

	if err = gist.Delete(); err != nil {
		return errorRes(500, "Cannot delete this gist", err)
	}

	addFlash(ctx, "Gist has been deleted", "success")
	return redirect(ctx, "/admin-panel/gists")
}

func adminSyncReposFromFS(ctx echo.Context) error {
	addFlash(ctx, "Syncing repositories from filesystem...", "success")
	go func() {
		if syncReposFromFS {
			return
		}
		syncReposFromFS = true

		gists, err := models.GetAllGistsRows()
		if err != nil {
			log.Error().Err(err).Msg("Cannot get gists")
			syncReposFromFS = false
			return
		}
		for _, gist := range gists {
			// if repository does not exist, delete gist from database
			if _, err := os.Stat(git.RepositoryPath(gist.User.Username, gist.Uuid)); err != nil && !os.IsExist(err) {
				if err2 := gist.Delete(); err2 != nil {
					log.Error().Err(err2).Msg("Cannot delete gist")
					syncReposFromFS = false
					return
				}
			}
		}
		syncReposFromFS = false
	}()
	return redirect(ctx, "/admin-panel")
}

func adminSyncReposFromDB(ctx echo.Context) error {
	addFlash(ctx, "Syncing repositories from database...", "success")
	go func() {
		if syncReposFromDB {
			return
		}
		syncReposFromDB = true
		entries, err := filepath.Glob(filepath.Join(config.GetHomeDir(), "repos", "*", "*"))
		if err != nil {
			log.Error().Err(err).Msg("Cannot read repos directories")
			syncReposFromDB = false
			return
		}

		for _, e := range entries {
			path := strings.Split(e, string(os.PathSeparator))
			gist, _ := models.GetGist(path[len(path)-2], path[len(path)-1])

			if gist.ID == 0 {
				if err := git.DeleteRepository(path[len(path)-2], path[len(path)-1]); err != nil {
					log.Error().Err(err).Msg("Cannot delete repository")
					syncReposFromDB = false
					return
				}
			}
		}
		syncReposFromDB = false
		return
	}()
	return redirect(ctx, "/admin-panel")
}

func adminSetSetting(ctx echo.Context) error {
	key := ctx.FormValue("key")
	value := ctx.FormValue("value")

	if err := models.UpdateSetting(key, value); err != nil {
		return errorRes(500, "Cannot set setting", err)
	}

	return ctx.JSON(200, map[string]interface{}{
		"success": true,
	})
}
