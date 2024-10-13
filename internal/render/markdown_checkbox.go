package render

import (
	"bufio"
	"bytes"
	"github.com/Kunde21/markdownfmt/v3"
	"github.com/rs/zerolog/log"
	"github.com/yuin/goldmark/ast"
	astex "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"strconv"
)

type checkboxTransformer struct{}

func (t *checkboxTransformer) Transform(node *ast.Document, _ text.Reader, _ parser.Context) {
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
