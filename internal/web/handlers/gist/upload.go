package gist

import (
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/google/uuid"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/web/context"
)

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func Upload(ctx *context.Context) error {
	err := ctx.Request().ParseMultipartForm(32 << 20) // 32 MB max
	if err != nil {
		return ctx.ErrorRes(400, ctx.Tr("error.bad-request"), err)
	}

	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		return ctx.ErrorRes(400, ctx.Tr("error.no-file-uploaded"), err)
	}

	file, err := fileHeader.Open()
	if err != nil {
		return ctx.ErrorRes(400, ctx.Tr("error.cannot-open-file"), err)
	}
	defer file.Close()

	fileUUID, err := uuid.NewRandom()
	if err != nil {
		return ctx.ErrorRes(500, "Error generating UUID", err)
	}

	uploadsDir := filepath.Join(config.GetHomeDir(), "uploads")
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		return ctx.ErrorRes(500, "Error creating uploads directory", err)
	}

	filename := fileUUID.String()
	filePath := filepath.Join(uploadsDir, filename)

	destFile, err := os.Create(filePath)
	if err != nil {
		return ctx.ErrorRes(500, "Error creating file", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, file); err != nil {
		return ctx.ErrorRes(500, "Error saving file", err)
	}

	return ctx.JSON(200, map[string]string{
		"uuid":     filename,
		"filename": fileHeader.Filename,
	})
}

func DeleteUpload(ctx *context.Context) error {
	fileUuid := filepath.Base(ctx.Param("uuid"))

	if fileUuid == "" || !uuidRegex.MatchString(fileUuid) {
		return ctx.ErrorRes(400, ctx.Tr("error.bad-request"), nil)
	}

	filePath := filepath.Join(config.GetHomeDir(), "uploads", fileUuid)

	if _, err := os.Stat(filePath); err == nil {
		if err := os.Remove(filePath); err != nil {
			return ctx.ErrorRes(500, "Error deleting file", err)
		}
	}

	return ctx.JSON(200, map[string]string{
		"status": "deleted",
	})
}
