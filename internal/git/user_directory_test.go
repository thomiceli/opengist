package git

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/config"
)

// setupUserDirectoryTest uses a temporary home directory, holding the
// repositories directory and another directory next to it.
func setupUserDirectoryTest(t *testing.T) (reposDir string, otherDir string) {
	t.Setenv("OPENGIST_SKIP_GIT_HOOKS", "1")
	require.NoError(t, config.InitConfig("", io.Discard))
	config.C.OpengistHome = t.TempDir()

	reposDir = filepath.Join(config.GetHomeDir(), ReposDirectory)
	otherDir = filepath.Join(config.GetHomeDir(), "other")
	require.NoError(t, os.MkdirAll(otherDir, 0755))
	require.NoError(t, InitRepository("thomas", "gist1"))
	return reposDir, otherDir
}

func TestInvalidUserDirectory(t *testing.T) {
	reposDir, otherDir := setupUserDirectoryTest(t)

	for _, user := range []string{"", ".", "..", "../other", "thomas/../../other", `..\other`, "thomas/gist1"} {
		t.Run(user, func(t *testing.T) {
			require.Error(t, InitRepository(user, "gist2"))
			require.Error(t, ForkClone("thomas", "gist1", user, "gist2"))
			require.Error(t, DeleteRepository(user, "gist1"))
			require.Error(t, DeleteUserDirectory(user))
			require.Error(t, RenameUserDirectory(user, "alice"))
			require.Error(t, RenameUserDirectory("thomas", user))
		})
	}

	require.DirExists(t, RepositoryPath("thomas", "gist1"))
	entries, err := os.ReadDir(otherDir)
	require.NoError(t, err)
	require.Empty(t, entries)
	entries, err = os.ReadDir(reposDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestUserDirectoryWithDots(t *testing.T) {
	reposDir, _ := setupUserDirectoryTest(t)

	require.NoError(t, InitRepository("first.last", "gist1"))
	require.NoError(t, ForkClone("thomas", "gist1", "first.last", "gist2"))
	require.DirExists(t, filepath.Join(reposDir, "first.last", "gist2"))
	require.NoError(t, DeleteRepository("first.last", "gist2"))

	require.NoError(t, RenameUserDirectory("first.last", "first_last"))
	require.NoDirExists(t, filepath.Join(reposDir, "first.last"))
	require.DirExists(t, filepath.Join(reposDir, "first_last", "gist1"))

	require.NoError(t, DeleteUserDirectory("first_last"))
	require.NoDirExists(t, filepath.Join(reposDir, "first_last"))
	require.DirExists(t, RepositoryPath("thomas", "gist1"))
}

func TestUserDirectoryUppercase(t *testing.T) {
	reposDir, _ := setupUserDirectoryTest(t)

	// Repositories are stored in the lowercased user directory
	require.NoError(t, InitRepository("Alice", "gist1"))
	require.DirExists(t, filepath.Join(reposDir, "alice", "gist1"))

	require.NoError(t, RenameUserDirectory("Alice", "alice"))
	require.DirExists(t, filepath.Join(reposDir, "alice", "gist1"))

	require.NoError(t, RenameUserDirectory("alice", "Bob"))
	require.NoDirExists(t, filepath.Join(reposDir, "alice"))
	require.DirExists(t, filepath.Join(reposDir, "bob", "gist1"))

	require.NoError(t, DeleteUserDirectory("Bob"))
	require.NoDirExists(t, filepath.Join(reposDir, "bob"))
	require.DirExists(t, RepositoryPath("thomas", "gist1"))
}
