package auth

import (
	"github.com/thomiceli/opengist/internal/auth/totp"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
	"net/url"
)

func BeginTotp(ctx *context.Context) error {
	user := ctx.User

	if _, hasTotp, err := user.HasMFA(); err != nil {
		return ctx.ErrorRes(500, "Cannot check for user MFA", err)
	} else if hasTotp {
		ctx.AddFlash(ctx.Tr("auth.totp.already-enabled"), "error")
		return ctx.RedirectTo("/settings/mfa")
	}

	ogUrl, err := url.Parse(ctx.GetData("baseHttpUrl").(string))
	if err != nil {
		return ctx.ErrorRes(500, "Cannot parse base URL", err)
	}

	sess := ctx.GetSession()
	generatedSecret, _ := sess.Values["generatedSecret"].([]byte)

	totpSecret, qrcode, err, generatedSecret := totp.GenerateQRCode(ctx.User.Username, ogUrl.Hostname(), generatedSecret)
	if err != nil {
		return ctx.ErrorRes(500, "Cannot generate TOTP QR code", err)
	}
	sess.Values["totpSecret"] = totpSecret
	sess.Values["generatedSecret"] = generatedSecret
	ctx.SaveSession(sess)

	ctx.SetData("totpSecret", totpSecret)
	ctx.SetData("totpQrcode", qrcode)

	return ctx.Html("totp.html")

}

func FinishTotp(ctx *context.Context) error {
	user := ctx.User

	if _, hasTotp, err := user.HasMFA(); err != nil {
		return ctx.ErrorRes(500, "Cannot check for user MFA", err)
	} else if hasTotp {
		ctx.AddFlash(ctx.Tr("auth.totp.already-enabled"), "error")
		return ctx.RedirectTo("/settings/mfa")
	}

	dto := &db.TOTPDTO{}
	if err := ctx.Bind(dto); err != nil {
		return ctx.ErrorRes(400, ctx.Tr("error.cannot-bind-data"), err)
	}

	if err := ctx.Validate(dto); err != nil {
		ctx.AddFlash("Invalid secret", "error")
		return ctx.RedirectTo("/settings/totp/generate")
	}

	sess := ctx.GetSession()
	secret, ok := sess.Values["totpSecret"].(string)
	if !ok {
		return ctx.ErrorRes(500, "Cannot get TOTP secret from session", nil)
	}

	if !totp.Validate(dto.Code, secret) {
		ctx.AddFlash(ctx.Tr("auth.totp.invalid-code"), "error")

		return ctx.RedirectTo("/settings/totp/generate")
	}

	userTotp := &db.TOTP{
		UserID: ctx.User.ID,
	}
	if err := userTotp.StoreSecret(secret); err != nil {
		return ctx.ErrorRes(500, "Cannot store TOTP secret", err)
	}

	if err := userTotp.Create(); err != nil {
		return ctx.ErrorRes(500, "Cannot create TOTP", err)
	}

	ctx.AddFlash("TOTP successfully enabled", "success")
	codes, err := userTotp.GenerateRecoveryCodes()
	if err != nil {
		return ctx.ErrorRes(500, "Cannot generate recovery codes", err)
	}

	delete(sess.Values, "totpSecret")
	delete(sess.Values, "generatedSecret")
	ctx.SaveSession(sess)

	ctx.SetData("recoveryCodes", codes)
	return ctx.Html("totp.html")
}

func AssertTotp(ctx *context.Context) error {
	var err error
	dto := &db.TOTPDTO{}
	if err := ctx.Bind(dto); err != nil {
		return ctx.ErrorRes(400, ctx.Tr("error.cannot-bind-data"), err)
	}

	if err := ctx.Validate(dto); err != nil {
		ctx.AddFlash(ctx.Tr("auth.totp.invalid-code"), "error")
		return ctx.RedirectTo("/mfa")
	}

	sess := ctx.GetSession()
	userId := sess.Values["mfaID"].(uint)
	var userTotp *db.TOTP
	if userTotp, err = db.GetTOTPByUserID(userId); err != nil {
		return ctx.ErrorRes(500, "Cannot get TOTP by UID", err)
	}

	redirectUrl := "/"

	var validCode, validRecoveryCode bool
	if validCode, err = userTotp.ValidateCode(dto.Code); err != nil {
		return ctx.ErrorRes(500, "Cannot validate TOTP code", err)
	}
	if !validCode {
		validRecoveryCode, err = userTotp.ValidateRecoveryCode(dto.Code)
		if err != nil {
			return ctx.ErrorRes(500, "Cannot validate TOTP code", err)
		}

		if !validRecoveryCode {
			ctx.AddFlash(ctx.Tr("auth.totp.invalid-code"), "error")
			return ctx.RedirectTo("/mfa")
		}

		ctx.AddFlash(ctx.Tr("auth.totp.code-used", dto.Code), "warning")
		redirectUrl = "/settings/mfa"
	}

	sess.Values["user"] = userId
	sess.Options.MaxAge = 60 * 60 * 24 * 365 // 1 year
	delete(sess.Values, "mfaID")
	ctx.SaveSession(sess)

	return ctx.RedirectTo(redirectUrl)
}

func DisableTotp(ctx *context.Context) error {
	user := ctx.User
	userTotp, err := db.GetTOTPByUserID(user.ID)
	if err != nil {
		return ctx.ErrorRes(500, "Cannot get TOTP by UID", err)
	}

	if err = userTotp.Delete(); err != nil {
		return ctx.ErrorRes(500, "Cannot delete TOTP", err)
	}

	ctx.AddFlash(ctx.Tr("auth.totp.disabled"), "success")
	return ctx.RedirectTo("/settings/mfa")
}

func RegenerateTotpRecoveryCodes(ctx *context.Context) error {
	user := ctx.User
	userTotp, err := db.GetTOTPByUserID(user.ID)
	if err != nil {
		return ctx.ErrorRes(500, "Cannot get TOTP by UID", err)
	}

	codes, err := userTotp.GenerateRecoveryCodes()
	if err != nil {
		return ctx.ErrorRes(500, "Cannot generate recovery codes", err)
	}

	ctx.SetData("recoveryCodes", codes)
	return ctx.Html("totp.html")
}
