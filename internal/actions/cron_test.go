package actions_test

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/actions"
	"github.com/thomiceli/opengist/internal/db"
	webtest "github.com/thomiceli/opengist/internal/web/test"
	"gorm.io/gorm"
)

// TestStartCronDeletesExpiredGists - starting the scheduler deletes the gists
// that expired in the meantime right away, without waiting for the first
// scheduled run.
func TestStartCronDeletesExpiredGists(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	_, expired, _, _ := s.CreateGist(t, "0")
	_, live, _, _ := s.CreateGist(t, "0")
	expired.ExpiresAt = time.Now().Add(-time.Minute).Unix()
	require.NoError(t, expired.UpdateNoTimestamps())

	stop := actions.StartCron()
	defer stop()

	require.Eventually(t, func() bool {
		_, err := db.GetGistByID(strconv.FormatUint(uint64(expired.ID), 10))
		return errors.Is(err, gorm.ErrRecordNotFound)
	}, 5*time.Second, 50*time.Millisecond, "the expired gist should be deleted at startup")

	_, err := db.GetGistByID(strconv.FormatUint(uint64(live.ID), 10))
	require.NoError(t, err)
}
