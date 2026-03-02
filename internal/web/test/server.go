package test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorilla/schema"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/index"
	"github.com/thomiceli/opengist/internal/web/context"
	"github.com/thomiceli/opengist/internal/web/handlers/metrics"
	"github.com/thomiceli/opengist/internal/web/server"
)

var databaseType string
var formEncoder *schema.Encoder

func init() {
	formEncoder = schema.NewEncoder()
	formEncoder.SetAliasTag("form")
}

type Server struct {
	server        *server.Server
	SessionCookie string
	contextData   echo.Map
}

func (s *Server) Request(t *testing.T, method, uri string, data interface{}, expectedCode int) *http.Response {
	return s.RequestWithHeaders(t, method, uri, data, expectedCode, nil)
}

func (s *Server) RequestWithHeaders(t *testing.T, method, uri string, data interface{}, expectedCode int, headers map[string]string) *http.Response {
	var bodyReader io.Reader
	if method == http.MethodPost || method == http.MethodPut || method == http.MethodDelete {
		if values, ok := data.(url.Values); ok {
			bodyReader = strings.NewReader(values.Encode())
		} else if data != nil {
			values := url.Values{}
			_ = formEncoder.Encode(data, values)
			bodyReader = strings.NewReader(values.Encode())
		}
	}

	req := httptest.NewRequest(method, uri, bodyReader)
	w := httptest.NewRecorder()

	if method == http.MethodPost || method == http.MethodPut {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	req.Header.Set("Sec-Fetch-Site", "same-origin")

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	if s.SessionCookie != "" {
		req.AddCookie(&http.Cookie{Name: "session", Value: s.SessionCookie})
	}

	s.server.ServeHTTP(w, req)
	if expectedCode != 0 {
		require.Equalf(t, expectedCode, w.Code, "Unexpected status code for %s %s: got %d, expected %d", method, uri, w.Code, expectedCode)
	}
	if method == http.MethodPost {
		if strings.Contains(uri, "/login") {
			cookie := ""
			h := w.Header().Get("Set-Cookie")
			parts := strings.Split(h, "; ")
			for _, p := range parts {
				if strings.HasPrefix(p, "session=") {
					cookie = p
					break
				}
			}
			s.SessionCookie = strings.TrimPrefix(cookie, "session=")
		} else if strings.Contains(uri, "/logout") {
			s.SessionCookie = ""
		}
	}

	return w.Result()
}

func (s *Server) RawRequest(t *testing.T, req *http.Request, expectedCode int) *http.Response {
	w := httptest.NewRecorder()

	req.Header.Set("Sec-Fetch-Site", "same-origin")

	if s.SessionCookie != "" {
		req.AddCookie(&http.Cookie{Name: "session", Value: s.SessionCookie})
	}

	s.server.ServeHTTP(w, req)

	require.Equal(t, expectedCode, w.Code, "unexpected status code for %s %s", req.Method, req.URL.Path)

	return w.Result()
}

func (s *Server) StartHttpServer(t *testing.T) string {
	hs := httptest.NewServer(s.server)
	t.Cleanup(hs.Close)
	return hs.URL
}

func (s *Server) User() *db.User {
	s.Request(nil, "GET", "/", nil, 0)
	if user, ok := s.contextData["userLogged"].(*db.User); ok {
		return user
	}
	return nil
}

func (s *Server) TestCtxData(t *testing.T, expected echo.Map) {
	for key, expectedValue := range expected {
		actualValue, exists := s.contextData[key]
		require.True(t, exists, "Key %q not found in context data", key)
		require.Equal(t, expectedValue, actualValue, "Context data mismatch for key %q", key)
	}
}

func (s *Server) Register(t *testing.T, user string) {
	s.Request(t, "POST", "/register", db.UserDTO{Username: user, Password: user}, 302)
}

func (s *Server) Login(t *testing.T, user string) {
	s.Request(t, "POST", "/login", db.UserDTO{Username: user, Password: user}, 302)
}

func (s *Server) Logout() {
	s.SessionCookie = ""
}

func (s *Server) CreateGist(t *testing.T, visibility string) (gistPath string, gist *db.Gist, username, identifier string) {
	s.Request(t, "POST", "/register", db.UserDTO{Username: "thomas", Password: "thomas"}, 0)
	s.Login(t, "thomas")

	resp := s.Request(t, "POST", "/", url.Values{
		"title":   {"Test"},
		"name":    {"file.txt", "otherfile.txt"},
		"content": {"hello world", "other content"},
		"topics":  {"hello opengist"},
		"private": {visibility},
	}, 302)

	// Extract gist identifier from redirect
	location := resp.Header.Get("Location")
	parts := strings.Split(strings.TrimPrefix(location, "/"), "/")
	require.Len(t, parts, 2, "Expected redirect format: /{username}/{identifier}")

	gistUsername := parts[0]
	gistIdentifier := parts[1]

	gist, err := db.GetGist(gistUsername, gistIdentifier)
	require.NoError(t, err)
	require.NotNil(t, gist)

	gistPath = filepath.Join(config.GetHomeDir(), git.ReposDirectory, "thomas", gist.Uuid)

	// Verify gist exists on filesystem
	_, err = os.Stat(gistPath)
	require.NoError(t, err, "Gist repository should exist at %s", gistPath)

	username = gist.User.Username
	identifier = gist.Identifier()

	s.Logout()
	return gistPath, gist, username, identifier
}

func Setup(t *testing.T) *Server {
	tmpDir := t.TempDir()
	t.Setenv("OPENGIST_SKIP_GIT_HOOKS", "1")

	err := config.InitConfig("", io.Discard)
	require.NoError(t, err, "Could not init config")

	config.C.LogLevel = "warn"
	config.C.LogOutput = "stdout"
	config.C.GitDefaultBranch = "master"
	config.C.OpengistHome = tmpDir

	config.SetupSecretKey()
	config.InitLog()

	tmpGitConfig := filepath.Join(tmpDir, "gitconfig")
	t.Setenv("GIT_CONFIG_GLOBAL", tmpGitConfig)

	err = exec.Command("git", "config", "--global", "--type", "bool", "push.autoSetupRemote", "true").Run()
	require.NoError(t, err)
	err = exec.Command("git", "config", "--global", "user.email", "test@opengist.io").Run()
	require.NoError(t, err)
	err = exec.Command("git", "config", "--global", "user.name", "test").Run()
	require.NoError(t, err)
	homePath := config.GetHomeDir()

	var databaseDsn string
	databaseType = os.Getenv("OPENGIST_TEST_DB")
	switch databaseType {
	case "postgres":
		databaseDsn = "postgres://postgres:opengist@localhost:5432/opengist_test"
	case "mysql":
		databaseDsn = "mysql://root:opengist@localhost:3306/opengist_test"
	default:
		databaseDsn = config.C.DBUri
	}

	err = os.MkdirAll(filepath.Join(homePath, "sessions"), 0755)
	require.NoError(t, err, "Could not create sessions directory")

	err = os.MkdirAll(filepath.Join(homePath, "repos"), 0755)
	require.NoError(t, err, "Could not create repos directory")

	err = os.MkdirAll(filepath.Join(homePath, "tmp", "repos"), 0755)
	require.NoError(t, err, "Could not create tmp repos directory")

	err = os.MkdirAll(filepath.Join(homePath, "custom"), 0755)
	require.NoError(t, err, "Could not create custom directory")

	err = db.Setup(databaseDsn)
	require.NoError(t, err, "Could not initialize database")

	t.Cleanup(func() {
		db.Close()
	})

	if index.IndexEnabled() {
		go index.NewIndexer(index.IndexType())
	}

	s := &Server{
		server: server.NewServer(true),
	}

	s.server.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)
			if data, ok := c.Request().Context().Value(context.DataKeyStr).(echo.Map); ok {
				s.contextData = data
			}
			return err
		}
	})

	return s
}

func Teardown(t *testing.T) {
	switch databaseType {
	case "postgres", "mysql":
		err := db.TruncateDatabase()
		require.NoError(t, err, "Could not truncate database")
	}
}

func NewTestMetricsServer() *metrics.Server {
	return metrics.NewServer()
}
