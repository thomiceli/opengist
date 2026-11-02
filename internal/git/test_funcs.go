package git

import (
	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/config"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

func SetupTest(t *testing.T) {
	_ = os.Setenv("OPENGIST_SKIP_GIT_HOOKS", "1")

	err := config.InitConfig("")
	require.NoError(t, err, "Could not init config")

	err = os.MkdirAll(path.Join(config.GetHomeDir(), "tests"), 0755)
	ReposDirectory = path.Join("tests")
	require.NoError(t, err)

	err = os.MkdirAll(filepath.Join(config.GetHomeDir(), "tmp", "repos"), 0755)
	require.NoError(t, err)

	err = InitRepository("thomas", "gist1")
	require.NoError(t, err)
}

func TeardownTest(t *testing.T) {
	err := os.RemoveAll(path.Join(config.GetHomeDir(), "tests"))
	require.NoError(t, err, "Could not remove repos directory")
}

func CommitToBare(t *testing.T, user string, gist string, files map[string]string) {
	gistTmpId, err := CloneTmp(user, gist, "thomas@mail.com", true)
	require.NoError(t, err, "Could not clone repository")
	defer func() {
		require.NoError(t, DeleteTmpRepository(gistTmpId))
	}()

	if len(files) > 0 {
		for filename, content := range files {
			if strings.Contains(filename, "/") {
				dir := filepath.Dir(filename)
				err := os.MkdirAll(filepath.Join(TmpRepositoryPath(gistTmpId), dir), os.ModePerm)
				require.NoError(t, err, "Could not create directory")
			}
			_ = os.WriteFile(filepath.Join(TmpRepositoryPath(gistTmpId), filename), []byte(content), 0644)

			if err := AddAll(gistTmpId); err != nil {
				require.NoError(t, err, "Could not add all to repository")
			}
		}
	}

	if err := CommitRepository(gistTmpId, user, "thomas@mail.com"); err != nil {
		require.NoError(t, err, "Could not commit to repository")
	}

	if err := Push(gistTmpId); err != nil {
		require.NoError(t, err, "Could not push to repository")
	}
}

func LastHashOfCommit(t *testing.T, user string, gist string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = RepositoryPath(user, gist)
	out, err := cmd.Output()
	require.NoError(t, err, "Could not run git command")
	return strings.TrimSpace(string(out))
}
