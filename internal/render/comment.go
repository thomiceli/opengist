package render

import (
	"html/template"

	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/db"
)

// RenderedComment is a gist comment with its Markdown content rendered to HTML.
type RenderedComment struct {
	*db.GistComment
	HTML string
}

// RenderComments renders the Markdown content of the comments, with the same
// renderer as Markdown files and previews. A comment that fails to render is
// shown as preformatted text instead.
func RenderComments(comments []*db.GistComment) []*RenderedComment {
	rendered := make([]*RenderedComment, 0, len(comments))
	for _, comment := range comments {
		html, err := MarkdownString(comment.Content)
		if err != nil {
			log.Error().Err(err).Msgf("Error rendering comment %d", comment.ID)
			html = "<pre>" + template.HTMLEscapeString(comment.Content) + "</pre>"
		}
		rendered = append(rendered, &RenderedComment{GistComment: comment, HTML: html})
	}
	return rendered
}
