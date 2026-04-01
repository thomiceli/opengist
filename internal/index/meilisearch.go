package index

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/meilisearch/meilisearch-go"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
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
	errChan := make(chan error, 1)

	go func() {
		meiliIndex, err := i.open()
		if err != nil {
			log.Error().Err(err).Msg("Failed to open Meilisearch index")
			i.Close()
			errChan <- err
			return
		}
		i.index = meiliIndex
		log.Info().Msg("Meilisearch indexer initialized")
		errChan <- nil
	}()

	return <-errChan
}

func (i *MeiliIndexer) open() (meilisearch.IndexManager, error) {
	i.client = meilisearch.New(i.host, meilisearch.WithAPIKey(i.apikey))
	indexResult, err := i.client.GetIndex(i.indexName)

	if indexResult == nil || err != nil {
		_, err = i.client.CreateIndex(&meilisearch.IndexConfig{
			Uid:        i.indexName,
			PrimaryKey: "GistID",
		})
		if err != nil {
			return nil, err
		}
	}

	_, _ = i.client.Index(i.indexName).UpdateSettings(&meilisearch.Settings{
		FilterableAttributes: []string{"GistID", "UserID", "Visibility", "Username", "Extensions", "Languages", "Topics"},
		SearchableAttributes: []string{"Content", "ContentSplit", "Username", "Title", "Description", "Filenames", "Extensions", "Languages", "Topics"},
		RankingRules:         []string{"words", "typo", "proximity", "attribute", "sort", "exactness"},
		TypoTolerance: &meilisearch.TypoTolerance{
			Enabled:             true,
			DisableOnNumbers:    true,
			MinWordSizeForTypos: meilisearch.MinWordSizeForTypos{OneTypo: 4, TwoTypos: 10},
		},
	})

	return i.client.Index(i.indexName), nil
}

func (i *MeiliIndexer) Reset() error {
	if i.client != nil {
		taskInfo, err := i.client.DeleteIndex(i.indexName)
		if err != nil {
			return fmt.Errorf("failed to delete Meilisearch index: %w", err)
		}
		_, err = i.client.WaitForTask(taskInfo.TaskUID, 0)
		if err != nil {
			return fmt.Errorf("failed to wait for Meilisearch index deletion: %w", err)
		}
		log.Info().Msg("Meilisearch index deleted, re-creating index")
	}
	return i.Init()
}

func (i *MeiliIndexer) Close() {
	if i.client != nil {
		i.client.Close()
		log.Info().Msg("Meilisearch indexer closed")
	}
	i.client = nil
}

type meiliGist struct {
	Gist
	ContentSplit string
}

func (i *MeiliIndexer) Add(gist *Gist) error {
	if gist == nil {
		return errors.New("failed to add nil gist to index")
	}
	doc := &meiliGist{
		Gist:         *gist,
		ContentSplit: splitCamelCase(gist.Content),
	}
	primaryKey := "GistID"
	_, err := (*atomicIndexer.Load()).(*MeiliIndexer).index.AddDocuments(doc, &meilisearch.DocumentOptions{PrimaryKey: &primaryKey})
	return err
}

func (i *MeiliIndexer) Remove(gistID uint) error {
	_, err := (*atomicIndexer.Load()).(*MeiliIndexer).index.DeleteDocument(strconv.Itoa(int(gistID)), nil)
	return err
}

func (i *MeiliIndexer) Search(queryMetadata SearchGistMetadata, userId uint, page int) ([]uint, uint64, map[string]int, error) {
	searchRequest := &meilisearch.SearchRequest{
		Offset:               int64((page - 1) * 10),
		Limit:                11,
		AttributesToRetrieve: []string{"GistID", "Languages"},
		Facets:               []string{"Languages"},
		AttributesToSearchOn: []string{"Content", "ContentSplit"},
		MatchingStrategy:     meilisearch.All,
	}

	var filters []string
	filters = append(filters, fmt.Sprintf("(Visibility = 0 OR UserID = %d)", userId))

	addFilter := func(field, value string) {
		if value != "" && value != "." {
			filters = append(filters, fmt.Sprintf("%s = \"%s\"", field, escapeFilterValue(value)))
		}
	}
	var query string
	if queryMetadata.All != "" {
		query = queryMetadata.All
		searchRequest.AttributesToSearchOn = append(AllSearchFields, "ContentSplit")
	} else {
		// Exact-match fields stay as filters
		addFilter("Username", queryMetadata.Username)
		if queryMetadata.Extension != "" {
			ext := queryMetadata.Extension
			if !strings.HasPrefix(ext, ".") {
				ext = "." + ext
			}
			addFilter("Extensions", ext)
		}
		addFilter("Languages", queryMetadata.Language)
		addFilter("Topics", queryMetadata.Topic)

		if queryMetadata.Default != "" {
			query = queryMetadata.Default
			var fields []string
			for _, f := range strings.Split(config.C.SearchDefault, ",") {
				f = strings.TrimSpace(f)
				if f == "all" {
					fields = AllSearchFields
					break
				}
				if indexField, ok := SearchFieldMap[f]; ok {
					fields = append(fields, indexField)
				}
			}
			if len(fields) > 0 {
				for _, f := range fields {
					if f == "Content" {
						fields = append(fields, "ContentSplit")
						break
					}
				}
				searchRequest.AttributesToSearchOn = fields
			}
		} else {
			// Fuzzy-matchable fields become part of the query
			var queryParts []string
			var searchFields []string

			if queryMetadata.Content != "" {
				queryParts = append(queryParts, queryMetadata.Content)
				searchFields = append(searchFields, "Content", "ContentSplit")
			}
			if queryMetadata.Title != "" {
				queryParts = append(queryParts, queryMetadata.Title)
				searchFields = append(searchFields, "Title")
			}
			if queryMetadata.Description != "" {
				queryParts = append(queryParts, queryMetadata.Description)
				searchFields = append(searchFields, "Description")
			}
			if queryMetadata.Filename != "" {
				queryParts = append(queryParts, queryMetadata.Filename)
				searchFields = append(searchFields, "Filenames")
			}

			query = strings.Join(queryParts, " ")
			if len(searchFields) > 0 {
				searchRequest.AttributesToSearchOn = searchFields
			}
		}
	}

	if len(filters) > 0 {
		searchRequest.Filter = strings.Join(filters, " AND ")
	}

	response, err := (*atomicIndexer.Load()).(*MeiliIndexer).index.Search(query, searchRequest)
	if err != nil {
		log.Error().Err(err).Msg("Failed to search Meilisearch index")
		return nil, 0, nil, err
	}
	gistIds := make([]uint, 0, len(response.Hits))
	for _, hit := range response.Hits {
		if gistIDRaw, ok := hit["GistID"]; ok {
			var gistID float64
			if err := json.Unmarshal(gistIDRaw, &gistID); err == nil {
				gistIds = append(gistIds, uint(gistID))
			}
		}
	}

	languageCounts := make(map[string]int)
	if len(response.FacetDistribution) > 0 {
		var facetDist map[string]map[string]int
		if err := json.Unmarshal(response.FacetDistribution, &facetDist); err == nil {
			if facets, ok := facetDist["Languages"]; ok {
				for lang, count := range facets {
					languageCounts[strings.ToLower(lang)] += count
				}
			}
		}
	}

	return gistIds, uint64(response.EstimatedTotalHits), languageCounts, nil
}

func splitCamelCase(text string) string {
	var result strings.Builder
	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if i > 0 {
			prev := runes[i-1]
			if unicode.IsUpper(r) {
				if unicode.IsLower(prev) || unicode.IsDigit(prev) {
					result.WriteRune(' ')
				} else if unicode.IsUpper(prev) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
					result.WriteRune(' ')
				}
			} else if unicode.IsDigit(r) && !unicode.IsDigit(prev) {
				result.WriteRune(' ')
			} else if !unicode.IsDigit(r) && unicode.IsDigit(prev) {
				result.WriteRune(' ')
			}
		}
		result.WriteRune(r)
	}
	return result.String()
}

func escapeFilterValue(value string) string {
	escaped := strings.ReplaceAll(value, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
	return escaped
}
