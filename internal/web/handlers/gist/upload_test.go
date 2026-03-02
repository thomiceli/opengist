package gist_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/config"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

func createMultipartRequest(t *testing.T, uri, fieldName, fileName, content string) *http.Request {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(fieldName, fileName)
	require.NoError(t, err)
	_, err = part.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, uri, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestUpload(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")

	t.Run("UploadFile", func(t *testing.T) {
		s.Login(t, "thomas")

		req := createMultipartRequest(t, "/upload", "file", "test.txt", "file content")

		resp := s.RawRequest(t, req, 200)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var result map[string]string
		err = json.Unmarshal(body, &result)
		require.NoError(t, err)

		require.Equal(t, "test.txt", result["filename"])
		require.NotEmpty(t, result["uuid"])
		require.Regexp(t, `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`, result["uuid"])

		filePath := filepath.Join(config.GetHomeDir(), "uploads", result["uuid"])
		data, err := os.ReadFile(filePath)
		require.NoError(t, err)
		require.Equal(t, "file content", string(data))
	})

	t.Run("NoFile", func(t *testing.T) {
		s.Login(t, "thomas")

		req := httptest.NewRequest(http.MethodPost, "/upload", nil)
		req.Header.Set("Content-Type", "multipart/form-data; boundary=xxx")

		s.RawRequest(t, req, 400)
	})

	t.Run("NoAuth", func(t *testing.T) {
		s.Logout()

		req := createMultipartRequest(t, "/upload", "file", "test.txt", "content")

		s.RawRequest(t, req, 302)
	})
}

func TestDeleteUpload(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")

	t.Run("DeleteExistingFile", func(t *testing.T) {
		s.Login(t, "thomas")

		req := createMultipartRequest(t, "/upload", "file", "todelete.txt", "delete me")

		uploadResp := s.RawRequest(t, req, 200)

		body, err := io.ReadAll(uploadResp.Body)
		require.NoError(t, err)
		var uploadResult map[string]string
		err = json.Unmarshal(body, &uploadResult)
		require.NoError(t, err)
		fileUUID := uploadResult["uuid"]

		filePath := filepath.Join(config.GetHomeDir(), "uploads", fileUUID)
		_, err = os.Stat(filePath)
		require.NoError(t, err)

		deleteReq := httptest.NewRequest(http.MethodDelete, "/upload/"+fileUUID, nil)

		deleteResp := s.RawRequest(t, deleteReq, 200)

		deleteBody, err := io.ReadAll(deleteResp.Body)
		require.NoError(t, err)
		var deleteResult map[string]string
		err = json.Unmarshal(deleteBody, &deleteResult)
		require.NoError(t, err)
		require.Equal(t, "deleted", deleteResult["status"])

		_, err = os.Stat(filePath)
		require.True(t, os.IsNotExist(err))
	})

	t.Run("DeleteNonExistentFile", func(t *testing.T) {
		s.Login(t, "thomas")

		req := httptest.NewRequest(http.MethodDelete, "/upload/00000000-0000-0000-0000-000000000000", nil)

		s.RawRequest(t, req, 200)
	})

	t.Run("InvalidUUID", func(t *testing.T) {
		s.Login(t, "thomas")

		req := httptest.NewRequest(http.MethodDelete, "/upload/not-a-valid-uuid", nil)

		s.RawRequest(t, req, 400)
	})

	t.Run("PathTraversal", func(t *testing.T) {
		s.Login(t, "thomas")

		req := httptest.NewRequest(http.MethodDelete, "/upload/../../etc/passwd", nil)

		s.RawRequest(t, req, 400)
	})

	t.Run("NoAuth", func(t *testing.T) {
		s.Logout()

		req := httptest.NewRequest(http.MethodDelete, "/upload/00000000-0000-0000-0000-000000000000", nil)

		s.RawRequest(t, req, 302)
	})
}
