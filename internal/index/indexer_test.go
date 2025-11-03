package index

import (
	"fmt"
	"testing"
)

// initTestGists initializes the indexer with 1000 test gists
func initTestGists(t *testing.T, indexer Indexer) {
	t.Helper()

	languages := []string{"Go", "Python", "JavaScript"}
	extensions := []string{"go", "py", "js"}
	usernames := []string{"alice", "bob", "charlie"}
	topics := []string{"algorithms", "web", "database"}

	for i := 0; i < 1000; i++ {
		langIdx := i % len(languages) // cycles 0,1,2,0,1,2,...
		userIdx := i % len(usernames) // cycles 0,1,2,0,1,2,...
		topicIdx := i % len(topics)   // cycles 0,1,2,0,1,2,...
		gistID := uint(i + 1)         // GistIDs start at 1
		visibility := uint(i % 3)     // cycles 0,1,2,0,1,2,...

		gist := &Gist{
			GistID:     gistID,
			UserID:     uint(userIdx + 1), // alice=1, bob=2, charlie=3
			Visibility: visibility,
			Username:   usernames[userIdx],
			Title:      fmt.Sprintf("Test Gist %d", gistID),
			Content:    fmt.Sprintf("This is test gist number %d with some searchable content", gistID),
			Filenames:  []string{fmt.Sprintf("file%d.%s", gistID, extensions[langIdx])},
			Extensions: []string{extensions[langIdx]},
			Languages:  []string{languages[langIdx]},
			Topics:     []string{topics[topicIdx]},
			CreatedAt:  1234567890 + int64(gistID),
			UpdatedAt:  1234567890 + int64(gistID),
		}

		err := indexer.Add(gist)
		if err != nil {
			t.Fatalf("Failed to initialize test gist %d: %v", gistID, err)
		}
	}
}

// testIndexerAddGist tests adding a gist to the index
func testIndexerAddGist(t *testing.T, indexer Indexer) {
	t.Helper()
	initTestGists(t, indexer)

	gist := &Gist{
		GistID:     1001,
		UserID:     11,
		Visibility: 0,
		Username:   "testuser",
		Title:      "Test Gist",
		Content:    "This is a test gist with some content",
		Filenames:  []string{"test.go", "readme.md"},
		Extensions: []string{"go", "md"},
		Languages:  []string{"Go", "Markdown"},
		Topics:     []string{"testing"},
		CreatedAt:  1234567890,
		UpdatedAt:  1234567890,
	}

	err := indexer.Add(gist)
	if err != nil {
		t.Fatalf("Failed to add gist to index: %v", err)
	}
}

// testIndexerAddNilGist tests that adding a nil gist returns an error
func testIndexerAddNilGist(t *testing.T, indexer Indexer) {
	t.Helper()
	initTestGists(t, indexer)

	err := indexer.Add(nil)
	if err == nil {
		t.Fatal("Expected error when adding nil gist, got nil")
	}
}

// testIndexerSearchBasic tests basic search functionality with edge cases
func testIndexerSearchBasic(t *testing.T, indexer Indexer) {
	t.Helper()
	initTestGists(t, indexer)

	// Test 1: Search by content - all init gists have "searchable content"
	t.Run("SearchByContent", func(t *testing.T) {
		gistIDs, total, languageCounts, err := indexer.Search("searchable", SearchGistMetadata{}, 1, 1)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		// Distribution: alice=334 public Go/algorithms, bob=333 private Python/web, charlie=333 private JS/database
		// As user 1 (alice), we only see alice's public gists: 334
		if total != 334 {
			t.Errorf("Expected alice to see 334 gists, got %d", total)
		}
		if len(gistIDs) == 0 {
			t.Error("Expected non-empty gist IDs")
		}
		// Only Go should appear in language facets for alice
		if len(languageCounts) == 0 {
			t.Error("Expected language facets to be populated")
		}
		if languageCounts["go"] != 334 {
			t.Errorf("Expected 334 Go gists in facets, got %d", languageCounts["go"])
		}
	})

	// Test 2: Search by specific language - Go
	t.Run("SearchByLanguage", func(t *testing.T) {
		metadata := SearchGistMetadata{Language: "Go"}
		gistIDs, total, _, err := indexer.Search("", metadata, 1, 1)
		if err != nil {
			t.Fatalf("Search by language failed: %v", err)
		}
		// All Go gists are alice's (i=0,3,6,...) = 334 gists
		// All are public
		if total != 334 {
			t.Errorf("Expected 334 Go gists, got %d", total)
		}
		// Verify GistID 1 (i=0) is in results
		foundGoGist := false
		for _, id := range gistIDs {
			if id == 1 {
				foundGoGist = true
				break
			}
		}
		if !foundGoGist && len(gistIDs) > 0 {
			t.Error("Expected to find GistID 1 (Go) in results")
		}
	})

	// Test 3: Search by specific username - alice
	t.Run("SearchByUsername", func(t *testing.T) {
		metadata := SearchGistMetadata{Username: "alice"}
		_, total, _, err := indexer.Search("", metadata, 1, 1)
		if err != nil {
			t.Fatalf("Search by username failed: %v", err)
		}
		// alice has 334 gists at i=0,3,6,...
		// All are public
		if total != 334 {
			t.Errorf("Expected 334 alice gists, got %d", total)
		}
	})

	// Test 4: Search by extension - Python (bob's private files)
	t.Run("SearchByExtension", func(t *testing.T) {
		metadata := SearchGistMetadata{Extension: "py"}
		_, total, _, err := indexer.Search("", metadata, 1, 1)
		if err != nil {
			t.Fatalf("Search by extension failed: %v", err)
		}
		// All .py files are bob's (i=1,4,7,...) = 333 files
		// All are private (visibility=1)
		// As user 1 (alice), we see 0 .py files
		if total != 0 {
			t.Errorf("Expected alice to see 0 .py files (bob's private), got %d", total)
		}
	})

	// Test 5: Search by topic - algorithms
	t.Run("SearchByTopic", func(t *testing.T) {
		metadata := SearchGistMetadata{Topic: "algorithms"}
		_, total, _, err := indexer.Search("", metadata, 1, 1)
		if err != nil {
			t.Fatalf("Search by topic failed: %v", err)
		}
		// All algorithms gists are alice's (i=0,3,6,...) = 334 gists
		// All are public
		if total != 334 {
			t.Errorf("Expected 334 algorithms gists, got %d", total)
		}
	})

	// Test 6: Combined filters - Go language + alice
	t.Run("SearchCombinedFilters", func(t *testing.T) {
		metadata := SearchGistMetadata{
			Language: "Go",
			Username: "alice",
		}
		_, total, _, err := indexer.Search("", metadata, 1, 1)
		if err != nil {
			t.Fatalf("Search with combined filters failed: %v", err)
		}
		// Go AND alice are the same set (i=0,3,6,...) = 334 gists
		// All are public
		if total != 334 {
			t.Errorf("Expected 334 Go+alice gists, got %d", total)
		}
	})

	// Test 7: Search with no results
	t.Run("SearchNoResults", func(t *testing.T) {
		gistIDs, total, _, err := indexer.Search("nonexistentquerystring12345", SearchGistMetadata{}, 1, 1)
		if err != nil {
			t.Fatalf("Search with no results failed: %v", err)
		}
		if total != 0 {
			t.Errorf("Expected 0 results for non-existent query, got %d", total)
		}
		if len(gistIDs) != 0 {
			t.Error("Expected empty gist IDs for non-existent query")
		}
	})

	// Test 8: Empty query returns all accessible gists
	t.Run("SearchEmptyQuery", func(t *testing.T) {
		gistIDs, total, languageCounts, err := indexer.Search("", SearchGistMetadata{}, 1, 1)
		if err != nil {
			t.Fatalf("Empty search failed: %v", err)
		}
		// As user 1 (alice), only sees alice's 334 public gists
		if total != 334 {
			t.Errorf("Expected 334 gists with empty query, got %d", total)
		}
		if len(gistIDs) == 0 {
			t.Error("Expected non-empty gist IDs with empty query")
		}
		// Should have only Go in facets (alice's language)
		if len(languageCounts) == 0 {
			t.Error("Expected language facets with empty query")
		}
		if languageCounts["go"] != 334 {
			t.Errorf("Expected 334 Go in facets, got %d", languageCounts["go"])
		}
	})

	// Test 9: Pagination
	t.Run("SearchPagination", func(t *testing.T) {
		// As user 1, we have 334 gists total
		// Page 1
		gistIDs1, total, _, err := indexer.Search("searchable", SearchGistMetadata{}, 1, 1)
		if err != nil {
			t.Fatalf("Page 1 search failed: %v", err)
		}
		if total != 334 {
			t.Errorf("Expected 334 total results, got %d", total)
		}
		if len(gistIDs1) == 0 {
			t.Error("Expected results on page 1")
		}

		// Page 2
		gistIDs2, _, _, err := indexer.Search("searchable", SearchGistMetadata{}, 1, 2)
		if err != nil {
			t.Fatalf("Page 2 search failed: %v", err)
		}

		// With 334 results and typical page size of 10, we should have page 2
		if len(gistIDs2) == 0 {
			t.Error("Expected results on page 2")
		}

		// Ensure pages are different
		if len(gistIDs1) > 0 && len(gistIDs2) > 0 && gistIDs1[0] == gistIDs2[0] {
			t.Error("Page 1 and page 2 should have different first results")
		}
	})

	// Test 10: Search as different user (visibility filtering)
	t.Run("SearchVisibilityFiltering", func(t *testing.T) {
		// Search as user 2 (bob)
		// bob has 333 gists at i=1,4,7,... with visibility=1 (private)
		// As user 2, bob sees: alice's 334 public gists + bob's own 333 gists = 667 total
		_, total, _, err := indexer.Search("", SearchGistMetadata{}, 2, 1)
		if err != nil {
			t.Fatalf("Search as user 2 failed: %v", err)
		}
		if total != 667 {
			t.Errorf("Expected bob to see 667 gists (334 public + 333 own), got %d", total)
		}

		// Search as non-existent user (should only see public gists)
		gistIDsPublic, totalPublic, _, err := indexer.Search("", SearchGistMetadata{}, 999, 1)
		if err != nil {
			t.Fatalf("Search as user 999 failed: %v", err)
		}
		// Non-existent user only sees alice's 334 public gists
		if totalPublic != 334 {
			t.Errorf("Expected non-existent user to see 334 public gists, got %d", totalPublic)
		}

		// Public gists (334) should be less than what user 2 sees (667)
		if totalPublic >= total {
			t.Errorf("Non-existent user sees %d gists, should be less than user 2's %d", totalPublic, total)
		}

		// Verify we can find GistID 1 (alice's public gist)
		foundPublic := false
		for _, id := range gistIDsPublic {
			if id == 1 {
				foundPublic = true
				break
			}
		}
		if !foundPublic {
			t.Error("Expected to find GistID 1 (alice's public gist) in results")
		}
	})

	// Test 11: Language facets validation
	t.Run("LanguageFacets", func(t *testing.T) {
		_, _, languageCounts, err := indexer.Search("", SearchGistMetadata{}, 1, 1)
		if err != nil {
			t.Fatalf("Search for facets failed: %v", err)
		}
		if len(languageCounts) != 1 {
			t.Errorf("Expected 1 language in facets (Go), got %d", len(languageCounts))
		}
		// As user 1 (alice), should only see Go with count 334
		if languageCounts["go"] != 334 {
			t.Errorf("Expected 334 Go in facets, got %d", languageCounts["go"])
		}
		// Python and JavaScript should not appear (bob's and charlie's private gists)
		if languageCounts["Python"] != 0 {
			t.Errorf("Expected 0 Python in facets, got %d", languageCounts["Python"])
		}
		if languageCounts["JavaScript"] != 0 {
			t.Errorf("Expected 0 JavaScript in facets, got %d", languageCounts["JavaScript"])
		}
	})
}

// testIndexerSearchWithMetadata tests search with metadata filters
func testIndexerSearchWithMetadata(t *testing.T, indexer Indexer) {
	t.Helper()
	initTestGists(t, indexer)

	// Add a test gist with unique username
	gist := &Gist{
		GistID:     1002,
		UserID:     11,
		Visibility: 0,
		Username:   "uniqueuser",
		Title:      "Python Script",
		Content:    "A useful Python script for data processing",
		Filenames:  []string{"script.py"},
		Extensions: []string{"py"},
		Languages:  []string{"Python"},
		Topics:     []string{"data", "scripts"},
		CreatedAt:  1234567890,
		UpdatedAt:  1234567890,
	}

	err := indexer.Add(gist)
	if err != nil {
		t.Fatalf("Failed to add gist: %v", err)
	}

	// Search with username filter
	metadata := SearchGistMetadata{
		Username: "uniqueuser",
	}
	gistIDs, total, _, err := indexer.Search("", metadata, 100, 1)
	if err != nil {
		t.Fatalf("Search with metadata failed: %v", err)
	}

	if total == 0 {
		t.Error("Expected to find gist by username")
	}

	foundGist := false
	for _, id := range gistIDs {
		if id == 1002 {
			foundGist = true
			break
		}
	}
	if !foundGist {
		t.Error("Expected to find gist ID 1002 in results")
	}
}

// testIndexerSearchEmpty tests search with empty query
func testIndexerSearchEmpty(t *testing.T, indexer Indexer) {
	t.Helper()
	initTestGists(t, indexer)

	// Search with empty query (should return all accessible gists)
	gistIDs, total, _, err := indexer.Search("", SearchGistMetadata{}, 100, 1)
	if err != nil {
		t.Fatalf("Empty search failed: %v", err)
	}

	if total < 1000 {
		t.Errorf("Expected to find at least 1000 gists with empty query, got %d", total)
	}

	if len(gistIDs) == 0 {
		t.Error("Expected non-empty gist IDs with empty query")
	}
}

// testIndexerRemoveGist tests removing a gist from the index
func testIndexerRemoveGist(t *testing.T, indexer Indexer) {
	t.Helper()
	initTestGists(t, indexer)

	// Add a test gist
	gist := &Gist{
		GistID:     1003,
		UserID:     11,
		Visibility: 0,
		Username:   "charlie",
		Title:      "To Be Removed",
		Content:    "This gist will be uniquelyremoved with special marker",
		Filenames:  []string{"remove.go"},
		Extensions: []string{"go"},
		Languages:  []string{"Go"},
		Topics:     []string{"removal"},
		CreatedAt:  1234567890,
		UpdatedAt:  1234567890,
	}

	err := indexer.Add(gist)
	if err != nil {
		t.Fatalf("Failed to add gist: %v", err)
	}

	// Verify it's there
	gistIDs, total, _, err := indexer.Search("uniquelyremoved", SearchGistMetadata{}, 100, 1)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if total == 0 {
		t.Error("Expected to find the gist before removal")
	}

	// Remove the gist
	err = indexer.Remove(1003)
	if err != nil {
		t.Fatalf("Failed to remove gist: %v", err)
	}

	// Verify it's gone
	gistIDs, total, _, err = indexer.Search("uniquelyremoved", SearchGistMetadata{}, 100, 1)
	if err != nil {
		t.Fatalf("Search after removal failed: %v", err)
	}

	// Should not find the removed gist
	for _, id := range gistIDs {
		if id == 1003 {
			t.Error("Found removed gist in search results")
		}
	}
}

// testIndexerMultipleGists tests indexing multiple gists
func testIndexerMultipleGists(t *testing.T, indexer Indexer) {
	t.Helper()
	initTestGists(t, indexer)

	gists := []*Gist{
		{
			GistID:     1004,
			UserID:     11,
			Visibility: 0,
			Username:   "user1",
			Title:      "First Gist",
			Content:    "First gist content",
			Filenames:  []string{"first.go"},
			Extensions: []string{"go"},
			Languages:  []string{"Go"},
			Topics:     []string{"first"},
			CreatedAt:  1234567890,
			UpdatedAt:  1234567890,
		},
		{
			GistID:     1005,
			UserID:     11,
			Visibility: 0,
			Username:   "user2",
			Title:      "Second Gist",
			Content:    "Second gist content",
			Filenames:  []string{"second.py"},
			Extensions: []string{"py"},
			Languages:  []string{"Python"},
			Topics:     []string{"second"},
			CreatedAt:  1234567891,
			UpdatedAt:  1234567891,
		},
		{
			GistID:     1006,
			UserID:     11,
			Visibility: 0,
			Username:   "user3",
			Title:      "Third Gist",
			Content:    "Third gist content",
			Filenames:  []string{"third.js"},
			Extensions: []string{"js"},
			Languages:  []string{"JavaScript"},
			Topics:     []string{"third"},
			CreatedAt:  1234567892,
			UpdatedAt:  1234567892,
		},
	}

	for _, gist := range gists {
		err := indexer.Add(gist)
		if err != nil {
			t.Fatalf("Failed to add gist %d: %v", gist.GistID, err)
		}
	}

	// Search for all gists (should include the 1000 initialized + 3 new ones)
	gistIDs, total, languageCounts, err := indexer.Search("", SearchGistMetadata{}, 100, 1)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if total < 1003 {
		t.Errorf("Expected at least 1003 gists, got %d", total)
	}

	if len(gistIDs) < 10 {
		t.Errorf("Expected at least 10 gist IDs in first page, got %d", len(gistIDs))
	}

	// Check language facets contain our languages
	if len(languageCounts) == 0 {
		t.Error("Expected language counts to be populated")
	}
}

// testIndexerPagination tests pagination in search results
func testIndexerPagination(t *testing.T, indexer Indexer) {
	t.Helper()
	initTestGists(t, indexer)

	// Add multiple gists with unique content for pagination testing
	for i := 1007; i < 1027; i++ {
		gist := &Gist{
			GistID:     uint(i),
			UserID:     11,
			Visibility: 0,
			Username:   "paginationtester",
			Title:      "Pagination Test",
			Content:    "uniquepaginationcontent test content",
			Filenames:  []string{"page.go"},
			Extensions: []string{"go"},
			Languages:  []string{"Go"},
			Topics:     []string{"pagination"},
			CreatedAt:  1234567890 + int64(i),
			UpdatedAt:  1234567890 + int64(i),
		}
		err := indexer.Add(gist)
		if err != nil {
			t.Fatalf("Failed to add gist %d: %v", i, err)
		}
	}

	// Get page 1
	gistIDs1, total, _, err := indexer.Search("uniquepaginationcontent", SearchGistMetadata{}, 100, 1)
	if err != nil {
		t.Fatalf("Page 1 search failed: %v", err)
	}

	if total < 20 {
		t.Errorf("Expected at least 20 total results, got %d", total)
	}

	if len(gistIDs1) == 0 {
		t.Error("Expected results on page 1")
	}

	// Test page 2
	gistIDs2, _, _, err := indexer.Search("uniquepaginationcontent", SearchGistMetadata{}, 100, 2)
	if err != nil {
		t.Fatalf("Page 2 search failed: %v", err)
	}

	if len(gistIDs2) == 0 {
		t.Error("Expected results on page 2")
	}

	// Ensure page 1 and page 2 have different results
	if len(gistIDs1) > 0 && len(gistIDs2) > 0 && gistIDs1[0] == gistIDs2[0] {
		t.Error("Page 1 and page 2 returned the same first result")
	}
}
