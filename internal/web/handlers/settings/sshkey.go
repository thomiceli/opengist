package settings

import (
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/i18n"
	"github.com/thomiceli/opengist/internal/validator"
	"github.com/thomiceli/opengist/internal/web/context"
	"golang.org/x/crypto/ssh"
	"strconv"
	"strings"
)

func SshKeysProcess(ctx *context.Context) error {
	user := ctx.User

	dto := new(db.SSHKeyDTO)
	if err := ctx.Bind(dto); err != nil {
		return ctx.ErrorRes(400, ctx.Tr("error.cannot-bind-data"), err)
	}

	if err := ctx.Validate(dto); err != nil {
		ctx.AddFlash(validator.ValidationMessages(&err, ctx.GetData("locale").(*i18n.Locale)), "error")
		return ctx.RedirectTo("/settings/ssh")
	}
	key := dto.ToSSHKey()

	key.UserID = user.ID

	pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(key.Content))
	if err != nil {
		ctx.AddFlash(ctx.Tr("flash.user.invalid-ssh-key"), "error")
		return ctx.RedirectTo("/settings/ssh")
	}
	key.Content = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pubKey)))

	if exists, err := db.SSHKeyDoesExists(key.Content); exists {
		if err != nil {
			return ctx.ErrorRes(500, "Cannot check if SSH key exists", err)
		}
		ctx.AddFlash(ctx.Tr("settings.ssh-key-exists"), "error")
		return ctx.RedirectTo("/settings/ssh")
	}

	if err := key.Create(); err != nil {
		return ctx.ErrorRes(500, "Cannot add SSH key", err)
	}

	ctx.AddFlash(ctx.Tr("flash.user.ssh-key-added"), "success")
	return ctx.RedirectTo("/settings/ssh")
}

func SshKeysDelete(ctx *context.Context) error {
	user := ctx.User
	keyId, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.RedirectTo("/settings/ssh")
	}

	key, err := db.GetSSHKeyByID(uint(keyId))

	if err != nil || key.UserID != user.ID {
		return ctx.RedirectTo("/settings/ssh")
	}

	if err := key.Delete(); err != nil {
		return ctx.ErrorRes(500, "Cannot delete SSH key", err)
	}

	ctx.AddFlash(ctx.Tr("flash.user.ssh-key-deleted"), "success")
	return ctx.RedirectTo("/settings/ssh")
}
