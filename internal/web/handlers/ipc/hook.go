// Package ipc implements the daemon side of Opengist's internal API:
// endpoints called by short-lived subprocesses (Git hooks, the SSH shim) so the
// DB and index work happens in the long-running server with its warm
// connection pool, instead of being opened fresh on every invocation.
//
// These routes are mounted under /api/ipc and protected by a token middleware;
// they are not part of the public site or the v1 API.
package ipc

import (
	"net/http"

	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/hooks"
	"github.com/thomiceli/opengist/internal/ipc"
	"github.com/thomiceli/opengist/internal/web/context"
)

// PreReceive handles a pre-receive event forwarded by the hook subprocess and
// returns whether the push may proceed.
func PreReceive(ctx *context.Context) error {
	var req ipc.HookPreReceiveRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.String(http.StatusBadRequest, "invalid request")
	}

	allowed, message := hooks.RunPreReceive(req.ChangedFiles)

	return ctx.JSON(http.StatusOK, ipc.HookPreReceiveResponse{Allowed: allowed, Message: message})
}

// PostReceive handles a post-receive event forwarded by the hook subprocess.
func PostReceive(ctx *context.Context) error {
	var req ipc.HookPostReceiveRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.String(http.StatusBadRequest, "invalid request")
	}

	gist, err := db.GetGistByID(req.GistID)
	if err != nil {
		return ctx.String(http.StatusInternalServerError, "failed to get gist")
	}

	repoDir := git.RepositoryPath(gist.User.Username, gist.Uuid)

	output, err := hooks.RunPostReceive(gist, repoDir, req.GistURL, req.References, req.PushOptions)
	if err != nil {
		return ctx.String(http.StatusInternalServerError, err.Error())
	}

	return ctx.JSON(http.StatusOK, ipc.HookPostReceiveResponse{Output: output})
}
