package render

import (
	"bytes"
	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/yuin/goldmark"
	emoji "github.com/yuin/goldmark-emoji"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"go.abhg.dev/goldmark/mermaid"
)

func MarkdownCode(code string) (string, error) {
	markdown := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle("catppuccin-latte"),
				highlighting.WithFormatOptions(html.WithClasses(true))),
			emoji.Emoji,
			&mermaid.Extender{},
		),
	)

	var buf bytes.Buffer
	err := markdown.Convert([]byte(code), &buf)

	return buf.String(), err
}

func MarkdownFile(file *git.File) (RenderedFile, error) {
	rendered, err := MarkdownCode(file.Content)

	return RenderedFile{
		File: file,
		HTML: rendered,
		Type: "Markdown",
	}, err
}
