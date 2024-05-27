package actions

import (
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/index"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type ActionStatus struct {
	Running bool
}

const (
	SyncReposFromFS = iota
	SyncReposFromDB
	GitGcRepos
	SyncGistPreviews
	ResetHooks
	IndexGists
)

var (
	mutex   sync.Mutex
	actions = make(map[int]ActionStatus)
)

func updateActionStatus(actionType int, running bool) {
	actions[actionType] = ActionStatus{
		Running: running,
	}
}

func IsRunning(actionType int) bool {
	mutex.Lock()
	defer mutex.Unlock()
	return actions[actionType].Running
}

func Run(actionType int) {
	mutex.Lock()

	if actions[actionType].Running {
		mutex.Unlock()
		return
	}

	updateActionStatus(actionType, true)
	mutex.Unlock()

	defer func() {
		mutex.Lock()
		updateActionStatus(actionType, false)
		mutex.Unlock()
	}()

	var functionToRun func()
	switch actionType {
	case SyncReposFromFS:
		functionToRun = syncReposFromFS
	case SyncReposFromDB:
		functionToRun = syncReposFromDB
	case GitGcRepos:
		functionToRun = gitGcRepos
	case SyncGistPreviews:
		functionToRun = syncGistPreviews
	case ResetHooks:
		functionToRun = resetHooks
	case IndexGists:
		functionToRun = indexGists
	default:
		log.Error().Msg("Unknown action type")
	}

	functionToRun()
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
		if err = gist.UpdatePreviewAndCount(false); err != nil {
			log.Error().Err(err).Msgf("Cannot update preview and count for gist %d", gist.ID)
		}
	}
}

func resetHooks() {
	log.Info().Msg("Resetting Git server hooks for all repositories...")
	entries, err := filepath.Glob(filepath.Join(config.GetHomeDir(), "repos", "*", "*"))
	if err != nil {
		log.Error().Err(err).Msg("Cannot read repos directories")
		return
	}

	for _, e := range entries {
		path := strings.Split(e, string(os.PathSeparator))
		if err := git.CreateDotGitFiles(path[len(path)-2], path[len(path)-1]); err != nil {
			log.Error().Err(err).Msgf("Cannot reset hooks for repository %s/%s", path[len(path)-2], path[len(path)-1])
		}
	}
}

func indexGists() {
	log.Info().Msg("Indexing all Gists...")
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
