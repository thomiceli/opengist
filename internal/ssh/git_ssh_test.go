package ssh_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/ssh"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

func TestAuthorizeGitCommandIdentifier(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")

	// Created first so it's the gist a wildcard or an empty identifier would
	// resolve to
	_, unlisted, user, _ := s.CreateGist(t, "1")
	_, public, _, _ := s.CreateGist(t, "0")

	for _, name := range []string{"", ".git", "_", "%", unlisted.Uuid[:8], unlisted.Uuid[:8] + ".git"} {
		_, _, err := ssh.AuthorizeGitCommand("git-upload-pack '"+user+"/"+name+"'", "", "127.0.0.1")
		require.Error(t, err, "gist name %q", name)
	}

	gist, verb, err := ssh.AuthorizeGitCommand("git-upload-pack '/"+user+"/"+unlisted.Uuid+".git'", "", "127.0.0.1")
	require.NoError(t, err)
	require.Equal(t, unlisted.ID, gist.ID)
	require.Equal(t, "upload-pack", verb)

	gist, _, err = ssh.AuthorizeGitCommand("git-upload-pack '"+user+"/"+public.Uuid[:8]+".git'", "", "127.0.0.1")
	require.NoError(t, err)
	require.Equal(t, public.ID, gist.ID)
}
