package types

import "time"

// CommitAuthor is the raw git-side author info for a commit (always
// populated from the commit metadata).
type CommitAuthor struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// CommitChangeStatus is the shortstat breakdown for a commit. Total equals
// additions + deletions.
type CommitChangeStatus struct {
	Files     int `json:"files_changed"`
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
	Total     int `json:"total"`
}

// GistCommit is one entry in the GET /gists/:id/commits response.
//   - `author` always carries the raw git author name + email.
//   - `user` is the Opengist account whose email matches; null when no
//     account matches the commit's email.
//   - `change_status` is the shortstat for the commit.
type GistCommit struct {
	Version      string             `json:"version"` // commit SHA
	Author       CommitAuthor       `json:"author"`
	User         *SimpleUser        `json:"user"`
	ChangeStatus CommitChangeStatus `json:"change_status"`
	CommittedAt  time.Time          `json:"committed_at"`
}
