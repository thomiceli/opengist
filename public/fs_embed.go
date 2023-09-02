//go:build fs_embed

package public

import "embed"

//go:embed manifest.json assets/*.js assets/*.css assets/*.svg assets/*.png
var Files embed.FS
