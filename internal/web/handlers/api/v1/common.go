package v1

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/web/context"
)

const (
	defaultPerPage = 30
	maxPerPage     = 100
)

func apiBaseURL(ctx *context.Context) string {
	if config.C.ExternalUrl != "" {
		return config.C.ExternalUrl
	}
	scheme := "http"
	if ctx.Request().TLS != nil || ctx.Request().Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + ctx.Request().Host
}

// writePaginationHeaders sets the pagination response headers for a list
// endpoint. X-Page and X-Per-Page are always set, alongside the RFC 5988 Link
// header (next/prev). When total is non-nil it also sets X-Total and the
// derived X-Total-Pages = ceil(total / perPage); endpoints whose total would
// cost an extra round-trip (e.g. commits, which would need a git call) pass nil
// to omit them.
func writePaginationHeaders(ctx *context.Context, baseURL string, page, perPage int, hasMore bool, total *int64) {
	h := ctx.Response().Header()
	h.Set("X-Page", strconv.Itoa(page))
	h.Set("X-Per-Page", strconv.Itoa(perPage))
	if total != nil {
		h.Set("X-Total", strconv.FormatInt(*total, 10))
		totalPages := 0
		if perPage > 0 {
			totalPages = int((*total + int64(perPage) - 1) / int64(perPage))
		}
		h.Set("X-Total-Pages", strconv.Itoa(totalPages))
	}
	writeLinkHeader(ctx, baseURL, page, hasMore)
}

func parsePage(ctx *context.Context) int {
	p, _ := strconv.Atoi(ctx.QueryParam("page"))
	if p < 1 {
		p = 1
	}
	return p
}

func parsePerPage(ctx *context.Context) int {
	pp, _ := strconv.Atoi(ctx.QueryParam("per_page"))
	if pp < 1 {
		return defaultPerPage
	}
	if pp > maxPerPage {
		return maxPerPage
	}
	return pp
}

// parseSince reads the optional `since` query param as an RFC 3339 timestamp.
// Returns (nil, nil) when absent and an HTTP-ready error envelope on parse failure.
func parseSince(ctx *context.Context) (*time.Time, error) {
	s := ctx.QueryParam("since")
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// writeLinkHeader writes an RFC 5988 Link header for paginated responses.
// Includes rel=next when there's a next page and rel=prev when page > 1.
// URLs are rebuilt from the base URL + original query (with page rewritten) so
// they survive whatever proxy stripped from the inbound request's path.
func writeLinkHeader(ctx *context.Context, baseURL string, page int, hasMore bool) {
	if !hasMore && page <= 1 {
		return
	}
	reqURL := ctx.Request().URL
	build := func(p int) string {
		q := reqURL.Query()
		q.Set("page", strconv.Itoa(p))
		u := url.URL{Path: reqURL.Path, RawQuery: q.Encode()}
		return strings.TrimRight(baseURL, "/") + u.RequestURI()
	}
	var links []string
	if hasMore {
		links = append(links, fmt.Sprintf(`<%s>; rel="next"`, build(page+1)))
	}
	if page > 1 {
		links = append(links, fmt.Sprintf(`<%s>; rel="prev"`, build(page-1)))
	}
	ctx.Response().Header().Set("Link", strings.Join(links, ", "))
}
