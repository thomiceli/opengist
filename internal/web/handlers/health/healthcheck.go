package health

import (
	"net/http"
	"time"

	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
)

// Healthcheck reports service health. A GET request returns the status as JSON;
// a HEAD request returns only the status code (no body), which is convenient
// for uptime monitors and load balancers probing the endpoint with `curl -I`.
func Healthcheck(ctx *context.Context) error {
	// Check database connection
	dbOk := "ok"
	httpStatus := 200

	err := db.Ping()
	if err != nil {
		dbOk = "ko"
		httpStatus = 503
	}

	// A HEAD request returns only the status code (no body), so uptime
	// monitors and load balancers can probe the endpoint with `curl -I`.
	if ctx.Request().Method == http.MethodHead {
		return ctx.NoContent(httpStatus)
	}

	return ctx.JSON(httpStatus, map[string]interface{}{
		"opengist": "ok",
		"database": dbOk,
		"time":     time.Now().Format(time.RFC3339),
	})
}
