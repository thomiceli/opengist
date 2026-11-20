package ssh_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/ssh"
	webtest "github.com/thomiceli/opengist/internal/web/test"
	gossh "golang.org/x/crypto/ssh"
)

// TestAuthorizeGitCommandExpiredGist - an expired gist not yet deleted by the
// background job can't be pulled or pushed over SSH, by its owner either.
func TestAuthorizeGitCommandExpiredGist(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	user, err := db.GetUserByUsername("thomas")
	require.NoError(t, err)

	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	sshPub, err := gossh.NewPublicKey(pub)
	require.NoError(t, err)
	key := strings.TrimSpace(string(gossh.MarshalAuthorizedKey(sshPub)))
	require.NoError(t, (&db.SSHKey{Title: "key", Content: key, UserID: user.ID}).Create())

	for _, vis := range []struct{ name, value string }{
		{"Public", "0"},
		{"Unlisted", "1"},
		{"Private", "2"},
	} {
		t.Run(vis.name, func(t *testing.T) {
			_, gist, username, identifier := s.CreateGist(t, vis.value)
			repo := " '/" + username + "/" + identifier + ".git'"

			_, _, err := ssh.AuthorizeGitCommand("git-upload-pack"+repo, key, "127.0.0.1")
			require.NoError(t, err)

			gist.ExpiresAt = time.Now().Add(-time.Minute).Unix()
			require.NoError(t, gist.UpdateNoTimestamps())

			for _, verb := range []string{"git-upload-pack", "git-receive-pack"} {
				_, _, err := ssh.AuthorizeGitCommand(verb+repo, key, "127.0.0.1")
				require.Error(t, err, verb)
			}
		})
	}
}
