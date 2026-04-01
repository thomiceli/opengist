package gist

import (
	"archive/zip"
	"bytes"
	"net/url"
	"strconv"

	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
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

	if file.MimeType.IsSVG() {
		ctx.Response().Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	} else if file.MimeType.IsPDF() {
		ctx.Response().Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	}

	if file.MimeType.CanBeEmbedded() {
		ctx.Response().Header().Set("Content-Type", file.MimeType.ContentType)
	} else if file.MimeType.IsText() {
		ctx.Response().Header().Set("Content-Type", "text/plain; charset=utf-8")
	} else {
		ctx.Response().Header().Set("Content-Type", "application/octet-stream")
	}

	ctx.Response().Header().Set("Content-Disposition", "inline; filename=\""+url.PathEscape(file.Filename)+"\"")
	ctx.Response().Header().Set("X-Content-Type-Options", "nosniff")
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

	ctx.Response().Header().Set("Content-Type", file.MimeType.ContentType)
	ctx.Response().Header().Set("Content-Disposition", "attachment; filename=\""+url.PathEscape(file.Filename)+"\"")
	ctx.Response().Header().Set("Content-Length", strconv.Itoa(len(file.Content)))
	ctx.Response().Header().Set("X-Content-Type-Options", "nosniff")
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
