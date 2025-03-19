package index

import (
	"errors"
	"fmt"
	"github.com/meilisearch/meilisearch-go"
	"github.com/rs/zerolog/log"
	"strconv"
	"strings"
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
	indexResult, err := client.GetIndex(i.indexName)
	if err != nil {
		return nil, err
	}

	if indexResult != nil {
		return indexResult.IndexManager, nil
	}
	_, err = client.CreateIndex(&meilisearch.IndexConfig{
		Uid:        i.indexName,
		PrimaryKey: "GistID",
	})
	if err != nil {
		return nil, err
	}

	_, _ = client.Index(i.indexName).UpdateSettings(&meilisearch.Settings{
		FilterableAttributes: []string{"GistID", "UserID", "Visibility", "Username", "Title", "Filenames", "Extensions", "Languages", "Topics"},
		DisplayedAttributes:  []string{"GistID"},
		SearchableAttributes: []string{"Content", "Username", "Title", "Filenames", "Extensions", "Languages", "Topics"},
		RankingRules:         []string{"words"},
	})

	return client.Index(i.indexName), nil
}

func (i *MeiliIndexer) Close() {
	if i.client != nil {
		i.client.Close()
		log.Info().Msg("Meilisearch indexer closed")
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

func (i *MeiliIndexer) Search(queryStr string, queryMetadata SearchGistMetadata, userId uint, page int) ([]uint, uint64, map[string]int, error) {
	searchRequest := &meilisearch.SearchRequest{
		Offset:               int64((page - 1) * 10),
		Limit:                11,
		AttributesToRetrieve: []string{"GistID", "Languages"},
		Facets:               []string{"Languages"},
		AttributesToSearchOn: []string{"Content"},
	}

	var filters []string
	filters = append(filters, fmt.Sprintf("(Visibility = 0 OR UserID = %d)", userId))

	addFilter := func(field, value string) {
		if value != "" && value != "." {
			filters = append(filters, fmt.Sprintf("%s = \"%s\"", field, escapeFilterValue(value)))
		}
	}
	addFilter("Username", queryMetadata.Username)
	addFilter("Title", queryMetadata.Title)
	addFilter("Filenames", queryMetadata.Filename)
	addFilter("Extensions", queryMetadata.Extension)
	addFilter("Languages", queryMetadata.Language)
	addFilter("Topics", queryMetadata.Topic)

	if len(filters) > 0 {
		searchRequest.Filter = strings.Join(filters, " AND ")
	}

	response, err := (*atomicIndexer.Load()).(*MeiliIndexer).index.Search(queryStr, searchRequest)
	if err != nil {
		log.Error().Err(err).Msg("Failed to search Meilisearch index")
		return nil, 0, nil, err
	}

	gistIds := make([]uint, 0, len(response.Hits))
	for _, hit := range response.Hits {
		if gistID, ok := hit.(map[string]interface{})["GistID"].(float64); ok {
			gistIds = append(gistIds, uint(gistID))
		}
	}

	languageCounts := make(map[string]int)
	if facets, ok := response.FacetDistribution.(map[string]interface{})["Languages"]; ok {
		for language, count := range facets.(map[string]interface{}) {
			if countValue, ok := count.(float64); ok {
				languageCounts[language] = int(countValue)
			}
		}
	}

	return gistIds, uint64(response.EstimatedTotalHits), languageCounts, nil
}

func escapeFilterValue(value string) string {
	escaped := strings.ReplaceAll(value, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
	return escaped
}
