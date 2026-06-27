// Package ipc is the client side of Opengist's internal API: the small HTTP
// surface that short-lived subprocesses Opengist spawns of itself (Git hooks,
// and the SSH shim) use to talk to the long-running daemon.
//
// Subprocesses do not open the database. Instead they call the running daemon —
// which holds the warm connection pool and the search index — over its existing
// HTTP listener, authenticated with a token derived from the secret key. This
// avoids paying a fresh DB connection (and, previously, a full AutoMigrate) on
// every invocation. It mirrors Gitea's private/internal API.
//
// This file holds the shared transport and auth; the per-feature calls live in
// hooks.go and keys.go.
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
