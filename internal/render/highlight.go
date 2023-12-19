package render

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
)

type RenderedFile struct {
	*git.File
	Type  string
	Lines []string
	HTML  string
}

type RenderedGist struct {
	*db.Gist
	Lines []string
	HTML  string
}

func HighlightFile(file *git.File) (RenderedFile, error) {
	rendered := RenderedFile{
		File: file,
	}

	style := newStyle()
	lexer := newLexer(file.Filename)
	if lexer.Config().Name == "markdown" {
		return MarkdownFile(file)
	}

	formatter := html.New(html.WithClasses(true), html.PreventSurroundingPre(true))

	iterator, err := lexer.Tokenise(nil, file.Content)
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
