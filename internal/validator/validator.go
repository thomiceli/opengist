package validator

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/thomiceli/opengist/internal/i18n"
)

type OpengistValidator struct {
	v *validator.Validate
}

func NewValidator() *OpengistValidator {
	v := validator.New()
	_ = v.RegisterValidation("notreserved", validateReservedKeywords)
	_ = v.RegisterValidation("alphanumdash", validateAlphaNumDash)
	_ = v.RegisterValidation("alphanumdashorempty", validateAlphaNumDashOrEmpty)
	_ = v.RegisterValidation("alphanumdashunder", validateAlphaNumDashUnder)
	_ = v.RegisterValidation("alphanumdashunderorempty", validateAlphaNumDashUnderOrEmpty)
	_ = v.RegisterValidation("gisttopics", validateGistTopics)
	_ = v.RegisterValidation("expirationdate", validateExpirationDate)
	return &OpengistValidator{v}
}

func (cv *OpengistValidator) Validate(i interface{}) error {
	return cv.v.Struct(i)
}

func (cv *OpengistValidator) Var(field interface{}, tag string) error {
	return cv.v.Var(field, tag)
}

func ValidationMessages(err *error, locale *i18n.Locale) string {
	errs := (*err).(validator.ValidationErrors)
	messages := make([]string, len(errs))
	for i, e := range errs {
		switch e.Tag() {
		case "max":
			messages[i] = locale.String("validation.is-too-long", e.Field())
		case "required":
			messages[i] = locale.String("validation.should-not-be-empty", e.Field())
		case "excludes":
			messages[i] = locale.String("validation.should-not-include-sub-directory", e.Field())
		case "alphanum":
			messages[i] = locale.String("validation.should-only-contain-alphanumeric-characters", e.Field())
		case "alphanumdash", "alphanumdashorempty":
			messages[i] = locale.String("validation.should-only-contain-alphanumeric-characters-and-dashes", e.Field())
		case "alphanumdashunder", "alphanumdashunderorempty":
			messages[i] = locale.String("validation.should-only-contain-alphanumeric-characters-and-dashes-and-underscores", e.Field())
		case "min":
			messages[i] = locale.String("validation.not-enough", e.Field())
		case "notreserved":
			messages[i] = locale.String("validation.invalid", e.Field())
		case "gisttopics":
			messages[i] = locale.String("validation.invalid-gist-topics")
		case "expirationdate":
			messages[i] = locale.String("validation.invalid-expiration-date")
		}
	}

	return strings.Join(messages, " ; ")
}

func validateReservedKeywords(fl validator.FieldLevel) bool {
	name := fl.Field().String()

	restrictedNames := map[string]struct{}{}
	for _, restrictedName := range []string{"assets", "register", "login", "logout", "settings", "admin-panel", "all", "search", "init", "healthcheck", "preview", "metrics", "mfa", "webauthn", "oauth"} {
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

func validateAlphaNumDashUnder(fl validator.FieldLevel) bool {
	return regexp.MustCompile(`^[a-zA-Z0-9-_]+$`).MatchString(fl.Field().String())
}

func validateAlphaNumDashUnderOrEmpty(fl validator.FieldLevel) bool {
	return regexp.MustCompile(`^$|^[a-zA-Z0-9-_]+$`).MatchString(fl.Field().String())
}

func validateGistTopics(fl validator.FieldLevel) bool {
	topicsInput := fl.Field().String()
	if topicsInput == "" {
		return true
	}

	topics := strings.Fields(topicsInput)

	if len(topics) > 10 {
		return false
	}

	for _, tag := range topics {
		if len(tag) > 50 {
			return false
		}
		if !regexp.MustCompile(`^[\p{L}\p{N}-]+$`).MatchString(tag) {
			return false
		}
	}

	return true
}

var dateTimeLayouts = []string{
	"2006-01-02T15:04",
	"2006-01-02T15:04:05",
	time.RFC3339,
}

func validateExpirationDate(fl validator.FieldLevel) bool {
	expire := fl.Parent().FieldByName("Expire")
	if !expire.IsValid() || expire.String() != "custom" {
		return true
	}

	value := strings.TrimSpace(fl.Field().String())
	if value == "" {
		return false
	}

	t, err := ParseDateTime(value)
	if err != nil {
		return false
	}
	return t.After(time.Now())
}

func ParseDateTime(value string) (time.Time, error) {
	for _, layout := range dateTimeLayouts {
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid datetime: %q", value)
}
