package gist_test

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

// Helper function to extract username and gist identifier from redirect URL
func getGistInfoFromRedirect(resp *http.Response) (username, identifier string) {
	location := resp.Header.Get("Location")
	// Location format: /{username}/{identifier}
	parts := strings.Split(strings.TrimPrefix(location, "/"), "/")
	if len(parts) >= 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

func verifyGistCreation(t *testing.T, gist *db.Gist, username, identifier string) {
	require.NotNil(t, gist)
	require.Equal(t, username, gist.User.Username)
	require.Equal(t, identifier, gist.Identifier())
	require.NotEmpty(t, gist.Uuid)
	require.Greater(t, gist.NbFiles, 0)

	gistPath := filepath.Join(config.GetHomeDir(), git.ReposDirectory, username, gist.Uuid)
	_, err := os.Stat(gistPath)
	require.NoError(t, err, "Gist repository should exist on filesystem at %s", gistPath)
}

func TestGistCreationPage(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")

	t.Run("NoAuth", func(t *testing.T) {
		s.Request(t, "GET", "/", nil, 302)
	})

	t.Run("Authenticated", func(t *testing.T) {
		s.Login(t, "thomas")
		s.Request(t, "GET", "/", nil, 200)
	})
}

func TestGistCreation(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")

	t.Run("NoAuth", func(t *testing.T) {
		s.Request(t, "POST", "/", url.Values{
			"title":   {"Test Gist"},
			"name":    {"test.txt"},
			"content": {"hello world"},
		}, 302) // Redirects to login

		// Verify no gist was created
		count, err := db.CountAll(db.Gist{})
		require.NoError(t, err)
		require.Equal(t, int64(0), count)
	})

	tests := []struct {
		name                 string
		data                 url.Values
		expectedCode         int
		expectGistCreated    bool
		expectedTitle        string
		expectedDescription  string
		expectedURL          string
		expectedTopics       string // Expected topics string
		expectedNbFiles      int
		expectedVisibility   db.Visibility
		expectedFileNames    []string          // Expected filenames in the gist
		expectedFileContents map[string]string // Expected content for each file (filename -> content)
	}{
		{
			name: "NoFiles",
			data: url.Values{
				"title": {"Test Gist"},
			},
			expectedCode:      400,
			expectGistCreated: false,
		},
		{
			name: "EmptyContent",
			data: url.Values{
				"title":   {"Test Gist"},
				"name":    {"test.txt"},
				"content": {""},
			},
			expectedCode:      400,
			expectGistCreated: false,
		},
		{
			name: "TitleTooLong",
			data: url.Values{
				"title":   {strings.Repeat("a", 251)}, // Max is 250
				"name":    {"test.txt"},
				"content": {"hello"},
			},
			expectedCode:      400,
			expectGistCreated: false,
		},
		{
			name: "DescriptionTooLong",
			data: url.Values{
				"title":       {"Test Gist"},
				"description": {strings.Repeat("a", 1001)}, // Max is 1000
				"name":        {"test.txt"},
				"content":     {"hello"},
			},
			expectedCode:      400,
			expectGistCreated: false,
		},
		{
			name: "URLTooLong",
			data: url.Values{
				"title":   {"Test Gist"},
				"url":     {strings.Repeat("a", 33)}, // Max is 32
				"name":    {"test.txt"},
				"content": {"hello"},
			},
			expectedCode:      400,
			expectGistCreated: false,
		},
		{
			name: "URLInvalidCharacters",
			data: url.Values{
				"title":   {"Test Gist"},
				"url":     {"invalid@url#here"}, // Only alphanumeric and dashes allowed
				"name":    {"test.txt"},
				"content": {"hello"},
			},
			expectedCode:      400,
			expectGistCreated: false,
		},
		{
			name: "InvalidVisibility",
			data: url.Values{
				"title":   {"Test Gist"},
				"name":    {"test.txt"},
				"content": {"hello"},
				"private": {"3"}, // Valid values are 0, 1, 2
			},
			expectedCode:      400,
			expectGistCreated: false,
		},
		{
			name: "Valid",
			data: url.Values{
				"title":   {"My Test Gist"},
				"name":    {"test.txt"},
				"url":     {"my-custom-url-123"}, // Alphanumeric + dashes should be valid
				"content": {"hello world"},
				"private": {"0"},
			},
			expectedCode:       302,
			expectGistCreated:  true,
			expectedTitle:      "My Test Gist",
			expectedNbFiles:    1,
			expectedVisibility: db.PublicVisibility,
			expectedFileNames:  []string{"test.txt"},
			expectedFileContents: map[string]string{
				"test.txt": "hello world",
			},
		},
		{
			name: "AutoNamedFile",
			data: url.Values{
				"title":   {"Auto Named"},
				"name":    {""},
				"content": {"content without name"},
				"private": {"0"},
			},
			expectedCode:       302,
			expectGistCreated:  true,
			expectedTitle:      "Auto Named",
			expectedNbFiles:    1,
			expectedVisibility: db.PublicVisibility,
			expectedFileNames:  []string{"gistfile1.txt"},
			expectedFileContents: map[string]string{
				"gistfile1.txt": "content without name",
			},
		},
		{
			name: "MultipleFiles",
			data: url.Values{
				"title":   {"Multi File Gist"},
				"name":    []string{"", "file2.md"},
				"content": []string{"content 1", "content 2"},
				"private": {"0"},
			},
			expectedCode:       302,
			expectGistCreated:  true,
			expectedTitle:      "Multi File Gist",
			expectedNbFiles:    2,
			expectedVisibility: db.PublicVisibility,
			expectedFileNames:  []string{"gistfile1.txt", "file2.md"},
			expectedFileContents: map[string]string{
				"gistfile1.txt": "content 1",
				"file2.md":      "content 2",
			},
		},
		{
			name: "NoTitle",
			data: url.Values{
				"name":    {"readme.md"},
				"content": {"# README"},
				"private": {"0"},
			},
			expectedCode:       302,
			expectGistCreated:  true,
			expectedTitle:      "readme.md",
			expectedNbFiles:    1,
			expectedVisibility: db.PublicVisibility,
			expectedFileNames:  []string{"readme.md"},
			expectedFileContents: map[string]string{
				"readme.md": "# README",
			},
		},
		{
			name: "Unlisted",
			data: url.Values{
				"title":   {"Unlisted Gist"},
				"name":    {"secret.txt"},
				"content": {"secret content"},
				"private": {"1"},
			},
			expectedCode:       302,
			expectGistCreated:  true,
			expectedTitle:      "Unlisted Gist",
			expectedNbFiles:    1,
			expectedVisibility: db.UnlistedVisibility,
			expectedFileNames:  []string{"secret.txt"},
			expectedFileContents: map[string]string{
				"secret.txt": "secret content",
			},
		},
		{
			name: "Private",
			data: url.Values{
				"title":   {"Private Gist"},
				"name":    {"secret.txt"},
				"content": {"secret content"},
				"private": {"2"},
			},
			expectedCode:       302,
			expectGistCreated:  true,
			expectedTitle:      "Private Gist",
			expectedNbFiles:    1,
			expectedVisibility: db.PrivateVisibility,
			expectedFileNames:  []string{"secret.txt"},
			expectedFileContents: map[string]string{
				"secret.txt": "secret content",
			},
		},
		{
			name: "Topics",
			data: url.Values{
				"title":   {"Gist With Topics"},
				"name":    {"test.txt"},
				"content": {"hello"},
				"topics":  {"golang testing webdev"},
			},
			expectedCode:       302,
			expectGistCreated:  true,
			expectedTitle:      "Gist With Topics",
			expectedTopics:     "golang,testing,webdev",
			expectedNbFiles:    1,
			expectedVisibility: db.PublicVisibility,
			expectedFileNames:  []string{"test.txt"},
			expectedFileContents: map[string]string{
				"test.txt": "hello",
			},
		},
		{
			name: "TopicsTooMany",
			data: url.Values{
				"title":   {"Test"},
				"name":    {"test.txt"},
				"content": {"hello"},
				"topics":  {"topic1 topic2 topic3 topic4 topic5 topic6 topic7 topic8 topic9 topic10 topic11"},
			},
			expectedCode:      400,
			expectGistCreated: false,
		},
		{
			name: "TopicTooLong",
			data: url.Values{
				"title":   {"Test"},
				"name":    {"test.txt"},
				"content": {"hello"},
				"topics":  {strings.Repeat("a", 51)}, // 51 chars - exceeds max of 50
			},
			expectedCode:      400,
			expectGistCreated: false,
		},
		{
			name: "TopicInvalidCharacters",
			data: url.Values{
				"title":   {"Test"},
				"name":    {"test.txt"},
				"content": {"hello"},
				"topics":  {"topic@name topic.name"},
			},
			expectedCode:      400,
			expectGistCreated: false,
		},
		{
			name: "TopicUnicode",
			data: url.Values{
				"title":   {"Unicode Topics"},
				"name":    {"test.txt"},
				"content": {"hello"},
				"topics":  {"编程 тест"},
			},
			expectedCode:       302,
			expectGistCreated:  true,
			expectedTitle:      "Unicode Topics",
			expectedTopics:     "编程,тест",
			expectedNbFiles:    1,
			expectedVisibility: db.PublicVisibility,
			expectedFileNames:  []string{"test.txt"},
		},
		{
			name: "DuplicateFileNames",
			data: url.Values{
				"title":   {"Duplicate Files"},
				"name":    []string{"test.txt", "test.txt"},
				"content": []string{"content1", "content2"},
			},
			expectedCode:       302,
			expectGistCreated:  true,
			expectedTitle:      "Duplicate Files",
			expectedNbFiles:    1,
			expectedVisibility: db.PublicVisibility,
			expectedFileNames:  []string{"test.txt"},
			expectedFileContents: map[string]string{
				"test.txt": "content2",
			},
		},
		{
			name: "FileNameTooLong",
			data: url.Values{
				"title":   {"Too Long Filename"},
				"name":    {strings.Repeat("a", 256) + ".txt"}, // 260 total - exceeds 255
				"content": {"hello"},
			},
			expectedCode:      400,
			expectGistCreated: false,
		},
		{
			name: "FileNameWithUnicode",
			data: url.Values{
				"title":   {"Unicode Filename"},
				"name":    {"文件.txt"},
				"content": {"hello world"},
			},
			expectedCode:       302,
			expectGistCreated:  true,
			expectedTitle:      "Unicode Filename",
			expectedNbFiles:    1,
			expectedVisibility: db.PublicVisibility,
			expectedFileNames:  []string{"文件.txt"},
			expectedFileContents: map[string]string{
				"文件.txt": "hello world",
			},
		},
		{
			name: "FileNamePathTraversal",
			data: url.Values{
				"title":   {"Path Traversal"},
				"name":    {"../../../etc/passwd"},
				"content": {"malicious"},
			},
			expectedCode:       302,
			expectGistCreated:  true,
			expectedTitle:      "Path Traversal",
			expectedNbFiles:    1,
			expectedVisibility: db.PublicVisibility,
			expectedFileNames:  []string{"passwd"},
			expectedFileContents: map[string]string{
				"passwd": "malicious",
			},
		},
		{
			name: "EmptyAndValidContent",
			data: url.Values{
				"title":   {"Mixed Content"},
				"name":    []string{"empty.txt", "valid.txt", "also-empty.txt"},
				"content": []string{"", "valid content", ""},
			},
			expectedCode:       302,
			expectGistCreated:  true,
			expectedTitle:      "Mixed Content",
			expectedNbFiles:    1,
			expectedVisibility: db.PublicVisibility,
			expectedFileNames:  []string{"valid.txt"},
			expectedFileContents: map[string]string{
				"valid.txt": "valid content",
			},
		},
		{
			name: "ContentWithSpecialCharacters",
			data: url.Values{
				"title":   {"Special Chars"},
				"name":    {"special.txt"},
				"content": {"Line1\nLine2\tTabbed\x00NullByte😀Emoji"},
			},
			expectedCode:       302,
			expectGistCreated:  true,
			expectedTitle:      "Special Chars",
			expectedNbFiles:    1,
			expectedVisibility: db.PublicVisibility,
			expectedFileNames:  []string{"special.txt"},
			expectedFileContents: map[string]string{
				"special.txt": "Line1\nLine2\tTabbed\x00NullByte😀Emoji",
			},
		},
		{
			name: "ContentMultibyteUnicode",
			data: url.Values{
				"title":   {"Unicode Content"},
				"name":    {"unicode.txt"},
				"content": {"Hello 世界 🌍 Привет"},
			},
			expectedCode:       302,
			expectGistCreated:  true,
			expectedTitle:      "Unicode Content",
			expectedNbFiles:    1,
			expectedVisibility: db.PublicVisibility,
			expectedFileNames:  []string{"unicode.txt"},
			expectedFileContents: map[string]string{
				"unicode.txt": "Hello 世界 🌍 Привет",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s.Login(t, "thomas")

			resp := s.Request(t, "POST", "/", tt.data, tt.expectedCode)

			if tt.expectGistCreated {
				// Get gist info from redirect
				username, gistIdentifier := getGistInfoFromRedirect(resp)
				require.Equal(t, "thomas", username)
				require.NotEmpty(t, gistIdentifier)

				// Verify gist was created
				gist, err := db.GetGist(username, gistIdentifier)
				require.NoError(t, err)

				// Run common verification (filesystem, git, etc.)
				verifyGistCreation(t, gist, username, gistIdentifier)

				// Verify all expected fields
				require.Equal(t, tt.expectedTitle, gist.Title, "Title mismatch")
				require.Equal(t, tt.expectedNbFiles, gist.NbFiles, "File count mismatch")
				require.Equal(t, tt.expectedVisibility, gist.Private, "Visibility mismatch")

				// Verify description if specified
				if tt.expectedDescription != "" {
					require.Equal(t, tt.expectedDescription, gist.Description, "Description mismatch")
				}

				// Verify URL if specified
				if tt.expectedURL != "" {
					require.Equal(t, tt.expectedURL, gist.Identifier(), "URL/Identifier mismatch")
				}

				// Verify topics if specified
				if tt.expectedTopics != "" {
					// Get gist topics
					topics, err := gist.GetTopics()
					require.NoError(t, err, "Failed to get gist topics")
					require.ElementsMatch(t, strings.Split(tt.expectedTopics, ","), topics, "Topics mismatch")
				}

				// Verify files if specified
				if len(tt.expectedFileNames) > 0 {
					files, err := gist.Files("HEAD", false)
					require.NoError(t, err, "Failed to get gist files")
					require.Len(t, files, len(tt.expectedFileNames), "File count mismatch")

					actualFileNames := make([]string, len(files))
					for i, file := range files {
						actualFileNames[i] = file.Filename
					}
					require.ElementsMatch(t, tt.expectedFileNames, actualFileNames, "File names mismatch")

					// Verify file contents if specified
					if len(tt.expectedFileContents) > 0 {
						for filename, expectedContent := range tt.expectedFileContents {
							content, _, err := git.GetFileContent(username, gist.Uuid, "HEAD", filename, false)
							require.NoError(t, err, "Failed to get content for file %s", filename)
							require.Equal(t, expectedContent, content, "Content mismatch for file %s", filename)
						}
					}
				}
			}
		})
	}
}
