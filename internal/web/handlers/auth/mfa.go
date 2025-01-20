package auth

import (
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
)

func Mfa(ctx *context.Context) error {
	var err error

	user := db.User{ID: ctx.GetSession().Values["mfaID"].(uint)}

	var hasWebauthn, hasTotp bool
	if hasWebauthn, hasTotp, err = user.HasMFA(); err != nil {
		return ctx.ErrorRes(500, "Cannot check for user MFA", err)
	}

	ctx.SetData("hasWebauthn", hasWebauthn)
	ctx.SetData("hasTotp", hasTotp)

	return ctx.Html("mfa.html")
}
