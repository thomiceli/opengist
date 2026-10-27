package config

import (
	"strings"
	"testing"
)

func TestCheckGitVersion(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		wantOk    bool
		wantError bool
	}{
		{"recent version", "2.50.1", true, false},
		{"git for windows suffix", "2.50.1.windows.1", true, false},
		{"full git --version output", "git version 2.50.1.windows.1", true, false},
		{"exactly 2.28", "2.28.0", true, false},
		{"too old", "2.27.0", false, false},
		{"major too old", "1.99.0", false, false},
		{"empty string", "", false, true},
		{"no version numbers", "git version unknown", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := CheckGitVersion(tt.version)
			if (err != nil) != tt.wantError {
				t.Fatalf("CheckGitVersion(%q) error = %v, wantError %v", tt.version, err, tt.wantError)
			}
			if ok != tt.wantOk {
				t.Errorf("CheckGitVersion(%q) ok = %v, want %v", tt.version, ok, tt.wantOk)
			}
		})
	}
}

func TestProxyAuthTrustedNetworks(t *testing.T) {
	tests := []struct {
		name      string
		cidrs     string
		want      []string
		wantError bool
	}{
		{"empty", "", nil, false},
		{"single cidr", "10.0.0.0/8", []string{"10.0.0.0/8"}, false},
		{"list with spaces", " 127.0.0.0/8 , ::1/128,", []string{"127.0.0.0/8", "::1/128"}, false},
		{"bare ipv4", "172.17.0.2", []string{"172.17.0.2/32"}, false},
		{"bare ipv6", "fd00::1", []string{"fd00::1/128"}, false},
		{"host bits are masked", "192.168.1.10/24", []string{"192.168.1.0/24"}, false},
		{"invalid address", "not-an-ip", nil, true},
		{"invalid prefix length", "10.0.0.0/33", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &config{ProxyAuthTrustedCIDRs: tt.cidrs}
			networks, err := c.ProxyAuthTrustedNetworks()
			if (err != nil) != tt.wantError {
				t.Fatalf("ProxyAuthTrustedNetworks(%q) error = %v, wantError %v", tt.cidrs, err, tt.wantError)
			}
			var got []string
			for _, network := range networks {
				got = append(got, network.String())
			}
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("ProxyAuthTrustedNetworks(%q) = %v, want %v", tt.cidrs, got, tt.want)
			}
		})
	}
}

func TestChecksProxyAuth(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		cidrs     string
		wantError bool
	}{
		{"disabled", "", "", false},
		{"disabled ignores cidrs", "", "not-an-ip", false},
		{"enabled with cidrs", "X-Auth-Request-User", "10.0.0.0/8", false},
		{"enabled without cidrs", "X-Auth-Request-User", "", true},
		{"enabled with invalid cidrs", "X-Auth-Request-User", "10.0.0.0/33", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := configWithDefaults()
			c.ProxyAuthUserHeader = tt.header
			c.ProxyAuthTrustedCIDRs = tt.cidrs
			if err := checks(c); (err != nil) != tt.wantError {
				t.Fatalf("checks() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}
