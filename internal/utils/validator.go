package utils

import (
	"github.com/go-playground/validator/v10"
	"github.com/thomiceli/opengist/internal/i18n"
	"regexp"
	"strings"
)

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
		case "alphanumdash":
		case "alphanumdashorempty":
			messages[i] = locale.String("validation.should-only-contain-alphanumeric-characters-and-dashes", e.Field())
		case "min":
			messages[i] = locale.String("validation.not-enough", e.Field())
		case "notreserved":
			messages[i] = locale.String("validation.invalid", e.Field())
		}
	}

	return strings.Join(messages, " ; ")
}

func validateReservedKeywords(fl validator.FieldLevel) bool {
	name := fl.Field().String()

	restrictedNames := map[string]struct{}{}
	for _, restrictedName := range []string{"assets", "register", "login", "logout", "settings", "admin-panel", "all", "search", "init", "healthcheck", "preview", "metrics", "mfa", "webauthn"} {
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
