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
	"regexp"
)

var svgRegex = regexp.MustCompile(`(?i)^[ ]{0,3}<(svg)(?:\s.*|>.*|/>.*|)(?:\r\n|\n)?$`)

type svgToImgBase64 struct{}

func (e *svgToImgBase64) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithBlockParsers(
		util.Prioritized(newSvgParser(), 1),
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(newSvgRenderer(), 1),
	))
}

// -- SVG Block -- //

type svgBlock struct {
	ast.BaseBlock
}

func (n *svgBlock) IsRaw() bool {
	return true
}

func (n *svgBlock) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

var svgBlockKind = ast.NewNodeKind("SVG")

func (n *svgBlock) Kind() ast.NodeKind {
	return svgBlockKind
}

func newSvgBlock() *svgBlock {
	return &svgBlock{
		BaseBlock: ast.BaseBlock{},
	}
}

// -- SVG Parser -- //

type svgParser struct {
}

var defaultSvgParser = &svgParser{}

func newSvgParser() parser.BlockParser {
	return defaultSvgParser
}

func (b *svgParser) Trigger() []byte {
	return []byte{'<'}
}

func (b *svgParser) Open(parent ast.Node, reader text.Reader, _ parser.Context) (ast.Node, parser.State) {
	var node *svgBlock
	line, segment := reader.PeekLine()

	if !bytes.HasPrefix(line, []byte("<svg")) {
		return nil, parser.None
	}

	if svgRegex.Match(line) {
		node = newSvgBlock()
	}

	if node != nil {
		reader.Advance(segment.Len() - util.TrimRightSpaceLength(line))
		node.Lines().Append(segment)
		return node, parser.NoChildren
	}
	return nil, parser.None
}

func (b *svgParser) Continue(node ast.Node, reader text.Reader, _ parser.Context) parser.State {
	line, segment := reader.PeekLine()
	if util.IsBlank(line) {
		return parser.Close
	}

	if !bytes.HasSuffix(util.TrimRightSpace(line), []byte("</svg>")) {
		node.Lines().Append(segment)
		return parser.Continue | parser.NoChildren
	}

	node.Lines().Append(segment)
	reader.Advance(segment.Len())
	return parser.Close
}

func (b *svgParser) Close(_ ast.Node, _ text.Reader, _ parser.Context) {}

func (b *svgParser) CanInterruptParagraph() bool {
	return true
}

func (b *svgParser) CanAcceptIndentedLine() bool {
	return false
}

// -- SVG Renderer -- //

type svgRenderer struct{}

func newSvgRenderer() renderer.NodeRenderer {
	return &svgRenderer{}
}

func (r *svgRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(svgBlockKind, r.renderSVG)
}

func (r *svgRenderer) renderSVG(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	rawHTML := node.(*svgBlock)
	var svgContent []byte
	for i := 0; i < rawHTML.Lines().Len(); i++ {
		segment := rawHTML.Lines().At(i)
		svgContent = append(svgContent, segment.Value(source)...)
	}
	encoded := base64.StdEncoding.EncodeToString(svgContent)
	imgTag := `<img src="data:image/svg+xml;base64,` + encoded + `" />`
	_, _ = w.Write([]byte(imgTag))
	return ast.WalkContinue, nil
}
