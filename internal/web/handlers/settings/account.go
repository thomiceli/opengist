package settings

import (
	"crypto/md5"
	"fmt"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/i18n"
	"github.com/thomiceli/opengist/internal/validator"
	"github.com/thomiceli/opengist/internal/web/context"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func EmailProcess(ctx *context.Context) error {
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

func AccountDeleteProcess(ctx *context.Context) error {
	user := ctx.User

	if err := user.Delete(); err != nil {
		return ctx.ErrorRes(500, "Cannot delete this user", err)
	}

	return ctx.RedirectTo("/all")
}

func UsernameProcess(ctx *context.Context) error {
	user := ctx.User

	dto := new(db.UserUsernameDTO)
	if err := ctx.Bind(dto); err != nil {
		return ctx.ErrorRes(400, ctx.Tr("error.cannot-bind-data"), err)
	}

	if err := ctx.Validate(dto); err != nil {
		ctx.AddFlash(validator.ValidationMessages(&err, ctx.GetData("locale").(*i18n.Locale)), "error")
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
