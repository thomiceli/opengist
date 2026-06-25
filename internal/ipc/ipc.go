// Package ipc is the client side of Opengist's internal API: the small HTTP
// surface that short-lived subprocesses Opengist spawns of itself (Git hooks,
// and later the SSH shim) use to talk to the long-running daemon.
//
// Subprocesses do not open the database. Instead they call the running daemon —
// which holds the warm connection pool and the search index — over its existing
// HTTP listener, authenticated with a token derived from the secret key. This
// avoids paying a fresh DB connection (and, previously, a full AutoMigrate) on
// every invocation. It mirrors Gitea's private/internal API.
package ipc

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/thomiceli/opengist/internal/config"
)

// AuthHeader carries the internal API token on requests from subprocesses to
// the daemon.
const AuthHeader = "X-Opengist-Internal"

// Token derives the shared secret used to authenticate internal API calls. It
// is derived from — not equal to — the session SecretKey, so the raw session
// secret never travels on the wire. The daemon and its subprocesses compute the
// same value because they load the same SecretKey.
func Token() string {
	mac := hmac.New(sha256.New, config.SecretKey)
	mac.Write([]byte("opengist-internal-api"))
	return hex.EncodeToString(mac.Sum(nil))
}

// HookRefUpdate is one line of a Git hook's stdin: the old/new revision of a
// ref being updated.
type HookRefUpdate struct {
	OldRev  string `json:"old_rev"`
	NewRev  string `json:"new_rev"`
	RefName string `json:"ref_name"`
}

// HookPreReceiveRequest is sent by the pre-receive hook subprocess to the
// daemon. The subprocess computes ChangedFiles itself — one raw `git log`
// output per updated ref — because the pushed objects are only visible to the
// hook's environment (push quarantine) until pre-receive succeeds; the daemon
// only applies the policy.
type HookPreReceiveRequest struct {
	ChangedFiles []string `json:"changed_files"`
}

// HookPreReceiveResponse carries the daemon's decision and, when rejected, the
// message to show the user pushing.
type HookPreReceiveResponse struct {
	Allowed bool   `json:"allowed"`
	Message string `json:"message"`
}

// HookPreReceive asks the daemon whether a push may proceed.
func HookPreReceive(req *HookPreReceiveRequest) (*HookPreReceiveResponse, error) {
	resp := &HookPreReceiveResponse{}
	if err := post("/api/ipc/hook/pre-receive", req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// HookPostReceiveRequest is sent by the post-receive hook subprocess to the
// daemon.
type HookPostReceiveRequest struct {
	GistID      string            `json:"gist_id"`
	GistURL     string            `json:"gist_url"`
	References  []HookRefUpdate   `json:"references"`
	PushOptions map[string]string `json:"push_options"`
}

// HookPostReceiveResponse carries the text the daemon wants shown to the user
// pushing.
type HookPostReceiveResponse struct {
	Output string `json:"output"`
}

// HookPostReceive forwards a post-receive event to the daemon, which performs
// the database and index work and returns any message to show the user.
func HookPostReceive(req *HookPostReceiveRequest) (*HookPostReceiveResponse, error) {
	resp := &HookPostReceiveResponse{}
	if err := post("/api/ipc/hook/post-receive", req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// client builds an HTTP client and base URL targeting the running daemon's
// listener, whether it is bound to a TCP address or a unix socket. On Windows
// the daemon is always on TCP, so the unix-socket branch is never taken there.
func client() (*http.Client, string) {
	host := config.C.HttpHost
	port := config.C.HttpPort

	if strings.ContainsAny(host, `/\`) {
		socket := host
		return &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return (&net.Dialer{}).DialContext(ctx, "unix", socket)
				},
			},
		}, "http://unix"
	}

	if host == "0.0.0.0" || host == "::" || host == "" {
		host = "127.0.0.1"
	}
	return &http.Client{}, "http://" + host + ":" + port
}

func post(path string, reqBody, respBody any) error {
	httpClient, baseURL := client()

	var buf bytes.Buffer
	if reqBody != nil {
		if err := json.NewEncoder(&buf).Encode(reqBody); err != nil {
			return err
		}
	}

	httpReq, err := http.NewRequest(http.MethodPost, baseURL+path, &buf)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set(AuthHeader, Token())

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("could not reach the Opengist server's internal API (is it running?): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("internal API error (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if respBody != nil {
		return json.NewDecoder(resp.Body).Decode(respBody)
	}
	return nil
}
