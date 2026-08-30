package render

import (
	"bytes"
	"regexp"

	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/yuin/goldmark"
	emoji "github.com/yuin/goldmark-emoji"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/mermaid"
)

// copyCodeBlockButton is injected into every fenced code block rendered from
// Markdown (README previews, gist descriptions, .md files, comments) so users
// can copy just that block's contents, the same way the file-level copy
// button in gist.html works.
const copyCodeBlockButton = `<button type="button" class="copy-btn copy-code-btn" aria-label="Copy code" _="on click
	call navigator.clipboard.writeText(the previous <pre/>'s textContent)
	set @data-copied to 'true'
	wait 1.2s
	remove @data-copied from me"><svg class="icon-copy size-4" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg><svg class="icon-check size-4" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg></button>`

// wrapCodeBlockWithCopyButton wraps every fenced code block that goldmark
// renders with a container div and a copy button. When chroma successfully
// highlighted the block, it already wrote its own <pre> internally, so this
// only adds the wrapping div and the button around it; otherwise (no lexer
// match, or highlighting disabled for the block) the usual <pre><code> tags
// have to be written here too, since goldmark-highlighting skips its default
// ones once a custom wrapper renderer is set.
func wrapCodeBlockWithCopyButton(w util.BufWriter, ctx highlighting.CodeBlockContext, entering bool) {
	if entering {
		_, _ = w.WriteString(`<div class="code-block-wrapper">`)
		if !ctx.Highlighted() {
			_, _ = w.WriteString("<pre><code")
			if language, ok := ctx.Language(); ok && language != nil {
				_, _ = w.WriteString(` class="language-`)
				_, _ = w.Write(language)
				_, _ = w.WriteString(`"`)
			}
			_, _ = w.WriteString(">")
		}
		return
	}

	if !ctx.Highlighted() {
		_, _ = w.WriteString("</code></pre>")
	}
	_, _ = w.WriteString(copyCodeBlockButton)
	_, _ = w.WriteString("</div>\n")
}

func MarkdownGistPreview(gist *db.Gist) (RenderedGist, error) {
	var buf bytes.Buffer
	err := newMarkdown().Convert([]byte(gist.Preview), &buf)

	// remove links in Markdown Preview, quick fix for now
	re := regexp.MustCompile(`<a\b[^>]*>(.*?)</a>`)
	return RenderedGist{
		Gist: gist,
		HTML: re.ReplaceAllString(buf.String(), `$1`),
	}, err
}

func renderMarkdownFile(file *git.File) (HighlightedFile, error) {
	var buf bytes.Buffer
	err := newMarkdownWithSvgExtension().Convert([]byte(file.Content), &buf)

	return HighlightedFile{
		File: file,
		HTML: buf.String(),
		Type: "Markdown",
	}, err
}

func MermaidGistPreview(gist *db.Gist) (RenderedGist, error) {
	var buf bytes.Buffer
	wrapped := "```mermaid\n" + gist.Preview + "\n```"
	err := newMarkdown().Convert([]byte(wrapped), &buf)

	return RenderedGist{
		Gist: gist,
		HTML: buf.String(),
	}, err
}

func renderMermaidFile(file *git.File) (HighlightedFile, error) {
	var buf bytes.Buffer
	wrapped := "```mermaid\n" + file.Content + "\n```"
	err := newMarkdown().Convert([]byte(wrapped), &buf)

	return HighlightedFile{
		File: file,
		HTML: buf.String(),
		Type: "Mermaid",
	}, err
}
func MarkdownString(content string) (string, error) {
	var buf bytes.Buffer
	err := newMarkdownWithSvgExtension().Convert([]byte(content), &buf)

	return buf.String(), err
}

func newMarkdown(extraExtensions ...goldmark.Extender) goldmark.Markdown {
	extensions := []goldmark.Extender{
		extension.GFM,
		highlighting.NewHighlighting(
			highlighting.WithStyle("catppuccin-latte"),
			highlighting.WithFormatOptions(html.WithClasses(true)),
			highlighting.WithWrapperRenderer(wrapCodeBlockWithCopyButton),
		),
		emoji.Emoji,
		&mermaid.Extender{},
		&alertExtension{},
	}

	extensions = append(extensions, extraExtensions...)

	return goldmark.New(
		goldmark.WithExtensions(extensions...),
		goldmark.WithParserOptions(
			parser.WithASTTransformers(
				util.Prioritized(&checkboxTransformer{}, 10000),
			),
		),
	)
}

func newMarkdownWithSvgExtension() goldmark.Markdown {
	return newMarkdown(&svgToImgBase64{})
}
