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
	"github.com/thomiceli/opengist/internal/config"
	"strconv"
)

var bleveIndex bleve.Index

func Enabled() bool {
	return config.C.IndexEnabled
}

func Open(indexFilename string) error {
	var err error
	bleveIndex, err = bleve.Open(indexFilename)
	if err == nil {
		return nil
	}

	if !errors.Is(err, bleve.ErrorIndexPathDoesNotExist) {
		return err
	}

	docMapping := bleve.NewDocumentMapping()
	docMapping.AddFieldMappingsAt("GistID", bleve.NewNumericFieldMapping())
	docMapping.AddFieldMappingsAt("Content", bleve.NewTextFieldMapping())

	mapping := bleve.NewIndexMapping()

	if err = mapping.AddCustomTokenFilter("unicodeNormalize", map[string]any{
		"type": unicodenorm.Name,
		"form": unicodenorm.NFC,
	}); err != nil {
		return err
	}

	if err = mapping.AddCustomAnalyzer("gistAnalyser", map[string]interface{}{
		"type":          custom.Name,
		"char_filters":  []string{},
		"tokenizer":     unicode.Name,
		"token_filters": []string{"unicodeNormalize", camelcase.Name, lowercase.Name},
	}); err != nil {
		return err
	}

	docMapping.DefaultAnalyzer = "gistAnalyser"

	bleveIndex, err = bleve.New(indexFilename, mapping)

	return err
}

func Close() error {
	return bleveIndex.Close()
}

func AddInIndex(gist *Gist) error {
	if !Enabled() {
		return nil
	}

	if gist == nil {
		return errors.New("failed to add nil gist to index")
	}
	return bleveIndex.Index(strconv.Itoa(int(gist.GistID)), gist)
}

func RemoveFromIndex(gistID uint) error {
	if !Enabled() {
		return nil
	}

	return bleveIndex.Delete(strconv.Itoa(int(gistID)))
}

func SearchGists(queryStr string, queryMetadata SearchGistMetadata, gistsIds []uint, page int) ([]uint, uint64, map[string]int, error) {
	if !Enabled() {
		return nil, 0, nil, nil
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

	if len(gistsIds) > 0 {
		repoQueries := make([]query.Query, 0, len(gistsIds))

		truee := true
		for _, id := range gistsIds {
			f := float64(id)
			qq := bleve.NewNumericRangeInclusiveQuery(&f, &f, &truee, &truee)
			qq.SetField("GistID")
			repoQueries = append(repoQueries, qq)
		}

		indexerQuery = bleve.NewConjunctionQuery(bleve.NewDisjunctionQuery(repoQueries...), indexerQuery)
	}

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

	languageFacet := bleve.NewFacetRequest("Languages", 10)

	perPage := 10
	offset := (page - 1) * perPage

	s := bleve.NewSearchRequestOptions(indexerQuery, perPage, offset, false)
	s.AddFacet("languageFacet", languageFacet)
	s.Fields = []string{"GistID"}
	s.IncludeLocations = false

	results, err := bleveIndex.Search(s)
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
