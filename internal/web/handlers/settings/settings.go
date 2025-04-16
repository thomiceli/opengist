package settings

import (
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
)

func UserAccount(ctx *context.Context) error {
	user := ctx.User

	ctx.SetData("email", user.Email)
	ctx.SetData("hasPassword", user.Password != "")
	ctx.SetData("disableForm", ctx.GetData("DisableLoginForm"))
	ctx.SetData("settingsHeaderPage", "account")
	ctx.SetData("htmlTitle", ctx.TrH("settings"))
	return ctx.Html("settings_account.html")
}

func UserMFA(ctx *context.Context) error {
	user := ctx.User

	passkeys, err := db.GetAllCredentialsForUser(user.ID)
	if err != nil {
		return ctx.ErrorRes(500, "Cannot get WebAuthn credentials", err)
	}

	_, hasTotp, err := user.HasMFA()
	if err != nil {
		return ctx.ErrorRes(500, "Cannot get MFA status", err)
	}

	ctx.SetData("passkeys", passkeys)
	ctx.SetData("hasTotp", hasTotp)
	ctx.SetData("settingsHeaderPage", "mfa")
	ctx.SetData("htmlTitle", ctx.TrH("settings"))
	return ctx.Html("settings_mfa.html")
}

func UserSSHKeys(ctx *context.Context) error {
	user := ctx.User

	keys, err := db.GetSSHKeysByUserID(user.ID)
	if err != nil {
		return ctx.ErrorRes(500, "Cannot get SSH keys", err)
	}

	ctx.SetData("sshKeys", keys)
	ctx.SetData("settingsHeaderPage", "ssh")
	ctx.SetData("htmlTitle", ctx.TrH("settings"))
	return ctx.Html("settings_ssh.html")
}

func UserStyle(ctx *context.Context) error {
	ctx.SetData("settingsHeaderPage", "style")
	ctx.SetData("htmlTitle", ctx.TrH("settings"))
	return ctx.Html("settings_style.html")
}

func ProcessUserStyle(ctx *context.Context) error {
	styleDto := new(db.UserStyleDTO)
	if err := ctx.Bind(styleDto); err != nil {
		return ctx.ErrorRes(400, ctx.Tr("error.cannot-bind-data"), err)
	}

	if err := ctx.Validate(styleDto); err != nil {
		return ctx.ErrorRes(400, "Invalid data", err)
	}
	user := ctx.User
	user.StylePreferences = styleDto.ToJson()
	if err := user.Update(); err != nil {
		return ctx.ErrorRes(500, "Cannot update user styles", err)
	}

	ctx.AddFlash("Updated style", "success")
	return ctx.RedirectTo("/settings/style")
}
