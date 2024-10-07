package web

import (
	"context"
	"errors"
	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/i18n"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"html/template"
	"net/http"
	"strconv"
	"strings"
)

type dataTypeKey string

type HTMLError struct {
	*echo.HTTPError
}

type JSONError struct {
	*echo.HTTPError
}

const dataKey dataTypeKey = "data"

func setData(ctx echo.Context, key string, value any) {
	data := ctx.Request().Context().Value(dataKey).(echo.Map)
	data[key] = value
	ctxValue := context.WithValue(ctx.Request().Context(), dataKey, data)
	ctx.SetRequest(ctx.Request().WithContext(ctxValue))
}

func getData(ctx echo.Context, key string) any {
	data := ctx.Request().Context().Value(dataKey).(echo.Map)
	return data[key]
}

func dataMap(ctx echo.Context) echo.Map {
	return ctx.Request().Context().Value(dataKey).(echo.Map)
}

func html(ctx echo.Context, template string) error {
	return htmlWithCode(ctx, 200, template)
}

func htmlWithCode(ctx echo.Context, code int, template string) error {
	setErrorFlashes(ctx)
	return ctx.Render(code, template, ctx.Request().Context().Value(dataKey))
}

func json(ctx echo.Context, code int, data any) error {
	return ctx.JSON(code, data)
}

func redirect(ctx echo.Context, location string) error {
	return ctx.Redirect(302, config.C.ExternalUrl+location)
}

func plainText(ctx echo.Context, code int, message string) error {
	return ctx.String(code, message)
}

func notFound(message string) error {
	return errorRes(404, message, nil)
}

func errorRes(code int, message string, err error) error {
	if code >= 500 {
		var skipLogger = log.With().CallerWithSkipFrameCount(3).Logger()
		skipLogger.Error().Err(err).Msg(message)
	}

	return &HTMLError{&echo.HTTPError{Code: code, Message: message, Internal: err}}
}

func jsonErrorRes(code int, message string, err error) error {
	if code >= 500 {
		var skipLogger = log.With().CallerWithSkipFrameCount(3).Logger()
		skipLogger.Error().Err(err).Msg(message)
	}

	return &JSONError{&echo.HTTPError{Code: code, Message: message, Internal: err}}
}

func getUserLogged(ctx echo.Context) *db.User {
	user := getData(ctx, "userLogged")
	if user != nil {
		return user.(*db.User)
	}
	return nil
}

func setErrorFlashes(ctx echo.Context) {
	sess, _ := flashStore.Get(ctx.Request(), "flash")

	setData(ctx, "flashErrors", sess.Flashes("error"))
	setData(ctx, "flashSuccess", sess.Flashes("success"))

	_ = sess.Save(ctx.Request(), ctx.Response())
}

func addFlash(ctx echo.Context, flashMessage string, flashType string) {
	sess, _ := flashStore.Get(ctx.Request(), "flash")
	sess.AddFlash(flashMessage, flashType)
	_ = sess.Save(ctx.Request(), ctx.Response())
}

func getSession(ctx echo.Context) *sessions.Session {
	sess, _ := userStore.Get(ctx.Request(), "session")
	return sess
}

func saveSession(sess *sessions.Session, ctx echo.Context) {
	_ = sess.Save(ctx.Request(), ctx.Response())
}

func deleteSession(ctx echo.Context) {
	sess := getSession(ctx)
	sess.Options.MaxAge = -1
	saveSession(sess, ctx)
}

func setCsrfHtmlForm(ctx echo.Context) {
	var csrf string
	if csrfToken, ok := ctx.Get("csrf").(string); ok {
		csrf = csrfToken
	}
	setData(ctx, "csrfHtml", template.HTML(`<input type="hidden" name="_csrf" value="`+csrf+`">`))
}

func deleteCsrfCookie(ctx echo.Context) {
	ctx.SetCookie(&http.Cookie{Name: "_csrf", Path: "/", MaxAge: -1})
}

func loadSettings(ctx echo.Context) error {
	settings, err := db.GetSettings()
	if err != nil {
		return err
	}

	for key, value := range settings {
		s := strings.ReplaceAll(key, "-", " ")
		s = cases.Title(language.English).String(s)
		setData(ctx, strings.ReplaceAll(s, " ", ""), value == "1")
	}
	return nil
}

func getPage(ctx echo.Context) int {
	page := ctx.QueryParam("page")
	if page == "" {
		page = "1"
	}
	pageInt, err := strconv.Atoi(page)
	if err != nil {
		pageInt = 1
	}
	setData(ctx, "currPage", pageInt)

	return pageInt
}

func paginate[T any](ctx echo.Context, data []*T, pageInt int, perPage int, templateDataName string, urlPage string, labels int, urlParams ...string) error {
	lenData := len(data)
	if lenData == 0 && pageInt != 1 {
		return errors.New("page not found")
	}

	if lenData > perPage {
		if lenData > 1 {
			data = data[:lenData-1]
		}
		setData(ctx, "nextPage", pageInt+1)
	}
	if pageInt > 1 {
		setData(ctx, "prevPage", pageInt-1)
	}

	if len(urlParams) > 0 {
		setData(ctx, "urlParams", template.URL(urlParams[0]))
	}

	switch labels {
	case 1:
		setData(ctx, "prevLabel", trH(ctx, "pagination.previous"))
		setData(ctx, "nextLabel", trH(ctx, "pagination.next"))
	case 2:
		setData(ctx, "prevLabel", trH(ctx, "pagination.newer"))
		setData(ctx, "nextLabel", trH(ctx, "pagination.older"))
	}

	setData(ctx, "urlPage", urlPage)
	setData(ctx, templateDataName, data)
	return nil
}

func trH(ctx echo.Context, key string, args ...any) template.HTML {
	l := getData(ctx, "locale").(*i18n.Locale)
	return l.Tr(key, args...)
}

func tr(ctx echo.Context, key string, args ...any) string {
	l := getData(ctx, "locale").(*i18n.Locale)
	return l.String(key, args...)
}

func parseSearchQueryStr(query string) (string, map[string]string) {
	words := strings.Fields(query)
	metadata := make(map[string]string)
	var contentBuilder strings.Builder

	for _, word := range words {
		if strings.Contains(word, ":") {
			keyValue := strings.SplitN(word, ":", 2)
			if len(keyValue) == 2 {
				key := keyValue[0]
				value := keyValue[1]
				metadata[key] = value
			}
		} else {
			contentBuilder.WriteString(word + " ")
		}
	}

	content := strings.TrimSpace(contentBuilder.String())
	return content, metadata
}

func addMetadataToSearchQuery(input, key, value string) string {
	content, metadata := parseSearchQueryStr(input)

	metadata[key] = value

	var resultBuilder strings.Builder
	resultBuilder.WriteString(content)

	for k, v := range metadata {
		resultBuilder.WriteString(" ")
		resultBuilder.WriteString(k)
		resultBuilder.WriteString(":")
		resultBuilder.WriteString(v)
	}

	return strings.TrimSpace(resultBuilder.String())
}
