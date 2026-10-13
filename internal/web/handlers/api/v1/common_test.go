package v1_test

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/web/handlers/api/v1/types"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

// setupPaginationEnv stands up a server with `count` public gists owned by
// "thomas", with the API enabled and no logged-in session. The list endpoint
// can then be hit anonymously - visibility plays no role in these tests, so
// keeping everything public + anon keeps the focus on pagination plumbing.
func setupPaginationEnv(t *testing.T, count int) *webtest.Server {
	s := webtest.Setup(t)
	t.Cleanup(func() { webtest.Teardown(t) })

	// First-registered = auto-admin in some flows; register a stub first so
	// the gist owner is a regular user.
	s.Register(t, "admin")
	s.Logout()
	s.Register(t, "thomas")

	for i := 0; i < count; i++ {
		s.CreateGistAs(t, "thomas", "0") // public
	}
	s.Logout()
	return s
}

// parseLinkHeader parses an RFC 5988 Link header into a {rel: url} map.
// Bare-bones - assumes well-formed input as emitted by writeLinkHeader.
func parseLinkHeader(t *testing.T, h string) map[string]string {
	out := map[string]string{}
	if h == "" {
		return out
	}
	for _, part := range strings.Split(h, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		end := strings.Index(part, ">")
		require.Greater(t, end, 0, "malformed link entry: %s", part)
		linkURL := part[1:end]
		rest := part[end+1:]
		const relPrefix = `rel="`
		i := strings.Index(rest, relPrefix)
		require.GreaterOrEqual(t, i, 0, "missing rel in link entry: %s", part)
		name := rest[i+len(relPrefix):]
		name = name[:strings.Index(name, `"`)]
		out[name] = linkURL
	}
	return out
}

// listAnonymous fires an anonymous GET /api/gists?... and returns both the
// decoded array and the Link header (parsed by rel).
func listAnonymous(t *testing.T, s *webtest.Server, query string) ([]types.GistSimple, map[string]string) {
	return apiList[types.GistSimple](t, s, "/api/gists?"+query, "", 200)
}

// apiList fires a GET against a list endpoint and returns the decoded array
// (the body is a bare JSON array) plus the Link header (parsed by rel).
func apiList[T any](t *testing.T, s *webtest.Server, uri, token string, status int) ([]T, map[string]string) {
	w, body := s.APIRequest(t, "GET", uri, token, nil, status)
	var arr []T
	require.NoError(t, json.Unmarshal(body, &arr))
	return arr, parseLinkHeader(t, w.Header().Get("Link"))
}

// --- per_page ---

func TestPerPage_Default_Is30(t *testing.T) {
	// Create 31 gists so the default 30 caps and "more" is signaled.
	s := setupPaginationEnv(t, 31)

	arr, links := listAnonymous(t, s, "")
	require.Len(t, arr, 30, "missing per_page must default to 30")
	require.Contains(t, links, "next", "31 gists / 30 per page must have rel=next")
}

func TestPerPage_Custom_LimitsResults(t *testing.T) {
	s := setupPaginationEnv(t, 5)

	arr, _ := listAnonymous(t, s, "per_page=2")
	require.Len(t, arr, 2)
}

func TestPerPage_BelowOne_FallsBackToDefault(t *testing.T) {
	// per_page=0 (and negative values) should not break the response - the
	// helper clamps to defaultPerPage (30). With 5 gists, all 5 come back.
	s := setupPaginationEnv(t, 5)

	arr, _ := listAnonymous(t, s, "per_page=0")
	require.Len(t, arr, 5)
}

func TestPerPage_OverMaxCapped(t *testing.T) {
	// per_page=999 is server-side clamped to 100. With 5 gists, the cap
	// doesn't visibly trim anything but we still expect a 200 (no error from
	// the oversize value) and all 5 rows returned.
	s := setupPaginationEnv(t, 5)

	arr, _ := listAnonymous(t, s, "per_page=999")
	require.Len(t, arr, 5)
}

// --- page + Link header ---

func TestPage_FirstPage_OnlyNext(t *testing.T) {
	s := setupPaginationEnv(t, 5)

	arr, links := listAnonymous(t, s, "per_page=2&page=1")
	require.Len(t, arr, 2)
	require.Contains(t, links, "next", "first page must advertise rel=next when there are more rows")
	require.NotContains(t, links, "prev", "first page must NOT advertise rel=prev")

	// rel=next URL preserves per_page and bumps page to 2.
	u, err := url.Parse(links["next"])
	require.NoError(t, err)
	require.Equal(t, "2", u.Query().Get("page"))
	require.Equal(t, "2", u.Query().Get("per_page"))
}

func TestPage_MiddlePage_PrevAndNext(t *testing.T) {
	s := setupPaginationEnv(t, 5)

	arr, links := listAnonymous(t, s, "per_page=2&page=2")
	require.Len(t, arr, 2)
	require.Contains(t, links, "next", "middle page must have rel=next")
	require.Contains(t, links, "prev", "middle page must have rel=prev")
}

func TestPage_LastPage_OnlyPrev(t *testing.T) {
	s := setupPaginationEnv(t, 5)

	arr, links := listAnonymous(t, s, "per_page=2&page=3")
	require.Len(t, arr, 1, "page 3 of (5 rows / per_page=2) must hold the trailing row")
	require.NotContains(t, links, "next", "last page must NOT advertise rel=next")
	require.Contains(t, links, "prev")
}

func TestPage_BelowOne_FallsBackToOne(t *testing.T) {
	s := setupPaginationEnv(t, 3)

	// page=0 and page=-1 should both behave like page=1 (no error, first page).
	arr, _ := listAnonymous(t, s, "per_page=2&page=0")
	require.Len(t, arr, 2)
}

// TestPagination_TotalHeaders - the X-Total / X-Total-Pages headers report the
// full match count and derived page count, independent of the page's size.
func TestPagination_TotalHeaders(t *testing.T) {
	s := setupPaginationEnv(t, 5)

	w, body := s.APIRequest(t, "GET", "/api/gists?per_page=2", "", nil, 200)
	var arr []types.GistSimple
	require.NoError(t, json.Unmarshal(body, &arr))

	require.Len(t, arr, 2)
	require.Equal(t, "1", w.Header().Get("X-Page"))
	require.Equal(t, "2", w.Header().Get("X-Per-Page"))
	require.Equal(t, "5", w.Header().Get("X-Total"), "total must count all matching gists")
	require.Equal(t, "3", w.Header().Get("X-Total-Pages"), "ceil(5/2) = 3 pages")
}

func TestLinkHeader_SinglePage_NoHeader(t *testing.T) {
	s := setupPaginationEnv(t, 1)

	w, _ := s.APIRequest(t, "GET", "/api/gists?per_page=10", "", nil, 200)
	require.Empty(t, w.Header().Get("Link"),
		"single-page response must omit the Link header (no prev, no next)")
}

// --- since ---

func TestSince_Future_ReturnsEmpty(t *testing.T) {
	s := setupPaginationEnv(t, 3)

	future := time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339)
	arr, _ := listAnonymous(t, s, "since="+url.QueryEscape(future))
	require.Empty(t, arr, "since=future must filter out everything")
}

func TestSince_Past_ReturnsAll(t *testing.T) {
	s := setupPaginationEnv(t, 3)

	past := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)
	arr, _ := listAnonymous(t, s, "since="+url.QueryEscape(past))
	require.Len(t, arr, 3)
}

func TestSince_InvalidFormat_400(t *testing.T) {
	s := setupPaginationEnv(t, 1)

	s.APIRequest(t, "GET", "/api/gists?since=not-a-date", "", nil, 400)
}
