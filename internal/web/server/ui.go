package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	webcontext "github.com/thomiceli/opengist/internal/web/context"
)

const uiCookieName = "opengist_ui"

func usesLegacyUI(ctx echo.Context) bool {
	cookie, err := ctx.Cookie(uiCookieName)
	return err == nil && cookie.Value == "old"
}

func switchUI(ctx *webcontext.Context) error {
	ui := ctx.Param("ui")
	if ui != "new" && ui != "old" {
		return ctx.NotFound("UI not found")
	}

	secure := ctx.Request().TLS != nil || ctx.Request().Header.Get("X-Forwarded-Proto") == "https"
	ctx.SetCookie(&http.Cookie{
		Name:     uiCookieName,
		Value:    ui,
		Path:     "/",
		MaxAge:   int((365 * 24 * time.Hour).Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})

	if ui == "old" {
		return ctx.RedirectTo("/")
	}
	return ctx.RedirectTo(safeUIRedirect(ctx.QueryParam("redirect")))
}

func safeUIRedirect(redirect string) string {
	normalized := strings.ReplaceAll(redirect, `\`, "/")
	if redirect == "" || !strings.HasPrefix(redirect, "/") || strings.HasPrefix(normalized, "//") {
		return "/"
	}
	return redirect
}
