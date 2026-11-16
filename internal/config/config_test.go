package config

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigFromYamlNestedKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	err := os.WriteFile(path, []byte(`gitea:
  name: My Gitea
  url: https://gitea.example.com
index:
  meili:
    host: http://localhost:7700
custom:
  static-links:
    - name: Help
      path: /help
`), 0600)
	if err != nil {
		t.Fatal(err)
	}
	c, err := configWithDefaults()
	if err != nil {
		t.Fatal(err)
	}
	if err := loadConfigFromYaml(c, path, io.Discard); err != nil {
		t.Fatal(err)
	}
	if c.GiteaName != "My Gitea" || c.GiteaUrl != "https://gitea.example.com" || c.MeiliHost != "http://localhost:7700" {
		t.Errorf("nested config values: name=%q url=%q host=%q", c.GiteaName, c.GiteaUrl, c.MeiliHost)
	}
	if c.Index != "bleve" {
		t.Errorf("Index = %q, want default bleve", c.Index)
	}
	if len(c.StaticLinks) != 1 || c.StaticLinks[0] != (StaticLink{Name: "Help", Path: "/help"}) {
		t.Errorf("StaticLinks = %v, want Help /help", c.StaticLinks)
	}
}

func TestLoadConfigFromYamlDottedKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	err := os.WriteFile(path, []byte(`gitea.name: My Gitea
index: meilisearch
index.meili.host: http://localhost:7700
custom.static-links:
  - name: Help
    path: /help
`), 0600)
	if err != nil {
		t.Fatal(err)
	}
	c := &config{}
	if err := loadConfigFromYaml(c, path, io.Discard); err != nil {
		t.Fatal(err)
	}
	if c.GiteaName != "My Gitea" || c.Index != "meilisearch" || c.MeiliHost != "http://localhost:7700" {
		t.Errorf("dotted config values: name=%q index=%q host=%q", c.GiteaName, c.Index, c.MeiliHost)
	}
	if len(c.StaticLinks) != 1 || c.StaticLinks[0] != (StaticLink{Name: "Help", Path: "/help"}) {
		t.Errorf("StaticLinks = %v, want Help /help", c.StaticLinks)
	}
}

func TestLoadConfigFromYamlNestedAndDottedConflict(t *testing.T) {
	for name, contents := range map[string]string{
		"dotted first": "gitea.name: First\ngitea:\n  name: Second\n",
		"nested first": "gitea:\n  name: First\ngitea.name: Second\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yml")
			if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
				t.Fatal(err)
			}
			err := loadConfigFromYaml(&config{}, path, io.Discard)
			if err == nil || !strings.Contains(err.Error(), "gitea.name") {
				t.Fatalf("error = %v, want conflict naming gitea.name", err)
			}
		})
	}
}

func TestLoadConfigFromYamlEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	c := &config{GiteaName: "Gitea"}
	if err := loadConfigFromYaml(c, path, io.Discard); err != nil {
		t.Fatal(err)
	}
	if c.GiteaName != "Gitea" {
		t.Errorf("GiteaName = %q, want unchanged default", c.GiteaName)
	}
}

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
