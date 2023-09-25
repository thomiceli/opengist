package git

import (
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/config"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

func setup(t *testing.T) {
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

func teardown(t *testing.T) {
	err := os.RemoveAll(path.Join(config.C.OpengistHome, "tests"))
	require.NoError(t, err, "Could not remove repos directory")
}

func TestInitDeleteRepository(t *testing.T) {
	setup(t)
	defer teardown(t)

	cmd := exec.Command("git", "rev-parse", "--is-bare-repository")
	cmd.Dir = RepositoryPath("thomas", "gist1")
	out, err := cmd.Output()
	require.NoError(t, err, "Could not run git command")
	require.Equal(t, "true", strings.TrimSpace(string(out)), "Repository is not bare")

	_, err = os.Stat(path.Join(RepositoryPath("thomas", "gist1"), "hooks", "pre-receive"))
	require.NoError(t, err, "pre-receive hook not found")

	_, err = os.Stat(path.Join(RepositoryPath("thomas", "gist1"), "git-daemon-export-ok"))
	require.NoError(t, err, "git-daemon-export-ok file not found")

	err = DeleteRepository("thomas", "gist1")
	require.NoError(t, err, "Could not delete repository")
	require.NoDirExists(t, RepositoryPath("thomas", "gist1"), "Repository should not exist")
}

func TestCommits(t *testing.T) {
	setup(t)
	defer teardown(t)

	hasNoCommits, err := HasNoCommits("thomas", "gist1")
	require.NoError(t, err, "Could not check if repository has no commits")
	require.True(t, hasNoCommits, "Repository should have no commits")

	commitToBare(t, "thomas", "gist1", nil)

	hasNoCommits, err = HasNoCommits("thomas", "gist1")
	require.NoError(t, err, "Could not check if repository has no commits")
	require.False(t, hasNoCommits, "Repository should have commits")

	nbCommits, err := CountCommits("thomas", "gist1")
	require.NoError(t, err, "Could not count commits")
	require.Equal(t, "1", nbCommits, "Repository should have 1 commit")

	commitToBare(t, "thomas", "gist1", nil)
	nbCommits, err = CountCommits("thomas", "gist1")
	require.NoError(t, err, "Could not count commits")
	require.Equal(t, "2", nbCommits, "Repository should have 2 commits")
}

func TestContent(t *testing.T) {
	setup(t)
	defer teardown(t)

	commitToBare(t, "thomas", "gist1", map[string]string{
		"my_file.txt": "I love Opengist\n",
		"my_other_file.txt": `I really
hate Opengist`,
		"rip.txt": "byebye",
	})

	files, err := GetFilesOfRepository("thomas", "gist1", "HEAD")
	require.NoError(t, err, "Could not get files of repository")
	require.Subset(t, []string{"my_file.txt", "my_other_file.txt", "rip.txt"}, files, "Files are not correct")

	content, truncated, err := GetFileContent("thomas", "gist1", "HEAD", "my_file.txt", false)
	require.NoError(t, err, "Could not get content")
	require.False(t, truncated, "Content should not be truncated")
	require.Equal(t, "I love Opengist\n", content, "Content is not correct")

	content, truncated, err = GetFileContent("thomas", "gist1", "HEAD", "my_other_file.txt", false)
	require.NoError(t, err, "Could not get content")
	require.False(t, truncated, "Content should not be truncated")
	require.Equal(t, "I really\nhate Opengist", content, "Content is not correct")

	commitToBare(t, "thomas", "gist1", map[string]string{
		"my_renamed_file.txt": "I love Opengist\n",
		"my_other_file.txt": `I really
like Opengist actually`,
		"new_file.txt": "Wait now there is a new file",
	})

	files, err = GetFilesOfRepository("thomas", "gist1", "HEAD")
	require.NoError(t, err, "Could not get files of repository")
	require.Subset(t, []string{"my_renamed_file.txt", "my_other_file.txt", "new_file.txt"}, files, "Files are not correct")

	content, truncated, err = GetFileContent("thomas", "gist1", "HEAD", "my_other_file.txt", false)
	require.NoError(t, err, "Could not get content")
	require.False(t, truncated, "Content should not be truncated")
	require.Equal(t, "I really\nlike Opengist actually", content, "Content is not correct")

	commits, err := GetLog("thomas", "gist1", 0)
	require.NoError(t, err, "Could not get log")
	require.Equal(t, 2, len(commits), "Commits count are not correct")
	require.Regexp(t, "[a-f0-9]{40}", commits[0].Hash, "Commit ID is not correct")
	require.Regexp(t, "[0-9]{10}", commits[0].Timestamp, "Commit timestamp is not correct")
	require.Equal(t, "thomas", commits[0].AuthorName, "Commit author name is not correct")
	require.Equal(t, "thomas@mail.com", commits[0].AuthorEmail, "Commit author email is not correct")
	require.Equal(t, "4 files changed, 2 insertions, 2 deletions", commits[0].Changed, "Commit author name is not correct")

	require.Contains(t, commits[0].Files, File{
		Filename:    "my_renamed_file.txt",
		OldFilename: "my_file.txt",
		Content:     "",
		Truncated:   false,
		IsCreated:   false,
		IsDeleted:   false,
	}, "File my_renamed_file.txt is not correct")

	require.Contains(t, commits[0].Files, File{
		Filename:    "rip.txt",
		OldFilename: "",
		Content: `@@ -1 +0,0 @@
-byebye
\ No newline at end of file
`,
		Truncated: false,
		IsCreated: false,
		IsDeleted: true,
	}, "File rip.txt is not correct")

	require.Contains(t, commits[0].Files, File{
		Filename:    "my_other_file.txt",
		OldFilename: "",
		Content: `@@ -1,2 +1,2 @@
 I really
-hate Opengist
\ No newline at end of file
+like Opengist actually
\ No newline at end of file
`,
		Truncated: false,
		IsCreated: false,
		IsDeleted: false,
	}, "File my_other_file.txt is not correct")

	require.Contains(t, commits[0].Files, File{
		Filename:    "new_file.txt",
		OldFilename: "",
		Content: `@@ -0,0 +1 @@
+Wait now there is a new file
\ No newline at end of file
`,
		Truncated: false,
		IsCreated: true,
		IsDeleted: false,
	}, "File new_file.txt is not correct")

	commitsSkip1, err := GetLog("thomas", "gist1", 1)
	require.NoError(t, err, "Could not get log")
	require.Equal(t, commitsSkip1[0], commits[1], "Commits skips are not correct")
}

func TestGitGc(t *testing.T) {
	setup(t)
	defer teardown(t)

	err := GcRepos()
	require.NoError(t, err, "Could not run git gc")
}

func TestFork(t *testing.T) {
	setup(t)
	defer teardown(t)

	commitToBare(t, "thomas", "gist1", map[string]string{
		"my_file.txt": "I love Opengist\n",
	})

	err := ForkClone("thomas", "gist1", "thomas", "gist2")
	require.NoError(t, err, "Could not fork repository")

	files1, err := GetFilesOfRepository("thomas", "gist1", "HEAD")
	require.NoError(t, err, "Could not get files of repository")
	files2, err := GetFilesOfRepository("thomas", "gist2", "HEAD")
	require.NoError(t, err, "Could not get files of repository")

	require.Equal(t, files1, files2, "Files are not the same")

}

func TestTruncate(t *testing.T) {
	setup(t)
	defer teardown(t)

	commitToBare(t, "thomas", "gist1", map[string]string{
		"my_file.txt": "A",
	})

	content, truncated, err := GetFileContent("thomas", "gist1", "HEAD", "my_file.txt", true)
	require.NoError(t, err, "Could not get content")
	require.False(t, truncated, "Content should not be truncated")
	require.Equal(t, 1, len(content), "Content size is not correct")

	var builder strings.Builder
	for i := 0; i < truncateLimit+10; i++ {
		builder.WriteString("A")
	}
	str := builder.String()
	commitToBare(t, "thomas", "gist1", map[string]string{
		"my_file.txt": str,
	})

	content, truncated, err = GetFileContent("thomas", "gist1", "HEAD", "my_file.txt", true)
	require.NoError(t, err, "Could not get content")
	require.True(t, truncated, "Content should be truncated")
	require.Equal(t, truncateLimit, len(content), "Content size should be at truncate limit")

	commitToBare(t, "thomas", "gist1", map[string]string{
		"my_file.txt": "AA\n" + str,
	})

	content, truncated, err = GetFileContent("thomas", "gist1", "HEAD", "my_file.txt", true)
	require.NoError(t, err, "Could not get content")
	require.True(t, truncated, "Content should be truncated")
	require.Equal(t, 2, len(content), "Content size is not correct")
}

func TestInitViaGitInit(t *testing.T) {
	setup(t)
	defer teardown(t)

	e := echo.New()

	// Create a mock HTTP request
	req := httptest.NewRequest(http.MethodPost, "/", nil)

	// Create a mock HTTP response recorder
	rec := httptest.NewRecorder()

	// Create a new Echo context
	c := e.NewContext(req, rec)

	// Define your user and gist
	user := "testUser"
	gist := "testGist"

	err := InitRepositoryViaInit(user, gist, c)

	require.NoError(t, err)
}

func commitToBare(t *testing.T, user string, gist string, files map[string]string) {
	err := CloneTmp(user, gist, gist, "thomas@mail.com")
	require.NoError(t, err, "Could not commit to repository")

	if len(files) > 0 {
		for filename, content := range files {
			if err := SetFileContent(gist, filename, content); err != nil {
				require.NoError(t, err, "Could not commit to repository")
			}

			if err := AddAll(gist); err != nil {
				require.NoError(t, err, "Could not commit to repository")
			}
		}

	}

	if err := CommitRepository(gist, user, "thomas@mail.com"); err != nil {
		require.NoError(t, err, "Could not commit to repository")
	}

	if err := Push(gist); err != nil {
		require.NoError(t, err, "Could not commit to repository")
	}
}
