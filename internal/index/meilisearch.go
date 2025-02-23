package index

import (
	"github.com/meilisearch/meilisearch-go"
	"github.com/rs/zerolog/log"
)

type MeiliIndexer struct {
	client    meilisearch.ServiceManager
	index     meilisearch.IndexManager
	indexName string
	host      string
	apikey    string
}

func NewMeiliIndexer(host, apikey, indexName string) *MeiliIndexer {
	return &MeiliIndexer{
		host:      host,
		apikey:    apikey,
		indexName: indexName,
	}
}

func (i *MeiliIndexer) Init() error {
	go func() {
		meiliIndex, err := i.open()
		if err != nil {
			log.Error().Err(err).Msg("Failed to open Meilisearch index")
			i.Close()
		}
		i.index = meiliIndex
		log.Info().Msg("Meilisearch indexer initialized")
	}()
}

func (i *MeiliIndexer) open() (meilisearch.ServiceManager, meilisearch.IndexManager, error) {
	client := meilisearch.New(i.host, meilisearch.WithAPIKey(i.apikey))
	index := client.Index(i.indexName)
	index.
}
