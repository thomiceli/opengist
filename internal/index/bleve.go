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
	"github.com/thomiceli/opengist/internal/config"
	"strconv"
	"sync/atomic"
)

var atomicIndexer atomic.Pointer[Indexer]

type Indexer struct {
	Index bleve.Index
}

func Enabled() bool {
	return config.C.IndexEnabled
}

func Init(indexFilename string) {
	atomicIndexer.Store(&Indexer{Index: nil})

	go func() {
		bleveIndex, err := open(indexFilename)
		if err != nil {
			log.Error().Err(err).Msg("Failed to open index")
			(*atomicIndexer.Load()).close()
		}
		atomicIndexer.Store(&Indexer{Index: bleveIndex})
		log.Info().Msg("Indexer initialized")
	}()
}

func open(indexFilename string) (bleve.Index, error) {
	bleveIndex, err := bleve.Open(indexFilename)
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

	return bleve.New(indexFilename, mapping)
}

func Close() {
	(*atomicIndexer.Load()).close()
}

func (i *Indexer) close() {
	if i == nil || i.Index == nil {
		return
	}

	err := i.Index.Close()
	if err != nil {
		log.Error().Err(err).Msg("Failed to close bleve index")
	}
	log.Info().Msg("Indexer closed")
	atomicIndexer.Store(&Indexer{Index: nil})
}

func checkForIndexer() error {
	if (*atomicIndexer.Load()).Index == nil {
		return errors.New("indexer is not initialized")
	}

	return nil
}

func AddInIndex(gist *Gist) error {
	if !Enabled() {
		return nil
	}
	if err := checkForIndexer(); err != nil {
		return err
	}

	if gist == nil {
		return errors.New("failed to add nil gist to index")
	}
	return (*atomicIndexer.Load()).Index.Index(strconv.Itoa(int(gist.GistID)), gist)
}

func RemoveFromIndex(gistID uint) error {
	if !Enabled() {
		return nil
	}
	if err := checkForIndexer(); err != nil {
		return err
	}

	return (*atomicIndexer.Load()).Index.Delete(strconv.Itoa(int(gistID)))
}

func SearchGists(queryStr string, queryMetadata SearchGistMetadata, gistsIds []uint, page int) ([]uint, uint64, map[string]int, error) {
	if !Enabled() {
		return nil, 0, nil, nil
	}
	if err := checkForIndexer(); err != nil {
		return nil, 0, nil, err
	}

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

	repoQueries := make([]query.Query, 0, len(gistsIds))

	truee := true
	for _, id := range gistsIds {
		f := float64(id)
		qq := bleve.NewNumericRangeInclusiveQuery(&f, &f, &truee, &truee)
		qq.SetField("GistID")
		repoQueries = append(repoQueries, qq)
	}

	indexerQuery = bleve.NewConjunctionQuery(bleve.NewDisjunctionQuery(repoQueries...), indexerQuery)

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
	addQuery("Tags", queryMetadata.Tag)

	languageFacet := bleve.NewFacetRequest("Languages", 10)

	perPage := 10
	offset := (page - 1) * perPage

	s := bleve.NewSearchRequestOptions(indexerQuery, perPage, offset, false)
	s.AddFacet("languageFacet", languageFacet)
	s.Fields = []string{"GistID"}
	s.IncludeLocations = false

	results, err := (*atomicIndexer.Load()).Index.Search(s)
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
