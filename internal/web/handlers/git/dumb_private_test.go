package git_test

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

func dumbGet(t *testing.T, baseUrl, urlPath, creds string) (int, []byte) {
	req, err := http.NewRequest("GET", baseUrl+urlPath, nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "git/2.43.0")
	if creds != "" {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(creds)))
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}

func masterCommitSha(t *testing.T, baseUrl, user, gistId, ownerCreds string) string {
	code, body := dumbGet(t, baseUrl, fmt.Sprintf("/%s/%s.git/info/refs", user, gistId), ownerCreds)
	require.Equalf(t, 200, code, "owner must be able to read dumb info/refs (got %d)", code)
	for _, line := range strings.Split(string(body), "\n") {
		if strings.Contains(line, "refs/heads/") {
			return strings.Fields(line)[0]
		}
	}
	t.Fatalf("no ref found in info/refs body %q", string(body))
	return ""
}

func looseObjectPath(user, gistId, sha string) string {
	return fmt.Sprintf("/%s/%s.git/objects/%s/%s", user, gistId, sha[:2], sha[2:])
}

func inflateLooseObject(t *testing.T, baseUrl, user, gistId, sha, creds string) (objType string, payload []byte) {
	code, raw := dumbGet(t, baseUrl, looseObjectPath(user, gistId, sha), creds)
	require.Equalf(t, 200, code, "loose object %s should be served to the owner (got %d)", sha, code)
	zr, err := zlib.NewReader(bytes.NewReader(raw))
	require.NoError(t, err)
	dec, err := io.ReadAll(zr)
	require.NoError(t, err)
	nul := bytes.IndexByte(dec, 0)
	require.Greater(t, nul, 0)
	objType = strings.SplitN(string(dec[:nul]), " ", 2)[0]
	payload = dec[nul+1:]
	return objType, payload
}

func TestDumbHttpPrivateGistNotDisclosed(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	baseUrl := s.StartHttpServer(t)

	s.Register(t, "alice")

	_, _, user, privateId := s.CreateGist(t, "2")

	const ownerCreds = "thomas:thomas"

	commitSha := masterCommitSha(t, baseUrl, user, privateId, ownerCreds)
	require.Regexp(t, "^[0-9a-f]{40}$", commitSha)

	_, commit := inflateLooseObject(t, baseUrl, user, privateId, commitSha, ownerCreds)
	var treeSha string
	for _, line := range strings.Split(string(commit), "\n") {
		if strings.HasPrefix(line, "tree ") {
			treeSha = strings.TrimPrefix(line, "tree ")
			break
		}
	}
	require.Regexp(t, "^[0-9a-f]{40}$", treeSha)

	_, tree := inflateLooseObject(t, baseUrl, user, privateId, treeSha, ownerCreds)
	var blobSha string
	for len(tree) > 0 {
		nul := bytes.IndexByte(tree, 0)
		entry := string(tree[:nul]) // "<mode> <name>"
		sha := hex.EncodeToString(tree[nul+1 : nul+21])
		tree = tree[nul+21:]
		if strings.HasSuffix(entry, " file.txt") {
			blobSha = sha
			break
		}
	}
	require.Regexp(t, "^[0-9a-f]{40}$", blobSha)

	_, blob := inflateLooseObject(t, baseUrl, user, privateId, blobSha, ownerCreds)
	require.Equal(t, "hello world", string(blob), "owner must be able to read the private gist over dumb-http")

	dumbPaths := []string{
		fmt.Sprintf("/%s/%s.git/info/refs", user, privateId),
		fmt.Sprintf("/%s/%s.git/HEAD", user, privateId),
		fmt.Sprintf("/%s/%s.git/objects/info/packs", user, privateId),
		looseObjectPath(user, privateId, commitSha),
		looseObjectPath(user, privateId, treeSha),
		looseObjectPath(user, privateId, blobSha),
	}

	for _, creds := range []string{"alice:alice", "bogus:not-a-real-account", "thomas:wrongpassword"} {
		t.Run("denied/"+creds, func(t *testing.T) {
			for _, p := range dumbPaths {
				code, body := dumbGet(t, baseUrl, p, creds)
				require.NotEqualf(t, 200, code,
					"dumb path %s must NOT be served to non-owner %q (leaked body: %q)", p, creds, string(body))
				require.Equalf(t, 404, code,
					"dumb path %s should be denied with 404 for non-owner %q", p, creds)
			}
		})
	}

	// Anonymous (no Authorization header at all) is challenged with 401.
	t.Run("denied/anonymous", func(t *testing.T) {
		for _, p := range dumbPaths {
			code, _ := dumbGet(t, baseUrl, p, "")
			require.Equalf(t, 401, code, "dumb path %s should challenge an anonymous caller with 401", p)
		}
	})

	// The smart pull of the private gist by a non-owner must also stay denied.
	t.Run("smart-pull-non-owner-denied", func(t *testing.T) {
		code, _ := dumbGet(t, baseUrl,
			fmt.Sprintf("/%s/%s.git/info/refs?service=git-upload-pack", user, privateId), "alice:alice")
		require.Equal(t, 404, code, "smart pull of a private gist by a non-owner must be denied")
	})
}
