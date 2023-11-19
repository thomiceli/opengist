package web

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/markbates/goth/gothic"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/i18n"
	"github.com/thomiceli/opengist/public"
	"github.com/thomiceli/opengist/templates"
	"golang.org/x/text/language"
	htmlpkg "html"
	"html/template"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var dev bool
var store *sessions.CookieStore
var re = regexp.MustCompile("[^a-z0-9]+")
var fm = template.FuncMap{
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
	"asset": func(file string) string {
		if dev {
			return "http://localhost:16157/" + file
		}
		return config.C.ExternalUrl + "/" + manifestEntries[file].File
	},
	"dev": func() bool {
		return dev
	},
	"defaultAvatar": defaultAvatar,
	"visibilityStr": func(visibility int, lowercase bool) string {
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
}

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

func NewServer(isDev bool) *Server {
	dev = isDev
	store = sessions.NewCookieStore([]byte("opengist"))
	gothic.Store = store

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
			log.Info().Str("URI", v.URI).Int("status", v.Status).Str("method", v.Method).
				Str("ip", ctx.RealIP()).
				Msg("HTTP")
			return nil
		},
	}))
	//e.Use(middleware.Recover())
	e.Use(middleware.Secure())

	e.Renderer = &Template{
		templates: template.Must(template.New("t").Funcs(fm).ParseFS(templates.Files, "*/*.html")),
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

	if !dev {
		parseManifestEntries()
		e.GET("/assets/*", cacheControl(echo.WrapHandler(http.FileServer(http.FS(public.Files)))))
	}

	// Web based routes
	g1 := e.Group("")
	{
		if !dev {
			g1.Use(middleware.CSRFWithConfig(middleware.CSRFConfig{
				TokenLookup:    "form:_csrf",
				CookiePath:     "/",
				CookieHTTPOnly: true,
				CookieSameSite: http.SameSiteStrictMode,
			}))
			g1.Use(csrfInit)
		}

		g1.GET("/", create, logged)
		g1.POST("/", processCreate, logged)

		g1.GET("/register", register)
		g1.POST("/register", processRegister)
		g1.GET("/login", login)
		g1.POST("/login", processLogin)
		g1.GET("/logout", logout)
		g1.GET("/oauth/:provider", oauth)
		g1.GET("/oauth/:provider/callback", oauthCallback)

		g1.GET("/settings", userSettings, logged)
		g1.POST("/settings/email", emailProcess, logged)
		g1.DELETE("/settings/account", accountDeleteProcess, logged)
		g1.POST("/settings/ssh-keys", sshKeysProcess, logged)
		g1.DELETE("/settings/ssh-keys/:id", sshKeysDelete, logged)
		g1.PUT("/settings/password", passwordProcess, logged)

		g2 := g1.Group("/admin-panel")
		{
			g2.Use(adminPermission)
			g2.GET("", adminIndex)
			g2.GET("/users", adminUsers)
			g2.POST("/users/:user/delete", adminUserDelete)
			g2.GET("/gists", adminGists)
			g2.POST("/gists/:gist/delete", adminGistDelete)
			g2.POST("/sync-fs", adminSyncReposFromFS)
			g2.POST("/sync-db", adminSyncReposFromDB)
			g2.POST("/gc-repos", adminGcRepos)
			g2.GET("/configuration", adminConfig)
			g2.PUT("/set-config", adminSetConfig)
		}

		if config.C.HttpGit {
			e.Any("/init/*", gitHttp, gistNewPushSoftInit)
		}

		g1.GET("/all", allGists, checkRequireLogin)
		g1.GET("/search", allGists, checkRequireLogin)
		g1.GET("/:user", allGists, checkRequireLogin)
		g1.GET("/:user/liked", allGists, checkRequireLogin)
		g1.GET("/:user/forked", allGists, checkRequireLogin)

		g3 := g1.Group("/:user/:gistname")
		{
			g3.Use(checkRequireLogin, gistInit)
			g3.GET("", gistIndex)
			g3.GET("/rev/:revision", gistIndex)
			g3.GET("/revisions", revisions)
			g3.GET("/archive/:revision", downloadZip)
			g3.POST("/visibility", toggleVisibility, logged, writePermission)
			g3.POST("/delete", deleteGist, logged, writePermission)
			g3.GET("/raw/:revision/:file", rawFile)
			g3.GET("/download/:revision/:file", downloadFile)
			g3.GET("/edit", edit, logged, writePermission)
			g3.POST("/edit", processCreate, logged, writePermission)
			g3.POST("/like", like, logged)
			g3.GET("/likes", likes)
			g3.POST("/fork", fork, logged)
			g3.GET("/forks", forks)
		}
	}

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
		setData(ctx, "giteaOauth", config.C.GiteaClientKey != "" && config.C.GiteaSecret != "")
		setData(ctx, "oidcOauth", config.C.OIDCClientKey != "" && config.C.OIDCSecret != "" && config.C.OIDCDiscoveryUrl != "")

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

		//3.Then check from 'Accept-Language' header.
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
			return redirect(ctx, "/"+gist.(*db.Gist).User.Username+"/"+gist.(*db.Gist).Uuid)
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

func checkRequireLogin(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		if user := getUserLogged(ctx); user != nil {
			return next(ctx)
		}

		require := getData(ctx, "RequireLogin")
		if require == true {
			addFlash(ctx, "You must be logged in to access gists", "error")
			return redirect(ctx, "/login")
		}
		return next(ctx)
	}
}

func cacheControl(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set(echo.HeaderCacheControl, "public, max-age=31536000")
		return next(c)
	}
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
	if err = json.Unmarshal(byteValue, &manifestEntries); err != nil {
		log.Fatal().Err(err).Msg("Failed to unmarshal manifest.json")
	}
}

func defaultAvatar() string {
	if dev {
		return "http://localhost:16157/default.png"
	}
	return config.C.ExternalUrl + "/" + manifestEntries["default.png"].File
}
