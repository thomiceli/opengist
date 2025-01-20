package handlers

import (
	"errors"
	"github.com/thomiceli/opengist/internal/web/context"
	"html/template"
	"strconv"
	"strings"
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

func Paginate[T any](ctx *context.Context, data []*T, pageInt int, perPage int, templateDataName string, urlPage string, labels int, urlParams ...string) error {
	lenData := len(data)
	if lenData == 0 && pageInt != 1 {
		return errors.New("page not found")
	}

	if lenData > perPage {
		if lenData > 1 {
			data = data[:lenData-1]
		}
		ctx.SetData("nextPage", pageInt+1)
	}
	if pageInt > 1 {
		ctx.SetData("prevPage", pageInt-1)
	}

	if len(urlParams) > 0 {
		ctx.SetData("urlParams", template.URL(urlParams[0]))
	}

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
