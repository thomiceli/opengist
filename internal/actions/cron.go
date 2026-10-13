package actions

import (
	"time"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

const cronDrainTimeout = 10 * time.Second

// StartCron registers every scheduled action in `registry` (those with a spec)
// and starts the scheduler. It returns a stop function that halts the scheduler
// and waits (up to cronDrainTimeout) for any in-flight job to finish — call it
// before tearing down the DB so a running action can release its lock cleanly.
// Panicking jobs are recovered so a single failed run can't take down the server.
func StartCron() (stop func()) {
	c := cron.New(cron.WithChain(cron.Recover(cronLogger{})))

	for actionType, a := range registry {
		if a.spec == "" {
			continue
		}
		if _, err := c.AddFunc(a.spec, func() { RunOnce(actionType) }); err != nil {
			log.Error().Err(err).Msgf("Invalid cron spec %q for action %d", a.spec, actionType)
		}
	}

	c.Start()

	return func() {
		log.Info().Msg("Stopping crons...")
		select {
		case <-c.Stop().Done():
		case <-time.After(cronDrainTimeout):
			log.Warn().Msg("cron: timed out waiting for jobs to finish")
		}
	}
}

type cronLogger struct{}

func (cronLogger) Info(msg string, _ ...interface{}) {
	log.Info().Msgf("cron: %s", msg)
}

func (cronLogger) Error(err error, msg string, _ ...interface{}) {
	log.Error().Err(err).Msgf("cron: %s", msg)
}
