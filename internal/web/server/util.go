package server

import (
	"github.com/labstack/echo/v4"
	"github.com/thomiceli/opengist/internal/web/context"
	"html/template"
)

func setCsrfHtmlForm(ctx *context.OGContext) {
	var csrf string
	if csrfToken, ok := ctx.Get("csrf").(string); ok {
		csrf = csrfToken
	}
	ctx.SetData("csrfHtml", template.HTML(`<input type="hidden" name="_csrf" value="`+csrf+`">`))
	ctx.SetData("csrfHtml", template.HTML(`<input type="hidden" name="_csrf" value="`+csrf+`">`))
}

type Handler func(ctx *context.OGContext) error
type Middleware func(next Handler) Handler

func (h Handler) ToEcho() echo.HandlerFunc {
	return func(c echo.Context) error {
		return h(c.(*context.OGContext))
	}
}

func (m Middleware) ToEcho() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return m(func(c *context.OGContext) error {
			return next(c)
		}).ToEcho()
	}
}
