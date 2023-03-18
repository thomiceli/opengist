package web

import (
	"context"
	"fmt"
	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog/log"
	"html/template"
	"io"
	"net/http"
	"opengist/internal/config"
	"opengist/internal/models"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var store *sessions.CookieStore
var re = regexp.MustCompile("[^a-z0-9]+")

type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, _ echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func Start() {
	store = sessions.NewCookieStore([]byte("opengist"))

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(dataInit)
	e.Pre(middleware.MethodOverrideWithConfig(middleware.MethodOverrideConfig{
		Getter: middleware.MethodFromForm("_method"),
	}))
	e.Pre(middleware.RemoveTrailingSlash())
	e.Use(middleware.CORS())
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogURI: true, LogStatus: true, LogMethod: true,
		LogValuesFunc: func(ctx echo.Context, v middleware.RequestLoggerValues) error {
			log.Info().Str("URI", v.URI).Int("status", v.Status).Str("method", v.Method).
				Msg("HTTP")
			return nil
		},
	}))
	e.Use(middleware.Recover())
	e.Use(middleware.Secure())

	e.Renderer = &Template{
		templates: template.Must(template.New("t").Funcs(
			template.FuncMap{
				"split":     strings.Split,
				"indexByte": strings.IndexByte,
				"toInt": func(i string) int64 {
					val, _ := strconv.ParseInt(i, 10, 64)
					return val
				},
				"inc": func(i int64) int64 {
					return i + 1
				},
				"splitGit": func(i string) []string {
					return strings.FieldsFunc(i, func(r rune) bool {
						return r == ',' || r == ' '
					})
				},
				"lines": func(i string) []string {
					return strings.Split(i, "\n")
				},
				"isMarkdown": func(i string) bool {
					return ".md" == strings.ToLower(filepath.Ext(i))
				},
				"httpStatusText": http.StatusText,
				"loadedTime": func(startTime time.Time) string {
					return fmt.Sprint(time.Since(startTime).Nanoseconds()/1e6) + "ms"
				},
				"slug": func(s string) string {
					return strings.Trim(re.ReplaceAllString(strings.ToLower(s), "-"), "-")
				},
			}).ParseGlob("templates/*/*.html")),
	}

	e.HTTPErrorHandler = func(er error, ctx echo.Context) {
		if err, ok := er.(*echo.HTTPError); ok {
			if err.Code >= 500 {
				log.Error().Int("code", err.Code).Err(err.Internal).Msg("HTTP: " + err.Message.(string))
			}

			setData(ctx, "error", err)
			if errHtml := htmlWithCode(ctx, err.Code, "error.html"); errHtml != nil {
				log.Fatal().Err(errHtml).Send()
			}
		} else {
			log.Fatal().Err(er).Send()
		}
	}

	e.Use(sessionInit)

	e.Validator = NewValidator()

	e.Static("/assets", "./public/assets")

	// Web based routes
	g1 := e.Group("")
	{
		g1.Use(middleware.CSRFWithConfig(middleware.CSRFConfig{
			TokenLookup:    "form:_csrf",
			CookiePath:     "/",
			CookieHTTPOnly: true,
			CookieSameSite: http.SameSiteStrictMode,
		}))
		g1.Use(csrfInit)

		g1.GET("/", create, logged)
		g1.POST("/", processCreate, logged)

		g1.GET("/register", register)
		g1.POST("/register", processRegister)
		g1.GET("/login", login)
		g1.POST("/login", processLogin)
		g1.GET("/logout", logout)

		g1.GET("/ssh-keys", sshKeys, logged)
		g1.POST("/ssh-keys", sshKeysProcess, logged)
		g1.DELETE("/ssh-keys/:id", sshKeysDelete, logged)

		g2 := g1.Group("/admin")
		{
			g2.Use(adminPermission)
			g2.GET("", adminIndex)
			g2.GET("/users", adminUsers)
			g2.POST("/users/:user/delete", adminUserDelete)
			g2.GET("/gists", adminGists)
			g2.POST("/gists/:gist/delete", adminGistDelete)
		}

		g1.GET("/all", allGists)
		g1.GET("/:user", allGists)

		g3 := g1.Group("/:user/:gistname")
		{
			g3.Use(gistInit)
			g3.GET("", gistIndex)
			g3.GET("/rev/:revision", gistIndex)
			g3.GET("/revisions", revisions)
			g3.GET("/archive/:revision", downloadZip)
			g3.POST("/visibility", toggleVisibility, logged, writePermission)
			g3.POST("/delete", deleteGist, logged, writePermission)
			g3.GET("/raw/:revision/:file", rawFile)
			g3.GET("/edit", edit, logged, writePermission)
			g3.POST("/edit", processCreate, logged, writePermission)
			g3.POST("/like", like, logged)
			g3.GET("/likes", likes)
			g3.POST("/fork", fork, logged)
			g3.GET("/forks", forks)
		}
	}

	debugStr := ""
	// Git HTTP routes
	if config.C.HTTP.Git {
		e.Any("/:user/:gistname/*", gitHttp, gistInit)
		debugStr = " (with Git over HTTP)"
	}

	e.Any("/*", noRouteFound)

	addr := config.C.HTTP.Host + ":" + config.C.HTTP.Port

	if config.C.HTTP.TLSEnabled {
		log.Info().Msg("Starting HTTPS server on https://" + addr + debugStr)
		if err := e.StartTLS(addr, config.C.HTTP.CertFile, config.C.HTTP.KeyFile); err != nil {
			log.Fatal().Err(err).Msg("Failed to start HTTPS server")
		}
	} else {
		log.Info().Msg("Starting HTTP server on http://" + addr + debugStr)
		if err := e.Start(addr); err != nil {
			log.Fatal().Err(err).Msg("Failed to start HTTP server")
		}
	}
}

func dataInit(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		ctxValue := context.WithValue(ctx.Request().Context(), "data", echo.Map{})
		ctx.SetRequest(ctx.Request().WithContext(ctxValue))
		setData(ctx, "loadStartTime", time.Now())
		setData(ctx, "signupDisabled", config.C.DisableSignup)

		return next(ctx)
	}
}

func sessionInit(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		sess := getSession(ctx)
		if sess.Values["user"] != nil {
			var err error
			var user *models.User

			if user, err = models.GetUserById(sess.Values["user"].(uint)); err != nil {
				sess.Values["user"] = nil
				saveSession(sess, ctx)
				setData(ctx, "userLogged", nil)
				return redirect(ctx, "/all")
			}
			if user != nil {
				setData(ctx, "userLogged", user)
			}
			return next(ctx)
		}

		setData(ctx, "userLogged", nil)
		return next(ctx)
	}
}

func csrfInit(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		setCsrfHtmlForm(ctx)
		return next(ctx)
	}
}

func writePermission(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		gist := getData(ctx, "gist")
		user := getUserLogged(ctx)
		if !gist.(*models.Gist).CanWrite(user) {
			return redirect(ctx, "/"+gist.(*models.Gist).User.Username+"/"+gist.(*models.Gist).Uuid)
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

func noRouteFound(echo.Context) error {
	return notFound("Page not found")
}
