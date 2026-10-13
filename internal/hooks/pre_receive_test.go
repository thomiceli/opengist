package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSymlinkFiles(t *testing.T) {
	output := []byte("100644 blob aaaaaaa\tregular.md\x00" +
		"120000 blob bbbbbbb\ttask.md\x00" +
		"120000 blob ccccccc\tfilename with spaces\x00")

	require.Equal(t, []string{"task.md", "filename with spaces"}, parseSymlinkFiles(output))
}

func TestGetSymlinkFilesRecursive(t *testing.T) {
	repositoryPath := t.TempDir()
	t.Chdir(repositoryPath)

	require.NoError(t, exec.Command("git", "init", "--quiet").Run())
	require.NoError(t, os.WriteFile("regular.md", []byte("content"), 0644))
	require.NoError(t, os.Mkdir("nested", 0755))
	if err := os.Symlink("../regular.md", filepath.Join("nested", "link.md")); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}
	require.NoError(t, exec.Command("git", "add", "--all").Run())
	require.NoError(t, exec.Command(
		"git", "-c", "user.name=Opengist", "-c", "user.email=opengist@example.com",
		"commit", "--quiet", "-m", "test",
	).Run())

	files, err := getSymlinkFiles("HEAD")
	require.NoError(t, err)
	require.Equal(t, []string{"nested/link.md"}, files)
}
