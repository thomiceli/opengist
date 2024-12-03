package context

import (
	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/i18n"
	"html/template"
	"net/http"
)

type OGContext struct {
	echo.Context

	data echo.Map

	store *Store
	User  *db.User
}

func NewContext(c echo.Context) *OGContext {
	return &OGContext{
		Context: c,
		data:    make(echo.Map),
	}
}

func (ctx *OGContext) SetData(key string, value any) {
	ctx.data[key] = value
}

func (ctx *OGContext) GetData(key string) any {
	return ctx.data[key]
}

func (ctx *OGContext) DataMap() echo.Map {
	return ctx.data
}

func (ctx *OGContext) ErrorRes(code int, message string, err error) error {
	if code >= 500 {
		var skipLogger = log.With().CallerWithSkipFrameCount(3).Logger()
		skipLogger.Error().Err(err).Msg(message)
	}

	return &echo.HTTPError{Code: code, Message: message, Internal: err}
}

func (ctx *OGContext) JsonErrorRes(code int, message string, err error) error {
	if code >= 500 {
		var skipLogger = log.With().CallerWithSkipFrameCount(3).Logger()
		skipLogger.Error().Err(err).Msg(message)
	}

	return &echo.HTTPError{Code: code, Message: message, Internal: err}
}

func (ctx *OGContext) RedirectTo(location string) error {
	return ctx.Context.Redirect(302, config.C.ExternalUrl+location)
}

func (ctx *OGContext) HTML_(template string) error {
	return ctx.HtmlWithCode(200, template)
}

func (ctx *OGContext) HtmlWithCode(code int, template string) error {
	ctx.setErrorFlashes()
	return ctx.Render(code, template, ctx.DataMap())
}

func (ctx *OGContext) JSON_(data any) error {
	return ctx.JsonWithCode(200, data)
}

func (ctx *OGContext) JsonWithCode(code int, data any) error {
	return ctx.JSON(code, data)
}

func (ctx *OGContext) PlainText(code int, message string) error {
	return ctx.String(code, message)
}

func (ctx *OGContext) NotFound(message string) error {
	return ctx.ErrorRes(404, message, nil)
}

func (ctx *OGContext) setErrorFlashes() {
	sess, _ := ctx.store.flashStore.Get(ctx.Request(), "flash")

	ctx.SetData("flashErrors", sess.Flashes("error"))
	ctx.SetData("flashSuccess", sess.Flashes("success"))
	ctx.SetData("flashWarnings", sess.Flashes("warning"))

	_ = sess.Save(ctx.Request(), ctx.Response())
}

func (ctx *OGContext) GetSession() *sessions.Session {
	sess, _ := ctx.store.UserStore.Get(ctx.Request(), "session")
	return sess
}

func (ctx *OGContext) SaveSession(sess *sessions.Session) {
	_ = sess.Save(ctx.Request(), ctx.Response())
}

func (ctx *OGContext) DeleteSession() {
	sess := ctx.GetSession()
	sess.Options.MaxAge = -1
	ctx.SaveSession(sess)
}

func (ctx *OGContext) AddFlash(flashMessage string, flashType string) {
	sess, _ := ctx.store.flashStore.Get(ctx.Request(), "flash")
	sess.AddFlash(flashMessage, flashType)
	_ = sess.Save(ctx.Request(), ctx.Response())
}

func (ctx *OGContext) getUserLogged() *db.User {
	user := ctx.GetData("userLogged")
	if user != nil {
		return user.(*db.User)
	}
	return nil
}

func (ctx *OGContext) DeleteCsrfCookie() {
	ctx.SetCookie(&http.Cookie{Name: "_csrf", Path: "/", MaxAge: -1})
}

func (ctx *OGContext) TrH(key string, args ...any) template.HTML {
	l := ctx.GetData("locale").(*i18n.Locale)
	return l.Tr(key, args...)
}

func (ctx *OGContext) Tr(key string, args ...any) string {
	l := ctx.GetData("locale").(*i18n.Locale)
	return l.String(key, args...)
}
