package web

import (
	"context"
	gojson "encoding/json"
	"errors"
	"fmt"
	htmlpkg "html"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/thomiceli/opengist/internal/index"
	"github.com/thomiceli/opengist/internal/utils"
	"github.com/thomiceli/opengist/templates"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/markbates/goth/gothic"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/auth"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/i18n"
	"github.com/thomiceli/opengist/public"
	"golang.org/x/text/language"
)

var (
	dev        bool
	flashStore *sessions.CookieStore     // session store for flash messages
	userStore  *sessions.FilesystemStore // session store for user sessions
	re         = regexp.MustCompile("[^a-z0-9]+")
	fm         = template.FuncMap{
		"split":     strings.Split,
		"indexByte": strings.IndexByte,
		"toInt": func(i string) int {
			val, _ := strconv.Atoi(i)
			return val
		},
		"inc": func(i int) int {
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
			return strings.ToLower(filepath.Ext(i)) == ".md"
		},
		"isCsv": func(i string) bool {
			return strings.ToLower(filepath.Ext(i)) == ".csv"
		},
		"csvFile": func(file *git.File) *git.CsvFile {
			if strings.ToLower(filepath.Ext(file.Filename)) != ".csv" {
				return nil
			}

			csvFile, err := git.ParseCsv(file)
			if err != nil {
				return nil
			}

			return csvFile
		},
		"httpStatusText": http.StatusText,
		"loadedTime": func(startTime time.Time) string {
			return fmt.Sprint(time.Since(startTime).Nanoseconds()/1e6) + "ms"
		},
		"slug": func(s string) string {
			return strings.Trim(re.ReplaceAllString(strings.ToLower(s), "-"), "-")
		},
		"avatarUrl": func(user *db.User, noGravatar bool) string {
			if user.AvatarURL != "" {
				return user.AvatarURL
			}

			if user.MD5Hash != "" && !noGravatar {
				return "https://www.gravatar.com/avatar/" + user.MD5Hash + "?d=identicon&s=200"
			}

			return defaultAvatar()
		},
		"asset":  asset,
		"custom": customAsset,
		"dev": func() bool {
			return dev
		},
		"defaultAvatar": defaultAvatar,
		"visibilityStr": func(visibility db.Visibility, lowercase bool) string {
			s := "Public"
			switch visibility {
			case 1:
				s = "Unlisted"
			case 2:
				s = "Private"
			}

			if lowercase {
				return strings.ToLower(s)
			}
			return s
		},
		"unescape": htmlpkg.UnescapeString,
		"join": func(s ...string) string {
			return strings.Join(s, "")
		},
		"toStr": func(i interface{}) string {
			return fmt.Sprint(i)
		},
		"safe": func(s string) template.HTML {
			return template.HTML(s)
		},
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, errors.New("invalid dict call")
			}
			dict := make(map[string]interface{})
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, errors.New("dict keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
		"addMetadataToSearchQuery": addMetadataToSearchQuery,
		"indexEnabled":             index.Enabled,
		"isUrl": func(s string) bool {
			_, err := url.ParseRequestURI(s)
			return err == nil
		},
	}
)

type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, _ echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

type Server struct {
	echo *echo.Echo
	dev  bool
}

func NewServer(isDev bool, sessionsPath string) *Server {
	dev = isDev
	flashStore = sessions.NewCookieStore([]byte("opengist"))
	userStore = sessions.NewFilesystemStore(sessionsPath,
		utils.ReadKey(path.Join(sessionsPath, "session-auth.key")),
		utils.ReadKey(path.Join(sessionsPath, "session-encrypt.key")),
	)
	userStore.MaxLength(10 * 1024)
	gothic.Store = userStore

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	if err := i18n.Locales.LoadAll(); err != nil {
		log.Fatal().Err(err).Msg("Failed to load locales")
	}

	e.Use(dataInit)
	e.Use(locale)
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

	t := template.Must(template.New("t").Funcs(fm).ParseFS(templates.Files, "*/*.html"))
	customPattern := filepath.Join(config.GetHomeDir(), "custom", "*.html")
	matches, err := filepath.Glob(customPattern)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to check for custom templates")
	}
	if len(matches) > 0 {
		t, err = t.ParseGlob(customPattern)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to parse custom templates")
		}
	}
	e.Renderer = &Template{
		templates: t,
	}

	e.HTTPErrorHandler = func(er error, ctx echo.Context) {
		if httpErr, ok := er.(*HTMLError); ok {
			setData(ctx, "error", er)
			if fatalErr := htmlWithCode(ctx, httpErr.Code, "error.html"); fatalErr != nil {
				log.Fatal().Err(fatalErr).Send()
			}
		} else if httpErr, ok := er.(*JSONError); ok {
			if fatalErr := json(ctx, httpErr.Code, httpErr); fatalErr != nil {
				log.Fatal().Err(fatalErr).Send()
			}
		} else {
			log.Fatal().Err(er).Send()
		}
	}

	e.Use(sessionInit)

	e.Validator = utils.NewValidator()

	if !dev {
		parseManifestEntries()
	}

	// Web based routes
	g1 := e.Group("")
	{
		if !dev {
			g1.Use(middleware.CSRFWithConfig(middleware.CSRFConfig{
				TokenLookup:    "form:_csrf,header:X-CSRF-Token",
				CookiePath:     "/",
				CookieHTTPOnly: true,
				CookieSameSite: http.SameSiteStrictMode,
			}))
		}
		g1.Use(csrfInit)
		g1.GET("/", create, logged)
		g1.POST("/", processCreate, logged)
		g1.GET("/preview", preview, logged)

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

		g1.GET("/settings", userSettings, logged)
		g1.POST("/settings/email", emailProcess, logged)
		g1.DELETE("/settings/account", accountDeleteProcess, logged)
		g1.POST("/settings/ssh-keys", sshKeysProcess, logged)
		g1.DELETE("/settings/ssh-keys/:id", sshKeysDelete, logged)
		g1.DELETE("/settings/passkeys/:id", passkeyDelete, logged)
		g1.PUT("/settings/password", passwordProcess, logged)
		g1.PUT("/settings/username", usernameProcess, logged)
		g2 := g1.Group("/admin-panel")
		{
			g2.Use(adminPermission)
			g2.GET("", adminIndex)
			g2.GET("/users", adminUsers)
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

	return &Server{echo: e, dev: dev}
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

func dataInit(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		ctxValue := context.WithValue(ctx.Request().Context(), dataKey, echo.Map{})
		ctx.SetRequest(ctx.Request().WithContext(ctxValue))
		setData(ctx, "loadStartTime", time.Now())

		if err := loadSettings(ctx); err != nil {
			return errorRes(500, "Cannot read settings from database", err)
		}

		setData(ctx, "c", config.C)

		setData(ctx, "githubOauth", config.C.GithubClientKey != "" && config.C.GithubSecret != "")
		setData(ctx, "gitlabOauth", config.C.GitlabClientKey != "" && config.C.GitlabSecret != "")
		setData(ctx, "giteaOauth", config.C.GiteaClientKey != "" && config.C.GiteaSecret != "")
		setData(ctx, "oidcOauth", config.C.OIDCClientKey != "" && config.C.OIDCSecret != "" && config.C.OIDCDiscoveryUrl != "")

		httpProtocol := "http"
		if ctx.Request().TLS != nil || ctx.Request().Header.Get("X-Forwarded-Proto") == "https" {
			httpProtocol = "https"
		}
		setData(ctx, "httpProtocol", strings.ToUpper(httpProtocol))

		var baseHttpUrl string
		// if a custom external url is set, use it
		if config.C.ExternalUrl != "" {
			baseHttpUrl = config.C.ExternalUrl
		} else {
			baseHttpUrl = httpProtocol + "://" + ctx.Request().Host
		}

		setData(ctx, "baseHttpUrl", baseHttpUrl)

		return next(ctx)
	}
}

func locale(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
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
			return errorRes(500, "Cannot get locale", err)
		}

		setData(ctx, "localeName", localeUsed.Name)
		setData(ctx, "locale", localeUsed)
		setData(ctx, "allLocales", i18n.Locales.Locales)

		return next(ctx)
	}
}

func sessionInit(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		sess := getSession(ctx)
		if sess.Values["user"] != nil {
			var err error
			var user *db.User

			if user, err = db.GetUserById(sess.Values["user"].(uint)); err != nil {
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

// ---

type Asset struct {
	File string `json:"file"`
}

var manifestEntries map[string]Asset

func parseManifestEntries() {
	file, err := public.Files.Open("manifest.json")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open manifest.json")
	}
	byteValue, err := io.ReadAll(file)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to read manifest.json")
	}
	if err = gojson.Unmarshal(byteValue, &manifestEntries); err != nil {
		log.Fatal().Err(err).Msg("Failed to unmarshal manifest.json")
	}
}

func defaultAvatar() string {
	if dev {
		return "http://localhost:16157/default.png"
	}
	return config.C.ExternalUrl + "/" + manifestEntries["default.png"].File
}

func asset(file string) string {
	if dev {
		return "http://localhost:16157/" + file
	}
	return config.C.ExternalUrl + "/" + manifestEntries[file].File
}

func customAsset(file string) string {
	assetpath, err := url.JoinPath("/", "assets", file)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to join path for custom file %s", file)
	}
	return config.C.ExternalUrl + assetpath
}
