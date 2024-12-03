package server

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/i18n"
	"github.com/thomiceli/opengist/internal/web/context"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"net/http"
	"strings"
	"time"
)

func (s *Server) useCustomContext() {
	s.echo.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			cc := context.NewContext(c)
			return next(cc)
		}
	})
}

func (s *Server) RegisterMiddlewares(e *echo.Echo) {
	e.Use(Middleware(dataInit).ToEcho())
	e.Use(Middleware(locale).ToEcho())

	e.Pre(middleware.MethodOverrideWithConfig(middleware.MethodOverrideConfig{
		Getter: middleware.MethodFromForm("_method"),
	}))
	e.Pre(middleware.RemoveTrailingSlash())
	e.Pre(middleware.CORS())
	e.Pre(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogURI: true, LogStatus: true, LogMethod: true,
		LogValuesFunc: func(ctx echo.Context, v middleware.RequestLoggerValues) error {
			log.Info().Str("uri", v.URI).Int("status", v.Status).Str("method", v.Method).
				Str("ip", ctx.RealIP()).TimeDiff("duration", time.Now(), v.StartTime).
				Msg("HTTP")
			return nil
		},
	}))
	e.Use(middleware.Recover())
	e.Use(middleware.Secure())

	e.Use(Middleware(sessionInit).ToEcho())
}

func dataInit(next Handler) Handler {
	return func(ctx *context.OGContext) error {
		ctx.SetData("loadStartTime", time.Now())

		if err := loadSettings(ctx); err != nil {
			return ctx.ErrorRes(500, "Cannot load settings", err)
		}

		ctx.SetData("c", config.C)

		ctx.SetData("githubOauth", config.C.GithubClientKey != "" && config.C.GithubSecret != "")
		ctx.SetData("gitlabOauth", config.C.GitlabClientKey != "" && config.C.GitlabSecret != "")
		ctx.SetData("giteaOauth", config.C.GiteaClientKey != "" && config.C.GiteaSecret != "")
		ctx.SetData("oidcOauth", config.C.OIDCClientKey != "" && config.C.OIDCSecret != "" && config.C.OIDCDiscoveryUrl != "")

		httpProtocol := "http"
		if ctx.Request().TLS != nil || ctx.Request().Header.Get("X-Forwarded-Proto") == "https" {
			httpProtocol = "https"
		}
		ctx.SetData("httpProtocol", strings.ToUpper(httpProtocol))

		var baseHttpUrl string
		// if a custom external url is set, use it
		if config.C.ExternalUrl != "" {
			baseHttpUrl = config.C.ExternalUrl
		} else {
			baseHttpUrl = httpProtocol + "://" + ctx.Request().Host
		}

		ctx.SetData("baseHttpUrl", baseHttpUrl)

		return next(ctx)
	}
}

func locale(next Handler) Handler {
	return func(ctx *context.OGContext) error {
		// Check URL arguments
		lang := ctx.Request().URL.Query().Get("lang")
		changeLang := lang != ""

		// Then check cookies
		if len(lang) == 0 {
			cookie, _ := ctx.Request().Cookie("lang")
			if cookie != nil {
				lang = cookie.Value
			}
		}

		// Check again in case someone changes the supported language list.
		if lang != "" && !i18n.Locales.HasLocale(lang) {
			lang = ""
			changeLang = false
		}

		// 3.Then check from 'Accept-Language' header.
		if len(lang) == 0 {
			tags, _, _ := language.ParseAcceptLanguage(ctx.Request().Header.Get("Accept-Language"))
			lang = i18n.Locales.MatchTag(tags)
		}

		if changeLang {
			ctx.SetCookie(&http.Cookie{Name: "lang", Value: lang, Path: "/", MaxAge: 1<<31 - 1})
		}

		localeUsed, err := i18n.Locales.GetLocale(lang)
		if err != nil {
			return ctx.ErrorRes(500, "Cannot get locale", err)
		}

		ctx.SetData("localeName", localeUsed.Name)
		ctx.SetData("locale", localeUsed)
		ctx.SetData("allLocales", i18n.Locales.Locales)

		return next(ctx)
	}
}

func sessionInit(next Handler) Handler {
	return func(ctx *context.OGContext) error {
		sess := ctx.GetSession()
		if sess.Values["user"] != nil {
			var err error
			var user *db.User

			if user, err = db.GetUserById(sess.Values["user"].(uint)); err != nil {
				sess.Values["user"] = nil
				ctx.SaveSession(sess)
				ctx.User = nil
				ctx.SetData("userLogged", nil)
				return ctx.RedirectTo("/all")
			}
			if user != nil {
				ctx.User = user
				ctx.SetData("userLogged", user)
			}
			return next(ctx)
		}

		ctx.User = nil
		ctx.SetData("userLogged", nil)
		return next(ctx)
	}
}

func csrfInit(next Handler) Handler {
	return func(ctx *context.OGContext) error {
		setCsrfHtmlForm(ctx)
		return next(ctx)
	}
}

func loadSettings(ctx *context.OGContext) error {
	settings, err := db.GetSettings()
	if err != nil {
		return err
	}

	for key, value := range settings {
		s := strings.ReplaceAll(key, "-", " ")
		s = cases.Title(language.English).String(s)
		ctx.SetData(strings.ReplaceAll(s, " ", ""), value == "1")
	}
	return nil
}
