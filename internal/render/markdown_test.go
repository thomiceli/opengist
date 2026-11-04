package render

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"golang.org/x/net/html"
)

func TestMarkdownCodeBlockLanguageEscaping(t *testing.T) {
	renderers := map[string]func(string) (string, error){
		"string": MarkdownString,
		"preview": func(source string) (string, error) {
			result, err := MarkdownGistPreview(&db.Gist{Preview: source})
			return result.HTML, err
		},
		"file": func(source string) (string, error) {
			result, err := renderMarkdownFile(&git.File{Content: source})
			return result.HTML, err
		},
	}

	for name, render := range renderers {
		t.Run(name, func(t *testing.T) {
			for _, language := range []string{
				"unknown-language",
				`"><svg/onload=alert(1)>`,
				`"><img/src=x/onerror=alert(1)>`,
				`unknown&"<`,
			} {
				t.Run(language, func(t *testing.T) {
					output, err := render("```" + language + "\nhello & <world>\n```\n")
					require.NoError(t, err)
					doc, err := html.Parse(strings.NewReader(output))
					require.NoError(t, err)

					var code *html.Node
					var walk func(*html.Node)
					walk = func(node *html.Node) {
						if node.Type == html.ElementNode {
							require.NotEqual(t, "img", node.Data, "fence language injected an element")
							for _, attr := range node.Attr {
								require.False(t, strings.HasPrefix(attr.Key, "on"), "fence language injected an event handler")
							}
							if node.Data == "code" {
								code = node
							}
						}
						for child := node.FirstChild; child != nil; child = child.NextSibling {
							walk(child)
						}
					}
					walk(doc)
					require.NotNil(t, code)
					require.Equal(t, []html.Attribute{{Key: "class", Val: "language-" + language}}, code.Attr)
					require.NotNil(t, code.FirstChild)
					require.Equal(t, html.TextNode, code.FirstChild.Type)
					require.Equal(t, "hello & <world>\n", code.FirstChild.Data)
				})
			}
		})
	}
}

func TestMarkdownCodeBlockCopyButton(t *testing.T) {
	source := "```go\nfmt.Println(1)\n```\n\n```\nplain\n```\n"

	file, err := renderMarkdownFile(&git.File{Content: source})
	require.NoError(t, err)
	require.Equal(t, 2, strings.Count(file.HTML, `class="copy-btn copy-code-btn"`))

	// List previews are truncated and rendered inside a link to the gist.
	preview, err := MarkdownGistPreview(&db.Gist{Preview: source})
	require.NoError(t, err)
	require.NotContains(t, preview.HTML, "copy-code-btn")
	require.NotContains(t, preview.HTML, "code-block-wrapper")
}
