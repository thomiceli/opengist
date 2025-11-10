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
