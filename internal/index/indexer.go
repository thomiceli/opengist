package index

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
	"path/filepath"
	"sync/atomic"
)

var atomicIndexer atomic.Pointer[Indexer]

type Indexer interface {
	Init() error
	Close()
	Add(gist *Gist) error
	Remove(gistID uint) error
	Search(query string, metadata SearchGistMetadata, userId uint, page int) ([]uint, uint64, map[string]int, error)
}

type IndexerType string

const (
	Bleve       IndexerType = "bleve"
	Meilisearch IndexerType = "meilisearch"
	None        IndexerType = ""
)

func IndexType() IndexerType {
	switch config.C.Index {
	case "bleve":
		return Bleve
	case "meilisearch":
		return Meilisearch
	default:
		return None
	}
}

func IndexEnabled() bool {
	switch config.C.Index {
	case "bleve", "meilisearch":
		return true
	default:
		return false
	}
}

func NewIndexer(idxType IndexerType) {
	if !IndexEnabled() {
		return
	}
	atomicIndexer.Store(nil)

	var idx Indexer

	switch idxType {
	case Bleve:
		idx = NewBleveIndexer(filepath.Join(config.GetHomeDir(), "opengist.index"))
	case Meilisearch:
		idx = NewMeiliIndexer(config.C.MeiliHost, config.C.MeiliAPIKey, "opengist")
	default:
		log.Warn().Msgf("Failed to create indexer, unknown indexer type: %s", idxType)
		return
	}

	if err := idx.Init(); err != nil {
		return
	}
	atomicIndexer.Store(&idx)
}

func Close() {
	if !IndexEnabled() {
		return
	}

	idx := atomicIndexer.Load()
	if idx == nil {
		return
	}

	(*idx).Close()
	atomicIndexer.Store(nil)
}

func AddInIndex(gist *Gist) error {
	if !IndexEnabled() {
		return nil
	}

	idx := atomicIndexer.Load()
	if idx == nil {
		return fmt.Errorf("indexer is not initialized")
	}

	return (*idx).Add(gist)
}

func RemoveFromIndex(gistID uint) error {
	if !IndexEnabled() {
		return nil
	}

	idx := atomicIndexer.Load()
	if idx == nil {
		return fmt.Errorf("indexer is not initialized")
	}

	return (*idx).Remove(gistID)
}

func SearchGists(query string, metadata SearchGistMetadata, userId uint, page int) ([]uint, uint64, map[string]int, error) {
	if !IndexEnabled() {
		return nil, 0, nil, nil
	}

	idx := atomicIndexer.Load()
	if idx == nil {
		return nil, 0, nil, fmt.Errorf("indexer is not initialized")
	}

	return (*idx).Search(query, metadata, userId, page)
}

func DepreactionIndexDirname() {
	if config.C.IndexEnabled {
		log.Warn().Msg("The 'index.enabled'/'OG_INDEX_ENABLED' configuration option is deprecated and will be removed in a future version. Please use 'index'/'OG_INDEX' instead.")
	}

	if config.C.Index == "" {
		config.C.Index = "bleve"
	}

	if config.C.BleveDirname != "" {
		log.Warn().Msg("The 'index.dirname'/'OG_INDEX_DIRNAME' configuration option is deprecated and will be removed in a future version.")
	}
}
