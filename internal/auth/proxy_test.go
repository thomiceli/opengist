package auth

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/config"
)

func TestIsTrustedProxy(t *testing.T) {
	require.NoError(t, config.InitConfig("", io.Discard))
	config.C.ProxyAuthTrustedCIDRs = "10.0.0.0/8, fd00::/8, 192.0.2.7"

	tests := []struct {
		remoteAddr string
		want       bool
	}{
		{"10.1.2.3:4567", true},
		{"11.1.2.3:4567", false},
		{"192.0.2.7:80", true},
		{"192.0.2.8:80", false},
		{"[fd00::1]:4567", true},
		{"[fe80::1]:4567", false},
		{"[::ffff:10.1.2.3]:4567", true},
		{"[fd00::1%eth0]:4567", true},
		{"@", false}, // Unix socket peer
		{"", false},
		{"10.1.2.3", false}, // no port, not a valid RemoteAddr
	}

	for _, tt := range tests {
		t.Run(tt.remoteAddr, func(t *testing.T) {
			require.Equal(t, tt.want, isTrustedProxy(tt.remoteAddr))
		})
	}
}
