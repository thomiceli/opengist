package config

import "testing"

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
