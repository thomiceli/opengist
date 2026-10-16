package server

import (
	"errors"
	"mime"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/thomiceli/opengist/internal/web/context"
)

const maxFormSize = 10 << 20

// Method override is the first middleware to read POST forms. FormValue ignores
// parsing errors, so preserve size errors here before CSRF or binding reads the
// cached, empty form. MaxBytesReader keeps Go's existing URL-encoded form limit
// and exposes a typed error even when the request has no Content-Length.
func methodFromForm(c echo.Context) string {
	req := c.Request()
	contentType, _, err := mime.ParseMediaType(req.Header.Get(echo.HeaderContentType))
	if err == nil && contentType == echo.MIMEApplicationForm {
		req.Body = http.MaxBytesReader(c.Response(), req.Body, maxFormSize)
		var sizeErr *http.MaxBytesError
		if err := req.ParseForm(); errors.As(err, &sizeErr) {
			c.Set("formTooLarge", true)
		}
	}
	return c.FormValue("_method")
}

func gistFormSize(next Handler) Handler {
	return func(ctx *context.Context) error {
		// Wait until routing and error-page data are initialized, but report the
		// size failure before CSRF attempts to extract a token from the form.
		if ctx.Get("formTooLarge") == true && (ctx.Path() == "/" || ctx.Path() == "/:user/:gistname/edit") {
			return ctx.ErrorRes(http.StatusRequestEntityTooLarge, ctx.Tr("error.gist-form-too-large"), nil)
		}
		return next(ctx)
	}
}
