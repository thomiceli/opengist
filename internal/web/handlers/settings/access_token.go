package settings

import (
	"strconv"

	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/i18n"
	"github.com/thomiceli/opengist/internal/validator"
	"github.com/thomiceli/opengist/internal/web/context"
)

func AccessTokens(ctx *context.Context) error {
	user := ctx.User

	tokens, err := db.GetAccessTokensByUserID(user.ID)
	if err != nil {
		return ctx.ErrorRes(500, "Cannot get access tokens", err)
	}

	ctx.SetData("accessTokens", tokens)
	ctx.SetData("settingsHeaderPage", "tokens")
	ctx.SetData("htmlTitle", ctx.TrH("settings"))
	return ctx.Html("settings_tokens.html")
}

func AccessTokensProcess(ctx *context.Context) error {
	user := ctx.User

	dto := new(db.AccessTokenDTO)
	if err := ctx.Bind(dto); err != nil {
		return ctx.ErrorRes(400, ctx.Tr("error.cannot-bind-data"), err)
	}

	if err := ctx.Validate(dto); err != nil {
		ctx.AddFlash(validator.ValidationMessages(&err, ctx.GetData("locale").(*i18n.Locale)), "error")
		return ctx.RedirectTo("/settings/access-tokens")
	}

	token := dto.ToAccessToken()
	token.UserID = user.ID

	plainToken, err := token.GenerateToken()
	if err != nil {
		return ctx.ErrorRes(500, "Cannot generate token", err)
	}

	if err := token.Create(); err != nil {
		return ctx.ErrorRes(500, "Cannot create access token", err)
	}

	// Show the token once to the user
	ctx.AddFlash(ctx.Tr("settings.token-created"), "success")
	ctx.AddFlash(plainToken, "success")
	return ctx.RedirectTo("/settings/access-tokens")
}

func AccessTokensDelete(ctx *context.Context) error {
	user := ctx.User
	tokenID, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.RedirectTo("/settings/access-tokens")
	}

	token, err := db.GetAccessTokenByID(uint(tokenID))
	if err != nil || token.UserID != user.ID {
		return ctx.RedirectTo("/settings/access-tokens")
	}

	if err := token.Delete(); err != nil {
		return ctx.ErrorRes(500, "Cannot delete access token", err)
	}

	ctx.AddFlash(ctx.Tr("settings.token-deleted"), "success")
	return ctx.RedirectTo("/settings/access-tokens")
}
