package render

import (
	"bytes"
	"encoding/base64"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

type svgToImg struct{}

func (e *svgToImg) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithInlineParsers(
		util.Prioritized(newInlineSvgParser(), 1),
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(newInlineSvgRenderer(), 1),
	))
}

type inlineSvgParser struct{}

func newInlineSvgParser() parser.InlineParser {
	return &inlineSvgParser{}
}

func (p *inlineSvgParser) Trigger() []byte {
	return []byte{'<'}
}

func (p *inlineSvgParser) Parse(_ ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, _ := block.PeekLine()
	if bytes.HasPrefix(line, []byte("<svg")) {
		node := ast.NewRawHTML()
		_, savedSegment := block.Position()
		node.Segments.Append(text.NewSegment(savedSegment.Start, savedSegment.Start+len(line)))
		block.Advance(len(line))
		return node
	}
	return nil
}

func (p *inlineSvgParser) CloseBlock() {}

type inlineSvgRenderer struct{}

func newInlineSvgRenderer() renderer.NodeRenderer {
	return &inlineSvgRenderer{}
}

func (r *inlineSvgRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindRawHTML, r.renderSVG)
}

func (r *inlineSvgRenderer) renderSVG(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	rawHTML := node.(*ast.RawHTML)
	var svgContent []byte
	for i := 0; i < rawHTML.Segments.Len(); i++ {
		segment := rawHTML.Segments.At(i)
		svgContent = append(svgContent, segment.Value(source)...)
	}
	encoded := base64.StdEncoding.EncodeToString(svgContent)
	imgTag := `<img src="data:image/svg+xml;base64,` + encoded + `" />`
	_, _ = w.Write([]byte(imgTag))
	return ast.WalkContinue, nil
}
