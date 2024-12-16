package server

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/index"
	"github.com/thomiceli/opengist/internal/web/context"
	"github.com/thomiceli/opengist/internal/web/handler"
	"github.com/thomiceli/opengist/public"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

func (s *Server) setupRoutes() {
	r := NewRouter(s.echo.Group(""))

	// Web based routes
	{
		if !s.ignoreCsrf {
			r.Use(middleware.CSRFWithConfig(middleware.CSRFConfig{
				TokenLookup:    "form:_csrf,header:X-CSRF-Token",
				CookiePath:     "/",
				CookieHTTPOnly: true,
				CookieSameSite: http.SameSiteStrictMode,
			}))
			r.Use(csrfInit)
		}

		r.GET("/", handler.Create, logged)
		r.POST("/", handler.ProcessCreate, logged)
		r.POST("/preview", handler.Preview, logged)

		r.GET("/healthcheck", handler.Healthcheck)
		r.GET("/metrics", handler.Metrics)

		r.GET("/register", handler.Register)
		r.POST("/register", handler.ProcessRegister)
		r.GET("/login", handler.Login)
		r.POST("/login", handler.ProcessLogin)
		r.GET("/logout", handler.Logout)
		r.GET("/oauth/:provider", handler.Oauth)
		r.GET("/oauth/:provider/callback", handler.OauthCallback)
		r.GET("/oauth/:provider/unlink", handler.OauthUnlink, logged)
		r.POST("/webauthn/bind", handler.BeginWebAuthnBinding, logged)
		r.POST("/webauthn/bind/finish", handler.FinishWebAuthnBinding, logged)
		r.POST("/webauthn/login", handler.BeginWebAuthnLogin)
		r.POST("/webauthn/login/finish", handler.FinishWebAuthnLogin)
		r.POST("/webauthn/assertion", handler.BeginWebAuthnAssertion, inMFASession)
		r.POST("/webauthn/assertion/finish", handler.FinishWebAuthnAssertion, inMFASession)
		r.GET("/mfa", handler.Mfa, inMFASession)
		r.POST("/mfa/totp/assertion", handler.AssertTotp, inMFASession)

		r.GET("/settings", handler.UserSettings, logged)
		r.POST("/settings/email", handler.EmailProcess, logged)
		r.DELETE("/settings/account", handler.AccountDeleteProcess, logged)
		r.POST("/settings/ssh-keys", handler.SshKeysProcess, logged)
		r.DELETE("/settings/ssh-keys/:id", handler.SshKeysDelete, logged)
		r.DELETE("/settings/passkeys/:id", handler.PasskeyDelete, logged)
		r.PUT("/settings/password", handler.PasswordProcess, logged)
		r.PUT("/settings/username", handler.UsernameProcess, logged)
		r.GET("/settings/totp/generate", handler.BeginTotp, logged)
		r.POST("/settings/totp/generate", handler.FinishTotp, logged)
		r.DELETE("/settings/totp", handler.DisableTotp, logged)
		r.POST("/settings/totp/regenerate", handler.RegenerateTotpRecoveryCodes, logged)

		g2 := g1.Group("/admin-panel")
		{
			g2.Use(adminPermission)
			g2.GET("", adminIndex)
			g2.GET("/users", Handler(adminUsers).ToEcho())
			g2.POST("/users/:user/delete", adminUserDelete)
			g2.GET("/gists", adminGists)
			g2.POST("/gists/:gist/delete", adminGistDelete)
			g2.GET("/invitations", adminInvitations)
			g2.POST("/invitations", adminInvitationsCreate)
			g2.POST("/invitations/:id/delete", adminInvitationsDelete)
			g2.POST("/sync-fs", adminSyncReposFromFS)
			g2.POST("/sync-db", adminSyncReposFromDB)
			g2.POST("/gc-repos", adminGcRepos)
			g2.POST("/sync-previews", adminSyncGistPreviews)
			g2.POST("/reset-hooks", adminResetHooks)
			g2.POST("/index-gists", adminIndexGists)
			g2.GET("/configuration", adminConfig)
			g2.PUT("/set-config", adminSetConfig)
		}

		if config.C.HttpGit {
			e.Any("/init/*", gitHttp, gistNewPushSoftInit)
		}

		g1.GET("/all", allGists, checkRequireLogin)

		if index.Enabled() {
			g1.GET("/search", search, checkRequireLogin)
		} else {
			g1.GET("/search", allGists, checkRequireLogin)
		}

		g1.GET("/:user", allGists, checkRequireLogin)
		g1.GET("/:user/liked", allGists, checkRequireLogin)
		g1.GET("/:user/forked", allGists, checkRequireLogin)

		g3 := g1.Group("/:user/:gistname")
		{
			g3.Use(makeCheckRequireLogin(true), gistInit)
			g3.GET("", gistIndex)
			g3.GET("/rev/:revision", gistIndex)
			g3.GET("/revisions", revisions)
			g3.GET("/archive/:revision", downloadZip)
			g3.POST("/visibility", editVisibility, logged, writePermission)
			g3.POST("/delete", deleteGist, logged, writePermission)
			g3.GET("/raw/:revision/:file", rawFile)
			g3.GET("/download/:revision/:file", downloadFile)
			g3.GET("/edit", edit, logged, writePermission)
			g3.POST("/edit", processCreate, logged, writePermission)
			g3.POST("/like", like, logged)
			g3.GET("/likes", likes, checkRequireLogin)
			g3.POST("/fork", fork, logged)
			g3.GET("/forks", forks, checkRequireLogin)
			g3.PUT("/checkbox", checkbox, logged, writePermission)
		}
	}

	customFs := os.DirFS(filepath.Join(config.GetHomeDir(), "custom"))
	e.GET("/assets/*", func(ctx echo.Context) error {
		if _, err := public.Files.Open(path.Join("assets", ctx.Param("*"))); !dev && err == nil {
			ctx.Response().Header().Set("Cache-Control", "public, max-age=31536000")
			ctx.Response().Header().Set("Expires", time.Now().AddDate(1, 0, 0).Format(http.TimeFormat))

			return echo.WrapHandler(http.FileServer(http.FS(public.Files)))(ctx)
		}

		// if the custom file is an .html template, render it
		if strings.HasSuffix(ctx.Param("*"), ".html") {
			if err := html(ctx, ctx.Param("*")); err != nil {
				return notFound("Page not found")
			}
			return nil
		}

		return echo.WrapHandler(http.StripPrefix("/assets/", http.FileServer(http.FS(customFs))))(ctx)
	})

	// Git HTTP routes
	if config.C.HttpGit {
		e.Any("/:user/:gistname/*", gitHttp, gistSoftInit)
	}

	e.Any("/*", noRouteFound)
}

// Router wraps echo.Group to provide custom Handler support
type Router struct {
	*echo.Group
}

// NewRouter creates a new Router instance
func NewRouter(g *echo.Group) *Router {
	return &Router{Group: g}
}

// SubGroup returns a new Router group with the given prefix and middleware
func (r *Router) SubGroup(prefix string, m ...Middleware) *Router {
	// Convert middleware only when creating group
	echoMiddleware := make([]echo.MiddlewareFunc, len(m))
	for i, mw := range m {
		mw := mw // capture for closure
		echoMiddleware[i] = func(next echo.HandlerFunc) echo.HandlerFunc {
			return Chain(func(c *context.OGContext) error {
				return next(c)
			}, mw).toEchoHandler()
		}
	}
	return NewRouter(r.Group.Group(prefix, echoMiddleware...))
}

// Route registration methods
func (r *Router) GET(path string, h Handler, m ...Middleware) {
	r.Group.GET(path, Chain(h, m...).toEchoHandler())
}

func (r *Router) POST(path string, h Handler, m ...Middleware) {
	r.Group.POST(path, Chain(h, m...).toEchoHandler())
}

func (r *Router) PUT(path string, h Handler, m ...Middleware) {
	r.Group.PUT(path, Chain(h, m...).toEchoHandler())
}

func (r *Router) DELETE(path string, h Handler, m ...Middleware) {
	r.Group.DELETE(path, Chain(h, m...).toEchoHandler())
}

func (r *Router) PATCH(path string, h Handler, m ...Middleware) {
	r.Group.PATCH(path, Chain(h, m...).toEchoHandler())
}

// Use registers middleware for the entire router group
func (r *Router) Use(middleware ...Middleware) {
	for _, m := range middleware {
		m := m // capture for closure
		r.Group.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
			return Chain(func(c *context.OGContext) error {
				return next(c)
			}, m).toEchoHandler()
		})
	}
}
