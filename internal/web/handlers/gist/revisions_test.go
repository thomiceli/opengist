package gist_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/git"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

func TestRevisions(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	s.Register(t, "alice")

	t.Run("Revisions", func(t *testing.T) {
		_, gist, username, identifier := s.CreateGist(t, "0")

		s.Login(t, "thomas")
		s.Request(t, "POST", "/"+username+"/"+identifier+"/edit", url.Values{
			"title":   {"Test"},
			"name":    {"file.txt", "ok.txt"},
			"content": {"updated content", "okay"},
		}, 302)

		s.Request(t, "POST", "/"+username+"/"+identifier+"/edit", url.Values{
			"title":   {"Test"},
			"name":    {"renamed.txt", "ok.txt"},
			"content": {"updated content", "okay"},
		}, 302)

		commits, err := gist.Log(0)
		require.NoError(t, err)

		require.Len(t, commits, 3)

		require.Regexp(t, "^[a-f0-9]{40}$", commits[0].Hash)
		require.Regexp(t, "^[a-f0-9]{40}$", commits[1].Hash)
		require.Regexp(t, "^[a-f0-9]{40}$", commits[2].Hash)

		require.Equal(t, &git.Commit{
			Hash:       commits[0].Hash,
			Timestamp:  commits[0].Timestamp,
			AuthorName: "thomas",
			Changed:    "1 file changed, 0 insertions, 0 deletions",
			Files: []git.File{
				{
					Filename:    "renamed.txt",
					Size:        0,
					HumanSize:   "",
					OldFilename: "file.txt",
					Content:     ``,
					Truncated:   false,
					IsCreated:   false,
					IsDeleted:   false,
					IsBinary:    false,
					MimeType:    git.MimeType{},
				},
			},
		}, commits[0])

		require.Equal(t, &git.Commit{
			Hash:       commits[1].Hash,
			Timestamp:  commits[1].Timestamp,
			AuthorName: "thomas",
			Changed:    "3 files changed, 2 insertions, 2 deletions",
			Files: []git.File{
				{
					Filename:    "file.txt",
					OldFilename: "file.txt",
					Content: `@@ -1 +1 @@
-hello world
\ No newline at end of file
+updated content
\ No newline at end of file
`,
					IsCreated: false,
					IsDeleted: false,
					IsBinary:  false,
				}, {
					Filename:    "ok.txt",
					OldFilename: "",
					Content: `@@ -0,0 +1 @@
+okay
\ No newline at end of file
`,
					IsCreated: true,
					IsDeleted: false,
					IsBinary:  false,
				}, {
					Filename:    "otherfile.txt",
					OldFilename: "",
					Content: `@@ -1 +0,0 @@
-other content
\ No newline at end of file
`,
					IsCreated: false,
					IsDeleted: true,
					IsBinary:  false,
				},
			},
		}, commits[1])

		require.Equal(t, &git.Commit{
			Hash:       commits[2].Hash,
			Timestamp:  commits[2].Timestamp,
			AuthorName: "thomas",
			Changed:    "2 files changed, 2 insertions",
			Files: []git.File{
				{
					Filename:    "file.txt",
					OldFilename: "",
					Content: `@@ -0,0 +1 @@
+hello world
\ No newline at end of file
`,
					IsCreated: true,
					IsDeleted: false,
					IsBinary:  false,
				}, {
					Filename:    "otherfile.txt",
					OldFilename: "",
					Content: `@@ -0,0 +1 @@
+other content
\ No newline at end of file
`,
					IsCreated: true,
					IsDeleted: false,
					IsBinary:  false,
				},
			},
		}, commits[2])

		s.Request(t, "GET", "/"+username+"/"+identifier+"/revisions", nil, 200)
	})

	t.Run("NoAuth", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Logout()
		s.Request(t, "GET", "/"+username+"/"+identifier+"/revisions", nil, 200)
	})

	t.Run("PrivateGist", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "2")

		s.Login(t, "alice")
		s.Request(t, "GET", "/"+username+"/"+identifier+"/revisions", nil, 404)
	})
}
