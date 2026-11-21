package server_test

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/web/test"
)

// The request logger is a Pre middleware, so the error handler writes the real
// status only after it has sampled the response. A denied or missing resource
// must not be recorded as a success.
func TestRequestLogRecordsErrorStatus(t *testing.T) {
	s := test.Setup(t)
	defer test.Teardown(t)

	var logs bytes.Buffer
	previous := log.Logger
	log.Logger = zerolog.New(&logs)
	t.Cleanup(func() { log.Logger = previous })

	s.Request(t, "GET", "/api/gists/doesnotexist000000000000000000", nil, 404)

	require.Contains(t, logs.String(), `"status":404`,
		"a 404 must be logged as 404, not as the default 200")
	require.NotContains(t, logs.String(), `"status":200`,
		"logging an error response as 200 makes denied reads indistinguishable from successful ones")
}
