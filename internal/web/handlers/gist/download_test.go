package gist_test

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

func TestDownloadZip(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	t.Run("MultipleFiles", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		resp := s.Request(t, "GET", "/"+username+"/"+identifier+"/archive/HEAD", nil, 200)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		zipReader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
		require.NoError(t, err)
		require.Len(t, zipReader.File, 2)

		fileNames := make([]string, len(zipReader.File))
		contents := make([]string, len(zipReader.File))
		for i, file := range zipReader.File {
			fileNames[i] = file.Name
			f, err := file.Open()
			require.NoError(t, err)
			content, err := io.ReadAll(f)
			require.NoError(t, err)
			contents[i] = string(content)
			f.Close()
		}
		require.ElementsMatch(t, []string{"file.txt", "otherfile.txt"}, fileNames)
		require.ElementsMatch(t, []string{"hello world", "other content"}, contents)
	})

	t.Run("PrivateGist", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "2")

		s.Request(t, "GET", "/"+username+"/"+identifier+"/archive/HEAD", nil, 404)
	})

	t.Run("NonExistentRevision", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		// TODO: return 404
		s.Request(t, "GET", "/"+username+"/"+identifier+"/archive/zz", nil, 0)
	})
}

func TestRawFile(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	t.Run("ExistingFile", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		resp := s.Request(t, "GET", "/"+username+"/"+identifier+"/raw/HEAD/file.txt", nil, 200)

		require.Equal(t, `inline; filename="file.txt"`, resp.Header.Get("Content-Disposition"))
		require.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
		require.Contains(t, resp.Header.Get("Content-Type"), "text/plain")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "hello world", string(body))
	})

	t.Run("NonExistentFile", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Request(t, "GET", "/"+username+"/"+identifier+"/raw/HEAD/nonexistent.txt", nil, 404)
	})

	t.Run("NonExistentRevision", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Request(t, "GET", "/"+username+"/"+identifier+"/raw/zz/file.txt", nil, 404)
	})

	t.Run("PrivateGist", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "2")

		s.Request(t, "GET", "/"+username+"/"+identifier+"/raw/HEAD/file.txt", nil, 404)
	})
}

func TestDownloadFile(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	t.Run("ExistingFile", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		resp := s.Request(t, "GET", "/"+username+"/"+identifier+"/download/HEAD/file.txt", nil, 200)

		require.Equal(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))
		require.Equal(t, `attachment; filename="file.txt"`, resp.Header.Get("Content-Disposition"))
		require.Equal(t, "11", resp.Header.Get("Content-Length"))
		require.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "hello world", string(body))
	})

	t.Run("NonExistentFile", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		resp := s.Request(t, "GET", "/"+username+"/"+identifier+"/download/HEAD/nonexistent.txt", nil, 404)

		_, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		// TODO: change the response to not found
		// require.Equal(t, "File not found", string(body))
	})

	t.Run("NonExistentRevision", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		resp := s.Request(t, "GET", "/"+username+"/"+identifier+"/download/zz/file.txt", nil, 404)

		_, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		// TODO: change the response to not found
		// require.Equal(t, "File not found", string(body))
	})

	t.Run("PrivateGist", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "2")

		s.Request(t, "GET", "/"+username+"/"+identifier+"/download/HEAD/file.txt", nil, 404)
	})
}
