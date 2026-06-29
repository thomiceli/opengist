package git_test

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
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

func TestGitPushArchived(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	baseUrl := s.StartHttpServer(t)

	s.Register(t, "thomas")

	_, _, user, gistId := s.CreateGist(t, "0")

	dest := t.TempDir()
	require.NoError(t, gitClone(baseUrl, "thomas:thomas", user, gistId, dest))

	// Pushing works before the gist is archived.
	require.NoError(t, gitPush(dest, "before.txt", "content"))

	// Archive the gist.
	s.Login(t, "thomas")
	s.Request(t, "POST", "/"+user+"/"+gistId+"/archive", nil, 302)

	// Pushing to an archived gist is rejected, even by the owner.
	require.Error(t, gitPush(dest, "after.txt", "content"))

	// Unarchive the gist.
	s.Request(t, "POST", "/"+user+"/"+gistId+"/archive", nil, 302)

	// Pushing works again once unarchived.
	require.NoError(t, gitPush(dest, "afterunarchive.txt", "content"))
}

// gitInitAndPushTo initializes a fresh repo with a single file and pushes it to
// remotePath (e.g. "/init") as creds. It returns whatever git push returns.
func gitInitAndPushTo(baseUrl, creds, remotePath, filename, content string, destDir string) error {
	if err := exec.Command("git", "init", "--initial-branch=master", destDir).Run(); err != nil {
		return err
	}
	remote := "http://" + creds + "@" + baseUrl[len("http://"):] + remotePath
	if err := exec.Command("git", "-C", destDir, "remote", "add", "origin", remote).Run(); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(destDir, filename), []byte(content), 0644); err != nil {
		return err
	}
	if err := exec.Command("git", "-C", destDir, "add", ".").Run(); err != nil {
		return err
	}
	if err := exec.Command("git", "-C", destDir, "commit", "-m", "init").Run(); err != nil {
		return err
	}
	return exec.Command("git", "-C", destDir, "push", "origin", "master").Run()
}

// TestGitInitPushParallel hammers the /init create-by-push flow concurrently.
// Before the per-push correlation token, the two HTTP requests git makes
// (info/refs then git-receive-pack) were matched via a per-user FIFO queue, so
// interleaved pushes desynced it: pushes 500'd or content landed in the wrong
// gist. This asserts every parallel push lands in its own gist with its own
// content, and that the queue drains to empty.
func TestGitInitPushParallel(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	baseUrl := s.StartHttpServer(t)
	s.Register(t, "thomas")

	const n = 10
	creds := "thomas:thomas"

	// Each push writes a distinct file/content so we can detect misrouting.
	want := make(map[string]string, n)
	for i := 0; i < n; i++ {
		want[fmt.Sprintf("file-%d.txt", i)] = fmt.Sprintf("content-%d", i)
	}

	// Pre-create the temp dirs on the main goroutine, then push from all of them
	// at once.
	dirs := make([]string, n)
	for i := 0; i < n; i++ {
		dirs[i] = t.TempDir()
	}

	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = gitInitAndPushTo(baseUrl, creds, "/init",
				fmt.Sprintf("file-%d.txt", i), fmt.Sprintf("content-%d", i), dirs[i])
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		require.NoErrorf(t, err, "parallel /init push %d failed", i)
	}

	user, err := db.GetUserByUsername("thomas")
	require.NoError(t, err)

	// Exactly n gists were created, one per push.
	count, err := db.CountAllGistsFromUser(user.ID, user.ID)
	require.NoError(t, err)
	require.EqualValues(t, n, count, "expected one gist per parallel push")

	// Every pushed file/content is present exactly once across the gists, proving
	// no push's content was routed into another push's gist.
	gists, err := db.GetAllGistsOfUser(user.ID, nil, 0, "created", "desc", 100, 100)
	require.NoError(t, err)

	got := make(map[string]string, n)
	for _, g := range gists {
		files, _, err := g.Files("HEAD", false)
		require.NoError(t, err)
		require.Len(t, files, 1, "each init gist should contain exactly one file")
		f := files[0]
		_, dup := got[f.Filename]
		require.Falsef(t, dup, "file %q appeared in more than one gist", f.Filename)
		got[f.Filename] = f.Content
	}
	require.Equal(t, want, got, "pushed files/content did not map one-to-one onto gists")

	// The init queue must drain completely, leaving no stale correlation entries.
	queued, err := db.CountAll(&db.GistInitQueue{})
	require.NoError(t, err)
	require.EqualValues(t, 0, queued, "init queue should be empty after all pushes complete")
}

func TestGitAuthWithAccessToken(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	baseUrl := s.StartHttpServer(t)

	s.Register(t, "thomas")

	_, _, user, privateId := s.CreateGist(t, "2")

	// CreateGist logs out at the end; log back in to create tokens for thomas.
	s.Login(t, "thomas")

	rwToken := s.CreateAccessToken(t, "rw", db.ReadWritePermission, db.NoPermission)
	roToken := s.CreateAccessToken(t, "ro", db.ReadPermission, db.NoPermission)
	noToken := s.CreateAccessToken(t, "none", db.NoPermission, db.NoPermission)

	tests := []struct {
		name  string
		token string
		// clone (pull) requires gist read permission, push requires gist write permission
		canClone bool
		canPush  bool
	}{
		{"ReadWriteToken", rwToken, true, true},
		{"ReadOnlyToken", roToken, true, false},
		{"NoGistPermissionToken", noToken, false, false},
		{"InvalidToken", "og_deadbeef", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := "thomas:" + tt.token

			// Clone (pull) the private gist using the token as the password.
			dest := t.TempDir()
			err := gitClone(baseUrl, creds, user, privateId, dest)
			if tt.canClone {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				return
			}

			// Push to the gist using the token as the password.
			err = gitPush(dest, "token.txt", "from token")
			if tt.canPush {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
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
