package index

import (
	"errors"
	"strconv"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/custom"
	"github.com/blevesearch/bleve/v2/analysis/token/camelcase"
	"github.com/blevesearch/bleve/v2/analysis/token/lowercase"
	"github.com/blevesearch/bleve/v2/analysis/token/unicodenorm"
	"github.com/blevesearch/bleve/v2/analysis/tokenizer/unicode"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
)

type BleveIndexer struct {
	index bleve.Index
	path  string
}

func NewBleveIndexer(path string) *BleveIndexer {
	return &BleveIndexer{path: path}
}

func (i *BleveIndexer) Init() error {
	errChan := make(chan error, 1)

	go func() {
		bleveIndex, err := i.open()
		if err != nil {
			log.Error().Err(err).Msg("Failed to open Bleve index")
			i.Close()
			errChan <- err
			return
		}
		i.index = bleveIndex
		log.Info().Msg("Bleve indexer initialized")
		errChan <- nil
	}()

	return <-errChan
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
	docMapping.AddFieldMappingsAt("UserID", bleve.NewNumericFieldMapping())
	docMapping.AddFieldMappingsAt("Visibility", bleve.NewNumericFieldMapping())
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
	mapping.DefaultMapping = docMapping

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

// Search returns a list of Gist IDs that match the given search metadata.
// The method returns an error if any.
//
// The queryMetadata parameter is used to filter the search results.
// For example, passing a non-empty Username will search for gists whose
// username matches the given string.
//
// If the "All" field in queryMetadata is non-empty, the method will
// search across all metadata fields with OR logic. Otherwise, the method
// will add each metadata field with AND logic.
//
// The page parameter is used to paginate the search results.
// The method returns the total number of search results in the second return
// value.
//
// The third return value is a map of language counts for the search results.
// The language counts are computed by asking Bleve to return the top 10
// facets for the "Languages" field.
func (i *BleveIndexer) Search(metadata SearchGistMetadata, userId uint, page int) ([]uint, uint64, map[string]int, error) {
	var err error
	var indexerQuery query.Query = bleve.NewMatchAllQuery()

	// Query factory
	factoryQuery := func(field, value string) query.Query {
		query := bleve.NewMatchPhraseQuery(value)
		query.SetField(field)
		return query
	}

	// Exact search
	addQuery := func(field, value string) {
		if value != "" && value != "." {
			indexerQuery = bleve.NewConjunctionQuery(indexerQuery, factoryQuery(field, value))
		}
	}

	// Exact+fuzzy query factory: exact match is boosted so it ranks above fuzzy-only matches
	factoryFuzzyQuery := func(field, value string) query.Query {
		exact := bleve.NewMatchPhraseQuery(value)
		exact.SetField(field)
		exact.SetBoost(2.0)

		fuzzy := bleve.NewMatchQuery(value)
		fuzzy.SetField(field)
		fuzzy.SetFuzziness(2)

		return bleve.NewDisjunctionQuery(exact, fuzzy)
	}

	// Exact+fuzzy search
	addFuzzy := func(field, value string) {
		if value != "" && value != "." {
			indexerQuery = bleve.NewConjunctionQuery(indexerQuery, factoryFuzzyQuery(field, value))
		}
	}

	// Visibility filtering: show public gists (Visibility=0) OR user's own gists
	visibilityZero := float64(0)
	truee := true
	publicQuery := bleve.NewNumericRangeInclusiveQuery(&visibilityZero, &visibilityZero, &truee, &truee)
	publicQuery.SetField("Visibility")

	userIdMatch := float64(userId)
	userIdQuery := bleve.NewNumericRangeInclusiveQuery(&userIdMatch, &userIdMatch, &truee, &truee)
	userIdQuery.SetField("UserID")

	accessQuery := bleve.NewDisjunctionQuery(publicQuery, userIdQuery)
	indexerQuery = bleve.NewConjunctionQuery(accessQuery, indexerQuery)

	buildFieldQuery := func(field, value string) query.Query {
		switch field {
		case "Title", "Description", "Filenames", "Content":
			return factoryFuzzyQuery(field, value)
		case "Extensions":
			return factoryQuery(field, "."+value)
		default: // Username, Languages, Topics
			return factoryQuery(field, value)
		}
	}

	// Handle "All" field - search across all metadata fields with OR logic
	if metadata.All != "" {
		allQueries := make([]query.Query, 0, len(AllSearchFields))
		for _, field := range AllSearchFields {
			allQueries = append(allQueries, buildFieldQuery(field, metadata.All))
		}
		indexerQuery = bleve.NewConjunctionQuery(indexerQuery, bleve.NewDisjunctionQuery(allQueries...))
	} else {
		// Original behavior: add each metadata field with AND logic
		addQuery("Username", metadata.Username)
		addFuzzy("Title", metadata.Title)
		addFuzzy("Description", metadata.Description)
		addQuery("Extensions", "."+metadata.Extension)
		addFuzzy("Filenames", metadata.Filename)
		addQuery("Languages", metadata.Language)
		addQuery("Topics", metadata.Topic)
		addFuzzy("Content", metadata.Content)

		// Handle default search fields from config with OR logic
		if metadata.Default != "" {
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
			if len(fields) == 1 {
				indexerQuery = bleve.NewConjunctionQuery(indexerQuery, buildFieldQuery(fields[0], metadata.Default))
			} else if len(fields) > 1 {
				defaultQueries := make([]query.Query, 0, len(fields))
				for _, field := range fields {
					defaultQueries = append(defaultQueries, buildFieldQuery(field, metadata.Default))
				}
				indexerQuery = bleve.NewConjunctionQuery(indexerQuery, bleve.NewDisjunctionQuery(defaultQueries...))
			}
		}
	}

	languageFacet := bleve.NewFacetRequest("Languages", 10)

	perPage := 10
	offset := (page - 1) * perPage

	s := bleve.NewSearchRequestOptions(indexerQuery, perPage+1, offset, false)
	s.AddFacet("languageFacet", languageFacet)
	s.Fields = []string{"GistID"}
	s.IncludeLocations = false

	log.Debug().Interface("searchRequest", s).Msg("Bleve search request")

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
