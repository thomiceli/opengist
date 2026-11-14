package hooks

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
)

func TestRemoteUrl(t *testing.T) {
	t.Setenv("OG_OPENGIST_HOME", t.TempDir())
	require.NoError(t, config.InitConfig("", io.Discard))

	gist := &db.Gist{Uuid: "abc123", User: db.User{Username: "thomas"}}
	gistUrl := "https://gist.example.com/thomas/abc123"

	tests := []struct {
		name        string
		protocol    string
		externalUrl string
		sshDomain   string
		expected    string
	}{
		{"HTTP", "", "https://gist.example.com", "", gistUrl},
		{"SSHHostFromExternalUrl", "ssh", "https://gist.example.com:6157", "", "ssh://gist.example.com:2222/thomas/abc123.git"},
		{"SSHHostFromExternalDomain", "ssh", "", "ssh.example.com", "ssh://ssh.example.com:2222/thomas/abc123.git"},
		{"SSHWithoutHost", "ssh", "", "", gistUrl},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.C.ExternalUrl = tt.externalUrl
			config.C.SshExternalDomain = tt.sshDomain
			require.Equal(t, tt.expected, remoteUrl(gist, gistUrl, tt.protocol))
		})
	}
}
