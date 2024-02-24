package render

import (
	"bufio"
	"bytes"
	"github.com/Kunde21/markdownfmt/v3"
	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/yuin/goldmark"
	emoji "github.com/yuin/goldmark-emoji"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	astex "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/mermaid"
	"strconv"
)

func MarkdownGistPreview(gist *db.Gist) (RenderedGist, error) {
	var buf bytes.Buffer
	err := newMarkdown().Convert([]byte(gist.Preview), &buf)

	return RenderedGist{
		Gist: gist,
		HTML: buf.String(),
	}, err
}

func MarkdownFile(file *git.File) (RenderedFile, error) {
	var buf bytes.Buffer
	err := newMarkdown().Convert([]byte(file.Content), &buf)

	return RenderedFile{
		File: file,
		HTML: buf.String(),
		Type: "Markdown",
	}, err
}
func MarkdownString(content string) (string, error) {
	var buf bytes.Buffer
	err := newMarkdown().Convert([]byte(content), &buf)

	return buf.String(), err
}

func newMarkdown() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle("catppuccin-latte"),
				highlighting.WithFormatOptions(html.WithClasses(true))),
			emoji.Emoji,
			&mermaid.Extender{},
		),
		goldmark.WithParserOptions(
			parser.WithASTTransformers(
				util.Prioritized(&CheckboxTransformer{}, 10000),
			),
		),
	)
}

type CheckboxTransformer struct{}

func (t *CheckboxTransformer) Transform(node *ast.Document, _ text.Reader, _ parser.Context) {
	i := 1
	err := ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if _, ok := n.(*astex.TaskCheckBox); ok {
				listitem := n.Parent().Parent()
				listitem.SetAttribute([]byte("data-checkbox-nb"), []byte(strconv.Itoa(i)))
				i += 1
			}
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		log.Err(err)
	}
}

func Checkbox(content string, checkboxNb int) (string, error) {
	buf := bytes.Buffer{}
	w := bufio.NewWriter(&buf)

	source := []byte(content)
	markdown := markdownfmt.NewGoldmark()
	reader := text.NewReader(source)
	document := markdown.Parser().Parse(reader)

	i := 1
	err := ast.Walk(document, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if listItem, ok := n.(*astex.TaskCheckBox); ok {
				if i == checkboxNb {
					listItem.IsChecked = !listItem.IsChecked
				}
				i += 1
			}
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return "", err
	}

	if err = markdown.Renderer().Render(w, source, document); err != nil {
		return "", err
	}
	_ = w.Flush()

	return buf.String(), nil
}
