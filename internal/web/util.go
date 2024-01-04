package web

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/i18n"
	"golang.org/x/crypto/argon2"
	"html/template"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

type dataTypeKey string

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
	return &echo.HTTPError{Code: code, Message: message, Internal: err}
}

func getUserLogged(ctx echo.Context) *db.User {
	user := getData(ctx, "userLogged")
	if user != nil {
		return user.(*db.User)
	}
	return nil
}

func setErrorFlashes(ctx echo.Context) {
	sess, _ := store.Get(ctx.Request(), "flash")

	setData(ctx, "flashErrors", sess.Flashes("error"))
	setData(ctx, "flashSuccess", sess.Flashes("success"))

	_ = sess.Save(ctx.Request(), ctx.Response())
}

func addFlash(ctx echo.Context, flashMessage string, flashType string) {
	sess, _ := store.Get(ctx.Request(), "flash")
	sess.AddFlash(flashMessage, flashType)
	_ = sess.Save(ctx.Request(), ctx.Response())
}

func getSession(ctx echo.Context) *sessions.Session {
	sess, _ := store.Get(ctx.Request(), "session")
	return sess
}

func saveSession(sess *sessions.Session, ctx echo.Context) {
	_ = sess.Save(ctx.Request(), ctx.Response())
}

func deleteSession(ctx echo.Context) {
	sess := getSession(ctx)
	sess.Options.MaxAge = -1
	sess.Values["user"] = nil
	saveSession(sess, ctx)
}

func setCsrfHtmlForm(ctx echo.Context) {
	if csrfToken, ok := ctx.Get("csrf").(string); ok {
		setData(ctx, "csrfHtml", template.HTML(`<input type="hidden" name="_csrf" value="`+csrfToken+`">`))
	}
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
		s = title.String(s)
		setData(ctx, strings.ReplaceAll(s, " ", ""), value == "1")
	}
	return nil
}

type OpengistValidator struct {
	v *validator.Validate
}

func NewValidator() *OpengistValidator {
	v := validator.New()
	_ = v.RegisterValidation("notreserved", validateReservedKeywords)
	_ = v.RegisterValidation("alphanumdash", validateAlphaNumDash)
	_ = v.RegisterValidation("alphanumdashorempty", validateAlphaNumDashOrEmpty)
	return &OpengistValidator{v}
}

func (cv *OpengistValidator) Validate(i interface{}) error {
	if err := cv.v.Struct(i); err != nil {
		return err
	}
	return nil
}

func validationMessages(err *error) string {
	errs := (*err).(validator.ValidationErrors)
	messages := make([]string, len(errs))
	for i, e := range errs {
		switch e.Tag() {
		case "max":
			messages[i] = e.Field() + " is too long"
		case "required":
			messages[i] = e.Field() + " should not be empty"
		case "excludes":
			messages[i] = e.Field() + " should not include a sub directory"
		case "alphanum":
			messages[i] = e.Field() + " should only contain alphanumeric characters"
		case "alphanumdash":
		case "alphanumdashorempty":
			messages[i] = e.Field() + " should only contain alphanumeric characters and dashes"
		case "min":
			messages[i] = "Not enough " + e.Field()
		case "notreserved":
			messages[i] = "Invalid " + e.Field()
		}
	}

	return strings.Join(messages, " ; ")
}

func validateReservedKeywords(fl validator.FieldLevel) bool {
	name := fl.Field().String()

	restrictedNames := map[string]struct{}{}
	for _, restrictedName := range []string{"assets", "register", "login", "logout", "settings", "admin-panel", "all", "search", "init", "healthcheck"} {
		restrictedNames[restrictedName] = struct{}{}
	}

	// if the name is not in the restricted names, it is valid
	_, ok := restrictedNames[name]
	return !ok
}

func validateAlphaNumDash(fl validator.FieldLevel) bool {
	return regexp.MustCompile(`^[a-zA-Z0-9-]+$`).MatchString(fl.Field().String())
}

func validateAlphaNumDashOrEmpty(fl validator.FieldLevel) bool {
	return regexp.MustCompile(`^$|^[a-zA-Z0-9-]+$`).MatchString(fl.Field().String())
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
		setData(ctx, "prevLabel", tr(ctx, "pagination.previous"))
		setData(ctx, "nextLabel", tr(ctx, "pagination.next"))
	case 2:
		setData(ctx, "prevLabel", tr(ctx, "pagination.newer"))
		setData(ctx, "nextLabel", tr(ctx, "pagination.older"))
	}

	setData(ctx, "urlPage", urlPage)
	setData(ctx, templateDataName, data)
	return nil
}

func tr(ctx echo.Context, key string) template.HTML {
	l := getData(ctx, "locale").(*i18n.Locale)
	return l.Tr(key)
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

type Argon2ID struct {
	format  string
	version int
	time    uint32
	memory  uint32
	keyLen  uint32
	saltLen uint32
	threads uint8
}

var argon2id = Argon2ID{
	format:  "$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
	version: argon2.Version,
	time:    1,
	memory:  64 * 1024,
	keyLen:  32,
	saltLen: 16,
	threads: 4,
}

func (a Argon2ID) hash(plain string) (string, error) {
	salt := make([]byte, a.saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(plain), salt, a.time, a.memory, a.threads, a.keyLen)

	return fmt.Sprintf(a.format, a.version, a.memory, a.time, a.threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func (a Argon2ID) verify(plain, hash string) (bool, error) {
	if hash == "" {
		return false, nil
	}

	hashParts := strings.Split(hash, "$")

	if len(hashParts) != 6 {
		return false, errors.New("invalid hash")
	}

	_, err := fmt.Sscanf(hashParts[3], "m=%d,t=%d,p=%d", &a.memory, &a.time, &a.threads)
	if err != nil {
		return false, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(hashParts[4])
	if err != nil {
		return false, err
	}

	decodedHash, err := base64.RawStdEncoding.DecodeString(hashParts[5])
	if err != nil {
		return false, err
	}

	hashToCompare := argon2.IDKey([]byte(plain), salt, a.time, a.memory, a.threads, uint32(len(decodedHash)))

	return subtle.ConstantTimeCompare(decodedHash, hashToCompare) == 1, nil
}
