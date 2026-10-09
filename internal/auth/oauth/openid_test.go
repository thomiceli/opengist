package oauth

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/markbates/goth"
	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
)

func TestOIDCCallbackProviderUpdateUserDBNormalizesAvatarURL(t *testing.T) {
	tempDir := t.TempDir()
	previousDir := oidcAvatarsDir
	oidcAvatarsDir = func() string { return tempDir }
	defer func() { oidcAvatarsDir = previousDir }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer oidc-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte{137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 0})
	}))
	defer server.Close()

	provider := NewOIDCCallbackProvider(&goth.User{
		UserID:      "oidc-user",
		AvatarURL:   server.URL + "/avatar",
		AccessToken: "oidc-token",
	})

	user := &db.User{}
	provider.UpdateUserDB(user)

	require.Equal(t, "oidc-user", user.OIDCID)
	require.NotEmpty(t, user.AvatarURL)
	require.False(t, filepath.IsAbs(user.AvatarURL))
	require.FileExists(t, filepath.Join(tempDir, user.AvatarURL))
}

func TestNormalizeProviderAvatarURLKeepsValidURLs(t *testing.T) {
	require.Equal(t, "https://example.com/avatar.png", normalizeProviderAvatarURL("https://example.com/avatar.png"))
	require.Equal(t, "/avatar/local.png", normalizeProviderAvatarURL("/avatar/local.png"))
	require.Equal(t, "https://cdn.example.com/avatar.png", normalizeProviderAvatarURL("//cdn.example.com/avatar.png"))
}

func TestOIDCCallbackProviderUpdateUserDBKeepsRenderableAvatarURL(t *testing.T) {
	provider := NewOIDCCallbackProvider(&goth.User{
		UserID:    "oidc-user",
		AvatarURL: "https://example.com/avatar.png",
	})

	user := &db.User{}
	provider.UpdateUserDB(user)

	require.Equal(t, "https://example.com/avatar.png", user.AvatarURL)
}

func TestResolveOIDCAvatarURLReturnsEmptyForNonPublicUndownloadableAvatar(t *testing.T) {
	require.Empty(t, resolveOIDCAvatarURL("urn:avatar:test", ""))
}

func TestResolveOIDCAvatarURLReturnsEmptyForProtectedRemoteAvatar(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer oidc-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	require.Empty(t, resolveOIDCAvatarURL(server.URL+"/avatar", "oidc-token"))
}

func TestResolveOIDCAvatarURLKeepsPublicRemoteAvatarWhenCachingFails(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			http.Error(w, "unauthorized bearer", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte{137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 0})
	}))
	defer server.Close()

	require.Equal(t, server.URL+"/avatar", resolveOIDCAvatarURL(server.URL+"/avatar", "oidc-token"))
}

func TestResolveOIDCAvatarURLStoresAvatarLocally(t *testing.T) {
	tempDir := t.TempDir()
	previousDir := oidcAvatarsDir
	oidcAvatarsDir = func() string { return tempDir }
	defer func() { oidcAvatarsDir = previousDir }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer oidc-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte{137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 0})
	}))
	defer server.Close()

	avatarURL := resolveOIDCAvatarURL(server.URL+"/avatar", "oidc-token")
	require.NotEmpty(t, avatarURL)
	require.FileExists(t, filepath.Join(tempDir, avatarURL))
	_, err := os.Stat(filepath.Join(tempDir, avatarURL))
	require.NoError(t, err)
}