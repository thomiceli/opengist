package handlers

import (
	"errors"
	"html/template"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gorilla/schema"

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
	var paginationParams PaginationParams
	if params == nil {
		paginationParams = PaginationParams{}
	} else {
		paginationParams = *params
	}
	paginationParams.Page = pageInt
	lenData := len(data)
	if lenData == 0 && pageInt != 1 {
		return errors.New("page not found")
	}

	if lenData > perPage {
		if lenData > 1 {
			data = data[:lenData-1]
		}
		paginationParams.HasNext = true
	}
	if pageInt > 1 {
		paginationParams.HasPrevious = true
	}

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
	ctx.SetData(templateDataName, data)
	return nil
}

func ParseSearchQueryStr(query string) (string, map[string]string) {
	words := strings.Fields(query)
	metadata := make(map[string]string)
	var contentBuilder strings.Builder

	for _, word := range words {
		if strings.Contains(word, ":") {
			keyValue := strings.SplitN(word, ":", 2)
			if len(keyValue) == 2 {
				key := keyValue[0]
				value := keyValue[1]
				metadata[key] = value
			}
		} else {
			contentBuilder.WriteString(word + " ")
		}
	}

	content := strings.TrimSpace(contentBuilder.String())
	return content, metadata
}

func GetContentTypeFromFilename(filename string) (ret string) {
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".css":
		ret = "text/css"
	default:
		ret = "text/plain"
	}

	// add charset=utf-8, if not, unicode charset will be broken
	ret += "; charset=utf-8"
	return
}

func GetContentDisposition(filename string) string {
	return "inline; filename=\"" + filename + "\""
}
