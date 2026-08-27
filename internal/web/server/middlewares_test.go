package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/require"
)

func TestCSRFMiddlewareIssuesRealTokenWithSecFetchSite(t *testing.T) {
	e := echo.New()
	e.Use(csrfMiddleware())
	var token string
	e.GET("/", func(c echo.Context) error {
		token, _ = c.Get("csrf").(string)
		require.Equal(t, "same-origin", c.Request().Header.Get(echo.HeaderSecFetchSite))
		return c.NoContent(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(echo.HeaderSecFetchSite, "same-origin")
	resp := httptest.NewRecorder()

	e.ServeHTTP(resp, req)

	require.Equal(t, http.StatusNoContent, resp.Code)
	require.NotEmpty(t, token)
	require.NotEqual(t, middleware.CSRFUsingSecFetchSite, token)
	require.Len(t, resp.Result().Cookies(), 1)
	require.Equal(t, token, resp.Result().Cookies()[0].Value)
}

func TestCSRFMiddlewareRequiresTokenWithSecFetchSite(t *testing.T) {
	for _, secFetchSite := range []string{"same-origin", "none"} {
		t.Run(secFetchSite, func(t *testing.T) {
			e := echo.New()
			e.Use(csrfMiddleware())
			e.POST("/", func(c echo.Context) error {
				return c.NoContent(http.StatusNoContent)
			})

			req := httptest.NewRequest(http.MethodPost, "/", nil)
			req.Header.Set(echo.HeaderSecFetchSite, secFetchSite)
			resp := httptest.NewRecorder()

			e.ServeHTTP(resp, req)

			require.Equal(t, http.StatusBadRequest, resp.Code)
		})
	}
}

func TestCSRFMiddlewareRejectsInvalidTokenWithSecFetchSite(t *testing.T) {
	e := echo.New()
	e.Use(csrfMiddleware())
	e.POST("/", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(echo.HeaderSecFetchSite, "same-origin")
	req.Header.Set(echo.HeaderXCSRFToken, "invalid")
	req.AddCookie(&http.Cookie{Name: "_csrf", Value: "valid"})
	resp := httptest.NewRecorder()

	e.ServeHTTP(resp, req)

	require.Equal(t, http.StatusForbidden, resp.Code)
}

func TestCSRFMiddlewareAcceptsValidTokenAndPreservesSecFetchSite(t *testing.T) {
	e := echo.New()
	e.Use(csrfMiddleware())
	e.POST("/", func(c echo.Context) error {
		require.Equal(t, "same-origin", c.Request().Header.Get(echo.HeaderSecFetchSite))
		return c.NoContent(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(echo.HeaderSecFetchSite, "same-origin")
	req.Header.Set(echo.HeaderXCSRFToken, "valid")
	req.AddCookie(&http.Cookie{Name: "_csrf", Value: "valid"})
	resp := httptest.NewRecorder()

	e.ServeHTTP(resp, req)

	require.Equal(t, http.StatusNoContent, resp.Code)
}

func TestCSRFMiddlewareStillBlocksCrossSiteWithValidToken(t *testing.T) {
	e := echo.New()
	e.Use(csrfMiddleware())
	e.POST("/", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(echo.HeaderSecFetchSite, "cross-site")
	req.Header.Set(echo.HeaderXCSRFToken, "valid")
	req.AddCookie(&http.Cookie{Name: "_csrf", Value: "valid"})
	resp := httptest.NewRecorder()

	e.ServeHTTP(resp, req)

	require.Equal(t, http.StatusForbidden, resp.Code)
}

func TestCSRFMiddlewareSkipsAPI(t *testing.T) {
	e := echo.New()
	e.Use(csrfMiddleware())
	e.POST("/api/gists", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/gists", nil)
	req.Header.Set(echo.HeaderSecFetchSite, "same-origin")
	resp := httptest.NewRecorder()

	e.ServeHTTP(resp, req)

	require.Equal(t, http.StatusNoContent, resp.Code)
}
