package gist

import (
	"archive/zip"
	"bytes"
	"strconv"

	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
	"github.com/thomiceli/opengist/internal/web/handlers"
)

func RawFile(ctx *context.Context) error {
	gist := ctx.GetData("gist").(*db.Gist)
	file, err := gist.File(ctx.Param("revision"), ctx.Param("file"), false)
	if err != nil {
		return ctx.ErrorRes(500, "Error getting file content", err)
	}

	if file == nil {
		return ctx.NotFound("File not found")
	}
	contentType := handlers.GetContentTypeFromFilename(file.Filename)
	ContentDisposition := handlers.GetContentDisposition(file.Filename)
	ctx.Response().Header().Set("Content-Type", contentType)
	ctx.Response().Header().Set("Content-Disposition", ContentDisposition)
	return ctx.PlainText(200, file.Content)
}

func DownloadFile(ctx *context.Context) error {
	gist := ctx.GetData("gist").(*db.Gist)
	file, err := gist.File(ctx.Param("revision"), ctx.Param("file"), false)
	if err != nil {
		return ctx.ErrorRes(500, "Error getting file content", err)
	}

	if file == nil {
		return ctx.NotFound("File not found")
	}

	ctx.Response().Header().Set("Content-Type", "text/plain")
	ctx.Response().Header().Set("Content-Disposition", "attachment; filename="+file.Filename)
	ctx.Response().Header().Set("Content-Length", strconv.Itoa(len(file.Content)))
	_, err = ctx.Response().Write([]byte(file.Content))
	if err != nil {
		return ctx.ErrorRes(500, "Error downloading the file", err)
	}

	return nil
}

func DownloadZip(ctx *context.Context) error {
	gist := ctx.GetData("gist").(*db.Gist)
	revision := ctx.Param("revision")

	files, err := gist.Files(revision, false)
	if err != nil {
		return ctx.ErrorRes(500, "Error fetching files from repository", err)
	}
	if len(files) == 0 {
		return ctx.NotFound("No files found in this revision")
	}

	zipFile := new(bytes.Buffer)

	zipWriter := zip.NewWriter(zipFile)

	for _, file := range files {
		fh := &zip.FileHeader{
			Name:   file.Filename,
			Method: zip.Deflate,
		}
		f, err := zipWriter.CreateHeader(fh)
		if err != nil {
			return ctx.ErrorRes(500, "Error adding a file the to the zip archive", err)
		}
		_, err = f.Write([]byte(file.Content))
		if err != nil {
			return ctx.ErrorRes(500, "Error adding file content the to the zip archive", err)
		}
	}
	err = zipWriter.Close()
	if err != nil {
		return ctx.ErrorRes(500, "Error closing the zip archive", err)
	}

	ctx.Response().Header().Set("Content-Type", "application/zip")
	ctx.Response().Header().Set("Content-Disposition", "attachment; filename="+gist.Identifier()+".zip")
	ctx.Response().Header().Set("Content-Length", strconv.Itoa(len(zipFile.Bytes())))
	_, err = ctx.Response().Write(zipFile.Bytes())
	if err != nil {
		return ctx.ErrorRes(500, "Error writing the zip archive", err)
	}
	return nil
}
