package index

import (
	"errors"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/custom"
	"github.com/blevesearch/bleve/v2/analysis/token/camelcase"
	"github.com/blevesearch/bleve/v2/analysis/token/lowercase"
	"github.com/blevesearch/bleve/v2/analysis/token/unicodenorm"
	"github.com/blevesearch/bleve/v2/analysis/tokenizer/unicode"
	"github.com/blevesearch/bleve/v2/search"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/thomiceli/opengist/internal/db"
	"path/filepath"
	"strconv"

	"github.com/blevesearch/bleve/v2"
)

var I bleve.Index

func Open(indexFilename string) error {
	var err error
	I, err = bleve.Open(indexFilename)
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

	I, err = bleve.New(indexFilename, mapping)

	return err
}

func Close() error {
	return I.Close()
}

func Index(gist *db.Gist) error {
	files, err := gist.Files("HEAD", true)
	if err != nil {
		return err
	}

	wholeContent := ""
	for _, file := range files {
		wholeContent += file.Content
	}

	fileNames, err := gist.FileNames("HEAD")
	if err != nil {
		return err
	}

	exts := make([]string, 0, len(fileNames))
	for _, file := range fileNames {
		exts = append(exts, filepath.Ext(file))
	}

	indexedGist := Gist{
		GistID:     gist.ID,
		Username:   gist.User.Username,
		Title:      gist.Title,
		Content:    wholeContent,
		Filenames:  fileNames,
		Extensions: exts,
		CreatedAt:  gist.CreatedAt,
		UpdatedAt:  gist.UpdatedAt,
	}

	return I.Index(strconv.Itoa(int(indexedGist.GistID)), indexedGist)
}

func SearchGists(queryStr string, queryMetadata SearchGistMetadata, userId uint, page int) ([]uint, error) {
	var err error
	var indexerQuery query.Query
	contentQuery := bleve.NewMatchPhraseQuery(queryStr)
	contentQuery.FieldVal = "Content"

	var gistsIds []uint
	if userId != 0 {
		gistsIds, err = db.GetAllGistsVisibleByUser(userId)
		if err != nil {
			return nil, err
		}
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

		indexerQuery = bleve.NewConjunctionQuery(bleve.NewDisjunctionQuery(repoQueries...), contentQuery)
	} else {
		indexerQuery = contentQuery
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

	sort := search.SortOrder{
		&search.SortField{
			Field: "UpdatedAt",
			Desc:  true,
		},
	}

	perPage := 10
	offset := (page - 1) * perPage

	s := bleve.NewSearchRequestOptions(indexerQuery, perPage, offset, false)
	s.Fields = []string{"GistID"}
	s.IncludeLocations = false
	s.Sort = sort

	results, err := I.Search(s)
	if err != nil {
		return nil, err
	}

	gistIds := make([]uint, 0, len(results.Hits))
	for _, hit := range results.Hits {
		gistIds = append(gistIds, uint(hit.Fields["GistID"].(float64)))
	}
	return gistIds, nil
}
