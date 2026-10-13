package types

import "time"

// GistFile is one entry in the `files` object of a list-shape gist (no
// content; clients fetch raw_url).
type GistFile struct {
	Filename  string `json:"filename"`
	Type      string `json:"type"`
	Language  string `json:"language,omitempty"`
	Size      int    `json:"size"`
	Truncated bool   `json:"truncated"`
	Content   string `json:"content"`
	Encoding  string `json:"encoding"`
}

// GistSimple is the list-shape gist. Used for the `GET /gists` list and
// any other place where content isn't included. The `Visibility` field
// preserves the public/unlisted/private distinction that the `Public` bool
// can't express.
type GistSimple struct {
	ID          string     `json:"id"`
	SlugUrl     string     `json:"slug_url"`
	Owner       SimpleUser `json:"owner"`
	Title       string     `json:"title"`
	HTMLUrl     string     `json:"html_url"`
	Description string     `json:"description"`
	Public      bool       `json:"public"`
	Visibility  string     `json:"visibility"` // Opengist extension: public/unlisted/private
	LikeCount   int        `json:"like_count"`
	ForkCount   int        `json:"fork_count"`
	CloneUrl    string     `json:"clone_url"`
	SSHUrl      string     `json:"ssh_url"`
	Topics      []string   `json:"topics"`
	Archived    bool       `json:"archived"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ExpiresAt   *time.Time `json:"expires_at"` // null when the gist never expires
}

type Gist struct {
	GistSimple
	ForkOf    interface{}         `json:"fork_of"`
	Forks     []GistSimple        `json:"forks"`
	Files     map[string]GistFile `json:"files"`
	Commits   []GistCommit        `json:"commits"`
	Truncated bool                `json:"truncated"`
}
