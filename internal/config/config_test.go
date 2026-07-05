package config

import (
	"os"
	"path/filepath"
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

func TestParseSecretLine(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantKey   string
		wantValue string
		wantOk    bool
	}{
		{"simple", "OG_LOG_LEVEL=info", "OG_LOG_LEVEL", "info", true},
		{"export prefix", "export OG_FOO=bar", "OG_FOO", "bar", true},
		{"double quotes", `OG_FOO="a b c"`, "OG_FOO", "a b c", true},
		{"single quotes", `OG_FOO='a b c'`, "OG_FOO", "a b c", true},
		{"surrounding spaces", "  OG_FOO = bar  ", "OG_FOO", "bar", true},
		{"value contains equals", "OG_FOO=a=b", "OG_FOO", "a=b", true},
		{"blank line", "   ", "", "", false},
		{"comment", "# a comment", "", "", false},
		{"no equals", "OG_FOO", "", "", false},
		{"empty key", "=value", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, value, ok := parseSecretLine(tt.line)
			if ok != tt.wantOk {
				t.Fatalf("parseSecretLine(%q) ok = %v, want %v", tt.line, ok, tt.wantOk)
			}
			if key != tt.wantKey || value != tt.wantValue {
				t.Errorf("parseSecretLine(%q) = (%q, %q), want (%q, %q)", tt.line, key, value, tt.wantKey, tt.wantValue)
			}
		})
	}
}

func TestLoadSecretsFile(t *testing.T) {
	// Use a key that is unlikely to already be set in the test environment.
	const key = "OG_LOAD_SECRETS_TEST_KEY"

	t.Setenv("OG_SECRETS_FILE", filepath.Join(t.TempDir(), "does-not-exist"))
	if err := loadSecretsFile(); err != nil {
		t.Fatalf("missing secrets file should be a no-op, got error: %v", err)
	}

	secrets := filepath.Join(t.TempDir(), "secrets.env")
	if err := os.WriteFile(secrets, []byte("# comment\n\nexport "+key+"=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OG_SECRETS_FILE", secrets)

	// An unset variable is populated from the file.
	os.Unsetenv(key)
	if err := loadSecretsFile(); err != nil {
		t.Fatalf("loadSecretsFile: %v", err)
	}
	if got := os.Getenv(key); got != "from-file" {
		t.Errorf("expected %s=from-file, got %q", key, got)
	}

	// An explicitly set variable is NOT overridden by the file.
	t.Setenv(key, "from-env")
	if err := loadSecretsFile(); err != nil {
		t.Fatalf("loadSecretsFile: %v", err)
	}
	if got := os.Getenv(key); got != "from-env" {
		t.Errorf("explicit env must win, expected from-env, got %q", got)
	}
}
