//go:build !fs_embed

package publicold

import (
	"os"
	"path/filepath"
	"runtime"
)

func filesRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Dir(filename)
}

var Files = os.DirFS(filesRoot())
