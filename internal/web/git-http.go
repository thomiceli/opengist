package web

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	"opengist/internal/git"
	"opengist/internal/models"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var routes = []struct {
	gitUrl  string
	method  string
	handler func(ctx echo.Context) error
}{
	{"(.*?)/git-upload-pack$", "POST", uploadPack},
	{"(.*?)/git-receive-pack$", "POST", receivePack},
	{"(.*?)/info/refs$", "GET", infoRefs},
	{"(.*?)/HEAD$", "GET", textFile},
	{"(.*?)/objects/info/alternates$", "GET", textFile},
	{"(.*?)/objects/info/http-alternates$", "GET", textFile},
	{"(.*?)/objects/info/packs$", "GET", infoPacks},
	{"(.*?)/objects/info/[^/]*$", "GET", textFile},
	{"(.*?)/objects/[0-9a-f]{2}/[0-9a-f]{38}$", "GET", looseObject},
	{"(.*?)/objects/pack/pack-[0-9a-f]{40}\\.pack$", "GET", packFile},
	{"(.*?)/objects/pack/pack-[0-9a-f]{40}\\.idx$", "GET", idxFile},
}

func gitHttp(ctx echo.Context) error {
	for _, route := range routes {
		matched, _ := regexp.MatchString(route.gitUrl, ctx.Request().URL.Path)
		if ctx.Request().Method == route.method && matched {
			if !strings.HasPrefix(ctx.Request().Header.Get("User-Agent"), "git/") {
				continue
			}

			gist := getData(ctx, "gist").(*models.Gist)

			noAuth := ctx.QueryParam("service") == "git-upload-pack" ||
				strings.HasSuffix(ctx.Request().URL.Path, "git-upload-pack") ||
				ctx.Request().Method == "GET"

			repositoryPath := git.RepositoryPath(gist.User.Username, gist.Uuid)

			if _, err := os.Stat(repositoryPath); os.IsNotExist(err) {
				if err != nil {
					return errorRes(500, "Repository does not exist", err)
				}
			}

			ctx.Set("repositoryPath", repositoryPath)

			// Requires Basic Auth if we push the repository
			if noAuth {
				return route.handler(ctx)
			}

			authHeader := ctx.Request().Header.Get("Authorization")
			if authHeader == "" {
				return basicAuth(ctx)
			}

			authFields := strings.Fields(authHeader)
			if len(authFields) != 2 || authFields[0] != "Basic" {
				return basicAuth(ctx)
			}

			authUsername, authPassword, err := basicAuthDecode(authFields[1])
			if err != nil {
				return basicAuth(ctx)
			}

			if ok, err := argon2id.verify(authPassword, gist.User.Password); !ok || gist.User.Username != authUsername {
				if err != nil {
					return errorRes(500, "Cannot verify password", err)
				}
				return errorRes(403, "Unauthorized", nil)
			}

			return route.handler(ctx)
		}
	}
	return notFound("Gist not found")
}

func uploadPack(ctx echo.Context) error {
	return pack(ctx, "upload-pack")
}

func receivePack(ctx echo.Context) error {
	return pack(ctx, "receive-pack")
}

func pack(ctx echo.Context, serviceType string) error {
	noCacheHeaders(ctx)
	defer ctx.Request().Body.Close()

	if ctx.Request().Header.Get("Content-Type") != "application/x-git-"+serviceType+"-request" {
		return errorRes(401, "Git client unsupported", nil)
	}
	ctx.Response().Header().Set("Content-Type", "application/x-git-"+serviceType+"-result")

	var err error
	reqBody := ctx.Request().Body

	if ctx.Request().Header.Get("Content-Encoding") == "gzip" {
		reqBody, err = gzip.NewReader(reqBody)
		if err != nil {
			return errorRes(500, "Cannot create gzip reader", err)
		}
	}

	repositoryPath := ctx.Get("repositoryPath").(string)

	var stderr bytes.Buffer
	cmd := exec.Command("git", serviceType, "--stateless-rpc", repositoryPath)
	cmd.Dir = repositoryPath
	cmd.Stdin = reqBody
	cmd.Stdout = ctx.Response().Writer
	cmd.Stderr = &stderr
	if err = cmd.Run(); err != nil {
		return errorRes(500, "Cannot run git "+serviceType+" ; "+stderr.String(), err)
	}

	// updatedAt is updated only if serviceType is receive-pack
	if serviceType == "receive-pack" {
		_ = getData(ctx, "gist").(*models.Gist).SetLastActiveNow()
	}
	return nil
}

func infoRefs(ctx echo.Context) error {
	noCacheHeaders(ctx)
	var service string

	gist := getData(ctx, "gist").(*models.Gist)

	serviceType := ctx.QueryParam("service")
	if !strings.HasPrefix(serviceType, "git-") {
		service = ""
	}
	service = strings.TrimPrefix(serviceType, "git-")

	if service != "upload-pack" && service != "receive-pack" {
		if err := git.UpdateServerInfo(gist.User.Username, gist.Uuid); err != nil {
			return errorRes(500, "Cannot update server info", err)
		}
		return sendFile(ctx, "text/plain; charset=utf-8")
	}

	refs, err := git.RPCRefs(gist.User.Username, gist.Uuid, service)
	if err != nil {
		return errorRes(500, "Cannot run git "+service, err)
	}

	ctx.Response().Header().Set("Content-Type", "application/x-git-"+service+"-advertisement")
	ctx.Response().WriteHeader(200)
	_, _ = ctx.Response().Write(packetWrite("# service=git-" + service + "\n"))
	_, _ = ctx.Response().Write([]byte("0000"))
	_, _ = ctx.Response().Write(refs)

	return nil
}

func textFile(ctx echo.Context) error {
	noCacheHeaders(ctx)
	return sendFile(ctx, "text/plain")
}

func infoPacks(ctx echo.Context) error {
	cacheHeadersForever(ctx)
	return sendFile(ctx, "text/plain; charset=utf-8")
}

func looseObject(ctx echo.Context) error {
	cacheHeadersForever(ctx)
	return sendFile(ctx, "application/x-git-loose-object")
}

func packFile(ctx echo.Context) error {
	cacheHeadersForever(ctx)
	return sendFile(ctx, "application/x-git-packed-objects")
}

func idxFile(ctx echo.Context) error {
	cacheHeadersForever(ctx)
	return sendFile(ctx, "application/x-git-packed-objects-toc")
}

func noCacheHeaders(ctx echo.Context) {
	ctx.Response().Header().Set("Expires", "Thu, 01 Jan 1970 00:00:00 UTC")
	ctx.Response().Header().Set("Pragma", "no-cache")
	ctx.Response().Header().Set("Cache-Control", "no-cache, max-age=0, must-revalidate")
}

func cacheHeadersForever(ctx echo.Context) {
	now := time.Now().Unix()
	expires := now + 31536000
	ctx.Response().Header().Set("Date", fmt.Sprintf("%d", now))
	ctx.Response().Header().Set("Expires", fmt.Sprintf("%d", expires))
	ctx.Response().Header().Set("Cache-Control", "public, max-age=31536000")
}

func basicAuth(ctx echo.Context) error {
	ctx.Response().Header().Set("WWW-Authenticate", `Basic realm="."`)
	return plainText(ctx, 401, "Requires authentication")
}

func basicAuthDecode(encoded string) (string, string, error) {
	s, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", "", err
	}

	auth := strings.SplitN(string(s), ":", 2)
	return auth[0], auth[1], nil
}

func sendFile(ctx echo.Context, contentType string) error {
	gitFile := "/" + strings.Join(strings.Split(ctx.Request().URL.Path, "/")[3:], "/")
	gitFile = path.Join(ctx.Get("repositoryPath").(string), gitFile)
	fi, err := os.Stat(gitFile)
	if os.IsNotExist(err) {
		return errorRes(404, "File not found", nil)
	}
	ctx.Response().Header().Set("Content-Type", contentType)
	ctx.Response().Header().Set("Content-Length", fmt.Sprintf("%d", fi.Size()))
	ctx.Response().Header().Set("Last-Modified", fi.ModTime().Format(http.TimeFormat))
	return ctx.File(gitFile)
}

func packetWrite(str string) []byte {
	s := strconv.FormatInt(int64(len(str)+4), 16)

	if len(s)%4 != 0 {
		s = strings.Repeat("0", 4-len(s)%4) + s
	}

	return []byte(s + str)
}
