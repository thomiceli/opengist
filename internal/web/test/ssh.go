package test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	"golang.org/x/crypto/ssh"
)

// AddSSHKey generates an SSH key and adds it to the given user's keys. It
// returns the stored key and its private key, PEM-encoded in OpenSSH format.
func (s *Server) AddSSHKey(t *testing.T, username string) (*db.SSHKey, []byte) {
	user, err := db.GetUserByUsername(username)
	require.NoError(t, err)

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	sshPub, err := ssh.NewPublicKey(pub)
	require.NoError(t, err)

	key := &db.SSHKey{
		Title:   "key",
		Content: strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub))),
		UserID:  user.ID,
	}
	require.NoError(t, key.Create())

	block, err := ssh.MarshalPrivateKey(priv, "")
	require.NoError(t, err)

	return key, pem.EncodeToMemory(block)
}
