package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/thomiceli/opengist/internal/db"
)

var (
	countUsersGauge   prometheus.Gauge
	countGistsGauge   prometheus.Gauge
	countSSHKeysGauge prometheus.Gauge

	metricsInitialized bool = false
)

func initMetrics() {
	if metricsInitialized {
		return
	}

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

func updateMetrics() {
	if !metricsInitialized {
		return
	}

	countUsers, err := db.CountAll(&db.User{})
	if err == nil {
		countUsersGauge.Set(float64(countUsers))
	}

	countGists, err := db.CountAll(&db.Gist{})
	if err == nil {
		countGistsGauge.Set(float64(countGists))
	}

	countKeys, err := db.CountAll(&db.SSHKey{})
	if err == nil {
		countSSHKeysGauge.Set(float64(countKeys))
	}
}
