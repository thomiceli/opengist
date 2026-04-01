package index

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/meilisearch/meilisearch-go"
	"github.com/rs/zerolog"
)

// syncMeiliIndexer wraps MeiliIndexer to make Add/Remove synchronous for tests.
type syncMeiliIndexer struct {
	*MeiliIndexer
}

func (s *syncMeiliIndexer) Add(gist *Gist) error {
	if gist == nil {
		return fmt.Errorf("failed to add nil gist to index")
	}
	doc := &meiliGist{
		Gist:         *gist,
		ContentSplit: splitCamelCase(gist.Content),
	}
	primaryKey := "GistID"
	taskInfo, err := s.index.AddDocuments(doc, &meilisearch.DocumentOptions{PrimaryKey: &primaryKey})
	if err != nil {
		return err
	}
	_, err = s.client.WaitForTask(taskInfo.TaskUID, 0)
	return err
}

func (s *syncMeiliIndexer) Remove(gistID uint) error {
	taskInfo, err := s.index.DeleteDocument(strconv.Itoa(int(gistID)), nil)
	if err != nil {
		return err
	}
	_, err = s.client.WaitForTask(taskInfo.TaskUID, 0)
	return err
}

func setupMeiliIndexer(t *testing.T) (Indexer, func()) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	t.Helper()

	host := os.Getenv("OG_TEST_MEILI_HOST")
	if host == "" {
		host = "http://localhost:47700"
	}
	apiKey := os.Getenv("OG_TEST_MEILI_API_KEY")

	indexName := fmt.Sprintf("test_%d", os.Getpid())

	inner := NewMeiliIndexer(host, apiKey, indexName)
	err := inner.Init()
	if err != nil {
		t.Skipf("MeiliSearch not available at %s: %v", host, err)
	}

	wrapped := &syncMeiliIndexer{MeiliIndexer: inner}

	// Store the inner MeiliIndexer in atomicIndexer, because MeiliIndexer.Search
	// type-asserts the global to *MeiliIndexer.
	var idx Indexer = inner
	atomicIndexer.Store(&idx)

	cleanup := func() {
		atomicIndexer.Store(nil)
		inner.Reset()
		inner.Close()
	}

	return wrapped, cleanup
}

func TestMeiliAddAndSearch(t *testing.T)        { testAddAndSearch(t, setupMeiliIndexer) }
func TestMeiliAccessControl(t *testing.T)       { testAccessControl(t, setupMeiliIndexer) }
func TestMeiliMetadataFilters(t *testing.T)     { testMetadataFilters(t, setupMeiliIndexer) }
func TestMeiliAllFieldSearch(t *testing.T)      { testAllFieldSearch(t, setupMeiliIndexer) }
func TestMeiliFuzzySearch(t *testing.T)         { testFuzzySearch(t, setupMeiliIndexer) }
func TestMeiliContentSearch(t *testing.T)       { testContentSearch(t, setupMeiliIndexer) }
func TestMeiliPagination(t *testing.T)          { testPagination(t, setupMeiliIndexer) }
func TestMeiliLanguageFacets(t *testing.T)      { testLanguageFacets(t, setupMeiliIndexer) }
func TestMeiliMetadataOnlySearch(t *testing.T)  { testMetadataOnlySearch(t, setupMeiliIndexer) }
func TestMeiliTitleFuzzySearch(t *testing.T)    { testTitleFuzzySearch(t, setupMeiliIndexer) }
func TestMeiliMultiLanguageFacets(t *testing.T) { testMultiLanguageFacets(t, setupMeiliIndexer) }
