package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/auth"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/i18n"
	"github.com/thomiceli/opengist/internal/ipc"
	"github.com/thomiceli/opengist/internal/web/context"
	"github.com/thomiceli/opengist/internal/web/handlers"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gorm.io/gorm"
)

func (s *Server) useCustomContext() {
	s.echo.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			cc := context.NewContext(c, filepath.Join(config.GetHomeDir(), "sessions"))
			return next(cc)
		}
	})
}

func (s *Server) registerMiddlewares() {
	s.echo.Use(Middleware(dataInit).toEcho())
	s.echo.Use(Middleware(locale).toEcho())
	if config.C.MetricsEnabled {
		s.echo.Use(echoprometheus.NewMiddleware("opengist"))
	}

	s.echo.Pre(middleware.MethodOverrideWithConfig(middleware.MethodOverrideConfig{
		Getter: middleware.MethodFromForm("_method"),
	}))
	s.echo.Pre(middleware.RemoveTrailingSlash())
	// Expose the API's pagination headers so cross-origin browser clients can
	// read them (the CORS spec hides non-safelisted response headers by default).
	s.echo.Pre(middleware.CORSWithConfig(middleware.CORSConfig{
		ExposeHeaders: []string{"Link", "X-Page", "X-Per-Page", "X-Total", "X-Total-Pages"},
	}))
	s.echo.Pre(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogURI: true, LogStatus: true, LogMethod: true,
		LogValuesFunc: func(ctx echo.Context, v middleware.RequestLoggerValues) error {
			log.Info().Str("uri", v.URI).Int("status", v.Status).Str("method", v.Method).
				Str("ip", ctx.RealIP()).TimeDiff("duration", time.Now(), v.StartTime).
				Msg("HTTP")
			return nil
		},
	}))
	s.echo.Use(middleware.Recover())
	s.echo.Use(middleware.Secure())
	s.echo.Use(Middleware(sessionInit).toEcho())
	s.echo.Use(middleware.CSRFWithConfig(middleware.CSRFConfig{
		TokenLookup:    "form:_csrf,header:X-CSRF-Token",
		CookiePath:     "/",
		CookieHTTPOnly: true,
		CookieSameSite: http.SameSiteStrictMode,
		Skipper: func(ctx echo.Context) bool {
			// skip CSRF for /api (uses bearer tokens, not session cookies); this
			// also covers the token-authenticated IPC API under /api/ipc
			if strings.HasPrefix(ctx.Request().URL.Path, "/api/") {
				return true
			}
			/* skip CSRF for embeds */
			gistName := ctx.Param("gistname")
			/* skip CSRF for git clients */
			matchUploadPack, _ := regexp.MatchString("(.*?)/git-upload-pack$", ctx.Request().URL.Path)
			matchReceivePack, _ := regexp.MatchString("(.*?)/git-receive-pack$", ctx.Request().URL.Path)
			return (filepath.Ext(gistName) == ".js" && ctx.Request().Method == "GET") || matchUploadPack || matchReceivePack
		},
		ErrorHandler: func(err error, c echo.Context) error {
			log.Info().Err(err).Msg("CSRF error")
			return err
		},
	}))
	s.echo.Use(Middleware(csrfInit).toEcho())

}

func (s *Server) errorHandler(err error, ctx echo.Context) {
	data, _ := ctx.Request().Context().Value(context.DataKeyStr).(echo.Map)
	if data == nil {
		data = echo.Map{}
	}

	var httpErr *context.HTTPError
	if errors.As(err, &httpErr) {
		data["error"] = err
	} else {
		if isClientGone(err) {
			return
		}
		log.Error().Err(err).Send()
		httpErr = &context.HTTPError{Message: err.Error(), Code: http.StatusInternalServerError}
		data["error"] = httpErr
	}

	if ctx.Response().Committed {
		return
	}

	acceptJson := strings.Contains(ctx.Request().Header.Get("Accept"), "application/json")
	var renderErr error
	if acceptJson || data["err_render"] == "json" {
		renderErr = ctx.JSON(httpErr.Code, httpErr)
	} else {
		renderErr = ctx.Render(httpErr.Code, "error.html", data)
	}

	if renderErr != nil && !isClientGone(renderErr) {
		log.Error().Err(renderErr).Send()
	}
}

func isClientGone(err error) bool {
	return errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET)
}

func dataInit(next Handler) Handler {
	return func(ctx *context.Context) error {
		ctx.SetData("loadStartTime", time.Now())
		ctx.SetData("cspNonce", newCSPNonce())

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
		ctx.SetData("canonicalUrl", baseHttpUrl+ctx.Request().URL.Path)

		return next(ctx)
	}
}

func writePermission(next Handler) Handler {
	return func(ctx *context.Context) error {
		gist := ctx.GetData("gist").(*db.Gist)
		user := ctx.User
		if !gist.CanWrite(user) {
			return ctx.ErrorRes(403, "You don't have permission to edit this gist", nil)
		}
		return next(ctx)
	}
}

// notArchived blocks write operations on an archived (read-only) gist. It must
// run after gistInit so the gist is available in the context.
func notArchived(next Handler) Handler {
	return func(ctx *context.Context) error {
		gist := ctx.GetData("gist").(*db.Gist)
		if gist.Archived {
			return ctx.ErrorRes(403, "This gist is archived and is read-only", nil)
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
		return ctx.RedirectTo("/-/all")
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

func inOAuthRegisterSession(next Handler) Handler {
	return func(ctx *context.Context) error {
		sess := ctx.GetSession()
		_, ok := sess.Values["oauthProvider"].(string)
		if !ok {
			return ctx.RedirectTo("/login")
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

			if getUserByToken(ctx) != nil {
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

func checkFileUploadEnabled(next Handler) Handler {
	return func(ctx *context.Context) error {
		if config.C.DisableFileUpload {
			return ctx.ErrorRes(403, ctx.Tr("error.file-upload-disabled"), nil)
		}
		return next(ctx)
	}
}

// makeApiCheckRequireLogin is the /api/v1 counterpart of makeCheckRequireLogin:
// it enforces the instance's RequireLogin / AllowGistsWithoutLogin settings on
// anonymous gist reads, but responds with a JSON 401 instead of redirecting to
// /login. ctx.User is already resolved from the Authorization header by
// apiBindAuth, so there is no token fallback to do here.
func makeApiCheckRequireLogin(isSingleGistAccess bool) Middleware {
	return func(next Handler) Handler {
		return func(ctx *context.Context) error {
			if ctx.User != nil {
				return next(ctx)
			}

			allow, err := auth.ShouldAllowUnauthenticatedGistAccess(handlers.ContextAuthInfo{Context: ctx}, isSingleGistAccess)
			if err != nil {
				return ctx.ErrorJson(500, "Failed to check if unauthenticated access is allowed", err)
			}

			if !allow {
				return ctx.ErrorJson(401, "Requires authentication", nil)
			}
			return next(ctx)
		}
	}
}

func noRouteFound(ctx *context.Context) error {
	return ctx.NotFound("Page not found")
}

func noRouteFoundApi(ctx *context.Context) error {
	return ctx.ErrorJson(404, "Not found", nil)
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
				return ctx.RedirectTo("/-/all")
			}
			if user != nil {
				ctx.User = user
				ctx.SetData("userLogged", user)
				ctx.SetData("currentStyle", user.GetStyle())
			}
			return next(ctx)
		}

		ctx.User = nil
		ctx.SetData("userLogged", nil)
		return next(ctx)
	}
}

// newCSPNonce returns a fresh random nonce for the Content-Security-Policy of
// the current request. The same value is exposed to templates (so inline
// scripts can carry a matching nonce attribute) and used in the CSP header set
// by handlers that opt into a strict policy (e.g. the gist view page).
func newCSPNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// rand.Read never returns an error on supported platforms; fall back to
		// an empty nonce rather than serving a non-functional inline script.
		return ""
	}
	return base64.RawStdEncoding.EncodeToString(b)
}

func csrfInit(next Handler) Handler {
	return func(ctx *context.Context) error {
		var csrf string
		if csrfToken, ok := ctx.Get("csrf").(string); ok {
			csrf = csrfToken
		}
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

func apiScope(scope, permission uint) Middleware {
	return func(next Handler) Handler {
		return func(ctx *context.Context) error {
			tok, ok := ctx.GetData("accessToken").(*db.AccessToken)
			if !ok || !tok.CheckForPermission(scope, permission) {
				return ctx.ErrorJson(403, "token lacks required scope/permission", nil)
			}
			return next(ctx)
		}
	}
}

// getUserByToken checks the Authorization header for token-based auth.
// Expects format: Authorization: Token <token>
// Returns the user if the token is valid and has gist read permission, nil otherwise.
func getUserByToken(ctx *context.Context) *db.User {
	authHeader := ctx.Request().Header.Get("Authorization")
	if authHeader == "" {
		return nil
	}

	if !strings.HasPrefix(authHeader, "Token ") {
		return nil
	}

	plainToken := strings.TrimPrefix(authHeader, "Token ")

	accessToken, err := db.GetAccessTokenByToken(plainToken)
	if err != nil {
		return nil
	}

	if accessToken.IsExpired() {
		return nil
	}

	if !accessToken.HasGistReadPermission() {
		return nil
	}

	_ = accessToken.UpdateLastUsed()

	return &accessToken.User
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

		// Expired gists are removed by a background job, but it may not have run
		// yet so hide them in the meantime so they're never served past expiry.
		if gist.IsExpired() {
			return ctx.NotFound("Gist not found")
		}

		if gist.Private == db.PrivateVisibility {
			if currUser == nil || currUser.ID != gist.UserID {
				// Check for token-based auth via Authorization header
				if tokenUser := getUserByToken(ctx); tokenUser != nil && tokenUser.ID == gist.UserID {
					// Token is valid and belongs to gist owner, allow access
				} else {
					return ctx.NotFound("Gist not found")
				}
			}
		}

		ctx.SetData("gist", gist)

		if ssh := gist.SSHCloneURL(ctx.Request().Host); ssh != "" {
			ctx.SetData("sshCloneUrl", ssh)
		}

		baseHttpUrl := ctx.GetData("baseHttpUrl").(string)

		if cloneURL := gist.HTTPCloneURL(baseHttpUrl); cloneURL != "" {
			ctx.SetData("httpCloneUrl", cloneURL)
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
func setAllGistsMode(mode string) Middleware {
	return func(next Handler) Handler {
		return func(ctx *context.Context) error {
			ctx.SetData("mode", mode)
			return next(ctx)
		}
	}
}

// apiBindAuth gates /api on the api.enabled config option and optionally
// resolves a caller identity from the Authorization header. A missing header is
// allowed (the downstream handler/scope middleware decides whether anonymous
// access is OK); a malformed/expired/unknown token is always rejected with 401.
// On success, sets ctx.User and stashes the access token under "accessToken".
func apiBindAuth(next Handler) Handler {
	return func(ctx *context.Context) error {
		if !config.C.ApiEnabled {
			return ctx.ErrorJson(403, "API is disabled", nil)
		}

		h := ctx.Request().Header.Get("Authorization")
		if h == "" {
			return next(ctx)
		}
		var plain string
		switch {
		case strings.HasPrefix(h, "Bearer "):
			plain = strings.TrimPrefix(h, "Bearer ")
		case strings.HasPrefix(h, "Token "):
			plain = strings.TrimPrefix(h, "Token ")
		default:
			return ctx.ErrorJson(401, "Bad crendentials", nil)
		}

		tok, err := db.GetAccessTokenByToken(plain)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ctx.ErrorJson(401, "Bad crendentials", nil)
			}
			return ctx.ErrorJson(500, "Error fetching access token", err)
		}
		if tok.IsExpired() {
			return ctx.ErrorJson(401, "Bad crendentials", nil)
		}

		ctx.User = &tok.User
		ctx.SetData("accessToken", tok)

		// Synchronous update so that test teardown (TRUNCATE on postgres)
		// can't race with an in-flight UPDATE on a separate goroutine.
		// Cost is one indexed UPDATE per request; negligible vs the network
		// round-trip the caller already paid.
		_ = tok.UpdateLastUsed()

		return next(ctx)
	}
}

// apiRequireAuth rejects requests that apiAuth didn't attach a user to. Use on
// routes where anonymous access is not allowed (e.g. /user, write endpoints).
func apiRequireAuth(next Handler) Handler {
	return func(ctx *context.Context) error {
		if ctx.User == nil {
			return ctx.ErrorJson(401, "Requires authentication", nil)
		}
		return next(ctx)
	}
}

// ipcAuth guards the IPC API: only callers presenting the token derived from the
// secret key (i.e. Opengist's own subprocesses) may proceed.
func ipcAuth(next Handler) Handler {
	return func(ctx *context.Context) error {
		got := []byte(ctx.Request().Header.Get(ipc.AuthHeader))
		want := []byte(ipc.Token())
		if subtle.ConstantTimeCompare(got, want) != 1 {
			return ctx.NoContent(http.StatusUnauthorized)
		}
		return next(ctx)
	}
}
