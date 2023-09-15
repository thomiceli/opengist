package test

import (
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/memdb"
	"github.com/thomiceli/opengist/internal/web"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type testServer struct {
	server        *web.Server
	sessionCookie string
}

func newTestServer() (*testServer, error) {
	s := &testServer{
		server: web.NewServer(true),
	}

	go s.start()
	return s, nil
}

func (s *testServer) start() {
	s.server.Start()
}

func (s *testServer) stop() {
	s.server.Stop()
}

func (s *testServer) request(method, uri string, data interface{}, expectedCode int) error {
	var bodyReader io.Reader
	if method == http.MethodPost || method == http.MethodPut {
		values := structToURLValues(data)
		bodyReader = strings.NewReader(values.Encode())
	}

	req := httptest.NewRequest(method, "http://localhost:6157"+uri, bodyReader)
	w := httptest.NewRecorder()

	if method == http.MethodPost || method == http.MethodPut {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	if s.sessionCookie != "" {
		req.AddCookie(&http.Cookie{Name: "session", Value: s.sessionCookie})
	}

	s.server.ServeHTTP(w, req)

	if w.Code != expectedCode {
		return errors.New(fmt.Sprintf("unexpected status code %d, expected %d", w.Code, expectedCode))
	}

	if method == "POST" {
		if strings.Contains(uri, "/login") || strings.Contains(uri, "/register") {
			cookie := ""
			h := w.Header().Get("Set-Cookie")
			parts := strings.Split(h, "; ")
			for _, p := range parts {
				if strings.HasPrefix(p, "session=") {
					cookie = p
					break
				}
			}
			if cookie == "" {
				return errors.New("unable to find access session token in response headers")
			}
			s.sessionCookie = strings.TrimPrefix(cookie, "session=")
		} else if strings.Contains(uri, "/logout") {
			s.sessionCookie = ""
		}
	}

	return nil
}

func structToURLValues(s interface{}) url.Values {
	v := url.Values{}
	if s == nil {
		return v
	}

	rValue := reflect.ValueOf(s)
	if rValue.Kind() != reflect.Struct {
		return v
	}

	for i := 0; i < rValue.NumField(); i++ {
		field := rValue.Type().Field(i)
		tag := field.Tag.Get("form")
		if tag != "" {
			fieldValue := rValue.Field(i).String()
			v.Add(tag, fieldValue)
		}
	}
	return v
}

func setup(t *testing.T) {
	err := config.InitConfig("")
	require.NoError(t, err, "Could not init config")

	err = os.MkdirAll(filepath.Join(config.GetHomeDir()), 0755)
	require.NoError(t, err, "Could not create Opengist home directory")

	git.ReposDirectory = path.Join("tests")

	config.InitLog()

	homePath := config.GetHomeDir()
	log.Info().Msg("Data directory: " + homePath)

	err = os.MkdirAll(filepath.Join(homePath, "repos"), 0755)
	require.NoError(t, err, "Could not create repos directory")

	err = os.MkdirAll(filepath.Join(homePath, "tmp", "repos"), 0755)
	require.NoError(t, err, "Could not create tmp repos directory")

	err = db.Setup("file::memory:", true)
	require.NoError(t, err, "Could not initialize database")

	err = memdb.Setup()
	require.NoError(t, err, "Could not initialize in memory database")
}

func teardown(t *testing.T, s *testServer) {
	s.stop()

	err := db.Close()
	require.NoError(t, err, "Could not close database")
}
