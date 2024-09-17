package test

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/memdb"
	"github.com/thomiceli/opengist/internal/web"
)

var databaseType string

type testServer struct {
	server        *web.Server
	sessionCookie string
}

func newTestServer() (*testServer, error) {
	s := &testServer{
		server: web.NewServer(true, path.Join(config.GetHomeDir(), "tmp", "sessions")),
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
		return fmt.Errorf("unexpected status code %d, expected %d", w.Code, expectedCode)
	}

	if method == http.MethodPost {
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
		if tag != "" || field.Anonymous {
			if field.Type.Kind() == reflect.Int {
				fieldValue := rValue.Field(i).Int()
				v.Add(tag, strconv.FormatInt(fieldValue, 10))
			} else if field.Type.Kind() == reflect.Slice {
				fieldValue := rValue.Field(i).Interface().([]string)
				for _, va := range fieldValue {
					v.Add(tag, va)
				}
			} else if field.Type.Kind() == reflect.Struct {
				for key, val := range structToURLValues(rValue.Field(i).Interface()) {
					for _, vv := range val {
						v.Add(key, vv)
					}
				}
			} else {
				fieldValue := rValue.Field(i).String()
				v.Add(tag, fieldValue)
			}
		}
	}
	return v
}

func setup(t *testing.T) {
	var databaseDsn string
	databaseType = os.Getenv("OPENGIST_TEST_DB")
	switch databaseType {
	case "sqlite":
		databaseDsn = "file::memory:"
	case "postgres":
		databaseDsn = "postgres://postgres:opengist@localhost:5432/opengist_test"
	case "mysql":
		databaseDsn = "mysql://root:opengist@localhost:3306/opengist_test"
	}

	_ = os.Setenv("OPENGIST_SKIP_GIT_HOOKS", "1")

	err := config.InitConfig("", io.Discard)
	require.NoError(t, err, "Could not init config")

	err = os.MkdirAll(filepath.Join(config.GetHomeDir()), 0755)
	require.NoError(t, err, "Could not create Opengist home directory")

	git.ReposDirectory = path.Join("tests")

	config.C.IndexEnabled = false
	config.C.LogLevel = "debug"
	config.InitLog()

	homePath := config.GetHomeDir()
	log.Info().Msg("Data directory: " + homePath)

	err = os.MkdirAll(filepath.Join(homePath, "tmp", "sessions"), 0755)
	require.NoError(t, err, "Could not create sessions directory")

	err = os.MkdirAll(filepath.Join(homePath, "tmp", "repos"), 0755)
	require.NoError(t, err, "Could not create tmp repos directory")

	err = db.Setup(databaseDsn, true)
	require.NoError(t, err, "Could not initialize database")

	if err != nil {
		log.Fatal().Err(err).Msg("Could not initialize database")
	}

	err = memdb.Setup()
	require.NoError(t, err, "Could not initialize in memory database")

	// err = index.Open(filepath.Join(homePath, "testsindex", "opengist.index"))
	// require.NoError(t, err, "Could not open index")
}

func teardown(t *testing.T, s *testServer) {
	s.stop()

	//err := db.Close()
	//require.NoError(t, err, "Could not close database")

	err := os.RemoveAll(path.Join(config.GetHomeDir(), "tests"))
	require.NoError(t, err, "Could not remove repos directory")

	err = os.RemoveAll(path.Join(config.GetHomeDir(), "tmp", "repos"))
	require.NoError(t, err, "Could not remove repos directory")

	err = os.RemoveAll(path.Join(config.GetHomeDir(), "tmp", "sessions"))
	require.NoError(t, err, "Could not remove repos directory")

	err = db.TruncateDatabase()
	require.NoError(t, err, "Could not truncate database")

	// err = os.RemoveAll(path.Join(config.C.OpengistHome, "testsindex"))
	// require.NoError(t, err, "Could not remove repos directory")

	// err = index.Close()
	// require.NoError(t, err, "Could not close index")
}
