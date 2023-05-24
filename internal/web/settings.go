package web

import (
	"crypto/md5"
	"fmt"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/ssh"
	"opengist/internal/models"
	"strconv"
	"strings"
	"time"
)

func userSettings(ctx echo.Context) error {
	user := getUserLogged(ctx)

	keys, err := models.GetSSHKeysByUserID(user.ID)
	if err != nil {
		return errorRes(500, "Cannot get SSH keys", err)
	}

	setData(ctx, "email", user.Email)
	setData(ctx, "sshKeys", keys)
	setData(ctx, "htmlTitle", "Settings")
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

	addFlash(ctx, "Email updated", "success")
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

	var dto = new(models.SSHKeyDTO)
	if err := ctx.Bind(dto); err != nil {
		return errorRes(400, "Cannot bind data", err)
	}

	if err := ctx.Validate(dto); err != nil {
		addFlash(ctx, validationMessages(&err), "error")
		return redirect(ctx, "/settings")
	}
	key := dto.ToSSHKey()

	key.UserID = user.ID

	pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(key.Content))
	if err != nil {
		addFlash(ctx, "Invalid SSH key", "error")
		return redirect(ctx, "/settings")
	}
	key.Content = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pubKey)))

	if err := key.Create(); err != nil {
		return errorRes(500, "Cannot add SSH key", err)
	}

	addFlash(ctx, "SSH key added", "success")
	return redirect(ctx, "/settings")
}

func sshKeysDelete(ctx echo.Context) error {
	user := getUserLogged(ctx)
	keyId, err := strconv.Atoi(ctx.Param("id"))

	if err != nil {
		return redirect(ctx, "/settings")
	}

	key, err := models.GetSSHKeyByID(uint(keyId))

	if err != nil || key.UserID != user.ID {
		return redirect(ctx, "/settings")
	}

	if err := key.Delete(); err != nil {
		return errorRes(500, "Cannot delete SSH key", err)
	}

	addFlash(ctx, "SSH key deleted", "success")
	return redirect(ctx, "/settings")
}
