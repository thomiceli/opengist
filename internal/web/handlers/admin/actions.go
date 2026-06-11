package admin

import (
	"github.com/thomiceli/opengist/internal/actions"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/web/context"
)

// actionView is the template model for a single row on the admin actions page.
type actionView struct {
	Path     string // POST endpoint suffix under /admin-panel, e.g. "sync-fs"
	LabelKey string // i18n key for the action's label
	Running  bool   // currently in progress in this instance
	Periodic bool   // also runs automatically on a schedule
	Spec     string // raw schedule spec, e.g. "@every 72h"; "" if not periodic
}

// adminActions lists every action shown on the actions page, in display order.
// The Path values match the POST routes registered in the router.
var adminActions = []struct {
	Type int
	Path string
	Key  string
}{
	{actions.SyncReposFromFS, "sync-fs", "admin.actions.sync-fs"},
	{actions.SyncReposFromDB, "sync-db", "admin.actions.sync-db"},
	{actions.GitGcRepos, "gc-repos", "admin.actions.git-gc"},
	{actions.SyncGistPreviews, "sync-previews", "admin.actions.sync-previews"},
	{actions.ResetHooks, "reset-hooks", "admin.actions.reset-hooks"},
	{actions.IndexGists, "index-gists", "admin.actions.index-gists"},
	{actions.SyncGistLanguages, "sync-languages", "admin.actions.sync-gist-languages"},
	{actions.DeleteExpiredGists, "delete-expired-gists", "admin.actions.delete-expired-gists"},
	{actions.SyncSSHKeys, "sync-ssh-keys", "admin.actions.sync-ssh-keys"},
}

func AdminActions(ctx *context.Context) error {
	ctx.SetData("htmlTitle", ctx.TrH("admin.actions")+" - "+ctx.TrH("admin.admin_panel"))
	ctx.SetData("adminHeaderPage", "actions")

	manageSSHKeys := config.C.SshManagesAuthorizedKeys()

	views := make([]actionView, 0, len(adminActions))
	anyRunning := false
	for _, a := range adminActions {
		// The authorized_keys action is only relevant when Opengist manages the file.
		if a.Type == actions.SyncSSHKeys && !manageSSHKeys {
			continue
		}

		running := actions.IsRunning(a.Type)
		if running {
			anyRunning = true
		}

		views = append(views, actionView{
			Path:     a.Path,
			LabelKey: a.Key,
			Running:  running,
			Periodic: actions.IsPeriodic(a.Type),
			Spec:     actions.Spec(a.Type),
		})
	}

	ctx.SetData("actions", views)
	ctx.SetData("anyRunning", anyRunning)
	// Set when arriving right after triggering a run: keeps the list polling for
	// a moment even if the goroutine hasn't flipped the running flag yet.
	ctx.SetData("pollNow", ctx.QueryParam("run") == "1")
	return ctx.Html("admin_actions.html")
}

func AdminSyncReposFromFS(ctx *context.Context) error {
	go actions.RunOnce(actions.SyncReposFromFS)
	return ctx.RedirectTo("/admin-panel/actions?run=1")
}

func AdminSyncReposFromDB(ctx *context.Context) error {
	go actions.RunOnce(actions.SyncReposFromDB)
	return ctx.RedirectTo("/admin-panel/actions?run=1")
}

func AdminGcRepos(ctx *context.Context) error {
	go actions.RunOnce(actions.GitGcRepos)
	return ctx.RedirectTo("/admin-panel/actions?run=1")
}

func AdminSyncGistPreviews(ctx *context.Context) error {
	go actions.RunOnce(actions.SyncGistPreviews)
	return ctx.RedirectTo("/admin-panel/actions?run=1")
}

func AdminResetHooks(ctx *context.Context) error {
	go actions.RunOnce(actions.ResetHooks)
	return ctx.RedirectTo("/admin-panel/actions?run=1")
}

func AdminIndexGists(ctx *context.Context) error {
	go actions.RunOnce(actions.IndexGists)
	return ctx.RedirectTo("/admin-panel/actions?run=1")
}

func AdminSyncGistLanguages(ctx *context.Context) error {
	go actions.RunOnce(actions.SyncGistLanguages)
	return ctx.RedirectTo("/admin-panel/actions?run=1")
}

func AdminDeleteExpiredGists(ctx *context.Context) error {
	go actions.RunOnce(actions.DeleteExpiredGists)
	return ctx.RedirectTo("/admin-panel/actions?run=1")
}

func AdminSyncSSHKeys(ctx *context.Context) error {
	go actions.RunOnce(actions.SyncSSHKeys)
	return ctx.RedirectTo("/admin-panel/actions?run=1")
}
