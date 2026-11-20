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

	// Scheduled actions first run one interval after startup, so an instance
	// restarted more often than that would never delete its expired gists:
	// also delete them right away.
	c.Schedule(&atStartup{}, cron.FuncJob(func() { RunOnce(DeleteExpiredGists) }))

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

// atStartup is a cron schedule that fires once, when the scheduler starts. Going
// through the scheduler lets the stop function wait for that run too.
type atStartup struct{ fired bool }

func (s *atStartup) Next(t time.Time) time.Time {
	if s.fired {
		return time.Time{}
	}
	s.fired = true
	return t
}

type cronLogger struct{}

func (cronLogger) Info(msg string, _ ...interface{}) {
	log.Info().Msgf("cron: %s", msg)
}

func (cronLogger) Error(err error, msg string, _ ...interface{}) {
	log.Error().Err(err).Msgf("cron: %s", msg)
}
