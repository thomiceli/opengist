package hooks

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/git"
	"os"
	"testing"
)

func TestPreReceiveHook(t *testing.T) {
	git.SetupTest(t)
	defer git.TeardownTest(t)
	var lastCommitHash string
	err := os.Chdir(git.RepositoryPath("thomas", "gist1"))
	require.NoError(t, err, "Could not change directory")

	git.CommitToBare(t, "thomas", "gist1", map[string]string{
		"my_file.txt":  "some allowed file",
		"my_file2.txt": "some allowed file\nagain",
	})
	lastCommitHash = git.LastHashOfCommit(t, "thomas", "gist1")
	err = PreReceive(bytes.NewBufferString(fmt.Sprintf("%s %s %s", BaseHash, lastCommitHash, "refs/heads/master")), os.Stdout, os.Stderr)
	require.NoError(t, err, "Should not have an error on pre-receive hook for commit+push 1")

	git.CommitToBare(t, "thomas", "gist1", map[string]string{
		"my_file.txt":     "some allowed file",
		"dir/my_file.txt": "some disallowed file suddenly",
	})
	lastCommitHash = git.LastHashOfCommit(t, "thomas", "gist1")
	err = PreReceive(bytes.NewBufferString(fmt.Sprintf("%s %s %s", BaseHash, lastCommitHash, "refs/heads/master")), os.Stdout, os.Stderr)
	require.Error(t, err, "Should have an error on pre-receive hook for commit+push 2")
	require.Equal(t, "pushing files in directories is not allowed: [dir/my_file.txt]", err.Error(), "Error message is not correct")

	git.CommitToBare(t, "thomas", "gist1", map[string]string{
		"my_file.txt":           "some allowed file",
		"dir/ok/afileagain.txt": "some disallowed file\nagain",
	})
	lastCommitHash = git.LastHashOfCommit(t, "thomas", "gist1")
	err = PreReceive(bytes.NewBufferString(fmt.Sprintf("%s %s %s", BaseHash, lastCommitHash, "refs/heads/master")), os.Stdout, os.Stderr)
	require.Error(t, err, "Should have an error on pre-receive hook for commit+push 3")
	require.Equal(t, "pushing files in directories is not allowed: [dir/ok/afileagain.txt dir/my_file.txt]", err.Error(), "Error message is not correct")

	git.CommitToBare(t, "thomas", "gist1", map[string]string{
		"allowedfile.txt": "some allowed file only",
	})
	lastCommitHash = git.LastHashOfCommit(t, "thomas", "gist1")
	err = PreReceive(bytes.NewBufferString(fmt.Sprintf("%s %s %s", BaseHash, lastCommitHash, "refs/heads/master")), os.Stdout, os.Stderr)
	require.Error(t, err, "Should have an error on pre-receive hook for commit+push 4")
	require.Equal(t, "pushing files in directories is not allowed: [dir/ok/afileagain.txt dir/my_file.txt]", err.Error(), "Error message is not correct")

	_ = os.Chdir(os.TempDir()) // Leave the current dir to avoid errors on teardown
}
