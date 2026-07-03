package settings

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
)

const maxAvatarSize = 5 << 20 // 5 MiB

var allowedAvatarTypes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

func AvatarsDir() string {
	return filepath.Join(config.GetHomeDir(), "avatars", "users")
}

func AvatarProcess(ctx *context.Context) error {
	user := ctx.User

	header, err := ctx.FormFile("avatar")
	if err != nil {
		ctx.AddFlash(ctx.Tr("flash.user.avatar-invalid"), "error")
		return ctx.RedirectTo("/settings")
	}

	if header.Size > maxAvatarSize {
		ctx.AddFlash(ctx.Tr("flash.user.avatar-too-large"), "error")
		return ctx.RedirectTo("/settings")
	}

	src, err := header.Open()
	if err != nil {
		return ctx.ErrorRes(500, "Cannot open uploaded avatar", err)
	}
	defer src.Close()

	// Detect the content type from the file content rather than trusting the
	// client-provided header, then map it to an allowed extension.
	sniff := make([]byte, 512)
	n, _ := io.ReadFull(src, sniff)
	contentType := detectImageType(sniff[:n])
	ext, ok := allowedAvatarTypes[contentType]
	if !ok {
		ctx.AddFlash(ctx.Tr("flash.user.avatar-invalid"), "error")
		return ctx.RedirectTo("/settings")
	}

	if _, err = src.Seek(0, io.SeekStart); err != nil {
		return ctx.ErrorRes(500, "Cannot read uploaded avatar", err)
	}

	if err = os.MkdirAll(AvatarsDir(), 0755); err != nil {
		return ctx.ErrorRes(500, "Cannot create avatars directory", err)
	}

	removeAvatarFile(user)

	filename := fmt.Sprintf("%d%s", user.ID, ext)
	dstPath := filepath.Join(AvatarsDir(), filename)
	dst, err := os.Create(dstPath)
	if err != nil {
		return ctx.ErrorRes(500, "Cannot save uploaded avatar", err)
	}
	defer dst.Close()

	if _, err = io.Copy(dst, src); err != nil {
		return ctx.ErrorRes(500, "Cannot save uploaded avatar", err)
	}

	user.AvatarURL = filename
	if err = user.Update(); err != nil {
		return ctx.ErrorRes(500, "Cannot update avatar", err)
	}

	ctx.AddFlash(ctx.Tr("flash.user.avatar-updated"), "success")
	return ctx.RedirectTo("/settings")
}

func AvatarDelete(ctx *context.Context) error {
	user := ctx.User

	if !user.HasUploadedAvatar() {
		return ctx.RedirectTo("/settings")
	}

	removeAvatarFile(user)

	user.AvatarURL = ""
	if err := user.Update(); err != nil {
		return ctx.ErrorRes(500, "Cannot delete avatar", err)
	}

	ctx.AddFlash(ctx.Tr("flash.user.avatar-deleted"), "success")
	return ctx.RedirectTo("/settings")
}

func removeAvatarFile(user *db.User) {
	if !user.HasUploadedAvatar() {
		return
	}
	_ = os.Remove(filepath.Join(AvatarsDir(), filepath.Base(user.AvatarURL)))
}

// detectImageType returns the image content type for the given header bytes,
// including webp which Go's http.DetectContentType does not recognize.
func detectImageType(data []byte) string {
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}
	ct := http.DetectContentType(data)
	// http.DetectContentType may append parameters, keep only the media type.
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return ct
}
