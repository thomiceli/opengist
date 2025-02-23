package server

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/index"
	"github.com/thomiceli/opengist/internal/web/context"
	"github.com/thomiceli/opengist/internal/web/handlers/admin"
	"github.com/thomiceli/opengist/internal/web/handlers/auth"
	"github.com/thomiceli/opengist/internal/web/handlers/gist"
	"github.com/thomiceli/opengist/internal/web/handlers/git"
	"github.com/thomiceli/opengist/internal/web/handlers/health"
	"github.com/thomiceli/opengist/internal/web/handlers/metrics"
	"github.com/thomiceli/opengist/internal/web/handlers/settings"
	"github.com/thomiceli/opengist/public"
)

func (s *Server) registerRoutes() {
	r := NewRouter(s.echo.Group(""))

	{
		r.GET("/", gist.Create, logged)
		r.POST("/", gist.ProcessCreate, logged)
		r.POST("/preview", gist.Preview, logged)

		r.GET("/healthcheck", health.Healthcheck)

		if config.C.MetricsEnabled {
			r.GET("/metrics", metrics.Metrics)
		}

		r.GET("/register", auth.Register)
		r.POST("/register", auth.ProcessRegister)
		r.GET("/login", auth.Login)
		r.POST("/login", auth.ProcessLogin)
		r.GET("/logout", auth.Logout)
		r.GET("/oauth/:provider", auth.Oauth)
		r.GET("/oauth/:provider/callback", auth.OauthCallback)
		r.GET("/oauth/:provider/unlink", auth.OauthUnlink, logged)
		r.POST("/webauthn/bind", auth.BeginWebAuthnBinding, logged)
		r.POST("/webauthn/bind/finish", auth.FinishWebAuthnBinding, logged)
		r.POST("/webauthn/login", auth.BeginWebAuthnLogin)
		r.POST("/webauthn/login/finish", auth.FinishWebAuthnLogin)
		r.POST("/webauthn/assertion", auth.BeginWebAuthnAssertion, inMFASession)
		r.POST("/webauthn/assertion/finish", auth.FinishWebAuthnAssertion, inMFASession)
		r.GET("/mfa", auth.Mfa, inMFASession)
		r.POST("/mfa/totp/assertion", auth.AssertTotp, inMFASession)

		sA := r.SubGroup("/settings")
		{
			sA.Use(logged)
			sA.GET("", settings.UserSettings)
			sA.POST("/email", settings.EmailProcess)
			sA.DELETE("/account", settings.AccountDeleteProcess)
			sA.POST("/ssh-keys", settings.SshKeysProcess)
			sA.DELETE("/ssh-keys/:id", settings.SshKeysDelete)
			sA.DELETE("/passkeys/:id", settings.PasskeyDelete)
			sA.PUT("/password", settings.PasswordProcess)
			sA.PUT("/username", settings.UsernameProcess)
			sA.GET("/totp/generate", auth.BeginTotp)
			sA.POST("/totp/generate", auth.FinishTotp)
			sA.DELETE("/totp", auth.DisableTotp)
			sA.POST("/totp/regenerate", auth.RegenerateTotpRecoveryCodes)
		}

		sB := r.SubGroup("/admin-panel")
		{
			sB.Use(adminPermission)
			sB.GET("", admin.AdminIndex)
			sB.GET("/users", admin.AdminUsers)
			sB.POST("/users/:user/delete", admin.AdminUserDelete)
			sB.GET("/gists", admin.AdminGists)
			sB.POST("/gists/:gist/delete", admin.AdminGistDelete)
			sB.GET("/invitations", admin.AdminInvitations)
			sB.POST("/invitations", admin.AdminInvitationsCreate)
			sB.POST("/invitations/:id/delete", admin.AdminInvitationsDelete)
			sB.POST("/sync-fs", admin.AdminSyncReposFromFS)
			sB.POST("/sync-db", admin.AdminSyncReposFromDB)
			sB.POST("/gc-repos", admin.AdminGcRepos)
			sB.POST("/sync-previews", admin.AdminSyncGistPreviews)
			sB.POST("/reset-hooks", admin.AdminResetHooks)
			sB.POST("/index-gists", admin.AdminIndexGists)
			sB.POST("/sync-languages", admin.AdminSyncGistLanguages)
			sB.GET("/configuration", admin.AdminConfig)
			sB.PUT("/set-config", admin.AdminSetConfig)
		}

		if config.C.HttpGit {
			r.Any("/init/*", git.GitHttp, gistNewPushSoftInit)
		}

		r.GET("/all", gist.AllGists, checkRequireLogin, setAllGistsMode("all"))

		if index.IndexEnabled() {
			r.GET("/search", gist.Search, checkRequireLogin)
		} else {
			r.GET("/search", gist.AllGists, checkRequireLogin, setAllGistsMode("search"))
		}

		r.GET("/:user", gist.AllGists, checkRequireLogin, setAllGistsMode("fromUser"))
		r.GET("/:user/liked", gist.AllGists, checkRequireLogin, setAllGistsMode("liked"))
		r.GET("/:user/forked", gist.AllGists, checkRequireLogin, setAllGistsMode("forked"))

		r.GET("/topics/:topic", gist.AllGists, checkRequireLogin, setAllGistsMode("topics"))

		sC := r.SubGroup("/:user/:gistname")
		{
			sC.Use(makeCheckRequireLogin(true), gistInit)
			sC.GET("", gist.GistIndex)
			sC.GET("/rev/:revision", gist.GistIndex)
			sC.GET("/revisions", gist.Revisions)
			sC.GET("/archive/:revision", gist.DownloadZip)
			sC.POST("/visibility", gist.EditVisibility, logged, writePermission)
			sC.POST("/delete", gist.DeleteGist, logged, writePermission)
			sC.GET("/raw/:revision/:file", gist.RawFile)
			sC.GET("/download/:revision/:file", gist.DownloadFile)
			sC.GET("/edit", gist.Edit, logged, writePermission)
			sC.POST("/edit", gist.ProcessCreate, logged, writePermission)
			sC.POST("/like", gist.Like, logged)
			sC.GET("/likes", gist.Likes, checkRequireLogin)
			sC.POST("/fork", gist.Fork, logged)
			sC.GET("/forks", gist.Forks, checkRequireLogin)
			sC.PUT("/checkbox", gist.Checkbox, logged, writePermission)
		}
	}

	customFs := os.DirFS(filepath.Join(config.GetHomeDir(), "custom"))
	r.GET("/assets/*", func(ctx *context.Context) error {
		if _, err := public.Files.Open(path.Join("assets", ctx.Param("*"))); !s.dev && err == nil {
			ctx.Response().Header().Set("Cache-Control", "public, max-age=31536000")
			ctx.Response().Header().Set("Expires", time.Now().AddDate(1, 0, 0).Format(http.TimeFormat))

			return echo.WrapHandler(http.FileServer(http.FS(public.Files)))(ctx)
		}

		// if the custom file is an .html template, render it
		if strings.HasSuffix(ctx.Param("*"), ".html") {
			if err := ctx.Html(ctx.Param("*")); err != nil {
				return ctx.NotFound("Page not found")
			}
			return nil
		}

		return echo.WrapHandler(http.StripPrefix("/assets/", http.FileServer(http.FS(customFs))))(ctx)
	})

	// Git HTTP routes
	if config.C.HttpGit {
		r.Any("/:user/:gistname/*", git.GitHttp, gistSoftInit)
	}

	r.Any("/*", noRouteFound)
}

// Router wraps echo.Group to provide custom Handler support
type Router struct {
	*echo.Group
}

func NewRouter(g *echo.Group) *Router {
	return &Router{Group: g}
}

func (r *Router) SubGroup(prefix string, m ...Middleware) *Router {
	echoMiddleware := make([]echo.MiddlewareFunc, len(m))
	for i, mw := range m {
		mw := mw // capture for closure
		echoMiddleware[i] = func(next echo.HandlerFunc) echo.HandlerFunc {
			return chain(func(c *context.Context) error {
				return next(c)
			}, mw).toEchoHandler()
		}
	}
	return NewRouter(r.Group.Group(prefix, echoMiddleware...))
}

func (r *Router) GET(path string, h Handler, m ...Middleware) {
	r.Group.GET(path, chain(h, m...).toEchoHandler())
}

func (r *Router) POST(path string, h Handler, m ...Middleware) {
	r.Group.POST(path, chain(h, m...).toEchoHandler())
}

func (r *Router) PUT(path string, h Handler, m ...Middleware) {
	r.Group.PUT(path, chain(h, m...).toEchoHandler())
}

func (r *Router) DELETE(path string, h Handler, m ...Middleware) {
	r.Group.DELETE(path, chain(h, m...).toEchoHandler())
}

func (r *Router) PATCH(path string, h Handler, m ...Middleware) {
	r.Group.PATCH(path, chain(h, m...).toEchoHandler())
}

func (r *Router) Any(path string, h Handler, m ...Middleware) {
	r.Group.Any(path, chain(h, m...).toEchoHandler())
}

func (r *Router) Use(middleware ...Middleware) {
	for _, m := range middleware {
		m := m // capture for closure
		r.Group.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
			return chain(func(c *context.Context) error {
				return next(c)
			}, m).toEchoHandler()
		})
	}
}
