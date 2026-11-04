package context

import (
	"io"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/config"
)

func TestHardenCookieSecureFollowsExternalUrl(t *testing.T) {
	require.NoError(t, config.InitConfig("", io.Discard))

	tests := []struct {
		externalUrl string
		secure      bool
	}{
		{"", false},
		{"http://gist.example.com", false},
		{"https://gist.example.com", true},
		{"HTTPS://gist.example.com", true},
	}
	for _, tt := range tests {
		t.Run(tt.externalUrl, func(t *testing.T) {
			config.C.ExternalUrl = tt.externalUrl

			// The flash store is a cookie store, whose default Options already set
			// Secure; a plain-HTTP deployment has to clear it.
			cookieOpts := sessions.NewCookieStore([]byte("opengist")).Options
			hardenCookie(cookieOpts)
			require.Equal(t, tt.secure, cookieOpts.Secure)

			fsOpts := sessions.NewFilesystemStore(t.TempDir()).Options
			hardenCookie(fsOpts)
			require.Equal(t, tt.secure, fsOpts.Secure)
		})
	}
}
