package server_test

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/test"
)

// When the database is unreachable, loading settings fails and an error page
// is rendered. That page uses the full layout, which reads .c, so the config
// must already be attached or rendering the error page itself fails.
func TestErrorPageRendersWhenDatabaseIsUnavailable(t *testing.T) {
	s := test.Setup(t)
	defer test.Teardown(t)

	require.NoError(t, db.Close())
	// deferred after Teardown, so it runs first and Teardown has a connection
	defer test.ReopenDatabase(t)

	resp := s.Request(t, "GET", "/", nil, 500)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Contains(t, string(body), "<html",
		"the error page should render, not come back empty")
}
