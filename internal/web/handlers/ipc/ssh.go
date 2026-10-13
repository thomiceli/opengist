package ipc

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/ipc"
	"github.com/thomiceli/opengist/internal/ssh"
	"github.com/thomiceli/opengist/internal/web/context"
	"gorm.io/gorm"
)

// SSHKeys resolves a public key offered to sshd (forwarded by the `keys`
// command) to its stored SSH key id, so the AuthorizedKeysCommand can emit a
// forced command that identifies the connecting key.
func SSHKeys(ctx *context.Context) error {
	var req ipc.SSHKeyLookupRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.String(http.StatusBadRequest, "invalid request")
	}

	key, err := db.GetSSHKeyByContent(req.Key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusOK, ipc.SSHKeyLookupResponse{Found: false})
		}
		return ctx.String(http.StatusInternalServerError, "lookup failed")
	}

	return ctx.JSON(http.StatusOK, ipc.SSHKeyLookupResponse{Found: true, KeyID: key.ID})
}

// SSHCommand authorizes a git command for a connecting SSH key (forwarded by the
// `shell` forced command) and tells it which git pack command to run, and where.
// The git data plane stays in the shim; only this control plane runs here.
func SSHCommand(ctx *context.Context) error {
	var req ipc.SSHCommandRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.String(http.StatusBadRequest, "invalid request")
	}

	sshKey, err := db.GetSSHKeyByID(req.KeyID)
	if err != nil {
		return ctx.JSON(http.StatusOK, ipc.SSHCommandResponse{Authorized: false, Message: "key not recognized"})
	}

	gist, verb, err := ssh.AuthorizeGitCommand(req.Command, sshKey.Content, req.IP)
	if err != nil {
		return ctx.JSON(http.StatusOK, ipc.SSHCommandResponse{Authorized: false, Message: err.Error()})
	}

	return ctx.JSON(http.StatusOK, ipc.SSHCommandResponse{
		Authorized: true,
		Verb:       verb,
		RepoPath:   git.RepositoryPath(gist.User.Username, gist.Uuid),
		GistID:     strconv.FormatUint(uint64(gist.ID), 10),
	})
}
