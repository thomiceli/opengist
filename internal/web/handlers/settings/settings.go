package settings

import (
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
)

func UserSettings(ctx *context.Context) error {
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
	ctx.SetData("disableChangeUsernameEmail", config.C.DisableChangeUsernameEmail)
	ctx.SetData("htmlTitle", ctx.TrH("settings"))
	return ctx.Html("settings.html")
}
