package metrics

import (
	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
)

var (
	// Using promauto to automatically register metrics with the default registry
	countUsersGauge   prometheus.Gauge
	countGistsGauge   prometheus.Gauge
	countSSHKeysGauge prometheus.Gauge

	metricsInitialized bool = false
)

// initMetrics initializes metrics if they're not already initialized
func initMetrics() {
	if metricsInitialized {
		return
	}

	// Only initialize metrics if they're enabled
	if config.C.MetricsEnabled {
		countUsersGauge = promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "opengist_users_total",
				Help: "Total number of users",
			},
		)

		countGistsGauge = promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "opengist_gists_total",
				Help: "Total number of gists",
			},
		)

		countSSHKeysGauge = promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "opengist_ssh_keys_total",
				Help: "Total number of SSH keys",
			},
		)

		metricsInitialized = true
	}
}

// updateMetrics refreshes all metric values from the database
func updateMetrics() {
	// Only update metrics if they're enabled
	if !config.C.MetricsEnabled || !metricsInitialized {
		return
	}

	// Update users count
	countUsers, err := db.CountAll(&db.User{})
	if err == nil {
		countUsersGauge.Set(float64(countUsers))
	}

	// Update gists count
	countGists, err := db.CountAll(&db.Gist{})
	if err == nil {
		countGistsGauge.Set(float64(countGists))
	}

	// Update SSH keys count
	countKeys, err := db.CountAll(&db.SSHKey{})
	if err == nil {
		countSSHKeysGauge.Set(float64(countKeys))
	}
}

// Metrics handles prometheus metrics endpoint requests.
func Metrics(ctx *context.Context) error {
	// If metrics are disabled, return 404
	if !config.C.MetricsEnabled {
		return ctx.NotFound("Metrics endpoint is disabled")
	}

	// Initialize metrics if not already done
	initMetrics()

	// Update metrics
	updateMetrics()

	// Get the Echo context
	echoCtx := ctx.Context

	// Use the Prometheus metrics handler
	handler := echoprometheus.NewHandler()

	// Call the handler
	return handler(echoCtx)
}
