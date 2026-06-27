package render

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// GitHub-style markdown alerts (admonitions), e.g.
//
//	> [!NOTE]
//	> Useful information that users should know.
//
// Supported types: note, tip, important, warning, caution.
// See https://docs.github.com/en/get-started/writing-on-github/getting-started-with-writing-and-formatting-on-github/basic-writing-and-formatting-syntax#alerts

var alertRegex = regexp.MustCompile(`(?i)^\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]$`)

// titles displayed for each alert type
var alertTitles = map[string]string{
	"note":      "Note",
	"tip":       "Tip",
	"important": "Important",
	"warning":   "Warning",
	"caution":   "Caution",
}

// octicon SVGs matching GitHub's alerts, inheriting the title color via `fill: currentColor`
var alertIcons = map[string]string{
	"note":      `<svg class="octicon octicon-info" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path d="M0 8a8 8 0 1 1 16 0A8 8 0 0 1 0 8Zm8-6.5a6.5 6.5 0 1 0 0 13 6.5 6.5 0 0 0 0-13ZM6.5 7.75A.75.75 0 0 1 7.25 7h1a.75.75 0 0 1 .75.75v2.75h.25a.75.75 0 0 1 0 1.5h-2a.75.75 0 0 1 0-1.5h.25v-2h-.25a.75.75 0 0 1-.75-.75ZM8 6a1 1 0 1 1 0-2 1 1 0 0 1 0 2Z"></path></svg>`,
	"tip":       `<svg class="octicon octicon-light-bulb" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path d="M8 1.5c-2.363 0-4 1.69-4 3.75 0 .984.424 1.625.984 2.304l.214.253c.223.264.47.556.673.848.284.411.537.896.621 1.49a.75.75 0 0 1-1.484.211c-.04-.282-.163-.547-.37-.847a8.456 8.456 0 0 0-.542-.68c-.084-.1-.173-.205-.268-.32C3.201 7.75 2.5 6.766 2.5 5.25 2.5 2.31 4.863 0 8 0s5.5 2.31 5.5 5.25c0 1.516-.701 2.5-1.328 3.259-.095.115-.184.22-.268.319-.207.245-.383.453-.541.681-.208.3-.33.565-.37.847a.751.751 0 0 1-1.485-.212c.084-.593.337-1.078.621-1.489.203-.292.45-.584.673-.848.075-.088.147-.173.213-.253.561-.679.985-1.32.985-2.304 0-2.06-1.637-3.75-4-3.75ZM5.75 12h4.5a.75.75 0 0 1 0 1.5h-4.5a.75.75 0 0 1 0-1.5ZM6 15.25a.75.75 0 0 1 .75-.75h2.5a.75.75 0 0 1 0 1.5h-2.5a.75.75 0 0 1-.75-.75Z"></path></svg>`,
	"important": `<svg class="octicon octicon-report" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path d="M0 1.75A1.75 1.75 0 0 1 1.75 0h12.5C15.216 0 16 .784 16 1.75v9.5A1.75 1.75 0 0 1 14.25 13H8.06l-2.573 2.573A1.458 1.458 0 0 1 3 14.543V13H1.75A1.75 1.75 0 0 1 0 11.25Zm1.75-.25a.25.25 0 0 0-.25.25v9.5c0 .138.112.25.25.25h2a.75.75 0 0 1 .75.75v2.19l2.72-2.72a.749.749 0 0 1 .53-.22h6.5a.25.25 0 0 0 .25-.25v-9.5a.25.25 0 0 0-.25-.25Zm7 2.25v2.5a.75.75 0 0 1-1.5 0v-2.5a.75.75 0 0 1 1.5 0ZM9 9a1 1 0 1 1-2 0 1 1 0 0 1 2 0Z"></path></svg>`,
	"warning":   `<svg class="octicon octicon-alert" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path d="M6.457 1.047c.659-1.234 2.427-1.234 3.086 0l6.082 11.378A1.75 1.75 0 0 1 14.082 15H1.918a1.75 1.75 0 0 1-1.543-2.575Zm1.763.707a.25.25 0 0 0-.44 0L1.698 13.132a.25.25 0 0 0 .22.368h12.164a.25.25 0 0 0 .22-.368Zm.53 3.996v2.5a.75.75 0 0 1-1.5 0v-2.5a.75.75 0 0 1 1.5 0ZM9 11a1 1 0 1 1-2 0 1 1 0 0 1 2 0Z"></path></svg>`,
	"caution":   `<svg class="octicon octicon-stop" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path d="M4.47.22A.749.749 0 0 1 5 0h6c.199 0 .389.079.53.22l4.25 4.25c.141.141.22.331.22.53v6a.749.749 0 0 1-.22.53l-4.25 4.25A.749.749 0 0 1 11 16H5a.749.749 0 0 1-.53-.22L.22 11.53A.749.749 0 0 1 0 11V5c0-.199.079-.389.22-.53Zm.84 1.28L1.5 5.31v5.38l3.81 3.81h5.38l3.81-3.81V5.31L10.69 1.5ZM8 4a.75.75 0 0 1 .75.75v3.5a.75.75 0 0 1-1.5 0v-3.5A.75.75 0 0 1 8 4Zm0 8a1 1 0 1 1 0-2 1 1 0 0 1 0 2Z"></path></svg>`,
}

// -- Alert extension -- //

type alertExtension struct{}

func (e *alertExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithASTTransformers(
		util.Prioritized(&alertTransformer{}, 100),
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(newAlertRenderer(), 100),
	))
}

// -- Alert node -- //

type alertNode struct {
	ast.BaseBlock
	kind string
}

var alertNodeKind = ast.NewNodeKind("Alert")

func (n *alertNode) Kind() ast.NodeKind { return alertNodeKind }

func (n *alertNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{"kind": n.kind}, nil)
}

// -- Alert transformer -- //

// alertTransformer turns blockquotes that start with an alert marker
// (e.g. `[!NOTE]`) into alert nodes.
type alertTransformer struct{}

func (t *alertTransformer) Transform(node *ast.Document, reader text.Reader, _ parser.Context) {
	source := reader.Source()

	// collect blockquotes first, since we mutate the tree while iterating
	var blockquotes []*ast.Blockquote
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if bq, ok := n.(*ast.Blockquote); ok {
				blockquotes = append(blockquotes, bq)
			}
		}
		return ast.WalkContinue, nil
	})

	for _, bq := range blockquotes {
		kind := detectAndStripAlert(bq, source)
		if kind == "" {
			continue
		}

		alert := &alertNode{kind: kind}
		for child := bq.FirstChild(); child != nil; {
			next := child.NextSibling()
			bq.RemoveChild(bq, child)
			alert.AppendChild(alert, child)
			child = next
		}

		if parent := bq.Parent(); parent != nil {
			parent.ReplaceChild(parent, bq, alert)
		}
	}
}

// detectAndStripAlert returns the alert kind if the blockquote's first line is
// an alert marker, removing that marker from the tree. It returns "" otherwise.
func detectAndStripAlert(bq *ast.Blockquote, source []byte) string {
	para, ok := bq.FirstChild().(*ast.Paragraph)
	if !ok || para.Lines().Len() == 0 {
		return ""
	}

	firstLine := para.Lines().At(0)
	match := alertRegex.FindSubmatch(bytes.TrimSpace(firstLine.Value(source)))
	if match == nil {
		return ""
	}

	// drop the inline nodes belonging to the marker line
	for child := para.FirstChild(); child != nil; {
		next := child.NextSibling()
		txt, ok := child.(*ast.Text)
		if !ok || txt.Segment.Start >= firstLine.Stop {
			break
		}
		para.RemoveChild(para, child)
		child = next
	}

	// if the marker was the only content of its paragraph, drop the paragraph
	if para.FirstChild() == nil {
		bq.RemoveChild(bq, para)
	}

	return strings.ToLower(string(match[1]))
}

// -- Alert renderer -- //

type alertRenderer struct{}

func newAlertRenderer() renderer.NodeRenderer { return &alertRenderer{} }

func (r *alertRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(alertNodeKind, r.render)
}

func (r *alertRenderer) render(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*alertNode)
	if entering {
		_, _ = w.WriteString(`<div class="markdown-alert markdown-alert-` + n.kind + `">` + "\n")
		_, _ = w.WriteString(`<p class="markdown-alert-title">`)
		_, _ = w.WriteString(alertIcons[n.kind])
		_, _ = w.WriteString(alertTitles[n.kind])
		_, _ = w.WriteString("</p>\n")
	} else {
		_, _ = w.WriteString("</div>\n")
	}
	return ast.WalkContinue, nil
}
