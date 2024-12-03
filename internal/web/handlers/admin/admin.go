package admin

import (
	"github.com/thomiceli/opengist/internal/actions"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/web/context"
	"github.com/thomiceli/opengist/internal/web/handlers"
	"runtime"
	"strconv"
	"time"
)

func AdminIndex(ctx *context.Context) error {
	ctx.SetData("htmlTitle", ctx.TrH("admin.admin_panel"))
	ctx.SetData("adminHeaderPage", "index")

	ctx.SetData("opengistVersion", config.OpengistVersion)
	ctx.SetData("goVersion", runtime.Version())
	gitVersion, err := git.GetGitVersion()
	if err != nil {
		return ctx.ErrorRes(500, "Cannot get git version", err)
	}
	ctx.SetData("gitVersion", gitVersion)

	countUsers, err := db.CountAll(&db.User{})
	if err != nil {
		return ctx.ErrorRes(500, "Cannot count users", err)
	}
	ctx.SetData("countUsers", countUsers)

	countGists, err := db.CountAll(&db.Gist{})
	if err != nil {
		return ctx.ErrorRes(500, "Cannot count gists", err)
	}
	ctx.SetData("countGists", countGists)

	countKeys, err := db.CountAll(&db.SSHKey{})
	if err != nil {
		return ctx.ErrorRes(500, "Cannot count SSH keys", err)
	}
	ctx.SetData("countKeys", countKeys)

	ctx.SetData("syncReposFromFS", actions.IsRunning(actions.SyncReposFromFS))
	ctx.SetData("syncReposFromDB", actions.IsRunning(actions.SyncReposFromDB))
	ctx.SetData("gitGcRepos", actions.IsRunning(actions.GitGcRepos))
	ctx.SetData("syncGistPreviews", actions.IsRunning(actions.SyncGistPreviews))
	ctx.SetData("resetHooks", actions.IsRunning(actions.ResetHooks))
	ctx.SetData("indexGists", actions.IsRunning(actions.IndexGists))
	return ctx.Html("admin_index.html")
}

func AdminUsers(ctx *context.Context) error {
	ctx.SetData("htmlTitle", ctx.TrH("admin.users")+" - "+ctx.TrH("admin.admin_panel"))
	ctx.SetData("adminHeaderPage", "users")
	ctx.SetData("loadStartTime", time.Now())

	pageInt := handlers.GetPage(ctx)

	var data []*db.User
	var err error
	if data, err = db.GetAllUsers(pageInt - 1); err != nil {
		return ctx.ErrorRes(500, "Cannot get users", err)
	}

	if err = handlers.Paginate(ctx, data, pageInt, 10, "data", "admin-panel/users", 1); err != nil {
		return ctx.ErrorRes(404, ctx.Tr("error.page-not-found"), nil)
	}

	return ctx.Html("admin_users.html")
}

func AdminGists(ctx *context.Context) error {
	ctx.SetData("htmlTitle", ctx.TrH("admin.gists")+" - "+ctx.TrH("admin.admin_panel"))
	ctx.SetData("adminHeaderPage", "gists")
	pageInt := handlers.GetPage(ctx)

	var data []*db.Gist
	var err error
	if data, err = db.GetAllGists(pageInt - 1); err != nil {
		return ctx.ErrorRes(500, "Cannot get gists", err)
	}

	if err = handlers.Paginate(ctx, data, pageInt, 10, "data", "admin-panel/gists", 1); err != nil {
		return ctx.ErrorRes(404, ctx.Tr("error.page-not-found"), nil)
	}

	return ctx.Html("admin_gists.html")
}

func AdminUserDelete(ctx *context.Context) error {
	userId, _ := strconv.ParseUint(ctx.Param("user"), 10, 64)
	user, err := db.GetUserById(uint(userId))
	if err != nil {
		return ctx.ErrorRes(500, "Cannot retrieve user", err)
	}

	if err := user.Delete(); err != nil {
		return ctx.ErrorRes(500, "Cannot delete this user", err)
	}

	ctx.AddFlash(ctx.Tr("flash.admin.user-deleted"), "success")
	return ctx.RedirectTo("/admin-panel/users")
}

func AdminGistDelete(ctx *context.Context) error {
	gist, err := db.GetGistByID(ctx.Param("gist"))
	if err != nil {
		return ctx.ErrorRes(500, "Cannot retrieve gist", err)
	}

	if err = gist.DeleteRepository(); err != nil {
		return ctx.ErrorRes(500, "Cannot delete the repository", err)
	}

	if err = gist.Delete(); err != nil {
		return ctx.ErrorRes(500, "Cannot delete this gist", err)
	}

	gist.RemoveFromIndex()

	ctx.AddFlash(ctx.Tr("flash.admin.gist-deleted"), "success")
	return ctx.RedirectTo("/admin-panel/gists")
}

func AdminConfig(ctx *context.Context) error {
	ctx.SetData("htmlTitle", ctx.TrH("admin.configuration")+" - "+ctx.TrH("admin.admin_panel"))
	ctx.SetData("adminHeaderPage", "config")

	ctx.SetData("dbtype", db.DatabaseInfo.Type.String())
	ctx.SetData("dbname", db.DatabaseInfo.Database)

	return ctx.Html("admin_config.html")
}

func AdminSetConfig(ctx *context.Context) error {
	key := ctx.FormValue("key")
	value := ctx.FormValue("value")

	if err := db.UpdateSetting(key, value); err != nil {
		return ctx.ErrorRes(500, "Cannot set setting", err)
	}

	return ctx.JSON(200, map[string]interface{}{
		"success": true,
	})
}

func AdminInvitations(ctx *context.Context) error {
	ctx.SetData("htmlTitle", ctx.TrH("admin.invitations")+" - "+ctx.TrH("admin.admin_panel"))
	ctx.SetData("adminHeaderPage", "invitations")

	var invitations []*db.Invitation
	var err error
	if invitations, err = db.GetAllInvitations(); err != nil {
		return ctx.ErrorRes(500, "Cannot get invites", err)
	}

	ctx.SetData("invitations", invitations)
	return ctx.Html("admin_invitations.html")
}

func AdminInvitationsCreate(ctx *context.Context) error {
	code := ctx.FormValue("code")
	nbMax, err := strconv.ParseUint(ctx.FormValue("nbMax"), 10, 64)
	if err != nil {
		nbMax = 10
	}

	expiresAtUnix, err := strconv.ParseInt(ctx.FormValue("expiredAtUnix"), 10, 64)
	if err != nil {
		expiresAtUnix = time.Now().Unix() + 604800 // 1 week
	}

	invitation := &db.Invitation{
		Code:      code,
		ExpiresAt: expiresAtUnix,
		NbMax:     uint(nbMax),
	}

	if err := invitation.Create(); err != nil {
		return ctx.ErrorRes(500, "Cannot create invitation", err)
	}

	ctx.AddFlash(ctx.Tr("flash.admin.invitation-created"), "success")
	return ctx.RedirectTo("/admin-panel/invitations")
}

func AdminInvitationsDelete(ctx *context.Context) error {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	invitation, err := db.GetInvitationByID(uint(id))
	if err != nil {
		return ctx.ErrorRes(500, "Cannot retrieve invitation", err)
	}

	if err := invitation.Delete(); err != nil {
		return ctx.ErrorRes(500, "Cannot delete this invitation", err)
	}

	ctx.AddFlash(ctx.Tr("flash.admin.invitation-deleted"), "success")
	return ctx.RedirectTo("/admin-panel/invitations")
}
