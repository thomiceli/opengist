package health

import "github.com/thomiceli/opengist/internal/web/context"

// Metrics is a dummy handler to satisfy the /metrics endpoint (for Prometheus, Openmetrics, etc.)
// until we have a proper metrics endpoint
func Metrics(ctx *context.Context) error {
	return ctx.String(200, "")
}
