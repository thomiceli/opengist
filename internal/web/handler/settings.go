package handler

import (
	"crypto/md5"
	"fmt"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/i18n"
	"github.com/thomiceli/opengist/internal/utils"
	"github.com/thomiceli/opengist/internal/web/context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/thomiceli/opengist/internal/db"
	"golang.org/x/crypto/ssh"
)

func UserSettings(ctx *context.OGContext) error {
	user := ctx.User

	keys, err := db.GetSSHKeysByUserID(user.ID)
	if err != nil {
		return ctx.ErrorRes(500, "Cannot get SSH keys", err)
	}

	passkeys, err := db.GetAllCredentialsForUser(user.ID)
	if err != nil {
		return ctx.ErrorRes(500, "Cannot get WebAuthn credentials", err)
	}

	_, hasTotp, err := user.HasMFA()
	if err != nil {
		return ctx.ErrorRes(500, "Cannot get MFA status", err)
	}

	ctx.SetData("email", user.Email)
	ctx.SetData("sshKeys", keys)
	ctx.SetData("passkeys", passkeys)
	ctx.SetData("hasTotp", hasTotp)
	ctx.SetData("hasPassword", user.Password != "")
	ctx.SetData("disableForm", ctx.GetData("DisableLoginForm"))
	ctx.SetData("htmlTitle", ctx.TrH("settings"))
	return ctx.HTML_("settings.html")
}

func EmailProcess(ctx *context.OGContext) error {
	user := ctx.User
	email := ctx.FormValue("email")
	var hash string

	if email == "" {
		// generate random md5 string
		hash = fmt.Sprintf("%x", md5.Sum([]byte(time.Now().String())))
	} else {
		hash = fmt.Sprintf("%x", md5.Sum([]byte(strings.ToLower(strings.TrimSpace(email)))))
	}

	user.Email = strings.ToLower(email)
	user.MD5Hash = hash

	if err := user.Update(); err != nil {
		return ctx.ErrorRes(500, "Cannot update email", err)
	}

	ctx.AddFlash(ctx.Tr("flash.user.email-updated"), "success")
	return ctx.RedirectTo("/settings")
}

func AccountDeleteProcess(ctx *context.OGContext) error {
	user := ctx.User

	if err := user.Delete(); err != nil {
		return ctx.ErrorRes(500, "Cannot delete this user", err)
	}

	return ctx.RedirectTo("/all")
}

func SshKeysProcess(ctx *context.OGContext) error {
	user := ctx.User

	dto := new(db.SSHKeyDTO)
	if err := ctx.Bind(dto); err != nil {
		return ctx.ErrorRes(400, ctx.Tr("error.cannot-bind-data"), err)
	}

	if err := ctx.Validate(dto); err != nil {
		ctx.AddFlash(utils.ValidationMessages(&err, ctx.GetData("locale").(*i18n.Locale)), "error")
		return ctx.RedirectTo("/settings")
	}
	key := dto.ToSSHKey()

	key.UserID = user.ID

	pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(key.Content))
	if err != nil {
		ctx.AddFlash(ctx.Tr("flash.user.invalid-ssh-key"), "error")
		return ctx.RedirectTo("/settings")
	}
	key.Content = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pubKey)))

	if exists, err := db.SSHKeyDoesExists(key.Content); exists {
		if err != nil {
			return ctx.ErrorRes(500, "Cannot check if SSH key exists", err)
		}
		ctx.AddFlash(ctx.Tr("settings.ssh-key-exists"), "error")
		return ctx.RedirectTo("/settings")
	}

	if err := key.Create(); err != nil {
		return ctx.ErrorRes(500, "Cannot add SSH key", err)
	}

	ctx.AddFlash(ctx.Tr("flash.user.ssh-key-added"), "success")
	return ctx.RedirectTo("/settings")
}

func SshKeysDelete(ctx *context.OGContext) error {
	user := ctx.User
	keyId, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.RedirectTo("/settings")
	}

	key, err := db.GetSSHKeyByID(uint(keyId))

	if err != nil || key.UserID != user.ID {
		return ctx.RedirectTo("/settings")
	}

	if err := key.Delete(); err != nil {
		return ctx.ErrorRes(500, "Cannot delete SSH key", err)
	}

	ctx.AddFlash(ctx.Tr("flash.user.ssh-key-deleted"), "success")
	return ctx.RedirectTo("/settings")
}

func PasskeyDelete(ctx *context.OGContext) error {
	user := ctx.User
	keyId, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.RedirectTo("/settings")
	}

	passkey, err := db.GetCredentialByIDDB(uint(keyId))
	if err != nil || passkey.UserID != user.ID {
		return ctx.RedirectTo("/settings")
	}

	if err := passkey.Delete(); err != nil {
		return ctx.ErrorRes(500, "Cannot delete passkey", err)
	}

	ctx.AddFlash(ctx.Tr("flash.auth.passkey-deleted"), "success")
	return ctx.RedirectTo("/settings")
}

func PasswordProcess(ctx *context.OGContext) error {
	user := ctx.User

	dto := new(db.UserDTO)
	if err := ctx.Bind(dto); err != nil {
		return ctx.ErrorRes(400, ctx.Tr("error.cannot-bind-data"), err)
	}
	dto.Username = user.Username

	if err := ctx.Validate(dto); err != nil {
		ctx.AddFlash(utils.ValidationMessages(&err, ctx.GetData("locale").(*i18n.Locale)), "error")
		return ctx.HTML_("settings.html")
	}

	password, err := utils.Argon2id.Hash(dto.Password)
	if err != nil {
		return ctx.ErrorRes(500, "Cannot hash password", err)
	}
	user.Password = password

	if err = user.Update(); err != nil {
		return ctx.ErrorRes(500, "Cannot update password", err)
	}

	ctx.AddFlash(ctx.Tr("flash.user.password-updated"), "success")
	return ctx.RedirectTo("/settings")
}

func UsernameProcess(ctx *context.OGContext) error {
	user := ctx.User

	dto := new(db.UserDTO)
	if err := ctx.Bind(dto); err != nil {
		return ctx.ErrorRes(400, ctx.Tr("error.cannot-bind-data"), err)
	}
	dto.Password = user.Password

	if err := ctx.Validate(dto); err != nil {
		ctx.AddFlash(utils.ValidationMessages(&err, ctx.GetData("locale").(*i18n.Locale)), "error")
		return ctx.RedirectTo("/settings")
	}

	if exists, err := db.UserExists(dto.Username); err != nil || exists {
		ctx.AddFlash(ctx.Tr("flash.auth.username-exists"), "error")
		return ctx.RedirectTo("/settings")
	}

	sourceDir := filepath.Join(config.GetHomeDir(), git.ReposDirectory, strings.ToLower(user.Username))
	destinationDir := filepath.Join(config.GetHomeDir(), git.ReposDirectory, strings.ToLower(dto.Username))

	if _, err := os.Stat(sourceDir); !os.IsNotExist(err) {
		err := os.Rename(sourceDir, destinationDir)
		if err != nil {
			return ctx.ErrorRes(500, "Cannot rename user directory", err)
		}
	}

	user.Username = dto.Username

	if err := user.Update(); err != nil {
		return ctx.ErrorRes(500, "Cannot update username", err)
	}

	ctx.AddFlash(ctx.Tr("flash.user.username-updated"), "success")
	return ctx.RedirectTo("/settings")
}
