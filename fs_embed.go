//go:build fs_embed

package main

import "embed"

//go:embed templates/*/*.html public/manifest.json public/assets/*.js public/assets/*.css public/assets/*.svg
var dirFS embed.FS
