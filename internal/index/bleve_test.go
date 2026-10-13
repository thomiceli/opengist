package index

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

// setupBleveIndexer creates a new BleveIndexer for testing
func setupBleveIndexer(t *testing.T) (Indexer, func()) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "bleve-test-*")
	require.NoError(t, err)

	indexPath := filepath.Join(tmpDir, "test.index")
	indexer := NewBleveIndexer(indexPath)

	err = indexer.Init()
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to initialize BleveIndexer: %v", err)
	}

	var idx Indexer = indexer
	atomicIndexer.Store(&idx)

	cleanup := func() {
		atomicIndexer.Store(nil)
		indexer.Close()
		os.RemoveAll(tmpDir)
	}

	return indexer, cleanup
}

func TestBleveAddAndSearch(t *testing.T)        { testAddAndSearch(t, setupBleveIndexer) }
func TestBleveAccessControl(t *testing.T)       { testAccessControl(t, setupBleveIndexer) }
func TestBleveMetadataFilters(t *testing.T)     { testMetadataFilters(t, setupBleveIndexer) }
func TestBleveAllFieldSearch(t *testing.T)      { testAllFieldSearch(t, setupBleveIndexer) }
func TestBleveFuzzySearch(t *testing.T)         { testFuzzySearch(t, setupBleveIndexer) }
func TestBleveContentSearch(t *testing.T)       { testContentSearch(t, setupBleveIndexer) }
func TestBlevePagination(t *testing.T)          { testPagination(t, setupBleveIndexer) }
func TestBleveLanguageFacets(t *testing.T)      { testLanguageFacets(t, setupBleveIndexer) }
func TestBleveWildcardSearch(t *testing.T)      { testWildcardSearch(t, setupBleveIndexer) }
func TestBleveMetadataOnlySearch(t *testing.T)  { testMetadataOnlySearch(t, setupBleveIndexer) }
func TestBleveTitleFuzzySearch(t *testing.T)    { testTitleFuzzySearch(t, setupBleveIndexer) }
func TestBleveMultiLanguageFacets(t *testing.T) { testMultiLanguageFacets(t, setupBleveIndexer) }

func TestBlevePersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bleve-persist-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	indexPath := filepath.Join(tmpDir, "test.index")

	// Create and populate index
	indexer1 := NewBleveIndexer(indexPath)
	require.NoError(t, indexer1.Init())

	var idx Indexer = indexer1
	atomicIndexer.Store(&idx)

	g := newGist(1, 1, 0, "persistent data survives restart")
	require.NoError(t, indexer1.Add(g))

	indexer1.Close()
	atomicIndexer.Store(nil)

	// Reopen at same path
	indexer2 := NewBleveIndexer(indexPath)
	require.NoError(t, indexer2.Init())
	defer indexer2.Close()

	idx = indexer2
	atomicIndexer.Store(&idx)
	defer atomicIndexer.Store(nil)

	ids, total, _, err := indexer2.Search(SearchGistMetadata{Content: "persistent"}, 1, 1)
	require.NoError(t, err)
	require.Equal(t, uint64(1), total, "data should survive close+reopen")
	require.Equal(t, uint(1), ids[0])
}
