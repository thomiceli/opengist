package git

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/thomiceli/opengist/internal/web/context"
)

func textFile(ctx *context.Context) error {
	noCacheHeaders(ctx)
	return sendFile(ctx, "text/plain")
}

func infoPacks(ctx *context.Context) error {
	cacheHeadersForever(ctx)
	return sendFile(ctx, "text/plain; charset=utf-8")
}

func looseObject(ctx *context.Context) error {
	cacheHeadersForever(ctx)
	return sendFile(ctx, "application/x-git-loose-object")
}

func packFile(ctx *context.Context) error {
	cacheHeadersForever(ctx)
	return sendFile(ctx, "application/x-git-packed-objects")
}

func idxFile(ctx *context.Context) error {
	cacheHeadersForever(ctx)
	return sendFile(ctx, "application/x-git-packed-objects-toc")
}

func sendFile(ctx *context.Context, contentType string) error {
	gitFile := "/" + strings.Join(strings.Split(ctx.Request().URL.Path, "/")[3:], "/")
	gitFile = path.Join(ctx.GetData("repositoryPath").(string), gitFile)
	fi, err := os.Stat(gitFile)
	if os.IsNotExist(err) {
		return ctx.ErrorRes(404, "File not found", nil)
	}
	ctx.Response().Header().Set("Content-Type", contentType)
	ctx.Response().Header().Set("Content-Length", fmt.Sprintf("%d", fi.Size()))
	ctx.Response().Header().Set("Last-Modified", fi.ModTime().Format(http.TimeFormat))
	return ctx.File(gitFile)
}

func noCacheHeaders(ctx *context.Context) {
	ctx.Response().Header().Set("Expires", "Thu, 01 Jan 1970 00:00:00 UTC")
	ctx.Response().Header().Set("Pragma", "no-cache")
	ctx.Response().Header().Set("Cache-Control", "no-cache, max-age=0, must-revalidate")
}

func cacheHeadersForever(ctx *context.Context) {
	now := time.Now().Unix()
	expires := now + 31536000
	ctx.Response().Header().Set("Date", fmt.Sprintf("%d", now))
	ctx.Response().Header().Set("Expires", fmt.Sprintf("%d", expires))
	ctx.Response().Header().Set("Cache-Control", "public, max-age=31536000")
}
