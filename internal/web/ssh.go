package web

import (
	"crypto/sha256"
	"encoding/base64"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/ssh"
	"opengist/internal/models"
	"strconv"
)

func sshKeys(ctx echo.Context) error {
	user := getUserLogged(ctx)

	keys, err := models.GetSSHKeysByUserID(user.ID)
	if err != nil {
		return errorRes(500, "Cannot get SSH keys", err)
	}

	setData(ctx, "sshKeys", keys)
	setData(ctx, "htmlTitle", "Manage SSH keys")
	return html(ctx, "ssh_keys.html")
}

func sshKeysProcess(ctx echo.Context) error {
	setData(ctx, "htmlTitle", "Manage SSH keys")

	user := getUserLogged(ctx)

	var key = new(models.SSHKey)
	if err := ctx.Bind(key); err != nil {
		return errorRes(400, "Cannot bind data", err)
	}

	if err := ctx.Validate(key); err != nil {
		addFlash(ctx, validationMessages(&err), "error")
		return redirect(ctx, "/ssh-keys")
	}

	key.UserID = user.ID

	pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(key.Content))
	if err != nil {
		addFlash(ctx, "Invalid SSH key", "error")
		return redirect(ctx, "/ssh-keys")
	}

	sha := sha256.Sum256(pubKey.Marshal())
	key.SHA = base64.StdEncoding.EncodeToString(sha[:])

	if err := models.AddSSHKey(key); err != nil {
		return errorRes(500, "Cannot add SSH key", err)
	}

	addFlash(ctx, "SSH key added", "success")
	return redirect(ctx, "/ssh-keys")
}

func sshKeysDelete(ctx echo.Context) error {
	user := getUserLogged(ctx)
	keyId, err := strconv.Atoi(ctx.Param("id"))

	if err != nil {
		return redirect(ctx, "/ssh-keys")
	}

	key, err := models.GetSSHKeyByID(uint(keyId))

	if err != nil || key.UserID != user.ID {
		return redirect(ctx, "/ssh-keys")
	}

	if err := models.RemoveSSHKey(key); err != nil {
		return errorRes(500, "Cannot delete SSH key", err)
	}

	addFlash(ctx, "SSH key deleted", "success")
	return redirect(ctx, "/ssh-keys")
}
