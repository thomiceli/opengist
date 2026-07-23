package templatesold

import "embed"

//go:embed base/*.html partials/*.html pages/*.html
var Files embed.FS
