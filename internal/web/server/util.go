package server

import (
	"github.com/labstack/echo/v4"
	"github.com/thomiceli/opengist/internal/web/context"
)

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

func (h Handler) toEchoHandler() echo.HandlerFunc {
	return func(c echo.Context) error {
		if ogc, ok := c.(*context.OGContext); ok {
			return h(ogc)
		}
		// Could also add error handling for incorrect context type
		return h(c.(*context.OGContext))
	}
}

// Chain applies middleware to a handler without conversion to echo types
func Chain(h Handler, middleware ...Middleware) Handler {
	// Apply middleware in reverse order
	for i := len(middleware) - 1; i >= 0; i-- {
		h = middleware[i](h)
	}
	return h
}
