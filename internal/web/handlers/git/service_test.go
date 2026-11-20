package git_test

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

// newReceivePackBody clones a gist as its owner, commits a new file and returns
// a receive-pack request body moving refs/heads/master to that commit, along
// with the current and the new commit IDs.
func newReceivePackBody(t *testing.T, baseUrl, user, gistId string) (body []byte, oldRev, newRev string) {
	dest := t.TempDir()
	require.NoError(t, gitClone(baseUrl, "thomas:thomas", user, gistId, dest))

	gitOut := func(stdin string, args ...string) []byte {
		cmd := exec.Command("git", append([]string{"-C", dest}, args...)...)
		cmd.Stdin = strings.NewReader(stdin)
		out, err := cmd.Output()
		require.NoError(t, err)
		return out
	}

	oldRev = strings.TrimSpace(string(gitOut("", "rev-parse", "HEAD")))
	require.NoError(t, os.WriteFile(filepath.Join(dest, "pushed.txt"), []byte(t.Name()), 0644))
	gitOut("", "add", "pushed.txt")
	gitOut("", "commit", "-m", "add pushed.txt")
	newRev = strings.TrimSpace(string(gitOut("", "rev-parse", "HEAD")))
	pack := gitOut(newRev+"\n^"+oldRev+"\n", "pack-objects", "--revs", "--stdout", "-q")

	command := oldRev + " " + newRev + " refs/heads/master\x00report-status\n"
	var buf bytes.Buffer
	_, _ = fmt.Fprintf(&buf, "%04x%s0000", len(command)+4, command)
	buf.Write(pack)
	return buf.Bytes(), oldRev, newRev
}

// postService sends a smart HTTP POST for the given service the way a git
// client does, and returns the response status code.
func postService(t *testing.T, baseUrl, urlPath, service, creds string, body []byte) int {
	req, err := http.NewRequest("POST", baseUrl+urlPath, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("User-Agent", "git/2.43.0")
	req.Header.Set("Content-Type", "application/x-"+service+"-request")
	if creds != "" {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(creds)))
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

// repoMasterRev reads refs/heads/master straight from a gist repository on disk.
func repoMasterRev(t *testing.T, repoPath string) string {
	out, err := exec.Command("git", "--git-dir", repoPath, "rev-parse", "refs/heads/master").Output()
	require.NoError(t, err)
	return strings.TrimSpace(string(out))
}

func TestGitReceivePackAuthorization(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	baseUrl := s.StartHttpServer(t)

	s.Register(t, "thomas")
	s.Register(t, "alice")

	type gistRepo struct{ id, path string }
	var public, unlisted, private, archived gistRepo
	var user string
	public.path, _, user, public.id = s.CreateGist(t, "0")
	unlisted.path, _, _, unlisted.id = s.CreateGist(t, "1")
	private.path, _, _, private.id = s.CreateGist(t, "2")
	archived.path, _, _, archived.id = s.CreateGist(t, "0")

	s.Login(t, "thomas")
	s.Request(t, "POST", "/"+user+"/"+archived.id+"/archive", nil, 302)
	roToken := s.CreateAccessToken(t, "ro", db.ReadPermission, db.NoPermission)
	rwToken := s.CreateAccessToken(t, "rw", db.ReadWritePermission, db.NoPermission)
	s.Logout()

	tests := []struct {
		name         string
		requireLogin bool
		creds        string
		gist         gistRepo
		query        string
		code         int
	}{
		// git-receive-pack never takes a service parameter, whoever sends it.
		{"UploadPackServiceAnonymousPublic", false, "", public, "?service=git-upload-pack", 400},
		{"UploadPackServiceAnonymousUnlisted", false, "", unlisted, "?service=git-upload-pack", 400},
		{"UploadPackServiceAnonymousArchived", false, "", archived, "?service=git-upload-pack", 400},
		{"UploadPackServiceOtherUserPublic", false, "alice:alice", public, "?service=git-upload-pack", 400},
		{"UploadPackServiceOtherUserPublicRequireLogin", true, "alice:alice", public, "?service=git-upload-pack", 400},
		{"UploadPackServiceOtherUserUnlistedRequireLogin", true, "alice:alice", unlisted, "?service=git-upload-pack", 400},
		{"UploadPackServiceReadOnlyTokenPrivate", false, "thomas:" + roToken, private, "?service=git-upload-pack", 400},
		{"UploadPackServiceOwnerPublic", false, "thomas:thomas", public, "?service=git-upload-pack", 400},
		{"ReceivePackServiceOwnerPublic", false, "thomas:thomas", public, "?service=git-receive-pack", 400},

		// Without it, only the owner with write access can push to a gist that
		// is not archived.
		{"AnonymousPublic", false, "", public, "", 401},
		{"OtherUserPublic", false, "alice:alice", public, "", 404},
		{"OtherUserUnlistedRequireLogin", true, "alice:alice", unlisted, "", 404},
		{"ReadOnlyTokenPrivate", false, "thomas:" + roToken, private, "", 404},
		{"OwnerArchived", false, "thomas:thomas", archived, "", 403},
		{"OwnerPublic", false, "thomas:thomas", public, "", 200},
		{"ReadWriteTokenPrivate", false, "thomas:" + rwToken, private, "", 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.requireLogin {
				require.NoError(t, db.UpdateSetting(db.SettingRequireLogin, "1"))
				defer func() {
					require.NoError(t, db.UpdateSetting(db.SettingRequireLogin, "0"))
				}()
			}

			body, oldRev, newRev := newReceivePackBody(t, baseUrl, user, tt.gist.id)
			code := postService(t, baseUrl, "/"+user+"/"+tt.gist.id+".git/git-receive-pack"+tt.query, "git-receive-pack", tt.creds, body)

			if tt.code == 200 {
				require.Equal(t, newRev, repoMasterRev(t, tt.gist.path), "the push should have updated the gist")
			} else {
				require.Equal(t, oldRev, repoMasterRev(t, tt.gist.path), "the gist must not change")
			}
			require.Equal(t, tt.code, code)
		})
	}
}

func TestGitServiceParameter(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	baseUrl := s.StartHttpServer(t)

	s.Register(t, "thomas")
	s.Register(t, "alice")

	_, _, user, gistId := s.CreateGist(t, "0")
	repoUrl := "/" + user + "/" + gistId + ".git"

	t.Run("InfoRefs", func(t *testing.T) {
		tests := []struct {
			name    string
			service string
			creds   string
			code    int
		}{
			{"UploadPackAnonymous", "git-upload-pack", "", 200},
			{"UploadPackOtherUser", "git-upload-pack", "alice:alice", 200},
			{"ReceivePackAnonymous", "git-receive-pack", "", 401},
			{"ReceivePackOtherUser", "git-receive-pack", "alice:alice", 404},
			{"ReceivePackOwner", "git-receive-pack", "thomas:thomas", 200},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				code, body := dumbGet(t, baseUrl, repoUrl+"/info/refs?service="+tt.service, tt.creds)
				require.Equal(t, tt.code, code)
				if tt.code == 200 {
					require.Contains(t, string(body), "# service="+tt.service+"\n")
				}
			})
		}
	})

	t.Run("DumbRead", func(t *testing.T) {
		code, _ := dumbGet(t, baseUrl, repoUrl+"/HEAD", "")
		require.Equal(t, 200, code)

		code, _ = dumbGet(t, baseUrl, repoUrl+"/HEAD?service=git-upload-pack", "")
		require.Equal(t, 400, code)
	})

	t.Run("UploadPack", func(t *testing.T) {
		code := postService(t, baseUrl, repoUrl+"/git-upload-pack?service=git-receive-pack", "git-upload-pack", "thomas:thomas", []byte("0000"))
		require.Equal(t, 400, code)
	})
}
