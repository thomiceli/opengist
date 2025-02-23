package index

type Gist struct {
	GistID     uint
	UserID     uint
	Visibility uint
	Username   string
	Title      string
	Content    string
	Filenames  []string
	Extensions []string
	Languages  []string
	Topics     []string
	CreatedAt  int64
	UpdatedAt  int64
}

type SearchGistMetadata struct {
	Username  string
	Title     string
	Filename  string
	Extension string
	Language  string
	Topic     string
}
