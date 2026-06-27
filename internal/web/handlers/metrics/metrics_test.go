package metrics_test

import (
	"io"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

func TestMetrics(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	s.Login(t, "thomas")

	s.Request(t, "POST", "/", db.GistDTO{
		Title:       "Simple Test Gist",
		Description: "A simple gist for testing",
		VisibilityDTO: db.VisibilityDTO{
			Private: 0,
		},
		Name:    []string{"file1.txt"},
		Content: []string{"This is the content of file1"},
		Topics:  "",
	}, 302)

	s.Request(t, "POST", "/settings/ssh-keys", db.SSHKeyDTO{
		Title:   "Test SSH Key",
		Content: `ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAklOUpkDHrfHY17SbrmTIpNLTGK9Tjom/BWDSUGPl+nafzlHDTYW7hdI4yZ5ew18JH4JW9jbhUFrviQzM7xlELEVf4h9lFX5QVkbPppSwg0cda3Pbv7kOdJ/MTyBlWXFCR+HAo3FXRitBqxiX1nKhXpHAZsMciLq8V6RjsNAQwdsdMFvSlVK/7XAt3FaoJoAsncM1Q9x5+3V0Ww68/eIFmb1zuUFljQJKprrX88XypNDvjYNby6vw/Pb0rwert/EnmZ+AW4OZPnTPI89ZPmVMLuayrD2cE86Z/il8b+gw3r3+1nKatmIkjn2so1d01QraTlMqVSsbxNrRFi9wrf+M7Q== admin@admin.local`,
	}, 302)

	metricsServer := webtest.NewTestMetricsServer()

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	metricsServer.ServeHTTP(w, req)

	require.Equal(t, 200, w.Code)

	body, err := io.ReadAll(w.Body)
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
