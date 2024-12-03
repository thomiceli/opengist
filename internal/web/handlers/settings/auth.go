package settings

import (
	passwordpkg "github.com/thomiceli/opengist/internal/auth/password"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/i18n"
	"github.com/thomiceli/opengist/internal/validator"
	"github.com/thomiceli/opengist/internal/web/context"
	"strconv"
)

func PasskeyDelete(ctx *context.Context) error {
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

func PasswordProcess(ctx *context.Context) error {
	user := ctx.User

	dto := new(db.UserDTO)
	if err := ctx.Bind(dto); err != nil {
		return ctx.ErrorRes(400, ctx.Tr("error.cannot-bind-data"), err)
	}
	dto.Username = user.Username

	if err := ctx.Validate(dto); err != nil {
		ctx.AddFlash(validator.ValidationMessages(&err, ctx.GetData("locale").(*i18n.Locale)), "error")
		return ctx.Html("settings.html")
	}

	password, err := passwordpkg.HashPassword(dto.Password)
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
