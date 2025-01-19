package context

import (
	"context"
	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/i18n"
	"html/template"
	"net/http"
	"sync"
)

type dataKey string

const dataKeyStr dataKey = "data"

type Context struct {
	echo.Context

	data echo.Map
	lock sync.RWMutex

	store *Store
	User  *db.User
}

func NewContext(c echo.Context, sessionPath string) *Context {
	return &Context{
		Context: c,
		data:    make(echo.Map),
		store:   NewStore(sessionPath),
	}
}

func (ctx *Context) SetData(key string, value any) {
	ctx.lock.Lock()
	defer ctx.lock.Unlock()

	ctx.data[key] = value
}

func (ctx *Context) GetData(key string) any {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	return ctx.data[key]
}

func (ctx *Context) DataMap() echo.Map {
	return ctx.data
}

func (ctx *Context) ErrorRes(code int, message string, err error) error {
	if code >= 500 {
		var skipLogger = log.With().CallerWithSkipFrameCount(3).Logger()
		skipLogger.Error().Err(err).Msg(message)
	}

	ctx.SetRequest(ctx.Request().WithContext(context.WithValue(ctx.Request().Context(), dataKeyStr, ctx.data)))

	return &echo.HTTPError{Code: code, Message: message, Internal: err}
}

func (ctx *Context) RedirectTo(location string) error {
	return ctx.Context.Redirect(302, config.C.ExternalUrl+location)
}

func (ctx *Context) Html(template string) error {
	return ctx.HtmlWithCode(200, template)
}

func (ctx *Context) HtmlWithCode(code int, template string) error {
	ctx.setErrorFlashes()
	return ctx.Render(code, template, ctx.DataMap())
}

func (ctx *Context) Json(data any) error {
	return ctx.JsonWithCode(200, data)
}

func (ctx *Context) JsonWithCode(code int, data any) error {
	return ctx.JSON(code, data)
}

func (ctx *Context) PlainText(code int, message string) error {
	return ctx.String(code, message)
}

func (ctx *Context) NotFound(message string) error {
	return ctx.ErrorRes(404, message, nil)
}

func (ctx *Context) GetSession() *sessions.Session {
	sess, _ := ctx.store.UserStore.Get(ctx.Request(), "session")
	return sess
}

func (ctx *Context) SaveSession(sess *sessions.Session) {
	_ = sess.Save(ctx.Request(), ctx.Response())
}

func (ctx *Context) DeleteSession() {
	sess := ctx.GetSession()
	sess.Options.MaxAge = -1
	ctx.SaveSession(sess)
}

func (ctx *Context) AddFlash(flashMessage string, flashType string) {
	sess, _ := ctx.store.flashStore.Get(ctx.Request(), "flash")
	sess.AddFlash(flashMessage, flashType)
	_ = sess.Save(ctx.Request(), ctx.Response())
}

func (ctx *Context) setErrorFlashes() {
	sess, _ := ctx.store.flashStore.Get(ctx.Request(), "flash")

	ctx.SetData("flashErrors", sess.Flashes("error"))
	ctx.SetData("flashSuccess", sess.Flashes("success"))
	ctx.SetData("flashWarnings", sess.Flashes("warning"))

	_ = sess.Save(ctx.Request(), ctx.Response())
}

func (ctx *Context) DeleteCsrfCookie() {
	ctx.SetCookie(&http.Cookie{Name: "_csrf", Path: "/", MaxAge: -1})
}

func (ctx *Context) TrH(key string, args ...any) template.HTML {
	l := ctx.GetData("locale").(*i18n.Locale)
	return l.Tr(key, args...)
}

func (ctx *Context) Tr(key string, args ...any) string {
	l := ctx.GetData("locale").(*i18n.Locale)
	return l.String(key, args...)
}

var ManifestEntries map[string]Asset

type Asset struct {
	File string `json:"file"`
}
