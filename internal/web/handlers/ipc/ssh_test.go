package ipc_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/ipc"
	webtest "github.com/thomiceli/opengist/internal/web/test"
	"gorm.io/gorm"
)

// ipcRequest posts body to the IPC API as a subprocess would, and decodes the
// response into resp when not nil.
func ipcRequest(t *testing.T, s *webtest.Server, path string, body, resp any) {
	payload, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(ipc.AuthHeader, ipc.Token())

	res := s.RawRequest(t, req, http.StatusOK)
	if resp != nil {
		require.NoError(t, json.NewDecoder(res.Body).Decode(resp))
	}
}

func TestSSHCommandCreatesGist(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	s.Register(t, "alice")
	key, _ := s.AddSSHKey(t, "thomas")

	tests := []struct {
		name       string
		command    string
		authorized bool
	}{
		{"Init", "git-receive-pack '/init.git'", true},
		{"InitScpLike", "git-receive-pack 'init.git'", true},
		{"OwnNamespace", "git-receive-pack '/thomas/mygist.git'", true},
		{"OtherUserNamespace", "git-receive-pack '/alice/mygist.git'", false},
		{"CloneInit", "git-upload-pack '/init.git'", false},
		{"CloneMissingGist", "git-upload-pack '/thomas/missing.git'", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp ipc.SSHCommandResponse
			ipcRequest(t, s, "/api/ipc/ssh/command", ipc.SSHCommandRequest{
				KeyID:   key.ID,
				Command: tt.command,
				IP:      "127.0.0.1",
			}, &resp)

			require.Equal(t, tt.authorized, resp.Authorized, resp.Message)
			require.Equal(t, tt.authorized, resp.Created)
			if !tt.authorized {
				return
			}
			require.Equal(t, "receive-pack", resp.Verb)
			require.DirExists(t, resp.RepoPath)

			// Nothing was pushed: once the shim reports the push is over, the
			// gist is deleted along with its repository.
			ipcRequest(t, s, "/api/ipc/ssh/discard-empty-gist", ipc.DiscardEmptyGistRequest{GistID: resp.GistID}, nil)
			_, err := db.GetGistByID(resp.GistID)
			require.ErrorIs(t, err, gorm.ErrRecordNotFound)
			require.NoDirExists(t, resp.RepoPath)

			// Discarding a gist that is already gone is a no-op.
			ipcRequest(t, s, "/api/ipc/ssh/discard-empty-gist", ipc.DiscardEmptyGistRequest{GistID: resp.GistID}, nil)
		})
	}
}

func TestSSHCommandExistingGist(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	_, gist, user, identifier := s.CreateGist(t, "0")
	key, _ := s.AddSSHKey(t, "thomas")

	var resp ipc.SSHCommandResponse
	ipcRequest(t, s, "/api/ipc/ssh/command", ipc.SSHCommandRequest{
		KeyID:   key.ID,
		Command: "git-receive-pack '/" + user + "/" + identifier + ".git'",
		IP:      "127.0.0.1",
	}, &resp)

	require.True(t, resp.Authorized, resp.Message)
	require.False(t, resp.Created)
	require.Equal(t, strconv.FormatUint(uint64(gist.ID), 10), resp.GistID)

	// A gist with commits is never discarded.
	ipcRequest(t, s, "/api/ipc/ssh/discard-empty-gist", ipc.DiscardEmptyGistRequest{GistID: resp.GistID}, nil)
	_, err := db.GetGistByID(resp.GistID)
	require.NoError(t, err)
}
