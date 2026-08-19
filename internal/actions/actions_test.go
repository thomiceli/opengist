package actions

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thomiceli/opengist/internal/config"
)

func setupTestConfig(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	if err := config.InitConfig("", io.Discard); err != nil {
		t.Fatal(err)
	}
	config.C.OpengistHome = tmp
	config.C.UploadOrphanTTL = "1h"
	return tmp
}

func TestDeleteOrphanedUploadFiles(t *testing.T) {
	tmp := setupTestConfig(t)

	uploads := filepath.Join(tmp, "uploads")
	if err := os.MkdirAll(uploads, 0755); err != nil {
		t.Fatal(err)
	}

	old := filepath.Join(uploads, "old-uuid")
	fresh := filepath.Join(uploads, "fresh-uuid")
	for _, p := range []string{old, fresh} {
		if err := os.WriteFile(p, []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	twoHoursAgo := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(old, twoHoursAgo, twoHoursAgo); err != nil {
		t.Fatal(err)
	}

	deleteOrphanedUploadFiles()

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("expected old upload to be deleted, got err=%v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("expected fresh upload to be kept, got err=%v", err)
	}
}

func TestDeleteOrphanedUploadFiles_MissingDir(t *testing.T) {
	setupTestConfig(t)
	deleteOrphanedUploadFiles()
}
