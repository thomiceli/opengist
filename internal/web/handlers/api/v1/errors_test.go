package v1_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/web/context"
	v1 "github.com/thomiceli/opengist/internal/web/handlers/api/v1"
)

func TestWriteJSONError(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := &context.Context{Context: e.NewContext(req, rec)}

	err := v1.WriteJSONError(ctx, http.StatusNotFound, "not_found", "gist not found")
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.True(t, strings.Contains(rec.Header().Get("Content-Type"), "application/json"))

	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "not_found", body["code"])
	require.Equal(t, "gist not found", body["error"])
}
