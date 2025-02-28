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
	index, err := client.GetIndex(i.indexName)
	_, _ = index.IndexManager.UpdateSettings(&meilisearch.Settings{
		FilterableAttributes: []string{"Username", "Title", "Filenames", "Extensions", "Languages", "Topics"},
		DisplayedAttributes:  []string{"GistID"},
		Pagination:           &meilisearch.Pagination{MaxTotalHits: 5000},
		SearchableAttributes: []string{"Content", "Username", "Title", "Filenames", "Extensions", "Languages", "Topics"},
	})
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

	indexManager := client.Index(i.indexName)

	return indexManager, nil
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
	searchRequest := &meilisearch.SearchRequest{
		Offset:               int64((page - 1) * 10),
		Limit:                11, // 10 + 1 to check if there are more results
		AttributesToRetrieve: []string{"GistID"},
	}

	if queryStr != "" {
		searchRequest.AttributesToSearchOn = []string{"Content"}
	}

	var filters []string

	// Add metadata filters
	if queryMetadata.Username != "" && queryMetadata.Username != "." {
		filters = append(filters, fmt.Sprintf("Username = \"%s\"", queryMetadata.Username))
	}

	if queryMetadata.Title != "" && queryMetadata.Title != "." {
		filters = append(filters, fmt.Sprintf("Title = \"%s\"", queryMetadata.Title))
	}

	if queryMetadata.Filename != "" && queryMetadata.Filename != "." {
		filters = append(filters, fmt.Sprintf("Filenames = \"%s\"", queryMetadata.Filename))
	}

	if queryMetadata.Extension != "" && queryMetadata.Extension != "." {
		filters = append(filters, fmt.Sprintf("Extensions = \".%s\"", queryMetadata.Extension))
	}

	if queryMetadata.Language != "" && queryMetadata.Language != "." {
		filters = append(filters, fmt.Sprintf("Languages = \"%s\"", queryMetadata.Language))
	}

	if queryMetadata.Topic != "" && queryMetadata.Topic != "." {
		filters = append(filters, fmt.Sprintf("Topics = \"%s\"", queryMetadata.Topic))
	}

	// Combine all filters with AND
	if len(filters) > 0 {
		fmt.Println("filters", strings.Join(filters, " AND "))
		searchRequest.Filter = strings.Join(filters, " AND ")
	}

	var response *meilisearch.SearchResponse
	var err error

	if queryStr != "" {
		fmt.Println("Query string:", queryStr)
		response, err = i.index.Search(queryStr, searchRequest)
	} else {
		response, err = i.index.Search("", searchRequest)
	}

	if err != nil {
		return nil, 0, nil, err
	}

	fmt.Println(response)
	fmt.Println(response.EstimatedTotalHits)
	gistIds := make([]uint, 0, len(response.Hits))
	for _, hit := range response.Hits {
		gistIds = append(gistIds, uint(hit.(map[string]interface{})["GistID"].(float64)))
	}

	return gistIds, uint64(response.EstimatedTotalHits), nil, nil
}
