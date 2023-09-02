//go:build !fs_embed

package public

import "os"

var Files = os.DirFS(".")
