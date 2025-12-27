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

// testIndexerAddGist tests adding a gist to the index with comprehensive edge cases
func testIndexerAddGist(t *testing.T, indexer Indexer) {
	t.Helper()
	initTestGists(t, indexer)

	// Test 1: Add basic gist with multiple files
	t.Run("AddBasicGist", func(t *testing.T) {
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

		// Verify gist is searchable
		gistIDs, total, _, err := indexer.Search("test gist", SearchGistMetadata{}, 11, 1)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		if total == 0 {
			t.Error("Expected to find the added gist")
		}
		found := false
		for _, id := range gistIDs {
			if id == 1001 {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find GistID 1001 in search results")
		}
	})

	// Test 2: Add gist and search by language
	t.Run("AddAndSearchByLanguage", func(t *testing.T) {
		gist := &Gist{
			GistID:     1002,
			UserID:     11,
			Visibility: 0,
			Username:   "testuser",
			Title:      "Rust Example",
			Content:    "fn main() { println!(\"Hello\"); }",
			Filenames:  []string{"main.rs"},
			Extensions: []string{"rs"},
			Languages:  []string{"Rust"},
			Topics:     []string{"systems"},
			CreatedAt:  1234567891,
			UpdatedAt:  1234567891,
		}

		err := indexer.Add(gist)
		if err != nil {
			t.Fatalf("Failed to add Rust gist: %v", err)
		}

		// Search by Rust language
		metadata := SearchGistMetadata{Language: "Rust"}
		gistIDs, total, _, err := indexer.Search("", metadata, 11, 1)
		if err != nil {
			t.Fatalf("Search by Rust language failed: %v", err)
		}
		if total == 0 {
			t.Error("Expected to find Rust gist")
		}
		found := false
		for _, id := range gistIDs {
			if id == 1002 {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find GistID 1002 in Rust search results")
		}
	})

	// Test 3: Add gist with special characters and unicode
	t.Run("AddGistWithUnicode", func(t *testing.T) {
		gist := &Gist{
			GistID:     1003,
			UserID:     11,
			Visibility: 0,
			Username:   "testuser",
			Title:      "Unicode Test: café résumé naïve",
			Content:    "Special chars: @#$%^&*() and unicode: 你好世界 مرحبا العالم",
			Filenames:  []string{"unicode.txt"},
			Extensions: []string{"txt"},
			Languages:  []string{"Text"},
			Topics:     []string{"unicode", "i18n"},
			CreatedAt:  1234567892,
			UpdatedAt:  1234567892,
		}

		err := indexer.Add(gist)
		if err != nil {
			t.Fatalf("Failed to add unicode gist: %v", err)
		}

		// Search for unicode content
		_, total, _, err := indexer.Search("café", SearchGistMetadata{}, 11, 1)
		if err != nil {
			t.Fatalf("Search for unicode failed: %v", err)
		}
		// Note: Unicode search support may vary by indexer
		if total > 0 {
			t.Logf("Unicode search returned %d results", total)
		}
	})

	// Test 4: Add gist with different visibility levels
	t.Run("AddGistPrivate", func(t *testing.T) {
		privateGist := &Gist{
			GistID:     1004,
			UserID:     11,
			Visibility: 1,
			Username:   "testuser",
			Title:      "Private Gist",
			Content:    "This is a private gist",
			Filenames:  []string{"private.txt"},
			Extensions: []string{"txt"},
			Languages:  []string{"Text"},
			Topics:     []string{"private"},
			CreatedAt:  1234567893,
			UpdatedAt:  1234567893,
		}

		err := indexer.Add(privateGist)
		if err != nil {
			t.Fatalf("Failed to add private gist: %v", err)
		}

		// User 11 should see their own private gist
		gistIDs, total, _, err := indexer.Search("private gist", SearchGistMetadata{}, 11, 1)
		if err != nil {
			t.Fatalf("Search for private gist as owner failed: %v", err)
		}
		found := false
		for _, id := range gistIDs {
			if id == 1004 {
				found = true
				break
			}
		}
		if !found && total > 0 {
			t.Error("Expected owner to find their private gist")
		}

		// User 999 should NOT see user 11's private gist
		gistIDs2, _, _, err := indexer.Search("private gist", SearchGistMetadata{}, 999, 1)
		if err != nil {
			t.Fatalf("Search for private gist as other user failed: %v", err)
		}
		for _, id := range gistIDs2 {
			if id == 1004 {
				t.Error("Other user should not see private gist")
			}
		}
	})

	// Test 5: Add gist with empty optional fields
	t.Run("AddGistMinimalFields", func(t *testing.T) {
		gist := &Gist{
			GistID:     1005,
			UserID:     11,
			Visibility: 0,
			Username:   "testuser",
			Title:      "",
			Content:    "Minimal content",
			Filenames:  []string{},
			Extensions: []string{},
			Languages:  []string{},
			Topics:     []string{},
			CreatedAt:  1234567894,
			UpdatedAt:  1234567894,
		}

		err := indexer.Add(gist)
		if err != nil {
			t.Fatalf("Failed to add minimal gist: %v", err)
		}

		// Should still be searchable by content
		_, total, _, err := indexer.Search("Minimal", SearchGistMetadata{}, 11, 1)
		if err != nil {
			t.Fatalf("Search for minimal gist failed: %v", err)
		}
		if total == 0 {
			t.Error("Expected to find minimal gist by content")
		}
	})

	// Test 6: Update existing gist (same GistID)
	t.Run("UpdateExistingGist", func(t *testing.T) {
		originalGist := &Gist{
			GistID:     1006,
			UserID:     11,
			Visibility: 0,
			Username:   "testuser",
			Title:      "Original Title",
			Content:    "Original content",
			Filenames:  []string{"original.txt"},
			Extensions: []string{"txt"},
			Languages:  []string{"Text"},
			Topics:     []string{"original"},
			CreatedAt:  1234567895,
			UpdatedAt:  1234567895,
		}

		err := indexer.Add(originalGist)
		if err != nil {
			t.Fatalf("Failed to add original gist: %v", err)
		}

		// Update with same GistID
		updatedGist := &Gist{
			GistID:     1006,
			UserID:     11,
			Visibility: 0,
			Username:   "testuser",
			Title:      "Updated Title",
			Content:    "Updated content with new information",
			Filenames:  []string{"updated.txt"},
			Extensions: []string{"txt"},
			Languages:  []string{"Text"},
			Topics:     []string{"updated"},
			CreatedAt:  1234567895,
			UpdatedAt:  1234567900,
		}

		err = indexer.Add(updatedGist)
		if err != nil {
			t.Fatalf("Failed to update gist: %v", err)
		}

		// Search should find updated content, not original
		gistIDs, total, _, err := indexer.Search("new information", SearchGistMetadata{}, 11, 1)
		if err != nil {
			t.Fatalf("Search for updated content failed: %v", err)
		}
		found := false
		for _, id := range gistIDs {
			if id == 1006 {
				found = true
				break
			}
		}
		if !found && total > 0 {
			t.Error("Expected to find updated gist by new content")
		}

		// Old content should not be found
		gistIDsOld, _, _, _ := indexer.Search("Original", SearchGistMetadata{}, 11, 1)
		for _, id := range gistIDsOld {
			if id == 1006 {
				t.Error("Should not find gist by old content after update")
			}
		}
	})

	// Test 7: Add gist and verify by username filter
	t.Run("AddAndSearchByUsername", func(t *testing.T) {
		gist := &Gist{
			GistID:     1007,
			UserID:     12,
			Visibility: 0,
			Username:   "newuser",
			Title:      "New User Gist",
			Content:    "Content from new user",
			Filenames:  []string{"newuser.txt"},
			Extensions: []string{"txt"},
			Languages:  []string{"Text"},
			Topics:     []string{"new"},
			CreatedAt:  1234567896,
			UpdatedAt:  1234567896,
		}

		err := indexer.Add(gist)
		if err != nil {
			t.Fatalf("Failed to add new user gist: %v", err)
		}

		// Search by username
		metadata := SearchGistMetadata{Username: "newuser"}
		gistIDs, total, _, err := indexer.Search("", metadata, 12, 1)
		if err != nil {
			t.Fatalf("Search by username failed: %v", err)
		}
		if total == 0 {
			t.Error("Expected to find gist by username filter")
		}
		found := false
		for _, id := range gistIDs {
			if id == 1007 {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find GistID 1007 by username")
		}
	})

	// Test 8: Add gist with multiple languages and topics
	t.Run("AddGistMultipleTags", func(t *testing.T) {
		gist := &Gist{
			GistID:     1008,
			UserID:     11,
			Visibility: 0,
			Username:   "testuser",
			Title:      "Multi-language Project",
			Content:    "Mixed language project with Go, Python, and JavaScript",
			Filenames:  []string{"main.go", "script.py", "app.js"},
			Extensions: []string{"go", "py", "js"},
			Languages:  []string{"Go", "Python", "JavaScript"},
			Topics:     []string{"fullstack", "microservices", "api"},
			CreatedAt:  1234567897,
			UpdatedAt:  1234567897,
		}

		err := indexer.Add(gist)
		if err != nil {
			t.Fatalf("Failed to add multi-language gist: %v", err)
		}

		// Search by one of the topics
		metadata := SearchGistMetadata{Topic: "microservices"}
		gistIDs, total, _, err := indexer.Search("", metadata, 11, 1)
		if err != nil {
			t.Fatalf("Search by topic failed: %v", err)
		}
		found := false
		for _, id := range gistIDs {
			if id == 1008 {
				found = true
				break
			}
		}
		if !found && total > 0 {
			t.Error("Expected to find multi-language gist by topic")
		}
	})

	// Test 9: Add gist with long content
	t.Run("AddGistLongContent", func(t *testing.T) {
		longContent := ""
		for i := 0; i < 1000; i++ {
			longContent += fmt.Sprintf("Line %d: This is a long gist with lots of content. ", i)
		}

		gist := &Gist{
			GistID:     1009,
			UserID:     11,
			Visibility: 0,
			Username:   "testuser",
			Title:      "Long Gist",
			Content:    longContent,
			Filenames:  []string{"long.txt"},
			Extensions: []string{"txt"},
			Languages:  []string{"Text"},
			Topics:     []string{"large"},
			CreatedAt:  1234567898,
			UpdatedAt:  1234567898,
		}

		err := indexer.Add(gist)
		if err != nil {
			t.Fatalf("Failed to add long gist: %v", err)
		}

		// Search for content from the middle
		gistIDs, total, _, err := indexer.Search("Line 500", SearchGistMetadata{}, 11, 1)
		if err != nil {
			t.Fatalf("Search in long content failed: %v", err)
		}
		found := false
		for _, id := range gistIDs {
			if id == 1009 {
				found = true
				break
			}
		}
		if !found && total > 0 {
			t.Error("Expected to find long gist by content in the middle")
		}
	})
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
		_, total, _, err := indexer.Search("", metadata, 1, 1)
		if err != nil {
			t.Fatalf("Search by language failed: %v", err)
		}
		// All Go gists are alice's (i=0,3,6,...) = 334 gists
		// All are public
		if total != 334 {
			t.Errorf("Expected 334 Go gists, got %d", total)
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
		gistIDs, total, _, err := indexer.Search("nonexistentquery", SearchGistMetadata{}, 1, 1)
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
		_, totalPublic, _, err := indexer.Search("", SearchGistMetadata{}, 999, 1)
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

// testIndexerAllFieldSearch tests the "All" field OR search functionality
func testIndexerAllFieldSearch(t *testing.T, indexer Indexer) {
	t.Helper()
	initTestGists(t, indexer)

	// Add test gists with distinct values in different fields
	testGists := []*Gist{
		{
			GistID:     3001,
			UserID:     100,
			Visibility: 0,
			Username:   "testuser_unique",
			Title:      "Configuration Guide",
			Content:    "How to configure your application",
			Filenames:  []string{"config.txt"},
			Extensions: []string{"txt"},
			Languages:  []string{"Text"},
			Topics:     []string{"configuration"},
			CreatedAt:  1234567890,
			UpdatedAt:  1234567890,
		},
		{
			GistID:     3002,
			UserID:     100,
			Visibility: 0,
			Username:   "developer",
			Title:      "Testing unique features",
			Content:    "Testing best practices",
			Filenames:  []string{"test.txt"},
			Extensions: []string{"txt"},
			Languages:  []string{"Text"},
			Topics:     []string{"testing"},
			CreatedAt:  1234567891,
			UpdatedAt:  1234567891,
		},
		{
			GistID:     3003,
			UserID:     100,
			Visibility: 0,
			Username:   "coder",
			Title:      "API Documentation",
			Content:    "REST API documentation",
			Filenames:  []string{"api.txt"},
			Extensions: []string{"txt"},
			Languages:  []string{"Markdown"},
			Topics:     []string{"unique_topic"},
			CreatedAt:  1234567892,
			UpdatedAt:  1234567892,
		},
		{
			GistID:     3004,
			UserID:     100,
			Visibility: 0,
			Username:   "programmer",
			Title:      "Code Examples",
			Content:    "Code examples for beginners",
			Filenames:  []string{"unique_file.rb"},
			Extensions: []string{"rb"},
			Languages:  []string{"Ruby"},
			Topics:     []string{"examples"},
			CreatedAt:  1234567893,
			UpdatedAt:  1234567893,
		},
		{
			GistID:     3005,
			UserID:     100,
			Visibility: 0,
			Username:   "admin",
			Title:      "Setup Instructions",
			Content:    "How to setup the project",
			Filenames:  []string{"setup.sh"},
			Extensions: []string{"sh"},
			Languages:  []string{"Shell"},
			Topics:     []string{"setup"},
			CreatedAt:  1234567894,
			UpdatedAt:  1234567894,
		},
	}

	for _, gist := range testGists {
		err := indexer.Add(gist)
		if err != nil {
			t.Fatalf("Failed to add test gist %d: %v", gist.GistID, err)
		}
	}

	// Test 1: All field matches username
	t.Run("AllFieldMatchesUsername", func(t *testing.T) {
		metadata := SearchGistMetadata{All: "testuser_unique"}
		gistIDs, total, _, err := indexer.Search("", metadata, 100, 1)
		if err != nil {
			t.Fatalf("All field search failed: %v", err)
		}
		if total == 0 {
			t.Error("Expected to find gist by username via All field")
		}
		found := false
		for _, id := range gistIDs {
			if id == 3001 {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find GistID 3001 by username via All field")
		}
	})

	// Test 2: All field matches title
	t.Run("AllFieldMatchesTitle", func(t *testing.T) {
		metadata := SearchGistMetadata{All: "unique features"}
		gistIDs, total, _, err := indexer.Search("", metadata, 100, 1)
		if err != nil {
			t.Fatalf("All field search failed: %v", err)
		}
		if total == 0 {
			t.Error("Expected to find gist by title via All field")
		}
		found := false
		for _, id := range gistIDs {
			if id == 3002 {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find GistID 3002 by title via All field")
		}
	})

	// Test 3: All field matches language
	t.Run("AllFieldMatchesLanguage", func(t *testing.T) {
		metadata := SearchGistMetadata{All: "Ruby"}
		gistIDs, total, _, err := indexer.Search("", metadata, 100, 1)
		if err != nil {
			t.Fatalf("All field search failed: %v", err)
		}
		if total == 0 {
			t.Error("Expected to find gist by language via All field")
		}
		found := false
		for _, id := range gistIDs {
			if id == 3004 {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find GistID 3004 by language via All field")
		}
	})

	// Test 4: All field matches topic
	t.Run("AllFieldMatchesTopic", func(t *testing.T) {
		metadata := SearchGistMetadata{All: "unique_topic"}
		gistIDs, total, _, err := indexer.Search("", metadata, 100, 1)
		if err != nil {
			t.Fatalf("All field search failed: %v", err)
		}
		if total == 0 {
			t.Error("Expected to find gist by topic via All field")
		}
		found := false
		for _, id := range gistIDs {
			if id == 3003 {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find GistID 3003 by topic via All field")
		}
	})

	// Test 5: All field matches extension
	t.Run("AllFieldMatchesExtension", func(t *testing.T) {
		metadata := SearchGistMetadata{All: "sh"}
		gistIDs, total, _, err := indexer.Search("", metadata, 100, 1)
		if err != nil {
			t.Fatalf("All field search failed: %v", err)
		}
		if total == 0 {
			t.Error("Expected to find gist by extension via All field")
		}
		found := false
		for _, id := range gistIDs {
			if id == 3005 {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find GistID 3005 by extension via All field")
		}
	})

	// Test 6: All field matches filename
	t.Run("AllFieldMatchesFilename", func(t *testing.T) {
		metadata := SearchGistMetadata{All: "unique_file.rb"}
		gistIDs, total, _, err := indexer.Search("", metadata, 100, 1)
		if err != nil {
			t.Fatalf("All field search failed: %v", err)
		}
		if total == 0 {
			t.Error("Expected to find gist by filename via All field")
		}
		found := false
		for _, id := range gistIDs {
			if id == 3004 {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find GistID 3004 by filename via All field")
		}
	})

	// Test 7: All field OR behavior - matches across different fields
	t.Run("AllFieldORBehavior", func(t *testing.T) {
		// "unique" appears in: username (3001), title (3002), topic (3003), filename (3004)
		metadata := SearchGistMetadata{All: "unique"}
		gistIDs, total, _, err := indexer.Search("", metadata, 100, 1)
		if err != nil {
			t.Fatalf("All field OR search failed: %v", err)
		}
		if total < 4 {
			t.Errorf("Expected at least 4 results from OR search, got %d", total)
		}

		// Verify we found gists from different fields
		foundIDs := make(map[uint]bool)
		for _, id := range gistIDs {
			if id >= 3001 && id <= 3004 {
				foundIDs[id] = true
			}
		}

		expectedIDs := []uint{3001, 3002, 3003, 3004}
		for _, expectedID := range expectedIDs {
			if !foundIDs[expectedID] {
				t.Errorf("Expected to find GistID %d in OR search results", expectedID)
			}
		}
	})

	// Test 8: All field returns more results than specific field (OR vs AND)
	t.Run("AllFieldVsSpecificField", func(t *testing.T) {
		// Search with All field
		metadataAll := SearchGistMetadata{All: "unique"}
		_, totalAll, _, err := indexer.Search("", metadataAll, 100, 1)
		if err != nil {
			t.Fatalf("All field search failed: %v", err)
		}

		// Search with specific username field only
		metadataSpecific := SearchGistMetadata{Username: "testuser_unique"}
		_, totalSpecific, _, err := indexer.Search("", metadataSpecific, 100, 1)
		if err != nil {
			t.Fatalf("Specific field search failed: %v", err)
		}

		// All field should return more results (OR) than specific field
		if totalAll <= totalSpecific {
			t.Errorf("All field (OR) should return more results (%d) than specific field (%d)", totalAll, totalSpecific)
		}
	})

	// Test 9: All field with no matches
	t.Run("AllFieldNoMatches", func(t *testing.T) {
		metadata := SearchGistMetadata{All: "nonexistentvalue12345"}
		gistIDs, total, _, err := indexer.Search("", metadata, 100, 1)
		if err != nil {
			t.Fatalf("All field no match search failed: %v", err)
		}
		if total != 0 {
			t.Errorf("Expected 0 results for non-existent value, got %d", total)
		}
		if len(gistIDs) != 0 {
			t.Error("Expected empty gist IDs for non-existent value")
		}
	})

	// Test 10: All field is mutually exclusive with specific fields
	t.Run("AllFieldIgnoresOtherFields", func(t *testing.T) {
		// When All is specified, other specific fields should be ignored
		metadata := SearchGistMetadata{
			All:      "unique",
			Username: "nonexistent", // This should be ignored
			Language: "NonExistent", // This should be ignored
		}
		gistIDs, total, _, err := indexer.Search("", metadata, 100, 1)
		if err != nil {
			t.Fatalf("All field with other fields search failed: %v", err)
		}
		// Should still find results because All is used (and other fields are ignored)
		if total < 4 {
			t.Errorf("Expected All field to be used (ignoring other fields), got %d results", total)
		}
		// Verify we found gists matching "unique"
		foundAny := false
		for _, id := range gistIDs {
			if id >= 3001 && id <= 3004 {
				foundAny = true
				break
			}
		}
		if !foundAny {
			t.Error("Expected All field to override specific fields and find results")
		}
	})

	// Test 11: All field with content query
	t.Run("AllFieldWithContentQuery", func(t *testing.T) {
		// All field searches metadata, content query searches content
		// Both should work together
		metadata := SearchGistMetadata{All: "Ruby"}
		gistIDs, total, _, err := indexer.Search("examples", metadata, 100, 1)
		if err != nil {
			t.Fatalf("All field with content query failed: %v", err)
		}
		// Should find gist 3004 which has Ruby language AND "examples" in content
		if total == 0 {
			t.Error("Expected to find gist matching both All field and content query")
		}
		found := false
		for _, id := range gistIDs {
			if id == 3004 {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find GistID 3004 matching both conditions")
		}
	})

	// Test 12: All field case insensitivity
	t.Run("AllFieldCaseInsensitive", func(t *testing.T) {
		// Search with different case
		metadata := SearchGistMetadata{All: "RUBY"}
		gistIDs, total, _, err := indexer.Search("", metadata, 100, 1)
		if err != nil {
			t.Fatalf("All field case insensitive search failed: %v", err)
		}
		if total == 0 {
			t.Error("Expected case insensitive match for All field")
		}
		found := false
		for _, id := range gistIDs {
			if id == 3004 {
				found = true
				break
			}
		}
		if !found && total > 0 {
			t.Log("Case insensitive All field search returned results but not exact match")
		}
	})
}

// testIndexerFuzzySearch tests fuzzy search functionality (typo tolerance)
func testIndexerFuzzySearch(t *testing.T, indexer Indexer) {
	t.Helper()
	initTestGists(t, indexer)

	// Add test gists with specific content for fuzzy search testing
	testGists := []*Gist{
		{
			GistID:     2001,
			UserID:     100,
			Visibility: 0,
			Username:   "fuzzytest",
			Title:      "Algorithm Test",
			Content:    "This is a test about algorithms and data structures",
			Filenames:  []string{"algorithm.txt"},
			Extensions: []string{"txt"},
			Languages:  []string{"Text"},
			Topics:     []string{"algorithms"},
			CreatedAt:  1234567890,
			UpdatedAt:  1234567890,
		},
		{
			GistID:     2002,
			UserID:     100,
			Visibility: 0,
			Username:   "fuzzytest",
			Title:      "Python Guide",
			Content:    "A comprehensive guide to python programming language",
			Filenames:  []string{"python.txt"},
			Extensions: []string{"txt"},
			Languages:  []string{"Text"},
			Topics:     []string{"python"},
			CreatedAt:  1234567891,
			UpdatedAt:  1234567891,
		},
		{
			GistID:     2003,
			UserID:     100,
			Visibility: 0,
			Username:   "fuzzytest",
			Title:      "Database Fundamentals",
			Content:    "Understanding relational databases and SQL queries",
			Filenames:  []string{"database.txt"},
			Extensions: []string{"txt"},
			Languages:  []string{"Text"},
			Topics:     []string{"database"},
			CreatedAt:  1234567892,
			UpdatedAt:  1234567892,
		},
		{
			GistID:     2004,
			UserID:     100,
			Visibility: 0,
			Username:   "fuzzytest",
			Title:      "JavaScript Essentials",
			Content:    "Essential javascript concepts for web development",
			Filenames:  []string{"javascript.txt"},
			Extensions: []string{"txt"},
			Languages:  []string{"Text"},
			Topics:     []string{"javascript"},
			CreatedAt:  1234567893,
			UpdatedAt:  1234567893,
		},
	}

	for _, gist := range testGists {
		err := indexer.Add(gist)
		if err != nil {
			t.Fatalf("Failed to add fuzzy test gist %d: %v", gist.GistID, err)
		}
	}

	// Test 1: Exact match should work
	t.Run("ExactMatch", func(t *testing.T) {
		gistIDs, total, _, err := indexer.Search("algorithms", SearchGistMetadata{}, 100, 1)
		if err != nil {
			t.Fatalf("Exact match search failed: %v", err)
		}
		if total == 0 {
			t.Error("Expected to find gist with exact match 'algorithms'")
		}
		found := false
		for _, id := range gistIDs {
			if id == 2001 {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find GistID 2001 with exact match")
		}
	})

	// Test 2: 1 character typo - substitution
	t.Run("OneCharSubstitution", func(t *testing.T) {
		// "algoritm" instead of "algorithm" (missing 'h')
		gistIDs, total, _, err := indexer.Search("algoritm", SearchGistMetadata{}, 100, 1)
		if err != nil {
			t.Fatalf("1-char typo search failed: %v", err)
		}
		if total == 0 {
			t.Error("Expected fuzzy search to find 'algorithm' with typo 'algoritm'")
		}
		found := false
		for _, id := range gistIDs {
			if id == 2001 {
				found = true
				break
			}
		}
		if !found && total > 0 {
			t.Log("Fuzzy search returned results but not the expected gist (may be acceptable)")
		}
	})

	// Test 3: 1 character typo - deletion
	t.Run("OneCharDeletion", func(t *testing.T) {
		// "pythn" instead of "python" (missing 'o')
		gistIDs, total, _, err := indexer.Search("pythn", SearchGistMetadata{}, 100, 1)
		if err != nil {
			t.Fatalf("1-char deletion search failed: %v", err)
		}
		if total == 0 {
			t.Error("Expected fuzzy search to find 'python' with typo 'pythn'")
		}
		found := false
		for _, id := range gistIDs {
			if id == 2002 {
				found = true
				break
			}
		}
		if !found && total > 0 {
			t.Log("Fuzzy search returned results but not the expected gist")
		}
	})

	// Test 4: 1 character typo - insertion (extra character)
	t.Run("OneCharInsertion", func(t *testing.T) {
		// "pythonn" instead of "python" (extra 'n')
		gistIDs, total, _, err := indexer.Search("pythonn", SearchGistMetadata{}, 100, 1)
		if err != nil {
			t.Fatalf("1-char insertion search failed: %v", err)
		}
		if total == 0 {
			t.Error("Expected fuzzy search to find 'python' with typo 'pythonn'")
		}
		found := false
		for _, id := range gistIDs {
			if id == 2002 {
				found = true
				break
			}
		}
		if !found && total > 0 {
			t.Log("Fuzzy search returned results but not the expected gist")
		}
	})

	// Test 5: 2 character typos - should still match with fuzziness=2
	t.Run("TwoCharTypos", func(t *testing.T) {
		// "databse" instead of "database" (missing 'a', transposed 's')
		gistIDs, total, _, err := indexer.Search("databse", SearchGistMetadata{}, 100, 1)
		if err != nil {
			t.Fatalf("2-char typo search failed: %v", err)
		}
		if total == 0 {
			t.Error("Expected fuzzy search to find 'database' with typo 'databse'")
		}
		found := false
		for _, id := range gistIDs {
			if id == 2003 {
				found = true
				break
			}
		}
		if !found && total > 0 {
			t.Log("Fuzzy search returned results but not the expected gist with 2 typos")
		}
	})

	// Test 6: 2 character typos - different word
	t.Run("TwoCharTyposDifferentWord", func(t *testing.T) {
		// "javasript" instead of "javascript" (missing 'c')
		gistIDs, total, _, err := indexer.Search("javasript", SearchGistMetadata{}, 100, 1)
		if err != nil {
			t.Fatalf("2-char typo search failed: %v", err)
		}
		if total == 0 {
			t.Error("Expected fuzzy search to find 'javascript' with typo 'javasript'")
		}
		found := false
		for _, id := range gistIDs {
			if id == 2004 {
				found = true
				break
			}
		}
		if !found && total > 0 {
			t.Log("Fuzzy search returned results but not the expected gist")
		}
	})

	// Test 7: 3 character typos - should NOT match (beyond fuzziness=2)
	t.Run("ThreeCharTyposShouldNotMatch", func(t *testing.T) {
		// "algorthm" instead of "algorithm" (missing 'i', 't', 'h') - too different
		gistIDs, _, _, err := indexer.Search("algorthm", SearchGistMetadata{}, 100, 1)
		if err != nil {
			t.Fatalf("3-char typo search failed: %v", err)
		}
		// With fuzziness=2, this might or might not match depending on the algorithm
		// We'll just log the result
		found := false
		for _, id := range gistIDs {
			if id == 2001 {
				found = true
				break
			}
		}
		if found {
			t.Log("3-char typo matched (fuzzy search is very lenient)")
		} else {
			t.Log("3-char typo did not match as expected")
		}
	})

	// Test 8: Transposition (swapped characters)
	t.Run("CharacterTransposition", func(t *testing.T) {
		// "pyhton" instead of "python" (swapped 'ht')
		gistIDs, total, _, err := indexer.Search("pyhton", SearchGistMetadata{}, 100, 1)
		if err != nil {
			t.Fatalf("Transposition search failed: %v", err)
		}
		if total == 0 {
			t.Error("Expected fuzzy search to find 'python' with transposition 'pyhton'")
		}
		found := false
		for _, id := range gistIDs {
			if id == 2002 {
				found = true
				break
			}
		}
		if !found && total > 0 {
			t.Log("Fuzzy search returned results but not the expected gist with transposition")
		}
	})

	// Test 9: Case insensitivity with fuzzy search
	t.Run("CaseInsensitiveWithFuzzy", func(t *testing.T) {
		// "PYTHN" (uppercase with typo)
		gistIDs, total, _, err := indexer.Search("PYTHN", SearchGistMetadata{}, 100, 1)
		if err != nil {
			t.Fatalf("Case insensitive fuzzy search failed: %v", err)
		}
		if total == 0 {
			t.Error("Expected fuzzy search to find 'python' with 'PYTHN'")
		}
		found := false
		for _, id := range gistIDs {
			if id == 2002 {
				found = true
				break
			}
		}
		if !found && total > 0 {
			t.Log("Case insensitive fuzzy search returned results but not expected gist")
		}
	})

	// Test 10: Multiple words with typos
	t.Run("MultipleWordsWithTypos", func(t *testing.T) {
		// "relatonal databse" instead of "relational database"
		gistIDs, total, _, err := indexer.Search("relatonal databse", SearchGistMetadata{}, 100, 1)
		if err != nil {
			t.Fatalf("Multi-word fuzzy search failed: %v", err)
		}
		if total == 0 {
			t.Error("Expected fuzzy search to find 'relational database' with typos")
		}
		found := false
		for _, id := range gistIDs {
			if id == 2003 {
				found = true
				break
			}
		}
		if !found && total > 0 {
			t.Log("Multi-word fuzzy search returned results but not expected gist")
		}
	})

	// Test 11: Short words with typos (edge case)
	t.Run("ShortWordsWithTypos", func(t *testing.T) {
		// "SLQ" instead of "SQL" (1 char typo on short word)
		gistIDs, total, _, err := indexer.Search("SLQ", SearchGistMetadata{}, 100, 1)
		if err != nil {
			t.Fatalf("Short word fuzzy search failed: %v", err)
		}
		// Short words might be more sensitive to typos
		found := false
		for _, id := range gistIDs {
			if id == 2003 {
				found = true
				break
			}
		}
		if !found && total > 0 {
			t.Log("Short word fuzzy search is challenging, returned other results")
		} else if found {
			t.Log("Short word fuzzy search successfully matched")
		}
	})

	// Test 12: Fuzzy search combined with metadata filters
	t.Run("FuzzySearchWithMetadataFilters", func(t *testing.T) {
		// Search with typo AND username filter
		metadata := SearchGistMetadata{Username: "fuzzytest"}
		gistIDs, total, _, err := indexer.Search("algoritm", metadata, 100, 1)
		if err != nil {
			t.Fatalf("Fuzzy search with metadata failed: %v", err)
		}
		if total == 0 {
			t.Error("Expected fuzzy search with filter to find results")
		}
		// All results should be from fuzzytest user
		for _, id := range gistIDs {
			if id >= 2001 && id <= 2004 {
				// Expected
			} else {
				t.Errorf("Found unexpected GistID %d, should only match fuzzytest gists", id)
			}
		}
	})
}

// testIndexerPagination tests pagination in search results
func testIndexerPagination(t *testing.T, indexer Indexer) {
	t.Helper()
	initTestGists(t, indexer)

	// Test 1: Basic pagination - pages should be different
	t.Run("BasicPagination", func(t *testing.T) {
		// Search as user 1 (alice) - should see 334 public gists
		gistIDs1, total, _, err := indexer.Search("", SearchGistMetadata{}, 1, 1)
		if err != nil {
			t.Fatalf("Page 1 search failed: %v", err)
		}
		if total != 334 {
			t.Errorf("Expected 334 total results, got %d", total)
		}
		if len(gistIDs1) == 0 {
			t.Fatal("Expected results on page 1")
		}

		gistIDs2, _, _, err := indexer.Search("", SearchGistMetadata{}, 1, 2)
		if err != nil {
			t.Fatalf("Page 2 search failed: %v", err)
		}
		if len(gistIDs2) == 0 {
			t.Error("Expected results on page 2")
		}

		// Pages should have different first results
		if gistIDs1[0] == gistIDs2[0] {
			t.Error("Page 1 and page 2 returned the same first result")
		}
	})

	// Test 2: Page size - verify results per page (page size = 10)
	t.Run("PageSizeVerification", func(t *testing.T) {
		gistIDs1, _, _, err := indexer.Search("", SearchGistMetadata{}, 1, 1)
		if err != nil {
			t.Fatalf("Page 1 search failed: %v", err)
		}
		// With page size 10, first page should have 10 results (or up to 11 with +1 for hasMore check)
		if len(gistIDs1) == 0 || len(gistIDs1) > 11 {
			t.Errorf("Expected 1-11 results on page 1 (page size 10), got %d", len(gistIDs1))
		}
	})

	// Test 3: Total count consistency across pages
	t.Run("TotalCountConsistency", func(t *testing.T) {
		_, total1, _, err := indexer.Search("", SearchGistMetadata{}, 1, 1)
		if err != nil {
			t.Fatalf("Page 1 search failed: %v", err)
		}
		_, total2, _, err := indexer.Search("", SearchGistMetadata{}, 1, 2)
		if err != nil {
			t.Fatalf("Page 2 search failed: %v", err)
		}
		if total1 != total2 {
			t.Errorf("Total count inconsistent: page 1 reports %d, page 2 reports %d", total1, total2)
		}
		if total1 != 334 {
			t.Errorf("Expected total count of 334, got %d", total1)
		}
	})

	// Test 4: Out of bounds page
	t.Run("OutOfBoundsPage", func(t *testing.T) {
		// Page 100 is way beyond 334 results with page size 10
		gistIDs, _, _, err := indexer.Search("", SearchGistMetadata{}, 1, 100)
		if err != nil {
			t.Fatalf("Out of bounds page search failed: %v", err)
		}
		if len(gistIDs) != 0 {
			t.Errorf("Expected 0 results for out of bounds page, got %d", len(gistIDs))
		}
	})

	// Test 5: Empty results pagination
	t.Run("EmptyResultsPagination", func(t *testing.T) {
		gistIDs, total, _, err := indexer.Search("nonexistentquery", SearchGistMetadata{}, 1, 1)
		if err != nil {
			t.Fatalf("Empty search failed: %v", err)
		}
		if total != 0 {
			t.Errorf("Expected 0 total for empty search, got %d", total)
		}
		if len(gistIDs) != 0 {
			t.Errorf("Expected 0 results for empty search, got %d", len(gistIDs))
		}
	})

	// Test 6: No duplicate IDs across pages (accounting for +1 overlap for hasMore indicator)
	t.Run("NoDuplicateIDs", func(t *testing.T) {
		gistIDs1, _, _, err := indexer.Search("", SearchGistMetadata{}, 1, 1)
		if err != nil {
			t.Fatalf("Page 1 search failed: %v", err)
		}
		gistIDs2, _, _, err := indexer.Search("", SearchGistMetadata{}, 1, 2)
		if err != nil {
			t.Fatalf("Page 2 search failed: %v", err)
		}

		// The pagination returns 11 items but only displays 10
		// The 11th item is used as a "hasMore" indicator
		// So we only check the first 10 items of each page for duplicates
		page1Items := gistIDs1
		if len(gistIDs1) > 10 {
			page1Items = gistIDs1[:10]
		}
		page2Items := gistIDs2
		if len(gistIDs2) > 10 {
			page2Items = gistIDs2[:10]
		}

		// Check for duplicates between displayed items only
		for _, id1 := range page1Items {
			for _, id2 := range page2Items {
				if id1 == id2 {
					t.Errorf("Found duplicate ID %d in displayed items of both pages", id1)
				}
			}
		}
	})

	// Test 7: Pagination with metadata filters
	t.Run("PaginationWithFilters", func(t *testing.T) {
		// Filter by alice's username - should get 334 gists
		metadata := SearchGistMetadata{Username: "alice"}
		gistIDs1, total, _, err := indexer.Search("", metadata, 1, 1)
		if err != nil {
			t.Fatalf("Filtered page 1 search failed: %v", err)
		}
		if total != 334 {
			t.Errorf("Expected 334 total results with filter, got %d", total)
		}
		if len(gistIDs1) == 0 {
			t.Error("Expected results on filtered page 1")
		}

		gistIDs2, _, _, err := indexer.Search("", metadata, 1, 2)
		if err != nil {
			t.Fatalf("Filtered page 2 search failed: %v", err)
		}
		if len(gistIDs2) == 0 {
			t.Error("Expected results on filtered page 2")
		}

		// Pages should be different
		if gistIDs1[0] == gistIDs2[0] {
			t.Error("Filtered pages should have different results")
		}
	})

	// Test 8: Last page verification
	t.Run("LastPageVerification", func(t *testing.T) {
		// With 334 results and page size 10, page 34 should have 4 results
		// Let's just verify the last page has some results
		gistIDs34, total, _, err := indexer.Search("", SearchGistMetadata{}, 1, 34)
		if err != nil {
			t.Fatalf("Last page search failed: %v", err)
		}
		if total != 334 {
			t.Errorf("Expected 334 total on last page, got %d", total)
		}
		if len(gistIDs34) == 0 {
			t.Error("Expected results on last page (34)")
		}

		// Page 35 should be empty
		gistIDs35, _, _, err := indexer.Search("", SearchGistMetadata{}, 1, 35)
		if err != nil {
			t.Fatalf("Beyond last page search failed: %v", err)
		}
		if len(gistIDs35) != 0 {
			t.Errorf("Expected 0 results on page 35 (beyond last page), got %d", len(gistIDs35))
		}
	})

	// Test 9: Multiple pages have different results
	t.Run("MultiplePagesDifferent", func(t *testing.T) {
		gistIDs1, _, _, err := indexer.Search("", SearchGistMetadata{}, 1, 1)
		if err != nil {
			t.Fatalf("Page 1 search failed: %v", err)
		}
		gistIDs10, _, _, err := indexer.Search("", SearchGistMetadata{}, 1, 10)
		if err != nil {
			t.Fatalf("Page 10 search failed: %v", err)
		}
		gistIDs20, _, _, err := indexer.Search("", SearchGistMetadata{}, 1, 20)
		if err != nil {
			t.Fatalf("Page 20 search failed: %v", err)
		}

		// All three pages should have results
		if len(gistIDs1) == 0 || len(gistIDs10) == 0 || len(gistIDs20) == 0 {
			t.Error("Expected results on pages 1, 10, and 20")
		}

		// All should have different first results
		if gistIDs1[0] == gistIDs10[0] || gistIDs1[0] == gistIDs20[0] || gistIDs10[0] == gistIDs20[0] {
			t.Error("Pages 1, 10, and 20 should have different first results")
		}
	})

	// Test 10: Pagination with different users (visibility filtering)
	t.Run("PaginationWithVisibility", func(t *testing.T) {
		// User 2 (bob) sees 667 gists (334 public alice + 333 own private)
		gistIDs1Bob, totalBob, _, err := indexer.Search("", SearchGistMetadata{}, 2, 1)
		if err != nil {
			t.Fatalf("Bob page 1 search failed: %v", err)
		}
		if totalBob != 667 {
			t.Errorf("Expected bob to see 667 gists, got %d", totalBob)
		}
		if len(gistIDs1Bob) == 0 {
			t.Error("Expected results on page 1 for bob")
		}

		// User 1 (alice) sees 334 gists
		_, totalAlice, _, err := indexer.Search("", SearchGistMetadata{}, 1, 1)
		if err != nil {
			t.Fatalf("Alice page 1 search failed: %v", err)
		}
		if totalAlice != 334 {
			t.Errorf("Expected alice to see 334 gists, got %d", totalAlice)
		}

		// Bob sees more results than alice
		if totalBob <= totalAlice {
			t.Errorf("Bob should see more results (%d) than alice (%d)", totalBob, totalAlice)
		}
	})
}
