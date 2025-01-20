package server

import (
	"github.com/labstack/echo/v4"
	"github.com/thomiceli/opengist/internal/web/context"
)

type Handler func(ctx *context.Context) error
type Middleware func(next Handler) Handler

func (h Handler) toEcho() echo.HandlerFunc {
	return func(c echo.Context) error {
		return h(c.(*context.Context))
	}
}

func (m Middleware) toEcho() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return m(func(c *context.Context) error {
			return next(c)
		}).toEcho()
	}
}

func (h Handler) toEchoHandler() echo.HandlerFunc {
	return func(c echo.Context) error {
		if ogc, ok := c.(*context.Context); ok {
			return h(ogc)
		}
		// Could also add error handling for incorrect context type
		return h(c.(*context.Context))
	}
}

func chain(h Handler, middleware ...Middleware) Handler {
	for i := len(middleware) - 1; i >= 0; i-- {
		h = middleware[i](h)
	}
	return h
}
