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
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/memdb"
	"github.com/thomiceli/opengist/internal/web/server"
)

var databaseType string

type TestServer struct {
	server        *server.Server
	sessionCookie string
}

func newTestServer() (*TestServer, error) {
	s := &TestServer{
		server: server.NewServer(true, path.Join(config.GetHomeDir(), "tmp", "sessions"), true),
	}

	go s.start()
	return s, nil
}

func (s *TestServer) start() {
	s.server.Start()
}

func (s *TestServer) stop() {
	s.server.Stop()
}

func (s *TestServer) Request(method, uri string, data interface{}, expectedCode int) error {
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

func Setup(t *testing.T) *TestServer {
	_ = os.Setenv("OPENGIST_SKIP_GIT_HOOKS", "1")

	err := config.InitConfig("", io.Discard)
	require.NoError(t, err, "Could not init config")

	err = os.MkdirAll(filepath.Join(config.GetHomeDir()), 0755)
	require.NoError(t, err, "Could not create Opengist home directory")

	config.SetupSecretKey()

	git.ReposDirectory = path.Join("tests")

	config.C.IndexEnabled = false
	config.C.LogLevel = "error"
	config.InitLog()

	homePath := config.GetHomeDir()
	log.Info().Msg("Data directory: " + homePath)

	var databaseDsn string
	databaseType = os.Getenv("OPENGIST_TEST_DB")
	switch databaseType {
	case "sqlite":
		databaseDsn = "file:" + filepath.Join(homePath, "tmp", "opengist.db")
	case "postgres":
		databaseDsn = "postgres://postgres:opengist@localhost:5432/opengist_test"
	case "mysql":
		databaseDsn = "mysql://root:opengist@localhost:3306/opengist_test"
	default:
		databaseDsn = ":memory:"
	}

	err = os.MkdirAll(filepath.Join(homePath, "tests"), 0755)
	require.NoError(t, err, "Could not create tests directory")

	err = os.MkdirAll(filepath.Join(homePath, "tmp", "sessions"), 0755)
	require.NoError(t, err, "Could not create sessions directory")

	err = os.MkdirAll(filepath.Join(homePath, "tmp", "repos"), 0755)
	require.NoError(t, err, "Could not create tmp repos directory")

	err = db.Setup(databaseDsn)
	require.NoError(t, err, "Could not initialize database")

	if err != nil {
		log.Fatal().Err(err).Msg("Could not initialize database")
	}

	err = memdb.Setup()
	require.NoError(t, err, "Could not initialize in memory database")

	// err = index.Open(filepath.Join(homePath, "testsindex", "opengist.index"))
	// require.NoError(t, err, "Could not open index")

	s, err := newTestServer()
	require.NoError(t, err, "Failed to create test server")

	return s
}

func Teardown(t *testing.T, s *TestServer) {
	s.stop()

	//err := db.Close()
	//require.NoError(t, err, "Could not close database")

	err := db.TruncateDatabase()
	require.NoError(t, err, "Could not truncate database")

	err = os.RemoveAll(path.Join(config.GetHomeDir(), "tests"))
	require.NoError(t, err, "Could not remove repos directory")

	if runtime.GOOS == "windows" {
		err = db.Close()
		require.NoError(t, err, "Could not close database")

		time.Sleep(2 * time.Second)
	}
	err = os.RemoveAll(path.Join(config.GetHomeDir(), "tmp"))
	require.NoError(t, err, "Could not remove tmp directory")

	// err = os.RemoveAll(path.Join(config.C.OpengistHome, "testsindex"))
	// require.NoError(t, err, "Could not remove repos directory")

	// err = index.Close()
	// require.NoError(t, err, "Could not close index")
}

type settingSet struct {
	key   string `form:"key"`
	value string `form:"value"`
}

type invitationAdmin struct {
	nbMax         string `form:"nbMax"`
	expiredAtUnix string `form:"expiredAtUnix"`
}
