package render

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
)

type HighlightedFile struct {
	*git.File
	Type  string   `json:"type"`
	Lines []string `json:"-"`
	HTML  string   `json:"-"`
}

func (r HighlightedFile) InternalType() string {
	return "HighlightedFile"
}

type RenderedGist struct {
	*db.Gist
	Lines []string
	HTML  string
}

func highlightFile(file *git.File) (HighlightedFile, error) {
	rendered := HighlightedFile{
		File: file,
	}
	if !file.MimeType.IsText() {
		return rendered, nil
	}
	style := newStyle()
	lexer := newLexer(file.Filename)

	formatter := html.New(html.WithClasses(true), html.PreventSurroundingPre(true))

	iterator, err := lexer.Tokenise(nil, file.Content+"\n")
	if err != nil {
		return rendered, err
	}

	htmlbuf := bytes.Buffer{}
	w := bufio.NewWriter(&htmlbuf)

	tokensLines := chroma.SplitTokensIntoLines(iterator.Tokens())
	lines := make([]string, 0, len(tokensLines))
	for _, tokens := range tokensLines {
		iterator = chroma.Literator(tokens...)
		err = formatter.Format(&htmlbuf, style, iterator)
		if err != nil {
			return rendered, fmt.Errorf("unable to format code: %w", err)
		}
		lines = append(lines, htmlbuf.String())
		htmlbuf.Reset()
	}

	_ = w.Flush()

	rendered.Lines = lines
	rendered.Type = parseFileTypeName(*lexer.Config())

	return rendered, err
}

func HighlightGistPreview(gist *db.Gist) (RenderedGist, error) {
	rendered := RenderedGist{
		Gist: gist,
	}

	style := newStyle()
	lexer := newLexer(gist.PreviewFilename)
	if lexer.Config().Name == "markdown" {
		return MarkdownGistPreview(gist)
	}

	formatter := html.New(html.WithClasses(true), html.PreventSurroundingPre(true))

	iterator, err := lexer.Tokenise(nil, gist.Preview)
	if err != nil {
		return rendered, err
	}

	htmlbuf := bytes.Buffer{}
	w := bufio.NewWriter(&htmlbuf)

	tokensLines := chroma.SplitTokensIntoLines(iterator.Tokens())
	lines := make([]string, 0, len(tokensLines))
	for _, tokens := range tokensLines {
		iterator = chroma.Literator(tokens...)
		err = formatter.Format(&htmlbuf, style, iterator)
		if err != nil {
			return rendered, fmt.Errorf("unable to format code: %w", err)
		}
		lines = append(lines, htmlbuf.String())
		htmlbuf.Reset()
	}

	_ = w.Flush()

	rendered.Lines = lines

	return rendered, err
}

func renderSvgFile(file *git.File) HighlightedFile {
	return HighlightedFile{
		File: file,
		HTML: `<img src="data:image/svg+xml;base64,` + base64.StdEncoding.EncodeToString([]byte(file.Content)) + `" />`,
		Type: "SVG",
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
	var lexer chroma.Lexer
	if lexer = lexers.Get(filename); lexer == nil {
		lexer = lexers.Fallback
	}

	return lexer
}

func newStyle() *chroma.Style {
	var style *chroma.Style
	if style = styles.Get("catppuccin-latte"); style == nil {
		style = styles.Fallback
	}

	return style
}
