package index

import (
	"errors"
	"github.com/meilisearch/meilisearch-go"
	"github.com/rs/zerolog/log"
	"strconv"
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

func (i *MeiliIndexer) Init() {
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

func (i *MeiliIndexer) open() (meilisearch.IndexManager, error) {
	client := meilisearch.New(i.host, meilisearch.WithAPIKey(i.apikey))
	index, err := client.GetIndex(i.indexName)
	if err == nil {
		return index.IndexManager, nil
	}
	_, err = client.CreateIndex(&meilisearch.IndexConfig{
		Uid:        i.indexName,
		PrimaryKey: "GistID",
	})
	if err != nil {
		return nil, err
	}

	return client.Index(i.indexName), nil
}

func (i *MeiliIndexer) Close() {
	if i.client != nil {
		i.client.Close()
	}
	i.client = nil
}

func (i *MeiliIndexer) Add(gist *Gist) error {
	if gist == nil {
		return errors.New("failed to add nil gist to index")
	}
	_, err := (*atomicIndexer.Load()).(*MeiliIndexer).index.AddDocuments(gist, "GistID")
	return err
}

func (i *MeiliIndexer) Remove(gistID uint) error {
	_, err := (*atomicIndexer.Load()).(*MeiliIndexer).index.DeleteDocument(strconv.Itoa(int(gistID)))
	return err
}

func (i *MeiliIndexer) Search(queryStr string, queryMetadata SearchGistMetadata, gistsIds []uint, page int) ([]uint, uint64, map[string]int, error) {
	return nil, 0, nil, nil
}
