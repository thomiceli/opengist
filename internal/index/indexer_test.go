package index

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// newGist creates a Gist with sensible defaults for testing.
func newGist(id uint, userID uint, visibility uint, content string) *Gist {
	return &Gist{
		GistID:     id,
		UserID:     userID,
		Visibility: visibility,
		Username:   fmt.Sprintf("user%d", userID),
		Title:      fmt.Sprintf("Gist %d", id),
		Content:    content,
		Filenames:  []string{"file.txt"},
		Extensions: []string{"txt"},
		Languages:  []string{"Text"},
		Topics:     []string{},
		CreatedAt:  1234567890,
		UpdatedAt:  1234567890,
	}
}

func testAddAndSearch(t *testing.T, setup func(t *testing.T) (Indexer, func())) {
	t.Run("add and search by content", func(t *testing.T) {
		indexer, cleanup := setup(t)
		defer cleanup()

		g := newGist(1, 1, 0, "the quick brown fox jumps over the lazy dog")
		require.NoError(t, indexer.Add(g))

		ids, total, _, err := indexer.Search(SearchGistMetadata{Content: "fox"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
		require.Equal(t, uint(1), ids[0])
	})

	t.Run("add nil gist returns error", func(t *testing.T) {
		indexer, cleanup := setup(t)
		defer cleanup()

		require.Error(t, indexer.Add(nil))
	})

	t.Run("add then remove", func(t *testing.T) {
		indexer, cleanup := setup(t)
		defer cleanup()

		g := newGist(1, 1, 0, "removable content here")
		require.NoError(t, indexer.Add(g))

		_, total, _, err := indexer.Search(SearchGistMetadata{Content: "removable"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)

		require.NoError(t, indexer.Remove(1))

		_, total, _, err = indexer.Search(SearchGistMetadata{Content: "removable"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(0), total)
	})

	t.Run("update gist replaces content", func(t *testing.T) {
		indexer, cleanup := setup(t)
		defer cleanup()

		g := newGist(1, 1, 0, "original content alpha")
		require.NoError(t, indexer.Add(g))

		g2 := newGist(1, 1, 0, "updated content beta")
		require.NoError(t, indexer.Add(g2))

		_, total, _, err := indexer.Search(SearchGistMetadata{Content: "alpha"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(0), total, "old content should not be found")

		ids, total, _, err := indexer.Search(SearchGistMetadata{Content: "beta"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
		require.Equal(t, uint(1), ids[0])
	})

	t.Run("gist with empty optional fields still searchable by content", func(t *testing.T) {
		indexer, cleanup := setup(t)
		defer cleanup()

		g := &Gist{
			GistID: 1, UserID: 1, Visibility: 0,
			Username: "user1", Title: "",
			Content:    "searchable bare content",
			Filenames:  []string{},
			Extensions: []string{},
			Languages:  []string{},
			Topics:     []string{},
			CreatedAt:  1234567890, UpdatedAt: 1234567890,
		}
		require.NoError(t, indexer.Add(g))

		_, total, _, err := indexer.Search(SearchGistMetadata{Content: "searchable"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
	})
}

func testAccessControl(t *testing.T, setup func(t *testing.T) (Indexer, func())) {
	t.Run("public gist visible to any user", func(t *testing.T) {
		indexer, cleanup := setup(t)
		defer cleanup()

		g := newGist(1, 1, 0, "public content")
		require.NoError(t, indexer.Add(g))

		_, total, _, err := indexer.Search(SearchGistMetadata{Content: "public"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total, "owner should see public gist")

		_, total, _, err = indexer.Search(SearchGistMetadata{Content: "public"}, 99, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total, "other user should see public gist")

		_, total, _, err = indexer.Search(SearchGistMetadata{Content: "public"}, 0, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total, "anonymous should see public gist")
	})

	t.Run("private gist only visible to owner", func(t *testing.T) {
		indexer, cleanup := setup(t)
		defer cleanup()

		g := newGist(1, 1, 1, "private secret")
		require.NoError(t, indexer.Add(g))

		_, total, _, err := indexer.Search(SearchGistMetadata{Content: "secret"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total, "owner should see private gist")

		_, total, _, err = indexer.Search(SearchGistMetadata{Content: "secret"}, 99, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(0), total, "other user should not see private gist")

		_, total, _, err = indexer.Search(SearchGistMetadata{Content: "secret"}, 0, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(0), total, "anonymous should not see private gist")
	})

	t.Run("unlisted gist only visible to owner", func(t *testing.T) {
		indexer, cleanup := setup(t)
		defer cleanup()

		g := newGist(1, 1, 2, "unlisted hidden")
		require.NoError(t, indexer.Add(g))

		_, total, _, err := indexer.Search(SearchGistMetadata{Content: "hidden"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total, "owner should see unlisted gist")

		_, total, _, err = indexer.Search(SearchGistMetadata{Content: "hidden"}, 99, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(0), total, "other user should not see unlisted gist")
	})

	t.Run("mixed visibility correct counts per user", func(t *testing.T) {
		indexer, cleanup := setup(t)
		defer cleanup()

		g1 := newGist(1, 1, 0, "alphaword content")
		g2 := newGist(2, 1, 1, "alphaword content")
		g3 := newGist(3, 2, 0, "alphaword content")
		g4 := newGist(4, 2, 1, "alphaword content")

		for _, g := range []*Gist{g1, g2, g3, g4} {
			require.NoError(t, indexer.Add(g))
		}

		_, total, _, err := indexer.Search(SearchGistMetadata{Content: "alphaword"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(3), total, "user1 sees 2 public + 1 own private")

		_, total, _, err = indexer.Search(SearchGistMetadata{Content: "alphaword"}, 2, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(3), total, "user2 sees 2 public + 1 own private")

		_, total, _, err = indexer.Search(SearchGistMetadata{Content: "alphaword"}, 0, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(2), total, "anonymous sees only public")
	})
}

func testMetadataFilters(t *testing.T, setup func(t *testing.T) (Indexer, func())) {
	indexer, cleanup := setup(t)
	defer cleanup()

	g1 := &Gist{
		GistID: 1, UserID: 1, Visibility: 0,
		Username: "alice", Title: "Go Tutorial",
		Description: "A helper utility for parsing JSON",
		Content:     "learning golang basics",
		Filenames:   []string{"main.go"},
		Extensions:  []string{"go"},
		Languages:   []string{"Go"},
		Topics:      []string{"tutorial"},
		CreatedAt:   1234567890, UpdatedAt: 1234567890,
	}
	g2 := &Gist{
		GistID: 2, UserID: 2, Visibility: 0,
		Username: "bob", Title: "Python Script",
		Description: "Database migration scripts",
		Content:     "learning python basics",
		Filenames:   []string{"script.py"},
		Extensions:  []string{"py"},
		Languages:   []string{"Python"},
		Topics:      []string{"scripting"},
		CreatedAt:   1234567890, UpdatedAt: 1234567890,
	}

	for _, g := range []*Gist{g1, g2} {
		require.NoError(t, indexer.Add(g))
	}

	t.Run("filter by Username", func(t *testing.T) {
		_, total, _, err := indexer.Search(SearchGistMetadata{Content: "learning", Username: "alice"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
	})

	t.Run("filter by Language", func(t *testing.T) {
		_, total, _, err := indexer.Search(SearchGistMetadata{Content: "learning", Language: "Python"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
	})

	t.Run("filter by Extension with dot prefix", func(t *testing.T) {
		g3 := &Gist{
			GistID: 3, UserID: 1, Visibility: 0,
			Username: "alice", Title: "Dot Extension Test",
			Content:    "extension test content",
			Filenames:  []string{"test.rs"},
			Extensions: []string{".rs"},
			Languages:  []string{"Rust"},
			Topics:     []string{},
			CreatedAt:  1234567890, UpdatedAt: 1234567890,
		}
		require.NoError(t, indexer.Add(g3))

		_, total, _, err := indexer.Search(SearchGistMetadata{Content: "extension", Extension: "rs"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
	})

	t.Run("filter by Topic", func(t *testing.T) {
		_, total, _, err := indexer.Search(SearchGistMetadata{Content: "learning", Topic: "tutorial"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
	})

	t.Run("filter by Filename", func(t *testing.T) {
		_, total, _, err := indexer.Search(SearchGistMetadata{Content: "learning", Filename: "script.py"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
	})

	t.Run("filter by Title", func(t *testing.T) {
		_, total, _, err := indexer.Search(SearchGistMetadata{Content: "learning", Title: "Go Tutorial"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
	})

	t.Run("filter by Description", func(t *testing.T) {
		ids, total, _, err := indexer.Search(SearchGistMetadata{Content: "learning", Description: "parsing"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
		require.Equal(t, uint(1), ids[0])
	})

	t.Run("combined filters narrow results", func(t *testing.T) {
		_, total, _, err := indexer.Search(SearchGistMetadata{Content: "learning", Language: "Go", Username: "alice"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)

		_, total, _, err = indexer.Search(SearchGistMetadata{Content: "learning", Language: "Python", Username: "alice"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(0), total, "mismatched filters should return 0")
	})

	t.Run("filter matches no gists", func(t *testing.T) {
		_, total, _, err := indexer.Search(SearchGistMetadata{Content: "learning", Username: "nonexistent"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(0), total)
	})
}

func testAllFieldSearch(t *testing.T, setup func(t *testing.T) (Indexer, func())) {
	indexer, cleanup := setup(t)
	defer cleanup()

	g := &Gist{
		GistID: 1, UserID: 1, Visibility: 0,
		Username: "alice", Title: "MyTitle",
		Description: "Database migration scripts",
		Content:     "somecontent",
		Filenames:   []string{"readme.md"},
		Extensions:  []string{".md"},
		Languages:   []string{"Markdown"},
		Topics:      []string{"documentation"},
		CreatedAt:   1234567890, UpdatedAt: 1234567890,
	}
	require.NoError(t, indexer.Add(g))

	g2 := newGist(2, 2, 0, "unrelated other stuff")
	require.NoError(t, indexer.Add(g2))

	t.Run("All matches username", func(t *testing.T) {
		ids, total, _, err := indexer.Search(SearchGistMetadata{All: "alice"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
		require.Equal(t, uint(1), ids[0])
	})

	t.Run("All matches title", func(t *testing.T) {
		_, total, _, err := indexer.Search(SearchGistMetadata{All: "MyTitle"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
	})

	t.Run("All matches description", func(t *testing.T) {
		ids, total, _, err := indexer.Search(SearchGistMetadata{All: "migration"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
		require.Equal(t, uint(1), ids[0])
	})

	t.Run("All matches language", func(t *testing.T) {
		_, total, _, err := indexer.Search(SearchGistMetadata{All: "Markdown"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
	})

	t.Run("All matches topic", func(t *testing.T) {
		_, total, _, err := indexer.Search(SearchGistMetadata{All: "documentation"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
	})

	t.Run("All matches filename", func(t *testing.T) {
		_, total, _, err := indexer.Search(SearchGistMetadata{All: "readme.md"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
	})

	t.Run("All ignores specific filters", func(t *testing.T) {
		_, total, _, err := indexer.Search(SearchGistMetadata{All: "alice", Username: "nonexistent"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total, "All should take precedence over specific filters")
	})

	t.Run("All combined with content query", func(t *testing.T) {
		_, total, _, err := indexer.Search(SearchGistMetadata{Content: "somecontent", All: "alice"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)

		_, total, _, err = indexer.Search(SearchGistMetadata{Content: "somecontent", All: "nonexistent"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(0), total, "All not matching should yield 0")
	})
}

func testFuzzySearch(t *testing.T, setup func(t *testing.T) (Indexer, func())) {
	indexer, cleanup := setup(t)
	defer cleanup()

	g := newGist(1, 1, 0, "the elephant danced gracefully")
	require.NoError(t, indexer.Add(g))

	t.Run("exact match", func(t *testing.T) {
		_, total, _, err := indexer.Search(SearchGistMetadata{Content: "elephant"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
	})

	t.Run("1-char substitution", func(t *testing.T) {
		_, total, _, err := indexer.Search(SearchGistMetadata{Content: "elephent"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
	})

	t.Run("1-char deletion", func(t *testing.T) {
		_, total, _, err := indexer.Search(SearchGistMetadata{Content: "elepant"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
	})

	t.Run("1-char insertion", func(t *testing.T) {
		_, total, _, err := indexer.Search(SearchGistMetadata{Content: "elephantz"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
	})

	t.Run("character transposition", func(t *testing.T) {
		_, total, _, err := indexer.Search(SearchGistMetadata{Content: "elehpant"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
	})

	t.Run("case insensitive", func(t *testing.T) {
		_, total, _, err := indexer.Search(SearchGistMetadata{Content: "ELEPHANT"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
	})

	t.Run("empty content query with metadata returns MatchAll", func(t *testing.T) {
		_, total, _, err := indexer.Search(SearchGistMetadata{}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
	})
}

func testContentSearch(t *testing.T, setup func(t *testing.T) (Indexer, func())) {
	indexer, cleanup := setup(t)
	defer cleanup()

	g := &Gist{
		GistID: 1, UserID: 1, Visibility: 0,
		Username: "user1", Title: "Kubernetes Deployment Helper",
		Content: `// café résumé naïve
func getHTTPResponse(url string) {
    xmlParser := NewXMLParser()
    myFunctionName := calculate(x, y)
    fmt.Println("hello world")
    self.cpuCard = initCard()
    // the quick brown fox jumps over the lazy dog
    elephant := fetchData()
    if result == 0 {
        return
    }
}`,
		Filenames:  []string{"file.txt"},
		Extensions: []string{"txt"},
		Languages:  []string{"Text"},
		Topics:     []string{},
		CreatedAt:  1234567890, UpdatedAt: 1234567890,
	}
	require.NoError(t, indexer.Add(g))

	t.Run("code content/calculate", func(t *testing.T) {
		ids, total, _, err := indexer.Search(SearchGistMetadata{Content: "calculate"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
		require.Equal(t, uint(1), ids[0])
	})

	t.Run("code content/Println", func(t *testing.T) {
		ids, total, _, err := indexer.Search(SearchGistMetadata{Content: "Println"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
		require.Equal(t, uint(1), ids[0])
	})

	t.Run("code content/hello", func(t *testing.T) {
		ids, total, _, err := indexer.Search(SearchGistMetadata{Content: "hello"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
		require.Equal(t, uint(1), ids[0])
	})

	t.Run("multi-word/both present", func(t *testing.T) {
		ids, total, _, err := indexer.Search(SearchGistMetadata{Content: "fox lazy"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
		require.Equal(t, uint(1), ids[0])
	})

	t.Run("multi-word/one missing", func(t *testing.T) {
		_, total, _, err := indexer.Search(SearchGistMetadata{Content: "fox unicorn"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(0), total)
	})

	t.Run("prefix/content eleph", func(t *testing.T) {
		ids, total, _, err := indexer.Search(SearchGistMetadata{Content: "eleph"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
		require.Equal(t, uint(1), ids[0])
	})

	t.Run("prefix/title Kube", func(t *testing.T) {
		ids, total, _, err := indexer.Search(SearchGistMetadata{Title: "Kube"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
		require.Equal(t, uint(1), ids[0])
	})

	t.Run("unicode/café", func(t *testing.T) {
		_, total, _, err := indexer.Search(SearchGistMetadata{Content: "café"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
	})

	t.Run("unicode/résumé", func(t *testing.T) {
		_, total, _, err := indexer.Search(SearchGistMetadata{Content: "résumé"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
	})

	t.Run("unicode/normalization cafe", func(t *testing.T) {
		_, total, _, err := indexer.Search(SearchGistMetadata{Content: "cafe"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
	})

	t.Run("camelCase/function", func(t *testing.T) {
		ids, total, _, err := indexer.Search(SearchGistMetadata{Content: "function"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
		require.Equal(t, uint(1), ids[0])
	})

	t.Run("camelCase/xml", func(t *testing.T) {
		ids, total, _, err := indexer.Search(SearchGistMetadata{Content: "xml"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
		require.Equal(t, uint(1), ids[0])
	})

	t.Run("camelCase/parser", func(t *testing.T) {
		ids, total, _, err := indexer.Search(SearchGistMetadata{Content: "parser"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
		require.Equal(t, uint(1), ids[0])
	})

	t.Run("camelCase/http", func(t *testing.T) {
		ids, total, _, err := indexer.Search(SearchGistMetadata{Content: "http"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
		require.Equal(t, uint(1), ids[0])
	})

	t.Run("camelCase/response", func(t *testing.T) {
		ids, total, _, err := indexer.Search(SearchGistMetadata{Content: "response"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
		require.Equal(t, uint(1), ids[0])
	})

	t.Run("camelCase/cpucard", func(t *testing.T) {
		ids, total, _, err := indexer.Search(SearchGistMetadata{Content: "cpucard"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
		require.Equal(t, uint(1), ids[0])
	})
}

func testPagination(t *testing.T, setup func(t *testing.T) (Indexer, func())) {
	indexer, cleanup := setup(t)
	defer cleanup()

	for i := uint(1); i <= 25; i++ {
		g := newGist(i, 1, 0, "pagination keyword content")
		require.NoError(t, indexer.Add(g))
	}

	t.Run("page sizes", func(t *testing.T) {
		ids1, total1, _, err := indexer.Search(SearchGistMetadata{Content: "pagination"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(25), total1)
		require.GreaterOrEqual(t, len(ids1), 10, "page 1 should have at least 10 results")

		ids2, total2, _, err := indexer.Search(SearchGistMetadata{Content: "pagination"}, 1, 2)
		require.NoError(t, err)
		require.Equal(t, uint64(25), total2)
		require.GreaterOrEqual(t, len(ids2), 10, "page 2 should have at least 10 results")

		ids3, _, _, err := indexer.Search(SearchGistMetadata{Content: "pagination"}, 1, 3)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(ids3), 5, "page 3 should have at least 5 results")
	})

	t.Run("no duplicates between pages", func(t *testing.T) {
		ids1, _, _, err := indexer.Search(SearchGistMetadata{Content: "pagination"}, 1, 1)
		require.NoError(t, err)
		ids2, _, _, err := indexer.Search(SearchGistMetadata{Content: "pagination"}, 1, 2)
		require.NoError(t, err)

		page1 := ids1
		if len(page1) > 10 {
			page1 = page1[:10]
		}
		page2 := ids2
		if len(page2) > 10 {
			page2 = page2[:10]
		}

		seen := make(map[uint]bool)
		for _, id := range page1 {
			seen[id] = true
		}
		for _, id := range page2 {
			require.False(t, seen[id], "duplicate gist ID %d found across pages", id)
		}
	})

	t.Run("out of bounds page returns 0 results", func(t *testing.T) {
		ids, total, _, err := indexer.Search(SearchGistMetadata{Content: "pagination"}, 1, 100)
		require.NoError(t, err)
		require.Empty(t, ids)
		require.Equal(t, uint64(25), total, "total should still reflect actual count")
	})
}

func testLanguageFacets(t *testing.T, setup func(t *testing.T) (Indexer, func())) {
	t.Run("facets reflect language counts", func(t *testing.T) {
		indexer, cleanup := setup(t)
		defer cleanup()

		languages := []string{"Go", "Go", "Python", "JavaScript"}
		for i, lang := range languages {
			g := newGist(uint(i+1), 1, 0, "facet test content")
			g.Languages = []string{lang}
			require.NoError(t, indexer.Add(g))
		}

		_, _, facets, err := indexer.Search(SearchGistMetadata{Content: "facet"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, 2, facets["go"])
		require.Equal(t, 1, facets["python"])
		require.Equal(t, 1, facets["javascript"])
	})

	t.Run("facets respect visibility", func(t *testing.T) {
		indexer, cleanup := setup(t)
		defer cleanup()

		g1 := newGist(1, 1, 0, "facet visibility test")
		g1.Languages = []string{"Go"}
		g2 := newGist(2, 1, 1, "facet visibility test")
		g2.Languages = []string{"Rust"}

		for _, g := range []*Gist{g1, g2} {
			require.NoError(t, indexer.Add(g))
		}

		_, _, facets, err := indexer.Search(SearchGistMetadata{Content: "facet"}, 99, 1)
		require.NoError(t, err)
		require.Equal(t, 1, facets["go"])
		require.Equal(t, 0, facets["rust"])

		_, _, facets, err = indexer.Search(SearchGistMetadata{Content: "facet"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, 1, facets["go"])
		require.Equal(t, 1, facets["rust"])
	})
}

func testWildcardSearch(t *testing.T, setup func(t *testing.T) (Indexer, func())) {
	indexer, cleanup := setup(t)
	defer cleanup()

	g := newGist(1, 1, 0, "the elephant danced gracefully")
	require.NoError(t, indexer.Add(g))

	t.Run("substring wildcard match on content", func(t *testing.T) {
		ids, total, _, err := indexer.Search(SearchGistMetadata{Content: "leph"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
		require.Equal(t, uint(1), ids[0])
	})

	t.Run("substring wildcard no match", func(t *testing.T) {
		_, total, _, err := indexer.Search(SearchGistMetadata{Content: "zzzz"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(0), total)
	})
}

func testMetadataOnlySearch(t *testing.T, setup func(t *testing.T) (Indexer, func())) {
	indexer, cleanup := setup(t)
	defer cleanup()

	g1 := &Gist{
		GistID: 1, UserID: 1, Visibility: 0,
		Username: "alice", Title: "Go Tutorial",
		Content:    "learning golang basics",
		Filenames:  []string{"main.go"},
		Extensions: []string{"go"},
		Languages:  []string{"Go"},
		Topics:     []string{},
		CreatedAt:  1234567890, UpdatedAt: 1234567890,
	}
	g2 := &Gist{
		GistID: 2, UserID: 2, Visibility: 0,
		Username: "bob", Title: "Python Script",
		Content:    "data processing pipeline",
		Filenames:  []string{"script.py"},
		Extensions: []string{"py"},
		Languages:  []string{"Python"},
		Topics:     []string{},
		CreatedAt:  1234567890, UpdatedAt: 1234567890,
	}

	for _, g := range []*Gist{g1, g2} {
		require.NoError(t, indexer.Add(g))
	}

	t.Run("Username only no content", func(t *testing.T) {
		ids, total, _, err := indexer.Search(SearchGistMetadata{Username: "alice"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
		require.Equal(t, uint(1), ids[0])
	})

	t.Run("Language only no content", func(t *testing.T) {
		ids, total, _, err := indexer.Search(SearchGistMetadata{Language: "Python"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
		require.Equal(t, uint(2), ids[0])
	})

	t.Run("Title only no content", func(t *testing.T) {
		ids, total, _, err := indexer.Search(SearchGistMetadata{Title: "Go Tutorial"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
		require.Equal(t, uint(1), ids[0])
	})
}

func testTitleFuzzySearch(t *testing.T, setup func(t *testing.T) (Indexer, func())) {
	indexer, cleanup := setup(t)
	defer cleanup()

	g := &Gist{
		GistID: 1, UserID: 1, Visibility: 0,
		Username: "alice", Title: "Kubernetes Deployment",
		Content:    "some content",
		Filenames:  []string{"deploy.yaml"},
		Extensions: []string{"yaml"},
		Languages:  []string{"YAML"},
		Topics:     []string{},
		CreatedAt:  1234567890, UpdatedAt: 1234567890,
	}
	require.NoError(t, indexer.Add(g))

	t.Run("title with typo still matches", func(t *testing.T) {
		ids, total, _, err := indexer.Search(SearchGistMetadata{Title: "Kuberntes"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
		require.Equal(t, uint(1), ids[0])
	})

	t.Run("title prefix matches", func(t *testing.T) {
		ids, total, _, err := indexer.Search(SearchGistMetadata{Title: "Kube"}, 1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), total)
		require.Equal(t, uint(1), ids[0])
	})
}

func testMultiLanguageFacets(t *testing.T, setup func(t *testing.T) (Indexer, func())) {
	indexer, cleanup := setup(t)
	defer cleanup()

	g := &Gist{
		GistID: 1, UserID: 1, Visibility: 0,
		Username: "alice", Title: "Multi-lang gist",
		Content:    "polyglot content",
		Filenames:  []string{"main.go", "script.py"},
		Extensions: []string{"go", "py"},
		Languages:  []string{"Go", "Python"},
		Topics:     []string{},
		CreatedAt:  1234567890, UpdatedAt: 1234567890,
	}
	require.NoError(t, indexer.Add(g))

	_, _, facets, err := indexer.Search(SearchGistMetadata{Content: "polyglot"}, 1, 1)
	require.NoError(t, err)
	require.Equal(t, 1, facets["go"], "Go should appear in facets")
	require.Equal(t, 1, facets["python"], "Python should appear in facets")
}
