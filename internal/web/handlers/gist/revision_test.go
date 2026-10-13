package gist_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

func TestRevisionArgInjection(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	repoPath, _, user, gistId := s.CreateGist(t, "0")

	markers := []string{
		filepath.Join(repoPath, "OGPOC_INJECTED:x"),
		filepath.Join(repoPath, "OGPOC_INJECTED"),
	}
	for _, m := range markers {
		_, err := os.Stat(m)
		require.Truef(t, os.IsNotExist(err), "marker %s must not exist before the attack", m)
	}

	base := "/" + user + "/" + gistId

	attacks := []string{
		base + "/raw/--output=OGPOC_INJECTED/x",      // git show --output= sink
		base + "/download/--output=OGPOC_INJECTED/x", // git show --output= sink
		base + "/archive/--output=OGPOC_INJECTED",    // git ls-tree sink
		base + "/archive/-p",                         // bare short option
		base + "/rev/--output=OGPOC_INJECTED",        // GistIndex, git ls-tree sink
		base + "/rev/--all",                          // GistIndex, git ls-tree sink
	}

	for _, path := range attacks {
		t.Run(path, func(t *testing.T) {
			code := s.Request(t, "GET", path, nil, 0).StatusCode
			require.NotEqualf(t, 200, code, "injection %s must not succeed", path)
		})
	}

	for _, m := range markers {
		_, err := os.Stat(m)
		require.Truef(t, os.IsNotExist(err),
			"argument injection must not create a server-side file, but %s exists", m)
	}

	t.Run("HEAD-still-works", func(t *testing.T) {
		s.Request(t, "GET", base+"/raw/HEAD/file.txt", nil, 200)
		s.Request(t, "GET", base+"/archive/HEAD", nil, 200)
	})
}
