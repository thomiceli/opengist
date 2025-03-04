package test

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
)

var (
	SSHKey = db.SSHKeyDTO{
		Title:   "Test SSH Key",
		Content: `ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAklOUpkDHrfHY17SbrmTIpNLTGK9Tjom/BWDSUGPl+nafzlHDTYW7hdI4yZ5ew18JH4JW9jbhUFrviQzM7xlELEVf4h9lFX5QVkbPppSwg0cda3Pbv7kOdJ/MTyBlWXFCR+HAo3FXRitBqxiX1nKhXpHAZsMciLq8V6RjsNAQwdsdMFvSlVK/7XAt3FaoJoAsncM1Q9x5+3V0Ww68/eIFmb1zuUFljQJKprrX88XypNDvjYNby6vw/Pb0rwert/EnmZ+AW4OZPnTPI89ZPmVMLuayrD2cE86Z/il8b+gw3r3+1nKatmIkjn2so1d01QraTlMqVSsbxNrRFi9wrf+M7Q== admin@admin.local`,
	}
	AdminUser = db.UserDTO{
		Username: "admin",
		Password: "admin",
	}

	SimpleGist = db.GistDTO{
		Title:       "Simple Test Gist",
		Description: "A simple gist for testing",
		VisibilityDTO: db.VisibilityDTO{
			Private: 0,
		},
		Name:    []string{"file1.txt"},
		Content: []string{"This is the content of file1"},
		Topics:  "",
	}
)

// TestMetrics tests the metrics endpoint functionality of the application.
// It verifies that the metrics endpoint correctly reports counts for:
// - Total number of users
// - Total number of gists
// - Total number of SSH keys
//
// The test follows these steps:
// 1. Enables metrics via environment variable
// 2. Sets up test environment
// 3. Registers and logs in an admin user
// 4. Creates a gist and adds an SSH key
// 5. Queries the metrics endpoint
// 6. Verifies the reported metrics match expected values
//
// Environment variables:
//   - OG_METRICS_ENABLED: Set to "true" for this test
func TestMetrics(t *testing.T) {
	originalValue := os.Getenv("OG_METRICS_ENABLED")

	os.Setenv("OG_METRICS_ENABLED", "true")

	defer os.Setenv("OG_METRICS_ENABLED", originalValue)

	s := Setup(t)
	defer Teardown(t, s)

	register(t, s, AdminUser)
	login(t, s, AdminUser)

	err := s.Request("GET", "/all", nil, 200)
	require.NoError(t, err)

	err = s.Request("POST", "/", SimpleGist, 302)
	require.NoError(t, err)

	err = s.Request("POST", "/settings/ssh-keys", SSHKey, 302)
	require.NoError(t, err)

	var metricsRes http.Response
	err = s.Request("GET", "/metrics", nil, 200, &metricsRes)
	require.NoError(t, err)

	body, err := io.ReadAll(metricsRes.Body)
	defer metricsRes.Body.Close()
	require.NoError(t, err)

	lines := strings.Split(string(body), "\n")
	var usersTotal float64
	var gistsTotal float64
	var sshKeysTotal float64

	for _, line := range lines {
		if strings.HasPrefix(line, "opengist_users_total") {
			parts := strings.Fields(line)
			if len(parts) == 2 {
				usersTotal, err = strconv.ParseFloat(parts[1], 64)
				assert.NoError(t, err)
			}
		} else if strings.HasPrefix(line, "opengist_gists_total") {
			parts := strings.Fields(line)
			if len(parts) == 2 {
				gistsTotal, err = strconv.ParseFloat(parts[1], 64)
				assert.NoError(t, err)
			}
		} else if strings.HasPrefix(line, "opengist_ssh_keys_total") {
			parts := strings.Fields(line)
			if len(parts) == 2 {
				sshKeysTotal, err = strconv.ParseFloat(parts[1], 64)
				assert.NoError(t, err)
			}
		}
	}

	assert.Equal(t, 1.0, usersTotal, "opengist_users_total should be 1")
	assert.Equal(t, 1.0, gistsTotal, "opengist_gists_total should be 1")
	assert.Equal(t, 1.0, sshKeysTotal, "opengist_ssh_keys_total should be 1")
}
