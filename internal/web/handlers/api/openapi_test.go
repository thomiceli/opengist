package api_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

func TestOpenAPISpec(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	resp := s.Request(t, "GET", "/api/v1/openapi.yaml", nil, 200)
	require.Contains(t, resp.Header.Get("Content-Type"), "yaml")
}
