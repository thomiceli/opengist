package v1

import "time"

// --- Common ---

type Pagination struct {
	Page    int   `json:"page"`
	PerPage int   `json:"per_page"`
	Total   int64 `json:"total"`
}

// --- User ---

type UserResponse struct {
	ID        uint      `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	IsAdmin   bool      `json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
}

// --- Gist ---

type FileSummary struct {
	Filename string `json:"filename"`
	Size     int    `json:"size"`
	Language string `json:"language,omitempty"`
}

type FileDetail struct {
	Filename  string `json:"filename"`
	Size      int    `json:"size"`
	Language  string `json:"language,omitempty"`
	Content   string `json:"content"`
	Binary    bool   `json:"binary"`
	Truncated bool   `json:"truncated"`
}

type GistOwner struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

type GistSummary struct {
	UUID        string        `json:"uuid"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Visibility  string        `json:"visibility"`
	HTMLURL     string        `json:"html_url"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	Owner       GistOwner     `json:"owner"`
	Files       []FileSummary `json:"files"`
}

type GistDetail struct {
	UUID        string       `json:"uuid"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Visibility  string       `json:"visibility"`
	HTMLURL     string       `json:"html_url"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	Owner       GistOwner    `json:"owner"`
	Files       []FileDetail `json:"files"`
}

type PaginatedGists struct {
	Data []GistSummary `json:"data"`
	Pagination
}

// --- Create / Update ---

type FileInput struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

type CreateGistRequest struct {
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Visibility  string      `json:"visibility"` // "public" | "unlisted" | "private"
	Files       []FileInput `json:"files"`
}

type UpdateGistRequest struct {
	Title       *string      `json:"title,omitempty"`
	Description *string      `json:"description,omitempty"`
	Visibility  *string      `json:"visibility,omitempty"`
	Files       *[]FileInput `json:"files,omitempty"` // nil = no change; non-nil = full replace
}
