package v1

import (
	"github.com/thomiceli/opengist/internal/web/context"
)

// ErrorBody is the unified envelope returned by every API error response.
type ErrorBody struct {
	Message string `json:"error"`
	Code    string `json:"code"`
}

// WriteJSONError writes an application/json error response.
// status is the HTTP status code, code is a machine-readable identifier (snake_case),
// msg is the human-readable message.
func WriteJSONError(ctx *context.Context, status int, code, msg string) error {
	return ctx.JSON(status, ErrorBody{Message: msg, Code: code})
}
