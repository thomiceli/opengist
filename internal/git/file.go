package git

import (
	"path/filepath"
	"strings"
)

func CleanTreePathName(s string) string {
	name := filepath.Base(s)

	if name == "." || name == ".." {
		return ""
	}

	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, "\\", "")

	return name
}
