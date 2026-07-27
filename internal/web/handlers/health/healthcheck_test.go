package health_test

import (
	"encoding/json"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

func TestHealthcheck(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	t.Run("OK", func(t *testing.T) {
		resp := s.Request(t, "GET", "/healthcheck", nil, 200)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		require.NoError(t, err)

		require.Equal(t, "ok", result["opengist"])
		require.Equal(t, "ok", result["database"])
		require.NotEmpty(t, result["time"])
	})
}

func TestHealthcheckHead(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	t.Run("Returns 200 without authentication", func(t *testing.T) {
		// A HEAD request to "/healthcheck" should return 200 (with no body) so
		// that uptime monitors and load balancers using `curl -I` do not see a
		// 404.
		resp := s.Request(t, "HEAD", "/healthcheck", nil, 200)

		// A HEAD response must not carry a body.
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Empty(t, body)
	})
}
