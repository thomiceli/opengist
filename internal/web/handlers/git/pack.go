package git

import (
	"bytes"
	"compress/gzip"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/web/context"
)

func uploadPack(ctx *context.Context) error {
	return pack(ctx, "upload-pack")
}

func receivePack(ctx *context.Context) error {
	return pack(ctx, "receive-pack")
}

func pack(ctx *context.Context, serviceType string) error {
	noCacheHeaders(ctx)
	defer ctx.Request().Body.Close()

	if ctx.Request().Header.Get("Content-Type") != "application/x-git-"+serviceType+"-request" {
		return ctx.ErrorRes(401, "Git client unsupported", nil)
	}
	ctx.Response().Header().Set("Content-Type", "application/x-git-"+serviceType+"-result")

	var err error
	reqBody := ctx.Request().Body

	if ctx.Request().Header.Get("Content-Encoding") == "gzip" {
		reqBody, err = gzip.NewReader(reqBody)
		if err != nil {
			return ctx.ErrorRes(500, "Cannot create gzip reader", err)
		}
	}

	repositoryPath := ctx.GetData("repositoryPath").(string)
	gist := ctx.GetData("gist").(*db.Gist)

	// Guard against a stale/desynced reference pointing at a repository that no
	// longer exists on disk (e.g. an init gist whose empty repo was cleaned up).
	// Without this, git would fail with an opaque "chdir ...: no such file or
	// directory" and 500 the push.
	if fi, err := os.Stat(repositoryPath); err != nil || !fi.IsDir() {
		return ctx.ErrorRes(404, "Repository not found", err)
	}

	var stderr bytes.Buffer
	cmd := exec.Command("git", serviceType, "--stateless-rpc", repositoryPath)
	cmd.Dir = repositoryPath
	cmd.Stdin = reqBody
	cmd.Stdout = ctx.Response().Writer
	cmd.Stderr = &stderr
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "OPENGIST_REPOSITORY_URL_INTERNAL="+git.RepositoryUrl(ctx, gist.User.Username, gist.Identifier()))
	cmd.Env = append(cmd.Env, "OPENGIST_REPOSITORY_ID="+strconv.Itoa(int(gist.ID)))

	if err = cmd.Run(); err != nil {
		return ctx.ErrorRes(500, "Cannot run git "+serviceType+" ; "+stderr.String(), err)
	}

	return nil
}

func infoRefs(ctx *context.Context) error {
	noCacheHeaders(ctx)
	var service string

	gist := ctx.GetData("gist").(*db.Gist)

	serviceType := ctx.QueryParam("service")
	if strings.HasPrefix(serviceType, "git-") {
		service = strings.TrimPrefix(serviceType, "git-")
	}

	if service != "upload-pack" && service != "receive-pack" {
		if err := gist.UpdateServerInfo(); err != nil {
			return ctx.ErrorRes(500, "Cannot update server info", err)
		}
		return sendFile(ctx, "text/plain; charset=utf-8")
	}

	refs, err := gist.RPC(service)
	if err != nil {
		return ctx.ErrorRes(500, "Cannot run git "+service, err)
	}

	ctx.Response().Header().Set("Content-Type", "application/x-git-"+service+"-advertisement")
	ctx.Response().WriteHeader(200)
	_, _ = ctx.Response().Write(packetWrite("# service=git-" + service + "\n"))
	_, _ = ctx.Response().Write([]byte("0000"))
	_, _ = ctx.Response().Write(refs)

	return nil
}

func packetWrite(str string) []byte {
	s := strconv.FormatInt(int64(len(str)+4), 16)

	if len(s)%4 != 0 {
		s = strings.Repeat("0", 4-len(s)%4) + s
	}

	return []byte(s + str)
}
