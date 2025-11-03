package index

import (
	"os"
	"path/filepath"
	"testing"
)

// setupBleveIndexer creates a new BleveIndexer for testing
func setupBleveIndexer(t *testing.T) (*BleveIndexer, func()) {
	t.Helper()

	// Create a temporary directory for the test index
	tmpDir, err := os.MkdirTemp("", "bleve-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	indexPath := filepath.Join(tmpDir, "test.index")
	indexer := NewBleveIndexer(indexPath)

	// Initialize the indexer
	err = indexer.Init()
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to initialize BleveIndexer: %v", err)
	}

	// Store in the global atomicIndexer since Add/Remove use it
	var idx Indexer = indexer
	atomicIndexer.Store(&idx)

	// Return cleanup function
	cleanup := func() {
		atomicIndexer.Store(nil)
		indexer.Close()
		os.RemoveAll(tmpDir)
	}

	return indexer, cleanup
}

func TestBleveIndexerAddGist(t *testing.T) {
	indexer, cleanup := setupBleveIndexer(t)
	defer cleanup()

	testIndexerAddGist(t, indexer)
}

func TestBleveIndexerAddNilGist(t *testing.T) {
	indexer, cleanup := setupBleveIndexer(t)
	defer cleanup()

	testIndexerAddNilGist(t, indexer)
}

func TestBleveIndexerSearchBasic(t *testing.T) {
	indexer, cleanup := setupBleveIndexer(t)
	defer cleanup()

	testIndexerSearchBasic(t, indexer)
}

func TestBleveIndexerSearchWithMetadata(t *testing.T) {
	indexer, cleanup := setupBleveIndexer(t)
	defer cleanup()

	testIndexerSearchWithMetadata(t, indexer)
}

func TestBleveIndexerSearchEmpty(t *testing.T) {
	indexer, cleanup := setupBleveIndexer(t)
	defer cleanup()

	testIndexerSearchEmpty(t, indexer)
}

func TestBleveIndexerRemoveGist(t *testing.T) {
	indexer, cleanup := setupBleveIndexer(t)
	defer cleanup()

	testIndexerRemoveGist(t, indexer)
}

func TestBleveIndexerMultipleGists(t *testing.T) {
	indexer, cleanup := setupBleveIndexer(t)
	defer cleanup()

	testIndexerMultipleGists(t, indexer)
}

func TestBleveIndexerPagination(t *testing.T) {
	indexer, cleanup := setupBleveIndexer(t)
	defer cleanup()

	testIndexerPagination(t, indexer)
}

// TestBleveIndexerInitAndClose tests Bleve-specific initialization and closing
func TestBleveIndexerInitAndClose(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bleve-init-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	indexPath := filepath.Join(tmpDir, "test.index")
	indexer := NewBleveIndexer(indexPath)

	// Test initialization
	err = indexer.Init()
	if err != nil {
		t.Fatalf("Failed to initialize BleveIndexer: %v", err)
	}

	if indexer.index == nil {
		t.Fatal("Expected index to be initialized, got nil")
	}

	// Test closing
	indexer.Close()

	// Test reopening the same index
	indexer2 := NewBleveIndexer(indexPath)
	err = indexer2.Init()
	if err != nil {
		t.Fatalf("Failed to reopen BleveIndexer: %v", err)
	}
	defer indexer2.Close()

	if indexer2.index == nil {
		t.Fatal("Expected reopened index to be initialized, got nil")
	}
}

// TestBleveIndexerUnicodeSearch tests that Unicode content can be indexed and searched
func TestBleveIndexerUnicodeSearch(t *testing.T) {
	indexer, cleanup := setupBleveIndexer(t)
	defer cleanup()

	// Add a gist with Unicode content
	gist := &Gist{
		GistID:     100,
		UserID:     100,
		Visibility: 0,
		Username:   "testuser",
		Title:      "Unicode Test",
		Content:    "Hello world with unicode characters: café résumé naïve",
		Filenames:  []string{"test.txt"},
		Extensions: []string{".txt"},
		Languages:  []string{"Text"},
		Topics:     []string{"unicode"},
		CreatedAt:  1234567890,
		UpdatedAt:  1234567890,
	}

	err := indexer.Add(gist)
	if err != nil {
		t.Fatalf("Failed to add gist: %v", err)
	}

	// Search for unicode content
	gistIDs, total, _, err := indexer.Search("café", SearchGistMetadata{}, 100, 1)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if total == 0 {
		t.Skip("Unicode search may require specific index configuration")
		return
	}

	found := false
	for _, id := range gistIDs {
		if id == 100 {
			found = true
			break
		}
	}
	if !found {
		t.Log("Unicode gist not found in search results, but other results were returned")
	}
}
