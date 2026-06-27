package admin

import (
	"github.com/thomiceli/opengist/internal/actions"
	"github.com/thomiceli/opengist/internal/web/context"
)

func AdminSyncReposFromFS(ctx *context.Context) error {
	ctx.AddFlash(ctx.Tr("flash.admin.sync-fs"), "success")
	go actions.RunOnce(actions.SyncReposFromFS)
	return ctx.RedirectTo("/admin-panel")
}

func AdminSyncReposFromDB(ctx *context.Context) error {
	ctx.AddFlash(ctx.Tr("flash.admin.sync-db"), "success")
	go actions.RunOnce(actions.SyncReposFromDB)
	return ctx.RedirectTo("/admin-panel")
}

func AdminGcRepos(ctx *context.Context) error {
	ctx.AddFlash(ctx.Tr("flash.admin.git-gc"), "success")
	go actions.RunOnce(actions.GitGcRepos)
	return ctx.RedirectTo("/admin-panel")
}

func AdminSyncGistPreviews(ctx *context.Context) error {
	ctx.AddFlash(ctx.Tr("flash.admin.sync-previews"), "success")
	go actions.RunOnce(actions.SyncGistPreviews)
	return ctx.RedirectTo("/admin-panel")
}

func AdminResetHooks(ctx *context.Context) error {
	ctx.AddFlash(ctx.Tr("flash.admin.reset-hooks"), "success")
	go actions.RunOnce(actions.ResetHooks)
	return ctx.RedirectTo("/admin-panel")
}

func AdminIndexGists(ctx *context.Context) error {
	ctx.AddFlash(ctx.Tr("flash.admin.index-gists"), "success")
	go actions.RunOnce(actions.IndexGists)
	return ctx.RedirectTo("/admin-panel")
}

func AdminSyncGistLanguages(ctx *context.Context) error {
	ctx.AddFlash(ctx.Tr("flash.admin.sync-gist-languages"), "success")
	go actions.RunOnce(actions.SyncGistLanguages)
	return ctx.RedirectTo("/admin-panel")
}

func AdminDeleteExpiredGists(ctx *context.Context) error {
	ctx.AddFlash(ctx.Tr("flash.admin.delete-expired-gists"), "success")
	go actions.RunOnce(actions.DeleteExpiredGists)
	return ctx.RedirectTo("/admin-panel")
}

func AdminSyncSSHKeys(ctx *context.Context) error {
	ctx.AddFlash(ctx.Tr("flash.admin.sync-ssh-keys"), "success")
	go actions.RunOnce(actions.SyncSSHKeys)
	return ctx.RedirectTo("/admin-panel")
}
