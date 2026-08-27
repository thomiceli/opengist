package hooks

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSymlinkFiles(t *testing.T) {
	output := []byte("100644 blob aaaaaaa\tregular.md\x00" +
		"120000 blob bbbbbbb\ttask.md\x00" +
		"120000 blob ccccccc\tfilename with spaces\x00")

	require.Equal(t, []string{"task.md", "filename with spaces"}, parseSymlinkFiles(output))
}
