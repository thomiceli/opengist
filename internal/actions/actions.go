package actions

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/index"
	"github.com/thomiceli/opengist/internal/ssh"
)

const (
	SyncReposFromFS = iota
	SyncReposFromDB
	GitGcRepos
	SyncGistPreviews
	ResetHooks
	IndexGists
	SyncGistLanguages
	DeleteExpiredGists
	SyncSSHKeys

	numActions // keep last — sizes the `running` array
)

// running tracks which actions are in progress in this instance, one slot per
// action type. It dedupes concurrent runs (e.g. a double-clicked admin button)
// and backs IsRunning for the admin panel; cross-instance single-flighting is
// handled separately by the DB action lock.
var running [numActions]atomic.Bool

const lockLease = time.Hour

type action struct {
	run  func()
	spec string
}

var registry = map[int]action{
	SyncReposFromFS:    {run: syncReposFromFS},
	SyncReposFromDB:    {run: syncReposFromDB},
	GitGcRepos:         {run: gitGcRepos},
	SyncGistPreviews:   {run: syncGistPreviews},
	ResetHooks:         {run: resetHooks},
	IndexGists:         {run: indexGists},
	SyncGistLanguages:  {run: syncGistLanguages},
	DeleteExpiredGists: {run: deleteExpiredGists, spec: "@every 1m"},
	SyncSSHKeys:        {run: syncSSHKeys, spec: "@every 72h"},
}

func IsRunning(actionType int) bool {
	return actionType >= 0 && actionType < numActions && running[actionType].Load()
}

func RunOnce(actionType int) {
	a, ok := registry[actionType]
	if !ok {
		log.Error().Msgf("Unknown action type %d", actionType)
		return
	}

	if !running[actionType].CompareAndSwap(false, true) {
		return // already running in this instance
	}
	defer running[actionType].Store(false)

	// Single-flight the action across instances sharing the database so only
	// one replica runs it at a time, whether triggered by the scheduler or
	// manually.
	acquired, err := db.AcquireLock(actionType, lockLease)
	if err != nil {
		log.Error().Err(err).Msgf("Could not acquire lock for action %d", actionType)
		return
	}
	if !acquired {
		return
	}
	defer func() {
		if err := db.ReleaseLock(actionType); err != nil {
			log.Error().Err(err).Msgf("Could not release lock for action %d", actionType)
		}
	}()

	log.Info().Msgf("Starting running action %d", actionType)
	a.run()
	log.Info().Msgf("Finished running action %d", actionType)
}

func syncReposFromFS() {
	log.Info().Msg("Syncing repositories from filesystem...")
	gists, err := db.GetAllGistsRows()
	if err != nil {
		log.Error().Err(err).Msg("Cannot get gists")
		return
	}
	for _, gist := range gists {
		// if repository does not exist, delete gist from database
		if _, err := os.Stat(git.RepositoryPath(gist.User.Username, gist.Uuid)); err != nil && !os.IsExist(err) {
			if err2 := gist.Delete(); err2 != nil {
				log.Error().Err(err2).Msgf("Cannot delete gist %d", gist.ID)
			}
		}
	}
}

func syncReposFromDB() {
	log.Info().Msg("Syncing repositories from database...")
	entries, err := filepath.Glob(filepath.Join(config.GetHomeDir(), "repos", "*", "*"))
	if err != nil {
		log.Error().Err(err).Msg("Cannot read repos directories")
		return
	}

	for _, e := range entries {
		path := strings.Split(e, string(os.PathSeparator))
		gist, _ := db.GetGist(path[len(path)-2], path[len(path)-1])

		if gist.ID == 0 {
			if err := git.DeleteRepository(path[len(path)-2], path[len(path)-1]); err != nil {
				log.Error().Err(err).Msgf("Cannot delete repository %s/%s", path[len(path)-2], path[len(path)-1])
			}
		}
	}
}

func gitGcRepos() {
	log.Info().Msg("Garbage collecting all repositories...")
	if err := git.GcRepos(); err != nil {
		log.Error().Err(err).Msg("Error garbage collecting repositories")
	}
}

func syncGistPreviews() {
	log.Info().Msg("Syncing all Gist previews...")

	gists, err := db.GetAllGistsRows()
	if err != nil {
		log.Error().Err(err).Msg("Cannot get gists")
		return
	}
	for _, gist := range gists {
		log.Info().Msgf("Syncing preview for gist %d", gist.ID)
		if err = gist.UpdatePreviewAndCount(false); err != nil {
			log.Error().Err(err).Msgf("Cannot update preview and count for gist %d", gist.ID)
		}
	}
}

func resetHooks() {
	log.Info().Msg("Resetting Git server hooks for all repositories...")
	if err := git.ResetHooks(); err != nil {
		log.Error().Err(err).Msg("Error resetting hooks for repositories")
	}
}

func indexGists() {
	log.Info().Msg("Rebuilding index from scratch...")
	if err := index.ResetIndex(); err != nil {
		log.Error().Err(err).Msg("Cannot reset index")
		return
	}

	gists, err := db.GetAllGistsRows()
	if err != nil {
		log.Error().Err(err).Msg("Cannot get gists")
		return
	}

	for _, gist := range gists {
		log.Info().Msgf("Indexing gist %d", gist.ID)
		indexedGist, err := gist.ToIndexedGist()
		if err != nil {
			log.Error().Err(err).Msgf("Cannot convert gist %d to indexed gist", gist.ID)
			continue
		}
		if err = index.AddInIndex(indexedGist); err != nil {
			log.Error().Err(err).Msgf("Cannot index gist %d", gist.ID)
		}
	}
}

func syncGistLanguages() {
	log.Info().Msg("Syncing all Gist languages...")
	gists, err := db.GetAllGistsRows()
	if err != nil {
		log.Error().Err(err).Msg("Cannot get gists")
		return
	}

	for _, gist := range gists {
		log.Info().Msgf("Syncing languages for gist %d", gist.ID)
		gist.UpdateLanguages()
	}
}

func syncSSHKeys() {
	if !config.C.SshManagesAuthorizedKeys() {
		return
	}
	log.Info().Msg("Regenerating the managed authorized_keys file...")
	if err := ssh.SyncAuthorizedKeys(); err != nil {
		log.Error().Err(err).Msg("Error regenerating the authorized_keys file")
	}
}

func deleteExpiredGists() {
	gists, err := db.DeleteExpiredGists()
	if err != nil {
		log.Error().Err(err).Msg("Cannot delete expired gists")
		return
	}

	if len(gists) > 0 {
		log.Info().Msgf("Deleted %d expired gist(s)", len(gists))
	}
}
