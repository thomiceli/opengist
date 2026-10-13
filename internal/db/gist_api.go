package db

import (
	"encoding/base64"
	"net/url"
	"strconv"
	"time"

	"github.com/thomiceli/opengist/internal/render/lang"
	"github.com/thomiceli/opengist/internal/web/handlers/api/v1/types"
)

// ToAPISimple returns the v1 API list-shape representation of the gist. baseURL is
// the scheme+host root (no trailing slash) the caller derived from config or
// the request.
func (gist *Gist) ToAPISimple(baseURL string) types.GistSimple {
	sshHost := ""
	if u, err := url.Parse(baseURL); err == nil {
		sshHost = u.Host
	}
	var expiresAt *time.Time
	if gist.ExpiresAt > 0 {
		t := time.Unix(gist.ExpiresAt, 0).UTC()
		expiresAt = &t
	}
	return types.GistSimple{
		ID:          gist.Uuid,
		Owner:       gist.User.ToSimpleAPI(),
		Title:       gist.Title,
		HTMLUrl:     baseURL + "/" + gist.User.Username + "/" + gist.Identifier(),
		SlugUrl:     gist.Identifier(),
		Description: gist.Description,
		Public:      gist.Private == PublicVisibility,
		Visibility:  gist.Private.String(),
		LikeCount:   gist.NbLikes,
		ForkCount:   gist.NbForks,
		CloneUrl:    gist.HTTPCloneURL(baseURL),
		SSHUrl:      gist.SSHCloneURL(sshHost),
		Topics:      gist.TopicsSlice(),
		Archived:    gist.Archived,
		CreatedAt:   time.Unix(gist.CreatedAt, 0).UTC(),
		UpdatedAt:   time.Unix(gist.UpdatedAt, 0).UTC(),
		ExpiresAt:   expiresAt,
	}
}

// ToAPI returns the v1 API detail-shape representation, including file
// contents at `revision` and the 10 most recent commits up to that
// revision. Pass "HEAD" for the current state; pass a SHA to render the
// gist (and its history) as it stood at that commit. Returns any error
// encountered while listing the gist's files or commit log (an unknown
// revision surfaces here as the Files error).
func (gist *Gist) ToAPI(baseURL string, revision string) (types.Gist, error) {
	files, truncated, err := gist.Files(revision, true)
	if err != nil {
		return types.Gist{}, err
	}
	fm := make(map[string]types.GistFile, len(files))
	for _, f := range files {
		ff := types.GistFile{
			Filename:  f.Filename,
			Type:      f.MimeType.ContentType,
			Language:  lang.Parse(f),
			Size:      int(f.Size),
			Truncated: f.Truncated,
		}
		if f.MimeType.CanBeEdited() {
			ff.Content = f.Content
			ff.Encoding = f.MimeType.Charset
		} else {
			ff.Content = base64.StdEncoding.EncodeToString([]byte(f.Content))
			ff.Encoding = "base64"
		}
		fm[f.Filename] = ff
	}
	var forked *types.GistSimple
	if gist.Forked != nil {
		ff := gist.Forked.ToAPISimple(baseURL)
		forked = &ff
	}
	forks, err := gist.GetForks(gist.UserID, 0, 11, 10)
	if err != nil {
		return types.Gist{}, err
	}
	forksMap := make([]types.GistSimple, 0)
	for _, fork := range forks {
		forksMap = append(forksMap, fork.ToAPISimple(baseURL))
	}

	logCommits, err := gist.Log(revision, 0, 10)
	if err != nil {
		return types.Gist{}, err
	}
	commits := make([]types.GistCommit, 0, len(logCommits))
	for _, c := range logCommits {
		commits = append(commits, c.ToAPI())
	}

	return types.Gist{
		GistSimple: gist.ToAPISimple(baseURL),
		ForkOf:     forked,
		Forks:      forksMap,
		Files:      fm,
		Commits:    commits,
		Truncated:  truncated,
	}, nil
}

// ToAPI converts a single resolved commit into its API shape. Shared by the
// gist-detail endpoint (10-most-recent embed) and the dedicated
// /gists/:uuid/commits endpoint so the wire shape is identical.
func (c *GistCommit) ToAPI() types.GistCommit {
	ts, _ := strconv.ParseInt(c.Timestamp, 10, 64)
	entry := types.GistCommit{
		Version: c.Hash,
		Author:  types.CommitAuthor{Name: c.AuthorName, Email: c.AuthorEmail},
		ChangeStatus: types.CommitChangeStatus{
			Files:     c.FilesChanged,
			Additions: c.Additions,
			Deletions: c.Deletions,
			Total:     c.Additions + c.Deletions,
		},
		CommittedAt: time.Unix(ts, 0).UTC(),
	}
	if c.User != nil {
		s := c.User.ToSimpleAPI()
		entry.User = &s
	}
	return entry
}
