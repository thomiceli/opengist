//go:build fs_embed

package publicold

import "embed"

//go:embed .vite/manifest.json assets/*.js assets/*.css assets/*.svg assets/*.png assets/*.ttf assets/*.woff assets/*.woff2
var Files embed.FS
