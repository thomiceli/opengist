package gist_test

import (
	"encoding/json"
	"io"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

func setupManifestEntries() {
	context.ManifestEntries = map[string]context.Asset{
		"embed.css":   {File: "assets/embed.css"},
		"ts/embed.ts": {Css: []string{"assets/embed.css"}},
		"ts/light.ts": {Css: []string{"assets/light.css"}},
		"ts/dark.ts":  {Css: []string{"assets/dark.css"}},
	}
}

func TestGistIndex(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	s.Register(t, "alice")

	t.Run("Public", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Request(t, "GET", "/"+username+"/"+identifier, nil, 200)
	})

	t.Run("NonExistentRevision", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Request(t, "GET", "/"+username+"/"+identifier+"/rev/nonexistent", nil, 404)
	})

	t.Run("NonExistentGist", func(t *testing.T) {
		s.Request(t, "GET", "/thomas/nonexistent", nil, 404)
	})

	t.Run("Unlisted", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "1")

		s.Login(t, "thomas")
		s.Request(t, "GET", "/"+username+"/"+identifier, nil, 200)

		s.Login(t, "alice")
		s.Request(t, "GET", "/"+username+"/"+identifier, nil, 200)

		s.Logout()
		s.Request(t, "GET", "/"+username+"/"+identifier, nil, 200)
	})

	t.Run("Private", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "2")

		s.Login(t, "thomas")
		s.Request(t, "GET", "/"+username+"/"+identifier, nil, 200)

		s.Login(t, "alice")
		s.Request(t, "GET", "/"+username+"/"+identifier, nil, 404)

		s.Logout()
		s.Request(t, "GET", "/"+username+"/"+identifier, nil, 404)
	})

	t.Run("SpecificRevision", func(t *testing.T) {
		_, gist, username, identifier := s.CreateGist(t, "0")

		s.Login(t, "thomas")
		s.Request(t, "POST", "/"+username+"/"+identifier+"/edit", url.Values{
			"title":   {"Test"},
			"name":    {"file.txt"},
			"content": {"updated content"},
		}, 302)

		files, err := gist.Files("HEAD", false)
		require.NoError(t, err)
		found := false
		for _, f := range files {
			if f.Filename == "file.txt" {
				require.Equal(t, "updated content", f.Content)
				found = true
			}
		}
		require.True(t, found)

		commits, err := gist.Log(0)
		require.NoError(t, err)
		require.Len(t, commits, 2)

		filesOld, err := gist.Files(commits[1].Hash, false)
		require.NoError(t, err)
		for _, f := range filesOld {
			if f.Filename == "file.txt" {
				require.Equal(t, "hello world", f.Content)
			}
		}

		s.Request(t, "GET", "/"+username+"/"+identifier+"/rev/HEAD", nil, 200)
		s.Request(t, "GET", "/"+username+"/"+identifier+"/rev/"+commits[1].Hash, nil, 200)
	})
}

func TestPreview(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")

	t.Run("Markdown", func(t *testing.T) {
		s.Login(t, "thomas")

		resp := s.Request(t, "POST", "/preview", url.Values{
			"content": {"# Hello\n\nThis is **bold** and *italic*."},
		}, 200)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		html := string(body)
		require.Contains(t, html, "<h1>")
		require.Contains(t, html, "Hello")
		require.Contains(t, html, "<strong>bold</strong>")
		require.Contains(t, html, "<em>italic</em>")
	})

	t.Run("NoAuth", func(t *testing.T) {
		s.Logout()
		s.Request(t, "POST", "/preview", url.Values{
			"content": {"# Hello"},
		}, 302)
	})
}

func TestGistJson(t *testing.T) {
	setupManifestEntries()

	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	s.Register(t, "alice")

	t.Run("Public", func(t *testing.T) {
		_, gist, username, identifier := s.CreateGist(t, "0")

		resp := s.Request(t, "GET", "/"+username+"/"+identifier+".json", nil, 200)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		require.NoError(t, err)
		t.Helper()

		require.Equal(t, username, result["owner"])
		require.Equal(t, identifier, result["id"])
		require.Equal(t, gist.Uuid, result["uuid"])
		require.Equal(t, gist.Title, result["title"])
		require.Equal(t, "public", result["visibility"])
		require.Equal(t, []interface{}{"hello", "opengist"}, result["topics"])
		require.Equal(t, []interface{}{
			map[string]interface{}{
				"content":    "hello world",
				"filename":   "file.txt",
				"human_size": "11 B",
				"size":       float64(11),
				"truncated":  false,
				"type":       "Text",
			},
			map[string]interface{}{
				"content":    "other content",
				"filename":   "otherfile.txt",
				"human_size": "13 B",
				"size":       float64(13),
				"truncated":  false,
				"type":       "Text",
			},
		}, result["files"])

		embed, ok := result["embed"].(map[string]interface{})
		require.True(t, ok)
		require.Contains(t, embed["js"], identifier+".js")
		require.Contains(t, embed["js_dark"], identifier+".js?dark")
		require.NotEmpty(t, embed["css"])
		require.NotEmpty(t, embed["html"])
	})

	t.Run("Unlisted", func(t *testing.T) {
		s.Logout()
		_, _, username, identifier := s.CreateGist(t, "1")

		s.Request(t, "GET", "/"+username+"/"+identifier+".json", nil, 200)
	})

	t.Run("Private", func(t *testing.T) {
		s.Logout()
		_, _, username, identifier := s.CreateGist(t, "2")

		s.Request(t, "GET", "/"+username+"/"+identifier+".json", nil, 404)
	})

	t.Run("NonExistentGist", func(t *testing.T) {
		s.Request(t, "GET", "/thomas/nonexistent.json", nil, 404)
	})
}

func TestGistAccess(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	s.Register(t, "alice")

	_, _, user, publicId := s.CreateGist(t, "0")
	_, _, _, unlistedId := s.CreateGist(t, "1")
	_, _, _, privateId := s.CreateGist(t, "2")

	tests := []struct {
		name     string
		settings map[string]string
		// expected codes: [owner, otherUser, anonymous] x [public, unlisted, private]
		owner, otherUser, anonymous []int
	}{
		{
			name:      "Default",
			owner:     []int{200, 200, 200},
			otherUser: []int{200, 200, 404},
			anonymous: []int{200, 200, 404},
		},
		{
			name:      "RequireLogin",
			settings:  map[string]string{db.SettingRequireLogin: "1"},
			owner:     []int{200, 200, 200},
			otherUser: []int{200, 200, 404},
			anonymous: []int{302, 302, 302},
		},
		{
			name:      "AllowGistsWithoutLogin",
			settings:  map[string]string{db.SettingRequireLogin: "1", db.SettingAllowGistsWithoutLogin: "1"},
			owner:     []int{200, 200, 200},
			otherUser: []int{200, 200, 404},
			anonymous: []int{200, 200, 404},
		},
	}

	gists := []string{publicId, unlistedId, privateId}
	labels := []string{"Public", "Unlisted", "Private"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s.Login(t, "thomas")
			for k, v := range tt.settings {
				s.Request(t, "PUT", "/admin-panel/set-config", url.Values{"key": {k}, "value": {v}}, 200)
			}

			t.Run("Owner", func(t *testing.T) {
				s.Login(t, "thomas")
				for i, id := range gists {
					s.Request(t, "GET", "/"+user+"/"+id, nil, tt.owner[i])
				}
			})

			t.Run("OtherUser", func(t *testing.T) {
				s.Login(t, "alice")
				for i, id := range gists {
					s.Request(t, "GET", "/"+user+"/"+id, nil, tt.otherUser[i])
				}
			})

			t.Run("Anonymous", func(t *testing.T) {
				s.Logout()
				for i, id := range gists {
					t.Run(labels[i], func(t *testing.T) {
						s.Request(t, "GET", "/"+user+"/"+id, nil, tt.anonymous[i])
					})
				}
			})

			s.Login(t, "thomas")
			for k := range tt.settings {
				s.Request(t, "PUT", "/admin-panel/set-config", url.Values{"key": {k}, "value": {"0"}}, 200)
			}
		})
	}
}

func TestGetGistCaseInsensitive(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "THOmas")
	s.Login(t, "THOmas")

	s.Request(t, "POST", "/", url.Values{
		"title":   {"Test"},
		"name":    {"file.txt"},
		"content": {"hello world"},
		"url":     {"my-GIST"},
		"private": {"0"},
	}, 302)

	gist, err := db.GetGistByID("1")
	require.NoError(t, err)

	s.Logout()

	t.Run("URL", func(t *testing.T) {
		s.Request(t, "GET", "/thomas/my-gist", nil, 200)
		s.Request(t, "GET", "/THOMAS/MY-GIST", nil, 200)
		s.Request(t, "GET", "/thomas/MY-GIST", nil, 200)
		s.Request(t, "GET", "/THOMAS/my-gist", nil, 200)
	})

	t.Run("UUID", func(t *testing.T) {
		s.Request(t, "GET", "/thomas/"+strings.ToLower(gist.Uuid), nil, 200)
		s.Request(t, "GET", "/THOMAS/"+strings.ToUpper(gist.Uuid), nil, 200)
	})
}
