// Package lang derives a human-readable language label for a gist file.
// Kept dependency-light (no db import) so callers in lower layers (db,
// API types) can use it without creating import cycles.
package lang

import (
	"path/filepath"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/thomiceli/opengist/internal/git"
)

// Parse returns the language label for a file (e.g. "Go", "Markdown", "SVG",
// "Text"). Falls back to "Text" for unknown / fallback lexers.
func Parse(file *git.File) string {
	mt := file.MimeType
	switch {
	case mt.IsCSV():
		return "CSV"
	case mt.IsText() && filepath.Ext(file.Filename) == ".md":
		return "Markdown"
	case mt.IsSVG():
		return "SVG"
	case mt.CanBeEmbedded():
		return mt.RenderType()
	case mt.CanBeRendered():
		return parseFileTypeName(*newLexer(file.Filename).Config())
	default:
		return mt.RenderType()
	}
}

func parseFileTypeName(config chroma.Config) string {
	fileType := config.Name
	if fileType == "fallback" || fileType == "plaintext" {
		return "Text"
	}
	return fileType
}

func newLexer(filename string) chroma.Lexer {
	if l := lexers.Get(filename); l != nil {
		return l
	}
	return lexers.Fallback
}
