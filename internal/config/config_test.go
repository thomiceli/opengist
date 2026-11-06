package config

import (
	"fmt"
	"io"
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

func TestPaginationPerPage(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		c, err := configWithDefaults()
		if err != nil {
			t.Fatal(err)
		}
		if c.PaginationPerPage != 10 {
			t.Errorf("default PaginationPerPage = %d, want 10", c.PaginationPerPage)
		}
	})

	t.Run("from env", func(t *testing.T) {
		t.Setenv("OG_PAGINATION_PER_PAGE", "25")
		c, _ := configWithDefaults()
		if err := loadConfigFromEnv(c, io.Discard); err != nil {
			t.Fatal(err)
		}
		if c.PaginationPerPage != 25 {
			t.Errorf("PaginationPerPage = %d, want 25", c.PaginationPerPage)
		}
	})

	t.Run("invalid env", func(t *testing.T) {
		t.Setenv("OG_PAGINATION_PER_PAGE", "lots")
		c, _ := configWithDefaults()
		if err := loadConfigFromEnv(c, io.Discard); err == nil {
			t.Error("expected an error for a non-integer OG_PAGINATION_PER_PAGE")
		}
	})

	for _, v := range []int{0, -1, MaxPaginationPerPage + 1} {
		t.Run(fmt.Sprintf("out of range %d", v), func(t *testing.T) {
			c, _ := configWithDefaults()
			c.PaginationPerPage = v
			if err := checks(c); err == nil {
				t.Errorf("checks() accepted pagination.per-page = %d", v)
			}
		})
	}

	for _, v := range []int{1, 50, MaxPaginationPerPage} {
		t.Run(fmt.Sprintf("in range %d", v), func(t *testing.T) {
			c, _ := configWithDefaults()
			c.PaginationPerPage = v
			if err := checks(c); err != nil {
				t.Errorf("checks() rejected pagination.per-page = %d: %v", v, err)
			}
		})
	}
}
