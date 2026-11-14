package ssh_test

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/ssh"
	webtest "github.com/thomiceli/opengist/internal/web/test"
	"gorm.io/gorm"
)

// startSSHServer runs the embedded SSH server on a random local port until the
// test ends, and returns its base URL.
func startSSHServer(t *testing.T) string {
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("ssh client not available")
	}

	serverConfig, err := ssh.NewServerConfig()
	require.NoError(t, err)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	go ssh.Serve(listener, serverConfig)
	return "ssh://" + listener.Addr().String()
}

// addSSHKey adds an SSH key to user and returns the path of its private key.
func addSSHKey(t *testing.T, s *webtest.Server, user string) string {
	_, privateKey := s.AddSSHKey(t, user)
	keyPath := filepath.Join(t.TempDir(), "id_ed25519")
	require.NoError(t, os.WriteFile(keyPath, privateKey, 0600))
	return keyPath
}

// gitSSH runs git in dir, authenticating over SSH with the private key at
// keyPath, and returns its combined output.
func gitSSH(t *testing.T, keyPath, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_SSH_COMMAND=ssh -F /dev/null -i "+keyPath+
		" -o IdentitiesOnly=yes -o BatchMode=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// newRepo initializes a Git repository holding one commit of hello.txt.
func newRepo(t *testing.T) string {
	dir := t.TempDir()
	require.NoError(t, exec.Command("git", "init", "--initial-branch=master", dir).Run())
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello"), 0644))
	require.NoError(t, exec.Command("git", "-C", dir, "add", "hello.txt").Run())
	require.NoError(t, exec.Command("git", "-C", dir, "commit", "-m", "initial").Run())
	return dir
}

// gistIDs returns the IDs of every gist in the database.
func gistIDs(t *testing.T) map[uint]bool {
	gists, err := db.GetAllGistsRows()
	require.NoError(t, err)
	ids := make(map[uint]bool, len(gists))
	for _, gist := range gists {
		ids[gist.ID] = true
	}
	return ids
}

// newGists returns the gists created since gistIDs returned before.
func newGists(t *testing.T, before map[uint]bool) []*db.Gist {
	gists, err := db.GetAllGistsRows()
	require.NoError(t, err)
	var created []*db.Gist
	for _, gist := range gists {
		if !before[gist.ID] {
			created = append(created, gist)
		}
	}
	return created
}

func TestSSHPushCreatesGist(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	s.Register(t, "alice")
	key := addSSHKey(t, s, "thomas")
	baseUrl := startSSHServer(t)

	tests := []struct {
		name    string
		path    string
		expect  bool
		gistUrl string // URL of the created gist, empty for a random identifier
	}{
		{"Init", "/init.git", true, ""},
		{"InitWithoutSuffix", "/init", true, ""},
		{"OwnNamespace", "/thomas/mygist.git", true, "mygist"},
		{"OwnNamespaceKeepsCase", "/thomas/MyGist2", true, "MyGist2"},
		{"OtherUserNamespace", "/alice/hackgist.git", false, ""},
		{"UnknownUser", "/nobody/gist.git", false, ""},
		{"InvalidUrl", "/thomas/not.valid.git", false, ""},
		{"NestedPath", "/thomas/a/b.git", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := gistIDs(t)

			out, err := gitSSH(t, key, newRepo(t), "push", baseUrl+tt.path, "master")
			created := newGists(t, before)
			if !tt.expect {
				require.Error(t, err, out)
				require.Empty(t, created, "a rejected push must not create a gist")
				return
			}
			require.NoError(t, err, out)
			require.Len(t, created, 1)

			gist := created[0]
			require.Equal(t, "thomas", gist.User.Username)
			require.Equal(t, tt.gistUrl, gist.URL)
			files, err := gist.FileNames("HEAD")
			require.NoError(t, err)
			require.Equal(t, []string{"hello.txt"}, files)

			// The new gist is a regular one, reachable at its own path.
			clone := filepath.Join(t.TempDir(), "clone")
			out, err = gitSSH(t, key, t.TempDir(), "clone", baseUrl+"/thomas/"+gist.Identifier()+".git", clone)
			require.NoError(t, err, out)
			require.FileExists(t, filepath.Join(clone, "hello.txt"))
		})
	}
}

func TestSSHPushCreatesNoEmptyGist(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	key := addSSHKey(t, s, "thomas")
	baseUrl := startSSHServer(t)

	before := gistIDs(t)

	// A dry run talks to receive-pack but sends nothing.
	out, err := gitSSH(t, key, newRepo(t), "push", "--dry-run", baseUrl+"/init.git", "master")
	require.NoError(t, err, out)
	require.Empty(t, newGists(t, before))

	// Deleting a branch the new gist doesn't have stores nothing either.
	out, err = gitSSH(t, key, newRepo(t), "push", baseUrl+"/thomas/nothing.git", ":master")
	require.Error(t, err, out)
	require.Empty(t, newGists(t, before))

	// "init" only accepts pushes.
	out, err = gitSSH(t, key, t.TempDir(), "clone", baseUrl+"/init.git", "clone")
	require.Error(t, err, out)
	require.Empty(t, newGists(t, before))
}

func TestSSHPushToExistingGist(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	_, gist, user, identifier := s.CreateGist(t, "0")
	s.Register(t, "alice")
	thomasKey := addSSHKey(t, s, "thomas")
	aliceKey := addSSHKey(t, s, "alice")
	baseUrl := startSSHServer(t)

	before := gistIDs(t)

	dir := filepath.Join(t.TempDir(), "clone")
	out, err := gitSSH(t, thomasKey, t.TempDir(), "clone", baseUrl+"/"+user+"/"+identifier+".git", dir)
	require.NoError(t, err, out)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new"), 0644))
	require.NoError(t, exec.Command("git", "-C", dir, "add", "new.txt").Run())
	require.NoError(t, exec.Command("git", "-C", dir, "commit", "-m", "new").Run())

	// Only the owner may push to the gist.
	out, err = gitSSH(t, aliceKey, dir, "push", "origin", "master")
	require.Error(t, err, out)

	out, err = gitSSH(t, thomasKey, dir, "push", "origin", "master")
	require.NoError(t, err, out)
	files, err := gist.FileNames("HEAD")
	require.NoError(t, err)
	require.Contains(t, files, "new.txt")

	require.Empty(t, newGists(t, before), "pushing to an existing gist must not create one")
}

func TestAuthorizeGitCommandCreatesGist(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	key, _ := s.AddSSHKey(t, "thomas")

	// scp-like remotes (host:init.git) send the path without a leading slash.
	gist, verb, created, err := ssh.AuthorizeGitCommand("git-receive-pack 'init.git'", key.Content, "127.0.0.1")
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, "receive-pack", verb)
	require.Equal(t, "thomas", gist.User.Username)
	require.Empty(t, gist.URL)

	repoPath := git.RepositoryPath(gist.User.Username, gist.Uuid)
	require.DirExists(t, repoPath)

	// Nothing was pushed to it, so the gist is deleted along with its repository.
	ssh.DiscardEmptyGist(gist)
	_, err = db.GetGistByUUID(gist.Uuid)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.NoDirExists(t, repoPath)

	// Cloning "init" is not a thing.
	_, _, _, err = ssh.AuthorizeGitCommand("git-upload-pack 'init.git'", key.Content, "127.0.0.1")
	require.EqualError(t, err, "invalid gist path")
}
