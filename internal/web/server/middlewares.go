package server

import (
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/auth"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/i18n"
	"github.com/thomiceli/opengist/internal/web/context"
	"github.com/thomiceli/opengist/internal/web/handlers"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"html/template"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func (s *Server) useCustomContext() {
	s.echo.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			cc := context.NewContext(c, s.sessionsPath)
			return next(cc)
		}
	})
}

func (s *Server) registerMiddlewares() {
	s.echo.Use(Middleware(dataInit).toEcho())
	s.echo.Use(Middleware(locale).toEcho())

	s.echo.Pre(middleware.MethodOverrideWithConfig(middleware.MethodOverrideConfig{
		Getter: middleware.MethodFromForm("_method"),
	}))
	s.echo.Pre(middleware.RemoveTrailingSlash())
	s.echo.Pre(middleware.CORS())
	s.echo.Pre(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogURI: true, LogStatus: true, LogMethod: true,
		LogValuesFunc: func(ctx echo.Context, v middleware.RequestLoggerValues) error {
			log.Info().Str("uri", v.URI).Int("status", v.Status).Str("method", v.Method).
				Str("ip", ctx.RealIP()).TimeDiff("duration", time.Now(), v.StartTime).
				Msg("HTTP")
			return nil
		},
	}))
	// s.echo.Use(middleware.Recover())
	s.echo.Use(middleware.Secure())
	s.echo.Use(Middleware(sessionInit).toEcho())

	if !s.ignoreCsrf {
		s.echo.Use(middleware.CSRFWithConfig(middleware.CSRFConfig{
			TokenLookup:    "form:_csrf,header:X-CSRF-Token",
			CookiePath:     "/",
			CookieHTTPOnly: true,
			CookieSameSite: http.SameSiteStrictMode,
			Skipper: func(ctx echo.Context) bool {
				/* skip CSRF for embeds */
				gistName := ctx.Param("gistname")

				/* skip CSRF for git clients */
				matchUploadPack, _ := regexp.MatchString("(.*?)/git-upload-pack$", ctx.Request().URL.Path)
				matchReceivePack, _ := regexp.MatchString("(.*?)/git-receive-pack$", ctx.Request().URL.Path)
				return filepath.Ext(gistName) == ".js" || matchUploadPack || matchReceivePack
			},
			ErrorHandler: func(err error, c echo.Context) error {
				log.Info().Err(err).Msg("CSRF error")
				return err
			},
		}))
		s.echo.Use(Middleware(csrfInit).toEcho())
	}
}

func (s *Server) errorHandler(err error, ctx echo.Context) {
	var httpErr *echo.HTTPError
	if errors.As(err, &httpErr) {
		acceptJson := strings.Contains(ctx.Request().Header.Get("Accept"), "application/json")
		data := ctx.Request().Context().Value(context.DataKeyStr).(echo.Map)
		data["error"] = err
		if acceptJson {
			if err := ctx.JSON(httpErr.Code, httpErr); err != nil {
				log.Fatal().Err(err).Send()
			}
			return
		}

		if err := ctx.Render(httpErr.Code, "error", data); err != nil {
			log.Fatal().Err(err).Send()
		}
		return
	}

	log.Fatal().Err(err).Send()
}

func dataInit(next Handler) Handler {
	return func(ctx *context.Context) error {
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

func writePermission(next Handler) Handler {
	return func(ctx *context.Context) error {
		gist := ctx.GetData("gist")
		user := ctx.User
		if !gist.(*db.Gist).CanWrite(user) {
			return ctx.RedirectTo("/" + gist.(*db.Gist).User.Username + "/" + gist.(*db.Gist).Identifier())
		}
		return next(ctx)
	}
}

func adminPermission(next Handler) Handler {
	return func(ctx *context.Context) error {
		user := ctx.User
		if user == nil || !user.IsAdmin {
			return ctx.NotFound("User not found")
		}
		return next(ctx)
	}
}

func logged(next Handler) Handler {
	return func(ctx *context.Context) error {
		user := ctx.User
		if user != nil {
			return next(ctx)
		}
		return ctx.RedirectTo("/all")
	}
}

func inMFASession(next Handler) Handler {
	return func(ctx *context.Context) error {
		sess := ctx.GetSession()
		_, ok := sess.Values["mfaID"].(uint)
		if !ok {
			return ctx.ErrorRes(400, ctx.Tr("error.not-in-mfa-session"), nil)
		}
		return next(ctx)
	}
}

func makeCheckRequireLogin(isSingleGistAccess bool) Middleware {
	return func(next Handler) Handler {
		return func(ctx *context.Context) error {
			if user := ctx.User; user != nil {
				return next(ctx)
			}

			allow, err := auth.ShouldAllowUnauthenticatedGistAccess(handlers.ContextAuthInfo{Context: ctx}, isSingleGistAccess)
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to check if unauthenticated access is allowed")
			}

			if !allow {
				ctx.AddFlash(ctx.Tr("flash.auth.must-be-logged-in"), "error")
				return ctx.RedirectTo("/login")
			}
			return next(ctx)
		}
	}
}

func checkRequireLogin(next Handler) Handler {
	return makeCheckRequireLogin(false)(next)
}

func noRouteFound(ctx *context.Context) error {
	return ctx.NotFound("Page not found")
}

func locale(next Handler) Handler {
	return func(ctx *context.Context) error {
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
	return func(ctx *context.Context) error {
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
	return func(ctx *context.Context) error {
		var csrf string
		if csrfToken, ok := ctx.Get("csrf").(string); ok {
			csrf = csrfToken
		}
		ctx.SetData("csrfHtml", template.HTML(`<input type="hidden" name="_csrf" value="`+csrf+`">`))
		ctx.SetData("csrfHtml", template.HTML(`<input type="hidden" name="_csrf" value="`+csrf+`">`))

		return next(ctx)
	}
}

func loadSettings(ctx *context.Context) error {
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

func gistInit(next Handler) Handler {
	return func(ctx *context.Context) error {
		currUser := ctx.User

		userName := ctx.Param("user")
		gistName := ctx.Param("gistname")

		switch filepath.Ext(gistName) {
		case ".js":
			ctx.SetData("gistpage", "js")
			gistName = strings.TrimSuffix(gistName, ".js")
		case ".json":
			ctx.SetData("gistpage", "json")
			gistName = strings.TrimSuffix(gistName, ".json")
		case ".git":
			ctx.SetData("gistpage", "git")
			gistName = strings.TrimSuffix(gistName, ".git")
		}

		gist, err := db.GetGist(userName, gistName)
		if err != nil {
			return ctx.NotFound("Gist not found")
		}

		if gist.Private == db.PrivateVisibility {
			if currUser == nil || currUser.ID != gist.UserID {
				return ctx.NotFound("Gist not found")
			}
		}

		ctx.SetData("gist", gist)

		if config.C.SshGit {
			var sshDomain string

			if config.C.SshExternalDomain != "" {
				sshDomain = config.C.SshExternalDomain
			} else {
				sshDomain = strings.Split(ctx.Request().Host, ":")[0]
			}

			if config.C.SshPort == "22" {
				ctx.SetData("sshCloneUrl", sshDomain+":"+userName+"/"+gistName+".git")
			} else {
				ctx.SetData("sshCloneUrl", "ssh://"+sshDomain+":"+config.C.SshPort+"/"+userName+"/"+gistName+".git")
			}
		}

		baseHttpUrl := ctx.GetData("baseHttpUrl").(string)

		if config.C.HttpGit {
			ctx.SetData("httpCloneUrl", baseHttpUrl+"/"+userName+"/"+gistName+".git")
		}

		ctx.SetData("httpCopyUrl", baseHttpUrl+"/"+userName+"/"+gistName)
		ctx.SetData("currentUrl", template.URL(ctx.Request().URL.Path))
		ctx.SetData("embedScript", fmt.Sprintf(`<script src="%s"></script>`, baseHttpUrl+"/"+userName+"/"+gistName+".js"))

		nbCommits, err := gist.NbCommits()
		if err != nil {
			return ctx.ErrorRes(500, "Error fetching number of commits", err)
		}
		ctx.SetData("nbCommits", nbCommits)

		if currUser != nil {
			hasLiked, err := currUser.HasLiked(gist)
			if err != nil {
				return ctx.ErrorRes(500, "Cannot get user like status", err)
			}
			ctx.SetData("hasLiked", hasLiked)
		}

		if gist.Private > 0 {
			ctx.SetData("NoIndex", true)
		}

		return next(ctx)
	}
}

// gistSoftInit try to load a gist (same as gistInit) but does not return a 404 if the gist is not found
// useful for git clients using HTTP to obfuscate the existence of a private gist
func gistSoftInit(next Handler) Handler {
	return func(ctx *context.Context) error {
		userName := ctx.Param("user")
		gistName := ctx.Param("gistname")

		gistName = strings.TrimSuffix(gistName, ".git")

		gist, _ := db.GetGist(userName, gistName)
		ctx.SetData("gist", gist)

		return next(ctx)
	}
}

// gistNewPushSoftInit has the same behavior as gistSoftInit but create a new gist empty instead
func gistNewPushSoftInit(next Handler) Handler {
	return func(ctx *context.Context) error {
		ctx.SetData("gist", new(db.Gist))
		return next(ctx)
	}
}
