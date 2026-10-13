package templates

import "embed"

//go:embed layouts/*.html partials/*.html pages/*.html
var Files embed.FS
