package oauth

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
)

const maxOIDCAvatarSize = 5 << 20 // 5 MiB

var oidcAvatarTypes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

var oidcAvatarHTTPClient = http.DefaultClient
var oidcAvatarsDir = func() string {
	return filepath.Join(config.GetHomeDir(), "avatars", "users")
}

func resolveOIDCAvatarURL(avatarURL string, accessToken string) string {
	if avatarURL == "" {
		return ""
	}

	filename, err := cacheOIDCAvatar(avatarURL, accessToken)
	if err == nil {
		return filename
	}

	log.Debug().Err(err).Str("avatar_url", avatarURL).Msg("Cannot cache OIDC avatar locally")

	if isPublicRenderableAvatarURL(avatarURL) {
		return avatarURL
	}

	return ""
}

func cacheOIDCAvatar(avatarURL string, accessToken string) (string, error) {
	body, err := fetchOIDCAvatar(avatarURL, accessToken)
	if err != nil {
		return "", err
	}

	if len(body) == 0 {
		return "", fmt.Errorf("empty avatar response")
	}

	if len(body) > maxOIDCAvatarSize {
		return "", fmt.Errorf("avatar exceeds maximum size")
	}

	contentType := detectOIDCAvatarType(body)
	ext, ok := oidcAvatarTypes[contentType]
	if !ok {
		return "", fmt.Errorf("unsupported avatar content type %q", contentType)
	}

	if err := os.MkdirAll(oidcAvatarsDir(), 0755); err != nil {
		return "", err
	}

	filename := uuid.NewString() + ext
	path := filepath.Join(oidcAvatarsDir(), filename)
	if err := os.WriteFile(path, body, 0644); err != nil {
		return "", err
	}

	return filename, nil
}

func isPublicRenderableAvatarURL(avatarURL string) bool {
	parsed, err := url.Parse(avatarURL)
	if err != nil {
		return false
	}

	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return false
	}

	_, err = fetchOIDCAvatar(avatarURL, "")
	return err == nil
}

func fetchOIDCAvatar(avatarURL string, accessToken string) ([]byte, error) {
	request, err := http.NewRequest(http.MethodGet, avatarURL, nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Accept", "image/*")
	if accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+accessToken)
	}

	response, err := oidcAvatarHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected avatar response status %d", response.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxOIDCAvatarSize+1))
	if err != nil {
		return nil, err
	}

	if len(body) == 0 {
		return nil, fmt.Errorf("empty avatar response")
	}

	if len(body) > maxOIDCAvatarSize {
		return nil, fmt.Errorf("avatar exceeds maximum size")
	}

	contentType := detectOIDCAvatarType(body)
	if _, ok := oidcAvatarTypes[contentType]; !ok {
		return nil, fmt.Errorf("unsupported avatar content type %q", contentType)
	}

	return body, nil
}

func detectOIDCAvatarType(data []byte) string {
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}

	contentType := http.DetectContentType(data)
	if index := strings.IndexByte(contentType, ';'); index >= 0 {
		contentType = contentType[:index]
	}

	return contentType
}
