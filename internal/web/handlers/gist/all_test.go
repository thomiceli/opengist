package gist_test

import (
	"io"
	"net/url"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/index"
	"github.com/thomiceli/opengist/internal/render"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

// TestSearchChecksCurrentVisibility - search results are checked against the
// gist's current visibility, so an index entry still marked public does not
// list an unlisted or private gist to anyone but its owner.
func TestSearchChecksCurrentVisibility(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	s.Register(t, "alice")

	for _, visibility := range []db.Visibility{db.UnlistedVisibility, db.PrivateVisibility} {
		t.Run(visibility.String(), func(t *testing.T) {
			_, gist, _, _ := s.CreateGist(t, "0")

			// Indexing runs in the background.
			require.Eventually(t, func() bool { return anonymousIndexHasGist(gist) }, 10*time.Second, 50*time.Millisecond)

			// Change the visibility in the database only, leaving the index
			// entry marked public.
			gist.Private = visibility
			require.NoError(t, gist.UpdateNoTimestamps())
			require.True(t, anonymousIndexHasGist(gist), "index entry must still be marked public")

			s.Logout()
			require.False(t, searchListsGist(t, s, gist), "anonymous user must not see the gist")

			s.Login(t, "alice")
			require.False(t, searchListsGist(t, s, gist), "other user must not see the gist")

			s.Login(t, "thomas")
			require.True(t, searchListsGist(t, s, gist), "owner must still see the gist")
			s.Logout()
		})
	}
}

// TestSearchPaginationWithHiddenGists - pages and their links follow the
// search index even when some of their gists are not shown: hiding the first
// gist of the next page keeps the link to that page, and a page whose gists
// are all hidden is shown empty, with links to the pages around it.
func TestSearchPaginationWithHiddenGists(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	s.Login(t, "thomas")

	// 22 gists make three pages of search results: 10, 10 and 2.
	for i := 0; i < 22; i++ {
		s.Request(t, "POST", "/", url.Values{
			"title":   {"Test"},
			"name":    {"file.txt"},
			"content": {"hello"},
			"private": {"0"},
		}, 302)
	}
	s.Logout()

	// Indexing runs in the background.
	metadata := index.SearchGistMetadata{Username: "thomas"}
	require.Eventually(t, func() bool {
		_, nbHits, _, err := index.SearchGists(metadata, 0, 1)
		return err == nil && nbHits == 22
	}, 10*time.Second, 50*time.Millisecond)

	// Each page from the index ends with the first gist of the next page, if
	// there is one.
	page1, _, _, err := index.SearchGists(metadata, 0, 1)
	require.NoError(t, err)
	page2, _, _, err := index.SearchGists(metadata, 0, 2)
	require.NoError(t, err)
	page3, _, _, err := index.SearchGists(metadata, 0, 3)
	require.NoError(t, err)
	require.Len(t, page1, 11)
	require.Len(t, page2, 11)
	require.Len(t, page3, 2)

	// Hide all the gists of page 2, and the first gist of page 3, in the
	// database only: the index still lists them as public.
	for _, id := range page2 {
		gist, err := db.GetGistByID(strconv.FormatUint(uint64(id), 10))
		require.NoError(t, err)
		gist.Private = db.PrivateVisibility
		require.NoError(t, gist.UpdateNoTimestamps())
	}

	ids, body := searchPage(t, s, "user:thomas", 1)
	require.Equal(t, page1[:10], ids)
	require.Contains(t, body, "/-/search?page=2", "link to the next page")

	ids, body = searchPage(t, s, "user:thomas", 2)
	require.Empty(t, ids)
	require.Contains(t, body, "/-/search?page=1", "link to the previous page")
	require.Contains(t, body, "/-/search?page=3", "link to the next page")

	ids, body = searchPage(t, s, "user:thomas", 3)
	require.Equal(t, page3[1:], ids)
	require.Contains(t, body, "/-/search?page=2", "link to the previous page")
	require.NotContains(t, body, "/-/search?page=4", "no link to a next page")

	s.Request(t, "GET", "/-/search?q="+url.QueryEscape("user:thomas")+"&page=4", nil, 404)
}

// searchPage requests a page of search results with the server's current
// session, and returns the IDs of the gists it lists and the page body.
func searchPage(t *testing.T, s *webtest.Server, query string, page int) ([]uint, string) {
	resp := s.Request(t, "GET", "/-/search?q="+url.QueryEscape(query)+"&page="+strconv.Itoa(page), nil, 200)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	data, ok := s.ContextData("gists")
	require.True(t, ok)
	ids := make([]uint, 0)
	for _, gist := range data.([]*render.RenderedGist) {
		ids = append(ids, gist.ID)
	}
	return ids, string(body)
}

// searchListsGist reports whether the first page of search results for the
// gist's owner, requested with the server's current session, lists gist.
func searchListsGist(t *testing.T, s *webtest.Server, gist *db.Gist) bool {
	ids, _ := searchPage(t, s, "user:"+gist.User.Username, 1)
	return slices.Contains(ids, gist.ID)
}

// anonymousIndexHasGist reports whether an anonymous query on the search index
// returns gist.
func anonymousIndexHasGist(gist *db.Gist) bool {
	ids, _, _, err := index.SearchGists(index.SearchGistMetadata{Username: gist.User.Username}, 0, 1)
	return err == nil && slices.Contains(ids, gist.ID)
}
