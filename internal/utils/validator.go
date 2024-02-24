package utils

import (
	"github.com/go-playground/validator/v10"
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

func ValidationMessages(err *error) string {
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
	for _, restrictedName := range []string{"assets", "register", "login", "logout", "settings", "admin-panel", "all", "search", "init", "healthcheck", "preview"} {
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
