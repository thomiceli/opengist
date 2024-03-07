package web

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/thomiceli/opengist/internal/actions"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"runtime"
	"strconv"
	"time"
)

func adminIndex(ctx echo.Context) error {
	setData(ctx, "title", "Admin panel")
	setData(ctx, "htmlTitle", "Admin panel")
	setData(ctx, "adminHeaderPage", "index")

	setData(ctx, "opengistVersion", config.OpengistVersion)
	setData(ctx, "goVersion", runtime.Version())
	gitVersion, err := git.GetGitVersion()
	if err != nil {
		return errorRes(500, "Cannot get git version", err)
	}
	setData(ctx, "gitVersion", gitVersion)

	countUsers, err := db.CountAll(&db.User{})
	if err != nil {
		return errorRes(500, "Cannot count users", err)
	}
	setData(ctx, "countUsers", countUsers)

	countGists, err := db.CountAll(&db.Gist{})
	if err != nil {
		return errorRes(500, "Cannot count gists", err)
	}
	setData(ctx, "countGists", countGists)

	countKeys, err := db.CountAll(&db.SSHKey{})
	if err != nil {
		return errorRes(500, "Cannot count SSH keys", err)
	}
	setData(ctx, "countKeys", countKeys)

	setData(ctx, "syncReposFromFS", actions.IsRunning(actions.SyncReposFromFS))
	setData(ctx, "syncReposFromDB", actions.IsRunning(actions.SyncReposFromDB))
	setData(ctx, "gitGcRepos", actions.IsRunning(actions.GitGcRepos))
	setData(ctx, "syncGistPreviews", actions.IsRunning(actions.SyncGistPreviews))
	setData(ctx, "resetHooks", actions.IsRunning(actions.ResetHooks))
	setData(ctx, "indexGists", actions.IsRunning(actions.IndexGists))
	return html(ctx, "admin_index.html")
}

func adminUsers(ctx echo.Context) error {
	setData(ctx, "title", "Users")
	setData(ctx, "htmlTitle", "Users - Admin panel")
	setData(ctx, "adminHeaderPage", "users")
	pageInt := getPage(ctx)

	var data []*db.User
	var err error
	if data, err = db.GetAllUsers(pageInt - 1); err != nil {
		return errorRes(500, "Cannot get users", err)
	}

	if err = paginate(ctx, data, pageInt, 10, "data", "admin-panel/users", 1); err != nil {
		return errorRes(404, "Page not found", nil)
	}

	return html(ctx, "admin_users.html")
}

func adminGists(ctx echo.Context) error {
	setData(ctx, "title", "Gists")
	setData(ctx, "htmlTitle", "Gists - Admin panel")
	setData(ctx, "adminHeaderPage", "gists")
	pageInt := getPage(ctx)

	var data []*db.Gist
	var err error
	if data, err = db.GetAllGists(pageInt - 1); err != nil {
		return errorRes(500, "Cannot get gists", err)
	}

	if err = paginate(ctx, data, pageInt, 10, "data", "admin-panel/gists", 1); err != nil {
		return errorRes(404, "Page not found", nil)
	}

	return html(ctx, "admin_gists.html")
}

func adminUserDelete(ctx echo.Context) error {
	userId, _ := strconv.ParseUint(ctx.Param("user"), 10, 64)
	user, err := db.GetUserById(uint(userId))
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
	gist, err := db.GetGistByID(ctx.Param("gist"))
	if err != nil {
		return errorRes(500, "Cannot retrieve gist", err)
	}

	if err = gist.DeleteRepository(); err != nil {
		return errorRes(500, "Cannot delete the repository", err)
	}

	if err = gist.Delete(); err != nil {
		return errorRes(500, "Cannot delete this gist", err)
	}

	gist.RemoveFromIndex()

	addFlash(ctx, "Gist has been deleted", "success")
	return redirect(ctx, "/admin-panel/gists")
}

func adminSyncReposFromFS(ctx echo.Context) error {
	addFlash(ctx, "Syncing repositories from filesystem...", "success")
	go actions.Run(actions.SyncReposFromFS)
	return redirect(ctx, "/admin-panel")
}

func adminSyncReposFromDB(ctx echo.Context) error {
	addFlash(ctx, "Syncing repositories from database...", "success")
	go actions.Run(actions.SyncReposFromDB)
	return redirect(ctx, "/admin-panel")
}

func adminGcRepos(ctx echo.Context) error {
	addFlash(ctx, "Garbage collecting repositories...", "success")
	go actions.Run(actions.GitGcRepos)
	return redirect(ctx, "/admin-panel")
}

func adminSyncGistPreviews(ctx echo.Context) error {
	addFlash(ctx, "Syncing Gist previews...", "success")
	go actions.Run(actions.SyncGistPreviews)
	return redirect(ctx, "/admin-panel")
}

func adminResetHooks(ctx echo.Context) error {
	addFlash(ctx, "Resetting Git server hooks for all repositories...", "success")
	go actions.Run(actions.ResetHooks)
	return redirect(ctx, "/admin-panel")
}

func adminIndexGists(ctx echo.Context) error {
	addFlash(ctx, "Indexing all gists...", "success")
	go actions.Run(actions.IndexGists)
	return redirect(ctx, "/admin-panel")
}

func adminConfig(ctx echo.Context) error {
	setData(ctx, "title", "Configuration")
	setData(ctx, "htmlTitle", "Configuration - Admin panel")
	setData(ctx, "adminHeaderPage", "config")

	return html(ctx, "admin_config.html")
}

func adminSetConfig(ctx echo.Context) error {
	key := ctx.FormValue("key")
	value := ctx.FormValue("value")

	if err := db.UpdateSetting(key, value); err != nil {
		return errorRes(500, "Cannot set setting", err)
	}

	return ctx.JSON(200, map[string]interface{}{
		"success": true,
	})
}

func adminInvitations(ctx echo.Context) error {
	setData(ctx, "title", "Invitations")
	setData(ctx, "htmlTitle", "Invitations - Admin panel")
	setData(ctx, "adminHeaderPage", "invitations")

	var invitations []*db.Invitation
	var err error
	if invitations, err = db.GetAllInvitations(); err != nil {
		return errorRes(500, "Cannot get invites", err)
	}

	setData(ctx, "invitations", invitations)
	return html(ctx, "admin_invitations.html")
}

func adminInvitationsCreate(ctx echo.Context) error {
	code := ctx.FormValue("code")
	nbMax, err := strconv.ParseUint(ctx.FormValue("nbMax"), 10, 64)
	if err != nil {
		nbMax = 10
	}
	expiresAt := ctx.FormValue("expiresAt")
	var expiresAtUnix int64
	fmt.Println(expiresAt)
	if expiresAt == "" {
		expiresAtUnix = time.Now().Add(7 * 24 * time.Hour).Unix()
	} else {
		parsedDate, err := time.Parse("2006-01-02T15:04", expiresAt)
		if err != nil {
			return errorRes(400, "Invalid date format", err)
		}
		parsedDateUTC := time.Date(parsedDate.Year(), parsedDate.Month(), parsedDate.Day(), parsedDate.Hour(), parsedDate.Minute(), 0, 0, time.Local)
		expiresAtUnix = parsedDateUTC.Unix()
	}

	invitation := &db.Invitation{
		Code:      code,
		ExpiresAt: expiresAtUnix,
		NbMax:     uint(nbMax),
	}

	if err := invitation.Create(); err != nil {
		return errorRes(500, "Cannot create invitation", err)
	}

	addFlash(ctx, "Invitation has been created", "success")
	return redirect(ctx, "/admin-panel/invitations")
}

func adminInvitationsDelete(ctx echo.Context) error {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	invitation, err := db.GetInvitationByID(uint(id))
	if err != nil {
		return errorRes(500, "Cannot retrieve invitation", err)
	}

	if err := invitation.Delete(); err != nil {
		return errorRes(500, "Cannot delete this invitation", err)
	}

	addFlash(ctx, "Invitation has been deleted", "success")
	return redirect(ctx, "/admin-panel/invitations")
}
