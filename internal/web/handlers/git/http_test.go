package git_test

import (
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

func gitClone(baseUrl, creds, user, gistId, destDir string) error {
	authUrl := baseUrl
	if creds != "" {
		authUrl = "http://" + creds + "@" + baseUrl[len("http://"):]
	}
	return exec.Command("git", "clone", authUrl+"/"+user+"/"+gistId+".git", destDir).Run()
}

func gitPush(repoDir, filename, content string) error {
	if err := os.WriteFile(filepath.Join(repoDir, filename), []byte(content), 0644); err != nil {
		return err
	}
	if err := exec.Command("git", "-C", repoDir, "add", filename).Run(); err != nil {
		return err
	}
	if err := exec.Command("git", "-C", repoDir, "commit", "-m", "add "+filename).Run(); err != nil {
		return err
	}
	return exec.Command("git", "-C", repoDir, "push", "origin").Run()
}

func TestGitClonePull(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	baseUrl := s.StartHttpServer(t)

	s.Register(t, "thomas")
	s.Register(t, "alice")

	_, _, user, publicId := s.CreateGist(t, "0")
	_, _, _, unlistedId := s.CreateGist(t, "1")
	_, _, _, privateId := s.CreateGist(t, "2")

	type credTest struct {
		name   string
		creds  string
		expect [3]bool // [public, unlisted, private]
	}

	tests := []struct {
		name     string
		settings map[string]string
		creds    []credTest
	}{
		{
			name: "Default",
			creds: []credTest{
				{"OwnerAuth", "thomas:thomas", [3]bool{true, true, true}},
				{"OtherUserAuth", "alice:alice", [3]bool{true, true, false}},
				{"WrongPassword", "thomas:wrong", [3]bool{true, true, false}},
				{"WrongUser", "aze:aze", [3]bool{true, true, false}},
				{"Anonymous", "", [3]bool{true, true, false}},
			},
		},
		{
			name:     "RequireLogin",
			settings: map[string]string{db.SettingRequireLogin: "1"},
			creds: []credTest{
				{"OwnerAuth", "thomas:thomas", [3]bool{true, true, true}},
				{"OtherUserAuth", "alice:alice", [3]bool{true, true, false}},
				{"WrongPassword", "thomas:wrong", [3]bool{false, false, false}},
				{"WrongUser", "aze:aze", [3]bool{false, false, false}},
				{"Anonymous", "", [3]bool{false, false, false}},
			},
		},
		{
			name:     "AllowGistsWithoutLogin",
			settings: map[string]string{db.SettingRequireLogin: "1", db.SettingAllowGistsWithoutLogin: "1"},
			creds: []credTest{
				{"OwnerAuth", "thomas:thomas", [3]bool{true, true, true}},
				{"OtherUserAuth", "alice:alice", [3]bool{true, true, false}},
				{"WrongPassword", "thomas:wrong", [3]bool{true, true, false}},
				{"WrongUser", "aze:aze", [3]bool{true, true, false}},
				{"Anonymous", "", [3]bool{true, true, false}},
			},
		},
	}

	gists := [3]string{publicId, unlistedId, privateId}
	labels := [3]string{"Public", "Unlisted", "Private"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s.Login(t, "thomas")
			for k, v := range tt.settings {
				s.Request(t, "PUT", "/admin-panel/set-config", url.Values{"key": {k}, "value": {v}}, 200)
			}

			for _, ct := range tt.creds {
				t.Run(ct.name, func(t *testing.T) {
					for i, id := range gists {
						t.Run(labels[i], func(t *testing.T) {
							dest := t.TempDir()
							err := gitClone(baseUrl, ct.creds, user, id, dest)
							if ct.expect[i] {
								require.NoError(t, err)
								_, err = os.Stat(filepath.Join(dest, "file.txt"))
								require.NoError(t, err)
							} else {
								require.Error(t, err)
							}
						})
					}
				})
			}

			// Reset settings
			s.Login(t, "thomas")
			for k := range tt.settings {
				s.Request(t, "PUT", "/admin-panel/set-config", url.Values{"key": {k}, "value": {"0"}}, 200)
			}
		})
	}
}

func TestGitPush(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	baseUrl := s.StartHttpServer(t)

	s.Register(t, "thomas")
	s.Register(t, "alice")

	_, _, user, publicId := s.CreateGist(t, "0")
	_, _, _, unlistedId := s.CreateGist(t, "1")
	_, _, _, privateId := s.CreateGist(t, "2")

	type credTest struct {
		name   string
		creds  string
		expect [3]bool // [public, unlisted, private]
	}

	tests := []credTest{
		{"OwnerAuth", "thomas:thomas", [3]bool{true, true, true}},
		{"OtherUserAuth", "alice:alice", [3]bool{false, false, false}},
		{"WrongPassword", "thomas:wrong", [3]bool{false, false, false}},
		{"WrongUser", "aze:aze", [3]bool{false, false, false}},
		{"Anonymous", "", [3]bool{false, false, false}},
	}

	gists := [3]string{publicId, unlistedId, privateId}
	labels := [3]string{"Public", "Unlisted", "Private"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i, id := range gists {
				t.Run(labels[i], func(t *testing.T) {
					dest := t.TempDir()
					require.NoError(t, gitClone(baseUrl, "thomas:thomas", user, id, dest))

					if tt.creds != "thomas:thomas" {
						require.NoError(t, exec.Command("git", "-C", dest, "remote", "set-url", "origin",
							"http://"+tt.creds+"@"+baseUrl[len("http://"):]+"/"+user+"/"+id+".git").Run())
					}

					err := gitPush(dest, "newfile.txt", "new content")
					if tt.expect[i] {
						require.NoError(t, err)
					} else {
						require.Error(t, err)
					}
				})
			}
		})
	}
}

func TestGitCreatePush(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	baseUrl := s.StartHttpServer(t)

	s.Register(t, "thomas")
	s.Register(t, "alice")

	gitInitAndPush := func(t *testing.T, creds, remoteUrl string) error {
		dest := t.TempDir()
		require.NoError(t, exec.Command("git", "init", "--initial-branch=master", dest).Run())
		require.NoError(t, exec.Command("git", "-C", dest, "remote", "add", "origin",
			"http://"+creds+"@"+baseUrl[len("http://"):]+remoteUrl).Run())

		require.NoError(t, os.WriteFile(filepath.Join(dest, "hello.txt"), []byte("hello"), 0644))
		require.NoError(t, exec.Command("git", "-C", dest, "add", "hello.txt").Run())
		require.NoError(t, exec.Command("git", "-C", dest, "commit", "-m", "initial").Run())
		return exec.Command("git", "-C", dest, "push", "origin").Run()
	}

	tests := []struct {
		name      string
		creds     string
		url       string
		expect    bool
		gistOwner string // if expect=true, verify gist exists at this owner/identifier
		gistId    string
	}{
		{"OwnerCreates", "thomas:thomas", "/thomas/mygist.git", true, "thomas", "mygist"},
		{"OtherUserCreatesOnOwnUrl", "alice:alice", "/alice/alicegist.git", true, "alice", "alicegist"},
		{"WrongPassword", "thomas:wrong", "/thomas/newgist.git", false, "", ""},
		{"OtherUserCannotCreateOnOwner", "alice:alice", "/thomas/hackgist.git", false, "", ""},
		{"WrongUser", "aze:aze", "/aze/azegist.git", false, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := gitInitAndPush(t, tt.creds, tt.url)
			if tt.expect {
				require.NoError(t, err)
				gist, err := db.GetGist(tt.gistOwner, tt.gistId)
				require.NoError(t, err)
				require.NotNil(t, gist)
				require.Equal(t, tt.gistId, gist.Identifier())
			} else {
				require.Error(t, err)
			}
		})
	}
}
