package index

type Gist struct {
	GistID      uint
	UserID      uint
	Visibility  uint
	Username    string
	Description string
	Title       string
	Content     string
	Filenames   []string
	Extensions  []string
	Languages   []string
	Topics      []string
	CreatedAt   int64
	UpdatedAt   int64
}

type SearchGistMetadata struct {
	Username    string
	Title       string
	Description string
	Content     string
	Filename    string
	Extension   string
	Language    string
	Topic       string
	All         string
}
