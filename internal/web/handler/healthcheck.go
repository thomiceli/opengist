package handler

import (
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
	"time"
)

func healthcheck(ctx *context.OGContext) error {
	// Check database connection
	dbOk := "ok"
	httpStatus := 200

	err := db.Ping()
	if err != nil {
		dbOk = "ko"
		httpStatus = 503
	}

	return ctx.JSON(httpStatus, map[string]interface{}{
		"opengist": "ok",
		"database": dbOk,
		"time":     time.Now().Format(time.RFC3339),
	})
}

// metrics is a dummy handler to satisfy the /metrics endpoint (for Prometheus, Openmetrics, etc.)
// until we have a proper metrics endpoint
func metrics(ctx *context.OGContext) error {
	return ctx.String(200, "")
}
