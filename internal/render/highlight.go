package render

import (
	"bufio"
	"bytes"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/thomiceli/opengist/internal/git"
)

type RenderedFile struct {
	File *git.File
	Type string
	HTML string
}

func File(file *git.File) (RenderedFile, error) {
	colorFile := RenderedFile{
		File: file,
	}

	var lexer chroma.Lexer
	if lexer = lexers.Get(file.Filename); lexer == nil {
		lexer = lexers.Fallback
	}

	style := styles.Get("catppuccin-latte")
	if style == nil {
		style = styles.Fallback
	}

	formatter := html.New(html.WithClasses(true), html.PreventSurroundingPre(true))

	iterator, err := lexer.Tokenise(nil, file.Content)
	if err != nil {
		return colorFile, err
	}

	htmlbuf := bytes.Buffer{}
	w := bufio.NewWriter(&htmlbuf)

	if err = formatter.Format(w, style, iterator); err != nil {
		return colorFile, err
	}

	_ = w.Flush()

	colorFile.HTML = htmlbuf.String()
	colorFile.Type = parseFileTypeName(*lexer.Config())

	return colorFile, err
}

func Code(filename, code string) (string, error) {
	var lexer chroma.Lexer
	if lexer = lexers.Get(filename); lexer == nil {
		lexer = lexers.Fallback
	}

	style := styles.Get("catppuccin-latte")
	if style == nil {
		style = styles.Fallback
	}

	formatter := html.New(html.WithClasses(true), html.PreventSurroundingPre(true))

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return code, err
	}

	htmlbuf := bytes.Buffer{}
	w := bufio.NewWriter(&htmlbuf)

	if err = formatter.Format(w, style, iterator); err != nil {
		return code, err
	}

	_ = w.Flush()

	return htmlbuf.String(), err
}

func parseFileTypeName(config chroma.Config) string {
	fileType := config.Name
	if fileType == "fallback" || fileType == "plaintext" {
		return "Text"
	}

	return fileType
}
