package git

import (
	"bufio"
	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/config"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"
)

func TestInitDeleteRepository(t *testing.T) {
	SetupTest(t)
	defer TeardownTest(t)

	cmd := exec.Command("git", "rev-parse", "--is-bare-repository")
	cmd.Dir = RepositoryPath("thomas", "gist1")
	out, err := cmd.Output()
	require.NoError(t, err, "Could not run git command")
	require.Equal(t, "true", strings.TrimSpace(string(out)), "Repository is not bare")

	_, err = os.Stat(path.Join(RepositoryPath("thomas", "gist1"), "git-daemon-export-ok"))
	require.NoError(t, err, "git-daemon-export-ok file not found")

	err = DeleteRepository("thomas", "gist1")
	require.NoError(t, err, "Could not delete repository")
	require.NoDirExists(t, RepositoryPath("thomas", "gist1"), "Repository should not exist")
}

func TestCommits(t *testing.T) {
	SetupTest(t)
	defer TeardownTest(t)

	hasNoCommits, err := HasNoCommits("thomas", "gist1")
	require.NoError(t, err, "Could not check if repository has no commits")
	require.True(t, hasNoCommits, "Repository should have no commits")

	CommitToBare(t, "thomas", "gist1", nil)

	hasNoCommits, err = HasNoCommits("thomas", "gist1")
	require.NoError(t, err, "Could not check if repository has no commits")
	require.False(t, hasNoCommits, "Repository should have commits")

	nbCommits, err := CountCommits("thomas", "gist1")
	require.NoError(t, err, "Could not count commits")
	require.Equal(t, "1", nbCommits, "Repository should have 1 commit")

	CommitToBare(t, "thomas", "gist1", nil)
	nbCommits, err = CountCommits("thomas", "gist1")
	require.NoError(t, err, "Could not count commits")
	require.Equal(t, "2", nbCommits, "Repository should have 2 commits")
}

func TestContent(t *testing.T) {
	SetupTest(t)
	defer TeardownTest(t)

	CommitToBare(t, "thomas", "gist1", map[string]string{
		"my_file.txt": "I love Opengist\n",
		"my_other_file.txt": `I really
hate Opengist`,
		"rip.txt": "byebye",
		"中文名.txt": "中文内容",
	})

	files, err := GetFilesOfRepository("thomas", "gist1", "HEAD")
	require.NoError(t, err, "Could not get files of repository")
	require.Subset(t, []string{"my_file.txt", "my_other_file.txt", "rip.txt", "中文名.txt"}, files, "Files are not correct")

	content, truncated, err := GetFileContent("thomas", "gist1", "HEAD", "my_file.txt", false)
	require.NoError(t, err, "Could not get content")
	require.False(t, truncated, "Content should not be truncated")
	require.Equal(t, "I love Opengist\n", content, "Content is not correct")

	content, truncated, err = GetFileContent("thomas", "gist1", "HEAD", "my_other_file.txt", false)
	require.NoError(t, err, "Could not get content")
	require.False(t, truncated, "Content should not be truncated")
	require.Equal(t, "I really\nhate Opengist", content, "Content is not correct")

	content, truncated, err = GetFileContent("thomas", "gist1", "HEAD", "中文名.txt", false)
	require.NoError(t, err, "Could not get content")
	require.False(t, truncated, "Content should not be truncated")
	require.Equal(t, "中文内容", content, "Content is not correct")

	CommitToBare(t, "thomas", "gist1", map[string]string{
		"my_renamed_file.txt": "I love Opengist\n",
		"my_other_file.txt": `I really
like Opengist actually`,
		"new_file.txt": "Wait now there is a new file",
		"中文名.txt":      "中文内容",
	})

	files, err = GetFilesOfRepository("thomas", "gist1", "HEAD")
	require.NoError(t, err, "Could not get files of repository")
	require.Subset(t, []string{"my_renamed_file.txt", "my_other_file.txt", "new_file.txt", "中文名.txt"}, files, "Files are not correct")

	content, truncated, err = GetFileContent("thomas", "gist1", "HEAD", "my_other_file.txt", false)
	require.NoError(t, err, "Could not get content")
	require.False(t, truncated, "Content should not be truncated")
	require.Equal(t, "I really\nlike Opengist actually", content, "Content is not correct")

	commits, err := GetLog("thomas", "gist1", "HEAD", 0, 11)
	require.NoError(t, err, "Could not get log")
	require.Equal(t, 2, len(commits), "Commits count are not correct")
	require.Regexp(t, "[a-f0-9]{40}", commits[0].Hash, "Commit ID is not correct")
	require.Regexp(t, "[0-9]{10}", commits[0].Timestamp, "Commit timestamp is not correct")
	require.Equal(t, "thomas", commits[0].AuthorName, "Commit author name is not correct")
	require.Equal(t, "thomas@mail.com", commits[0].AuthorEmail, "Commit author email is not correct")
	require.Equal(t, 4, commits[0].FilesChanged, "FilesChanged is not correct")
	require.Equal(t, 2, commits[0].Additions, "Additions is not correct")
	require.Equal(t, 2, commits[0].Deletions, "Deletions is not correct")

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
		OldFilename: "my_other_file.txt",
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

	commitsSkip1, err := GetLog("thomas", "gist1", "HEAD", 1, 11)
	require.NoError(t, err, "Could not get log")
	require.Equal(t, commitsSkip1[0], commits[1], "Commits skips are not correct")
}

func TestGitGc(t *testing.T) {
	SetupTest(t)
	defer TeardownTest(t)

	err := GcRepos()
	require.NoError(t, err, "Could not run git gc")
}

func TestFork(t *testing.T) {
	SetupTest(t)
	defer TeardownTest(t)

	CommitToBare(t, "thomas", "gist1", map[string]string{
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
	SetupTest(t)
	defer TeardownTest(t)

	CommitToBare(t, "thomas", "gist1", map[string]string{
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
	CommitToBare(t, "thomas", "gist1", map[string]string{
		"my_file.txt": str,
	})

	content, truncated, err = GetFileContent("thomas", "gist1", "HEAD", "my_file.txt", true)
	require.NoError(t, err, "Could not get content")
	require.True(t, truncated, "Content should be truncated")
	require.Equal(t, truncateLimit, len(content), "Content size should be at truncate limit")

	CommitToBare(t, "thomas", "gist1", map[string]string{
		"my_file.txt": "AA\n" + str,
	})

	content, truncated, err = GetFileContent("thomas", "gist1", "HEAD", "my_file.txt", true)
	require.NoError(t, err, "Could not get content")
	require.True(t, truncated, "Content should be truncated")
	require.Equal(t, 2, len(content), "Content size is not correct")
}

func TestLogDiffTruncation(t *testing.T) {
	SetupTest(t)
	defer TeardownTest(t)

	CommitToBare(t, "thomas", "gist1", map[string]string{
		"my_file.txt": "A",
	})

	// Write enough lines to guarantee the diff content exceeds maxBytes
	// (diffSize).
	var builder strings.Builder
	lineCount := diffSize/len("A\n") + 100 // comfortably past the threshold
	for range lineCount {
		builder.WriteString("A\n")
	}
	fullContent := builder.String()

	CommitToBare(t, "thomas", "gist1", map[string]string{
		"my_file.txt": fullContent,
	})

	// 11 is arbitrary but comfortably above the 2 commits we expect back;
	// it just ensures GetLog isn't itself limiting the result set.
	commits, err := GetLog("thomas", "gist1", "HEAD", 0, 11)
	require.NoError(t, err, "Could not get log")
	require.Len(t, commits, 2, "Commits count are not correct")

	// Large-file commit: content must be truncated and bounded near
	// maxBytes, not left to grow with the full file.
	largeFile := commits[0].Files[0]
	require.Len(t, commits[0].Files, 1, "Files count are not correct")
	require.True(t, largeFile.Truncated, "Diff content should be truncated for a large file")
	require.Less(t, len(largeFile.Content), len(fullContent),
		"Truncated content must be smaller than the original — content should not grow indefinitely")
	require.LessOrEqual(t, len(largeFile.Content), diffSize+len("A\n"),
		"Truncated content should be bounded at approximately maxBytes, not just under some loose multiple of it")

	// Small-file commit: sanity check that truncation doesn't kick in
	// when it shouldn't.
	smallFile := commits[1].Files[0]
	require.False(t, smallFile.Truncated, "Small diff content should not be truncated")
	require.Equal(t, "@@ -0,0 +1 @@\n+A\n\\ No newline at end of file\n",
		smallFile.Content, "Small file content should be preserved as-is")
}

// TestParseDiffContentBudget drives parseDiffContent directly, the way parseLog
// does, with a reader sized exactly maxBytes. Going through GetLog cannot cover
// this: a line longer than the buffer comes back from ReadLine as a fragment and
// is drained without ever reaching the per-line clamp, so the clamp is only
// reachable by a long line that still fits in the buffer.
func TestParseDiffContentBudget(t *testing.T) {
	const maxBytes = 64

	// filler emits n bytes as 2-byte lines, leaving currFileLineCount just shy
	// of the cap so the following line lands on the budget boundary.
	filler := func(b *strings.Builder, n int) {
		for range n / 2 {
			b.WriteString("A\n")
		}
	}

	tests := []struct {
		name     string
		longLine int
	}{
		// Below the buffer size, so ReadLine returns these whole rather than as
		// fragments. Lengths straddle the point where the old code set
		// Truncated, which is why the flag was silently missed just under it.
		{"long line just under maxBytes", maxBytes - 2},
		{"long line at maxBytes-1", maxBytes - 1},
		{"long line exactly maxBytes", maxBytes},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			filler(&b, maxBytes-2)
			b.WriteString(strings.Repeat("B", tt.longLine) + "\n")
			b.WriteString("diff --git a/x b/x\n") // ends the file's diff content

			currentFile := &File{}
			input := bufio.NewReaderSize(strings.NewReader(b.String()), maxBytes)
			_, _, err := parseDiffContent(currentFile, maxBytes, input)
			require.NoError(t, err, "Could not parse diff content")

			// The trailing newline of the final line is appended after the
			// budget check, so one byte of overshoot is expected.
			require.LessOrEqual(t, len(currentFile.Content), maxBytes+1,
				"Content must stay within the byte budget, not grow to a multiple of it")
			require.True(t, currentFile.Truncated,
				"Truncated must be set whenever content is clipped, otherwise the UI renders a clipped diff as complete")
		})
	}
}

func TestGitInitBranchNames(t *testing.T) {
	SetupTest(t)
	defer TeardownTest(t)

	cmd := exec.Command("git", "symbolic-ref", "HEAD")
	cmd.Dir = RepositoryPath("thomas", "gist1")
	out, err := cmd.Output()
	require.NoError(t, err, "Could not run git command")
	require.Equal(t, "refs/heads/master", strings.TrimSpace(string(out)), "Repository should have master branch as default")

	config.C.GitDefaultBranch = "main"

	err = InitRepository("thomas", "gist2")
	require.NoError(t, err)
	cmd = exec.Command("git", "symbolic-ref", "HEAD")
	cmd.Dir = RepositoryPath("thomas", "gist2")
	out, err = cmd.Output()
	require.NoError(t, err, "Could not run git command")
	require.Equal(t, "refs/heads/main", strings.TrimSpace(string(out)), "Repository should have main branch as default")
}
