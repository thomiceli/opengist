package server

import (
	"errors"
	"github.com/thomiceli/opengist/internal/web/context"
	"github.com/thomiceli/opengist/internal/web/handler"
	"html/template"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/thomiceli/opengist/internal/index"
	"github.com/thomiceli/opengist/internal/utils"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/auth"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/i18n"
	"github.com/thomiceli/opengist/public"
)

type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, _ echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

type Server struct {
	echo       *echo.Echo
	flashStore *sessions.CookieStore     // session store for flash messages
	UserStore  *sessions.FilesystemStore // session store for user sessions

	dev          bool
	sessionsPath string
	ignoreCsrf   bool
}

func NewServer(isDev bool, sessionsPath string, ignoreCsrf bool) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	s := &Server{echo: e, dev: isDev, sessionsPath: sessionsPath, ignoreCsrf: ignoreCsrf}

	s.useCustomContext()

	if err := i18n.Locales.LoadAll(); err != nil {
		log.Fatal().Err(err).Msg("Failed to load locales")
	}

	s.RegisterMiddlewares(e)
	s.setFuncMap()
	s.setHTTPErrorHandler()

	e.Validator = utils.NewValidator()

	if !s.dev {
		parseManifestEntries()
	}

	// Web based routes
	g1 := e.Group("")
	{
		if !ignoreCsrf {
			g1.Use(middleware.CSRFWithConfig(middleware.CSRFConfig{
				TokenLookup:    "form:_csrf,header:X-CSRF-Token",
				CookiePath:     "/",
				CookieHTTPOnly: true,
				CookieSameSite: http.SameSiteStrictMode,
			}))
			g1.Use(Middleware(csrfInit).ToEcho())
		}

		g1.GET("/", Handler(handler.Create).ToEcho(), logged)
		g1.POST("/", Handler(handler.ProcessCreate).ToEcho(), logged)
		g1.POST("/preview", preview, logged)

		g1.GET("/healthcheck", healthcheck)
		g1.GET("/metrics", metrics)

		g1.GET("/register", register)
		g1.POST("/register", processRegister)
		g1.GET("/login", login)
		g1.POST("/login", processLogin)
		g1.GET("/logout", logout)
		g1.GET("/oauth/:provider", oauth)
		g1.GET("/oauth/:provider/callback", oauthCallback)
		g1.GET("/oauth/:provider/unlink", oauthUnlink, logged)
		g1.POST("/webauthn/bind", beginWebAuthnBinding, logged)
		g1.POST("/webauthn/bind/finish", finishWebAuthnBinding, logged)
		g1.POST("/webauthn/login", beginWebAuthnLogin)
		g1.POST("/webauthn/login/finish", finishWebAuthnLogin)
		g1.POST("/webauthn/assertion", beginWebAuthnAssertion, inMFASession)
		g1.POST("/webauthn/assertion/finish", finishWebAuthnAssertion, inMFASession)
		g1.GET("/mfa", mfa, inMFASession)
		g1.POST("/mfa/totp/assertion", assertTotp, inMFASession)

		g1.GET("/settings", userSettings, logged)
		g1.POST("/settings/email", emailProcess, logged)
		g1.DELETE("/settings/account", accountDeleteProcess, logged)
		g1.POST("/settings/ssh-keys", sshKeysProcess, logged)
		g1.DELETE("/settings/ssh-keys/:id", sshKeysDelete, logged)
		g1.DELETE("/settings/passkeys/:id", passkeyDelete, logged)
		g1.PUT("/settings/password", passwordProcess, logged)
		g1.PUT("/settings/username", usernameProcess, logged)
		g1.GET("/settings/totp/generate", beginTotp, logged)
		g1.POST("/settings/totp/generate", finishTotp, logged)
		g1.DELETE("/settings/totp", disableTotp, logged)
		g1.POST("/settings/totp/regenerate", regenerateTotpRecoveryCodes, logged)

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

	return s
}

func (s *Server) Start() {
	addr := config.C.HttpHost + ":" + config.C.HttpPort

	log.Info().Msg("Starting HTTP server on http://" + addr)
	if err := s.echo.Start(addr); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("Failed to start HTTP server")
	}
}

func (s *Server) Stop() {
	if err := s.echo.Close(); err != nil {
		log.Fatal().Err(err).Msg("Failed to stop HTTP server")
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.echo.ServeHTTP(w, r)
}

func writePermission(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		gist := getData(ctx, "gist")
		user := getUserLogged(ctx)
		if !gist.(*db.Gist).CanWrite(user) {
			return redirect(ctx, "/"+gist.(*db.Gist).User.Username+"/"+gist.(*db.Gist).Identifier())
		}
		return next(ctx)
	}
}

func adminPermission(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		user := getUserLogged(ctx)
		if user == nil || !user.IsAdmin {
			return notFound("User not found")
		}
		return next(ctx)
	}
}

func logged(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		user := getUserLogged(ctx)
		if user != nil {
			return next(ctx)
		}
		return redirect(ctx, "/all")
	}
}

func inMFASession(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		sess := getSession(ctx)
		_, ok := sess.Values["mfaID"].(uint)
		if !ok {
			return errorRes(400, tr(ctx, "error.not-in-mfa-session"), nil)
		}
		return next(ctx)
	}
}

func makeCheckRequireLogin(isSingleGistAccess bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			if user := getUserLogged(ctx); user != nil {
				return next(ctx)
			}

			allow, err := auth.ShouldAllowUnauthenticatedGistAccess(ContextAuthInfo{ctx}, isSingleGistAccess)
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to check if unauthenticated access is allowed")
			}

			if !allow {
				addFlash(ctx, tr(ctx, "flash.auth.must-be-logged-in"), "error")
				return redirect(ctx, "/login")
			}
			return next(ctx)
		}
	}
}

func checkRequireLogin(next echo.HandlerFunc) echo.HandlerFunc {
	return makeCheckRequireLogin(false)(next)
}

func noRouteFound(echo.Context) error {
	return notFound("Page not found")
}

func (s *Server) setHTTPErrorHandler() {
	s.echo.HTTPErrorHandler = func(er error, c echo.Context) {
		ctx := c.(*context.OGContext)
		var httpErr *echo.HTTPError
		if errors.As(er, &httpErr) {
			acceptJson := strings.Contains(ctx.Request().Header.Get("Accept"), "application/json")
			ctx.SetData("error", er)
			if acceptJson {
				if fatalErr := jsonWithCode(ctx, httpErr.Code, httpErr); fatalErr != nil {
					log.Fatal().Err(fatalErr).Send()
				}
			} else {
				if fatalErr := htmlWithCode(ctx, httpErr.Code, "error.html"); fatalErr != nil {
					log.Fatal().Err(fatalErr).Send()
				}
			}
		} else {
			log.Fatal().Err(er).Send()
		}
	}
}
