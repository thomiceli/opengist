package web

import (
	"crypto/md5"
	"fmt"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/i18n"
	"github.com/thomiceli/opengist/internal/utils"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/thomiceli/opengist/internal/db"
	"golang.org/x/crypto/ssh"
)

func userSettings(ctx echo.Context) error {
	user := getUserLogged(ctx)

	keys, err := db.GetSSHKeysByUserID(user.ID)
	if err != nil {
		return errorRes(500, "Cannot get SSH keys", err)
	}

	passkeys, err := db.GetAllCredentialsForUser(user.ID)
	if err != nil {
		return errorRes(500, "Cannot get WebAuthn credentials", err)
	}

	setData(ctx, "email", user.Email)
	setData(ctx, "sshKeys", keys)
	setData(ctx, "passkeys", passkeys)
	setData(ctx, "hasPassword", user.Password != "")
	setData(ctx, "disableForm", getData(ctx, "DisableLoginForm"))
	setData(ctx, "htmlTitle", trH(ctx, "settings"))
	return html(ctx, "settings.html")
}

func emailProcess(ctx echo.Context) error {
	user := getUserLogged(ctx)
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
		return errorRes(500, "Cannot update email", err)
	}

	addFlash(ctx, tr(ctx, "flash.user.email-updated"), "success")
	return redirect(ctx, "/settings")
}

func accountDeleteProcess(ctx echo.Context) error {
	user := getUserLogged(ctx)

	if err := user.Delete(); err != nil {
		return errorRes(500, "Cannot delete this user", err)
	}

	return redirect(ctx, "/all")
}

func sshKeysProcess(ctx echo.Context) error {
	user := getUserLogged(ctx)

	dto := new(db.SSHKeyDTO)
	if err := ctx.Bind(dto); err != nil {
		return errorRes(400, tr(ctx, "error.cannot-bind-data"), err)
	}

	if err := ctx.Validate(dto); err != nil {
		addFlash(ctx, utils.ValidationMessages(&err, getData(ctx, "locale").(*i18n.Locale)), "error")
		return redirect(ctx, "/settings")
	}
	key := dto.ToSSHKey()

	key.UserID = user.ID

	pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(key.Content))
	if err != nil {
		addFlash(ctx, tr(ctx, "flash.user.invalid-ssh-key"), "error")
		return redirect(ctx, "/settings")
	}
	key.Content = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pubKey)))

	if exists, err := db.SSHKeyDoesExists(key.Content); exists {
		if err != nil {
			return errorRes(500, "Cannot check if SSH key exists", err)
		}
		addFlash(ctx, tr(ctx, "settings.ssh-key-exists"), "error")
		return redirect(ctx, "/settings")
	}

	if err := key.Create(); err != nil {
		return errorRes(500, "Cannot add SSH key", err)
	}

	addFlash(ctx, tr(ctx, "flash.user.ssh-key-added"), "success")
	return redirect(ctx, "/settings")
}

func sshKeysDelete(ctx echo.Context) error {
	user := getUserLogged(ctx)
	keyId, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return redirect(ctx, "/settings")
	}

	key, err := db.GetSSHKeyByID(uint(keyId))

	if err != nil || key.UserID != user.ID {
		return redirect(ctx, "/settings")
	}

	if err := key.Delete(); err != nil {
		return errorRes(500, "Cannot delete SSH key", err)
	}

	addFlash(ctx, tr(ctx, "flash.user.ssh-key-deleted"), "success")
	return redirect(ctx, "/settings")
}

func passkeyDelete(ctx echo.Context) error {
	user := getUserLogged(ctx)
	keyId, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return redirect(ctx, "/settings")
	}

	passkey, err := db.GetCredentialByIDDB(uint(keyId))
	if err != nil || passkey.UserID != user.ID {
		return redirect(ctx, "/settings")
	}

	if err := passkey.Delete(); err != nil {
		return errorRes(500, "Cannot delete passkey", err)
	}

	addFlash(ctx, tr(ctx, "flash.auth.passkey-deleted"), "success")
	return redirect(ctx, "/settings")
}

func passwordProcess(ctx echo.Context) error {
	user := getUserLogged(ctx)

	dto := new(db.UserDTO)
	if err := ctx.Bind(dto); err != nil {
		return errorRes(400, tr(ctx, "error.cannot-bind-data"), err)
	}
	dto.Username = user.Username

	if err := ctx.Validate(dto); err != nil {
		addFlash(ctx, utils.ValidationMessages(&err, getData(ctx, "locale").(*i18n.Locale)), "error")
		return html(ctx, "settings.html")
	}

	password, err := utils.Argon2id.Hash(dto.Password)
	if err != nil {
		return errorRes(500, "Cannot hash password", err)
	}
	user.Password = password

	if err = user.Update(); err != nil {
		return errorRes(500, "Cannot update password", err)
	}

	addFlash(ctx, tr(ctx, "flash.user.password-updated"), "success")
	return redirect(ctx, "/settings")
}

func usernameProcess(ctx echo.Context) error {
	user := getUserLogged(ctx)

	dto := new(db.UserDTO)
	if err := ctx.Bind(dto); err != nil {
		return errorRes(400, tr(ctx, "error.cannot-bind-data"), err)
	}
	dto.Password = user.Password

	if err := ctx.Validate(dto); err != nil {
		addFlash(ctx, utils.ValidationMessages(&err, getData(ctx, "locale").(*i18n.Locale)), "error")
		return redirect(ctx, "/settings")
	}

	if exists, err := db.UserExists(dto.Username); err != nil || exists {
		addFlash(ctx, tr(ctx, "flash.auth.username-exists"), "error")
		return redirect(ctx, "/settings")
	}

	sourceDir := filepath.Join(config.GetHomeDir(), git.ReposDirectory, strings.ToLower(user.Username))
	destinationDir := filepath.Join(config.GetHomeDir(), git.ReposDirectory, strings.ToLower(dto.Username))

	if _, err := os.Stat(sourceDir); !os.IsNotExist(err) {
		err := os.Rename(sourceDir, destinationDir)
		if err != nil {
			return errorRes(500, "Cannot rename user directory", err)
		}
	}

	user.Username = dto.Username

	if err := user.Update(); err != nil {
		return errorRes(500, "Cannot update username", err)
	}

	addFlash(ctx, tr(ctx, "flash.user.username-updated"), "success")
	return redirect(ctx, "/settings")
}
