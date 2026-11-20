package handlers

import (
	"errors"
	"html/template"
	"net/url"
	"strconv"
	"strings"

	"github.com/gorilla/schema"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/web/context"
)

func GetPage(ctx *context.Context) int {
	page := ctx.QueryParam("page")
	if page == "" {
		page = "1"
	}
	pageInt, err := strconv.Atoi(page)
	if err != nil {
		pageInt = 1
	}
	ctx.SetData("currPage", pageInt)

	return pageInt
}

type PaginationParams struct {
	Page       int    `schema:"page,omitempty"`
	Sort       string `schema:"sort,omitempty"`
	Order      string `schema:"order,omitempty"`
	Title      string `schema:"title,omitempty"`
	Visibility string `schema:"visibility,omitempty"`
	Language   string `schema:"language,omitempty"`
	Topics     string `schema:"topics,omitempty"`
	Query      string `schema:"q,omitempty"`

	HasPrevious bool `schema:"-"` // Exclude from URL parameters
	HasNext     bool `schema:"-"`
}

var encoder = schema.NewEncoder()

func (p PaginationParams) String() string {
	values := url.Values{}

	err := encoder.Encode(p, values)
	if err != nil {
		return ""
	}

	if len(values) == 0 {
		return ""
	}
	return "?" + values.Encode()
}

func (p PaginationParams) NextURL() template.URL {
	p.Page++
	return template.URL(p.String())
}

func (p PaginationParams) PreviousURL() template.URL {
	p.Page--
	return template.URL(p.String())
}

func (p PaginationParams) WithParams(pairs ...string) template.URL {
	values := url.Values{}
	_ = encoder.Encode(p, values)

	// reset page
	values.Del("page")

	for i := 0; i < len(pairs); i += 2 {
		values.Set(pairs[i], pairs[i+1])
	}

	return template.URL("?" + values.Encode())
}

func Paginate[T any](ctx *context.Context, data []*T, pageInt int, perPage int, templateDataName string, urlPage string, labels int, params *PaginationParams) error {
	lenData := len(data)
	if lenData == 0 && pageInt != 1 {
		return errors.New("page not found")
	}

	hasNext := lenData > perPage
	if hasNext && lenData > 1 {
		data = data[:lenData-1]
	}

	SetPagination(ctx, pageInt, hasNext, urlPage, labels, params)
	ctx.SetData(templateDataName, data)
	return nil
}

// SetPagination sets the template data for the pagination links of a page.
// Paginate calls it once it knows from the data whether there is a next page;
// handlers that know it some other way can call it directly.
func SetPagination(ctx *context.Context, pageInt int, hasNext bool, urlPage string, labels int, params *PaginationParams) {
	var paginationParams PaginationParams
	if params != nil {
		paginationParams = *params
	}
	paginationParams.Page = pageInt
	paginationParams.HasNext = hasNext
	paginationParams.HasPrevious = pageInt > 1

	ctx.SetData("pagination", paginationParams)

	switch labels {
	case 1:
		ctx.SetData("prevLabel", ctx.TrH("pagination.previous"))
		ctx.SetData("nextLabel", ctx.TrH("pagination.next"))
	case 2:
		ctx.SetData("prevLabel", ctx.TrH("pagination.newer"))
		ctx.SetData("nextLabel", ctx.TrH("pagination.older"))
	}

	ctx.SetData("urlPage", urlPage)
}

// ParseSearchQueryStr parses a search query string and returns a map of metadata.
// The query string is split into words and each word is checked if it contains a colon (:).
// If a word contains a colon, it is split into a key-value pair and added to the metadata map.
// If a word does not contain a colon, it is added to an "all" key in the metadata map.
// The "all" key is used to search all fields in the index.
// The function returns the metadata map.
func ParseSearchQueryStr(query string) map[string]string {
	words := strings.Fields(query)
	metadata := make(map[string]string)
	var allFieldsBuilder strings.Builder

	for _, word := range words {
		if strings.Contains(word, ":") {
			keyValue := strings.SplitN(word, ":", 2)
			if len(keyValue) == 2 {
				key := keyValue[0]
				value := keyValue[1]
				metadata[key] = value
			}
		} else {
			// Add to content search by default
			allFieldsBuilder.WriteString(word + " ")
		}
	}

	// Set the default search field
	allContent := strings.TrimSpace(allFieldsBuilder.String())
	if allContent != "" {
		metadata["default"] = allContent
	}

	log.Debug().Msgf("Metadata: %v", metadata)

	return metadata
}
