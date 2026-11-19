package types

import "time"

// GistComment is the shape returned by the gist comments endpoints. `body` is
// the raw Markdown content of the comment; `url` is its API URL and
// `html_url` its web permalink, which redirects to wherever it is shown.
type GistComment struct {
	ID        uint       `json:"id"`
	URL       string     `json:"url"`
	HTMLUrl   string     `json:"html_url"`
	Body      string     `json:"body"`
	User      SimpleUser `json:"user"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// GistCommentInput is the request body for POST and PATCH
// /gists/:uuid/comments: the Markdown content of the comment.
type GistCommentInput struct {
	Body string `json:"body"`
}
