package index

import (
	"errors"
	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/custom"
	"github.com/blevesearch/bleve/v2/analysis/token/camelcase"
	"github.com/blevesearch/bleve/v2/analysis/token/lowercase"
	"github.com/blevesearch/bleve/v2/analysis/token/unicodenorm"
	"github.com/blevesearch/bleve/v2/analysis/tokenizer/unicode"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/rs/zerolog/log"
	"strconv"
)

type BleveIndexer struct {
	index bleve.Index
	path  string
}

func NewBleveIndexer(path string) *BleveIndexer {
	return &BleveIndexer{path: path}
}

func (i *BleveIndexer) Init() {
	go func() {
		bleveIndex, err := i.open()
		if err != nil {
			log.Error().Err(err).Msg("Failed to open Bleve index")
			i.Close()
		}
		i.index = bleveIndex
		log.Info().Msg("Bleve indexer initialized")
	}()
}

func (i *BleveIndexer) open() (bleve.Index, error) {
	bleveIndex, err := bleve.Open(i.path)
	if err == nil {
		return bleveIndex, nil
	}

	if !errors.Is(err, bleve.ErrorIndexPathDoesNotExist) {
		return nil, err
	}

	docMapping := bleve.NewDocumentMapping()
	docMapping.AddFieldMappingsAt("GistID", bleve.NewNumericFieldMapping())
	docMapping.AddFieldMappingsAt("Content", bleve.NewTextFieldMapping())

	mapping := bleve.NewIndexMapping()

	if err = mapping.AddCustomTokenFilter("unicodeNormalize", map[string]any{
		"type": unicodenorm.Name,
		"form": unicodenorm.NFC,
	}); err != nil {
		return nil, err
	}

	if err = mapping.AddCustomAnalyzer("gistAnalyser", map[string]interface{}{
		"type":          custom.Name,
		"char_filters":  []string{},
		"tokenizer":     unicode.Name,
		"token_filters": []string{"unicodeNormalize", camelcase.Name, lowercase.Name},
	}); err != nil {
		return nil, err
	}

	docMapping.DefaultAnalyzer = "gistAnalyser"

	return bleve.New(i.path, mapping)
}

func (i *BleveIndexer) Close() {
	if i == nil || i.index == nil {
		return
	}

	err := i.index.Close()
	if err != nil {
		log.Error().Err(err).Msg("Failed to close Bleve index")
	}
	log.Info().Msg("Bleve indexer closed")
}

func (i *BleveIndexer) Add(gist *Gist) error {
	if gist == nil {
		return errors.New("failed to add nil gist to index")
	}
	return (*atomicIndexer.Load()).(*BleveIndexer).index.Index(strconv.Itoa(int(gist.GistID)), gist)
}

func (i *BleveIndexer) Remove(gistID uint) error {
	return (*atomicIndexer.Load()).(*BleveIndexer).index.Delete(strconv.Itoa(int(gistID)))
}

func (i *BleveIndexer) Search(queryStr string, queryMetadata SearchGistMetadata, userId uint, page int) ([]uint, uint64, map[string]int, error) {
	var err error
	var indexerQuery query.Query
	if queryStr != "" {
		contentQuery := bleve.NewMatchPhraseQuery(queryStr)
		contentQuery.FieldVal = "Content"
		indexerQuery = contentQuery
	} else {
		contentQuery := bleve.NewMatchAllQuery()
		indexerQuery = contentQuery
	}

	privateQuery := bleve.NewBoolFieldQuery(false)
	privateQuery.SetField("Private")

	userIdMatch := float64(userId)
	truee := true
	userIdQuery := bleve.NewNumericRangeInclusiveQuery(&userIdMatch, &userIdMatch, &truee, &truee)
	userIdQuery.SetField("UserID")

	accessQuery := bleve.NewDisjunctionQuery(privateQuery, userIdQuery)
	indexerQuery = bleve.NewConjunctionQuery(accessQuery, indexerQuery)

	addQuery := func(field, value string) {
		if value != "" && value != "." {
			q := bleve.NewMatchPhraseQuery(value)
			q.FieldVal = field
			indexerQuery = bleve.NewConjunctionQuery(indexerQuery, q)
		}
	}

	addQuery("Username", queryMetadata.Username)
	addQuery("Title", queryMetadata.Title)
	addQuery("Extensions", "."+queryMetadata.Extension)
	addQuery("Filenames", queryMetadata.Filename)
	addQuery("Languages", queryMetadata.Language)
	addQuery("Topics", queryMetadata.Topic)

	languageFacet := bleve.NewFacetRequest("Languages", 10)

	perPage := 10
	offset := (page - 1) * perPage

	s := bleve.NewSearchRequestOptions(indexerQuery, perPage+1, offset, false)
	s.AddFacet("languageFacet", languageFacet)
	s.Fields = []string{"GistID"}
	s.IncludeLocations = false

	results, err := (*atomicIndexer.Load()).(*BleveIndexer).index.Search(s)
	if err != nil {
		return nil, 0, nil, err
	}

	gistIds := make([]uint, 0, len(results.Hits))
	for _, hit := range results.Hits {
		gistIds = append(gistIds, uint(hit.Fields["GistID"].(float64)))
	}

	languageCounts := make(map[string]int)
	if facets, found := results.Facets["languageFacet"]; found {
		for _, term := range facets.Terms.Terms() {
			languageCounts[term.Term] = term.Count
		}
	}

	return gistIds, results.Total, languageCounts, nil
}
