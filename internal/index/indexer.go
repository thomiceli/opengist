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
	Init()
	Close()
	Add(gist *Gist) error
	Remove(gistID uint) error
	Search(query string, metadata SearchGistMetadata, gistIDs []uint, page int) ([]uint, uint64, map[string]int, error)
}

type IndexerType string

const (
	Bleve       IndexerType = "bleve"
	Meilisearch IndexerType = "meilisearch"
)

func Enabled() bool {
	return config.C.IndexEnabled
}

func NewIndexer(idxType IndexerType) {
	if !Enabled() {
		return
	}
	atomicIndexer.Store(nil)

	var idx Indexer

	switch idxType {
	case Bleve:
		idx = NewBleveIndexer(filepath.Join(config.GetHomeDir(), config.C.BleveDirname))
	case Meilisearch:
		idx = NewMeiliIndexer(config.C.MeiliHost, config.C.MeiliAPIKey, "opengist")
	default:
		log.Warn().Msgf("Failed to create indexer, unknown indexer type: %s", idxType)
		return
	}

	idx.Init()
	atomicIndexer.Store(&idx)
}

func Close() {
	if !Enabled() {
		return
	}

	idx := (*atomicIndexer.Load()).(Indexer)
	if idx == nil {
		return
	}

	idx.Close()
	atomicIndexer.Store(nil)
}

func AddInIndex(gist *Gist) error {
	if !Enabled() {
		return nil
	}

	idx := (*atomicIndexer.Load()).(Indexer)
	if idx == nil {
		return fmt.Errorf("indexer is not initialized")
	}

	return idx.Add(gist)
}

func RemoveFromIndex(gistID uint) error {
	if !Enabled() {
		return nil
	}

	idx := (*atomicIndexer.Load()).(Indexer)
	if idx == nil {
		return fmt.Errorf("indexer is not initialized")
	}

	return idx.Remove(gistID)
}

func SearchGists(query string, metadata SearchGistMetadata, gistIDs []uint, page int) ([]uint, uint64, map[string]int, error) {
	if !Enabled() {
		return nil, 0, nil, nil
	}

	idx := (*atomicIndexer.Load()).(Indexer)
	if idx == nil {
		return nil, 0, nil, fmt.Errorf("indexer is not initialized")
	}

	return idx.Search(query, metadata, gistIDs, page)
}
