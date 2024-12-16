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

	s.RegisterMiddlewares()
	s.setFuncMap()
	s.setHTTPErrorHandler()

	e.Validator = utils.NewValidator()

	if !s.dev {
		parseManifestEntries()
	}

	s.setupRoutes()

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
