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

	var lexer chroma.Lexer
	if lexer = lexers.Get(file.Filename); lexer == nil {
		lexer = lexers.Fallback
	}

	if lexer.Config().Name == "markdown" {
		return MarkdownFile(file)
	}

	style := styles.Get("catppuccin-latte")
	if style == nil {
		style = styles.Fallback
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

	var lexer chroma.Lexer
	if lexer = lexers.Get(gist.PreviewFilename); lexer == nil {
		lexer = lexers.Fallback
	}

	if lexer.Config().Name == "markdown" {
		return MarkdownGistPreview(gist)
	}

	style := styles.Get("catppuccin-latte")
	if style == nil {
		style = styles.Fallback
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
