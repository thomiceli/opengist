package index

import (
	"errors"
	"strconv"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/custom"
	"github.com/blevesearch/bleve/v2/analysis/token/camelcase"
	"github.com/blevesearch/bleve/v2/analysis/token/length"
	"github.com/blevesearch/bleve/v2/analysis/token/lowercase"
	"github.com/blevesearch/bleve/v2/analysis/token/unicodenorm"

	bleveUnicode "github.com/blevesearch/bleve/v2/analysis/tokenizer/unicode"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/rs/zerolog/log"
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

	// ==========================================
    // 1️⃣ Define mapping rules
	mapping := bleve.NewIndexMapping()

	// Length Filter
	if err = mapping.AddCustomTokenFilter("length_filter_min2", map[string]interface{}{
		"type": length.Name,
		"min":  2.0,
	}); err != nil {
		return nil, err
	}

	// Unicode Normalize Filter
	if err = mapping.AddCustomTokenFilter("unicodeNormalize", map[string]any{
		"type": unicodenorm.Name,
		"form": unicodenorm.NFC,
	}); err != nil {
		return nil, err
	}

	// --- Analyzer 1: Split mode (for partial search) Effect: "UserLogin" -> "user", "login" ---
	if err = mapping.AddCustomAnalyzer("code_split", map[string]interface{}{
		"type":      custom.Name,
		"tokenizer": bleveUnicode.Name,
		"token_filters": []string{
			"unicodeNormalize",
			camelcase.Name,       // Core: split camel case
			lowercase.Name,       // To lowercase
			"length_filter_min2", // Remove too short tokens after splitting
		},
	}); err != nil {
		return nil, err
	}

	// --- Analyzer 2: Exact mode (for full word search) Effect: "UserLogin" -> "userlogin" ---
	if err = mapping.AddCustomAnalyzer("code_exact", map[string]interface{}{
		"type":      custom.Name,
		"tokenizer": bleveUnicode.Name,
		"token_filters": []string{
			"unicodeNormalize",
			lowercase.Name,       // To lowercase only, no splitting!
			"length_filter_min2",
		},
	}); err != nil {
		return nil, err
	}

	docMapping := bleve.NewDocumentMapping()

	// Numeric fields
	docMapping.AddFieldMappingsAt("GistID", bleve.NewNumericFieldMapping())
	docMapping.AddFieldMappingsAt("UserID", bleve.NewNumericFieldMapping())
	docMapping.AddFieldMappingsAt("Visibility", bleve.NewNumericFieldMapping())

	// Metadata fields (title, filenames, etc., usually suitable for split search)
	metaMapping := bleve.NewTextFieldMapping()
	metaMapping.Analyzer = "code_split"
	docMapping.AddFieldMappingsAt("Username", metaMapping)
	docMapping.AddFieldMappingsAt("Title", metaMapping)
	docMapping.AddFieldMappingsAt("Filenames", metaMapping)
	docMapping.AddFieldMappingsAt("Extensions", metaMapping)
	docMapping.AddFieldMappingsAt("Languages", metaMapping)
	docMapping.AddFieldMappingsAt("Topics", metaMapping)


	// --- Core Content field dual mapping ---
    
    // Mapping A: Content (exact match) store: "userlogin"
	contentExact := bleve.NewTextFieldMapping()
	contentExact.Name = "Content" // Field name
	contentExact.Analyzer = "code_exact"
	contentExact.Store = false
	contentExact.IncludeTermVectors = true

	// Mapping B: ContentSplit (split match) store: "user", "login"
	contentSplit := bleve.NewTextFieldMapping()
	contentSplit.Name = "ContentSplit" // Virtual field name
	contentSplit.Analyzer = "code_split"
	contentSplit.Store = false
	contentSplit.IncludeTermVectors = true

	// Combine both mappings into the document mapping
	docMapping.AddFieldMappingsAt("Content", contentExact, contentSplit)
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

func (i *BleveIndexer) Search(queryStr string, queryMetadata SearchGistMetadata, userId uint, page int) ([]uint, uint64, map[string]int, error) {
	var err error
	var indexerQuery query.Query
	
	// ==========================================
    // Search Query Construction
	if queryStr != "" {
		queryStr = strings.ToLower(strings.TrimSpace(queryStr))

        // Search Content (exact): higher weight, matches "userlogin"
		qExact := bleve.NewMatchQuery(queryStr)
		qExact.SetField("Content")
		qExact.SetBoost(2.0)

		qSplit := bleve.NewMatchQuery(queryStr)
		qSplit.SetField("ContentSplit")
		qSplit.SetBoost(1.0)

		qPrefix := bleve.NewPrefixQuery(queryStr)
		qPrefix.SetField("Content") // Search exact field
		qPrefix.SetBoost(1.5)
		
		qWildcard := bleve.NewWildcardQuery("*" + queryStr + "*")
		qWildcard.SetField("Content")
		qWildcard.SetBoost(0.5)


        // Metadata queries
        titleQuery := bleve.NewMatchQuery(queryStr)
        titleQuery.SetField("Title")
        titleQuery.SetBoost(3.0)

		titleWildcard := bleve.NewWildcardQuery("*" + queryStr + "*")
		titleWildcard.SetField("Title")
		titleWildcard.SetBoost(1.5)

        usernameQuery := bleve.NewMatchQuery(queryStr)
        usernameQuery.SetField("Username")
        usernameQuery.SetBoost(2.0)
        
        filenameQuery := bleve.NewMatchQuery(queryStr)
        filenameQuery.SetField("Filenames")
        filenameQuery.SetBoost(2.5)

		queries := []query.Query{qExact, qSplit, titleQuery, usernameQuery, filenameQuery}
		runes := []rune(queryStr)		// For Chinese length
		qLen := len(runes)

		// Protect for cpu loading when query is too short
		if qLen >= 2 {
			qPrefix := bleve.NewPrefixQuery(queryStr)
			qPrefix.SetField("Content")
			qPrefix.SetBoost(1.5)
			queries = append(queries, qPrefix)
		}
		if qLen >= 4 {
			qWildcard := bleve.NewWildcardQuery("*" + queryStr + "*")
			qWildcard.SetField("Content")
			qWildcard.SetBoost(0.5)
			queries = append(queries, qWildcard)
			
			titleWildcard := bleve.NewWildcardQuery("*" + queryStr + "*")
			titleWildcard.SetField("Title")
			titleWildcard.SetBoost(1.5)
			queries = append(queries, titleWildcard)
		}

		indexerQuery = bleve.NewDisjunctionQuery(queries...)
	} else {
		contentQuery := bleve.NewMatchAllQuery()
		indexerQuery = contentQuery
	}

	
	// ==========================================
	// Permission filtering
	visibilityZero := float64(0)
	truee := true
	publicQuery := bleve.NewNumericRangeInclusiveQuery(&visibilityZero, &visibilityZero, &truee, &truee)
	publicQuery.SetField("Visibility")

	userIdMatch := float64(userId)
	userIdQuery := bleve.NewNumericRangeInclusiveQuery(&userIdMatch, &userIdMatch, &truee, &truee)
	userIdQuery.SetField("UserID")

	accessQuery := bleve.NewDisjunctionQuery(publicQuery, userIdQuery)
	indexerQuery = bleve.NewConjunctionQuery(accessQuery, indexerQuery)

	if queryMetadata.All != "" {
		allQueries := make([]query.Query, 0)
		fields := []string{"Username", "Title", "Filenames", "Languages", "Topics"}
		for _, f := range fields {
			q := bleve.NewMatchQuery(queryMetadata.All) // 用 MatchQuery 以支持分词
			q.SetField(f)
			allQueries = append(allQueries, q)
		}
        extQ := bleve.NewMatchQuery("." + queryMetadata.All)
        extQ.SetField("Extensions")
        allQueries = append(allQueries, extQ)

		allDisjunction := bleve.NewDisjunctionQuery(allQueries...)
		indexerQuery = bleve.NewConjunctionQuery(indexerQuery, allDisjunction)
	} else {
		addQuery := func(field, value string) {
			if value != "" && value != "." {
				q := bleve.NewMatchQuery(value) // 用 MatchQuery
				q.SetField(field)
				indexerQuery = bleve.NewConjunctionQuery(indexerQuery, q)
			}
		}
		addQuery("Username", queryMetadata.Username)
		addQuery("Title", queryMetadata.Title)
		addQuery("Extensions", "."+queryMetadata.Extension)
		addQuery("Filenames", queryMetadata.Filename)
		addQuery("Languages", queryMetadata.Language)
		addQuery("Topics", queryMetadata.Topic)
	}

	languageFacet := bleve.NewFacetRequest("Languages", 10)
	perPage := 10
	offset := (page - 1) * perPage

	s := bleve.NewSearchRequestOptions(indexerQuery, perPage+1, offset, false)
	s.AddFacet("languageFacet", languageFacet)
    
	s.Fields = []string{"GistID", "Title", "Username", "Filenames"}
	s.IncludeLocations = true 		// For debugging

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

