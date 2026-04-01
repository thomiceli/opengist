package auth

import (
	"crypto/md5"
	"errors"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/auth/oauth"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/i18n"
	"github.com/thomiceli/opengist/internal/validator"
	"github.com/thomiceli/opengist/internal/web/context"
	"gorm.io/gorm"
)

func Oauth(ctx *context.Context) error {
	providerStr := ctx.Param("provider")

	httpProtocol := "http"
	if ctx.Request().TLS != nil || ctx.Request().Header.Get("X-Forwarded-Proto") == "https" {
		httpProtocol = "https"
	}

	forwarded_hdr := ctx.Request().Header.Get("Forwarded")
	if forwarded_hdr != "" {
		fields := strings.Split(forwarded_hdr, ";")
		fwd := make(map[string]string)
		for _, v := range fields {
			p := strings.Split(v, "=")
			fwd[p[0]] = p[1]
		}
		val, ok := fwd["proto"]
		if ok && val == "https" {
			httpProtocol = "https"
		}
	}

	var opengistUrl string
	if config.C.ExternalUrl != "" {
		opengistUrl = config.C.ExternalUrl
	} else {
		opengistUrl = httpProtocol + "://" + ctx.Request().Host
	}

	provider, err := oauth.DefineProvider(providerStr, opengistUrl)
	if err != nil {
		ctx.AddFlash(ctx.Tr("error.oauth-unsupported"), "error")
		return ctx.Redirect(302, "/login")
	}

	if err = provider.RegisterProvider(); err != nil {
		return ctx.ErrorRes(500, "Cannot create provider", err)
	}

	provider.BeginAuthHandler(ctx)
	return nil
}

func OauthCallback(ctx *context.Context) error {
	provider, err := oauth.CompleteUserAuth(ctx)
	if err != nil {
		ctx.AddFlash(ctx.Tr("auth.oauth.no-provider"), "error")
		return ctx.Redirect(302, "/login")
	}

	currUser := ctx.User
	user := provider.GetProviderUser()

	// if user is logged in, link account to user and update its avatar URL
	if currUser != nil {
		// check if this OAuth account is already linked to another user
		if existingUser, err := db.GetUserByProvider(user.UserID, provider.GetProvider()); err == nil && existingUser != nil {
			ctx.AddFlash(ctx.Tr("flash.auth.oauth-already-linked", config.C.OIDCProviderName), "error")
			return ctx.RedirectTo("/settings")
		}

		provider.UpdateUserDB(currUser)

		if err = currUser.Update(); err != nil {
			return ctx.ErrorRes(500, "Cannot update user "+config.C.OIDCProviderName+" id", err)
		}

		ctx.AddFlash(ctx.Tr("flash.auth.account-linked-oauth", config.C.OIDCProviderName), "success")
		return ctx.RedirectTo("/settings")
	}

	userDB, err := db.GetUserByProvider(user.UserID, provider.GetProvider())
	// if user is not in database, redirect to OAuth registration page
	if err != nil {
		if ctx.GetData("DisableSignup") == true {
			ctx.AddFlash(ctx.Tr("error.signup-disabled"), "error")
			return ctx.Redirect(302, "/login")
		}

		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.ErrorRes(500, "Cannot get user", err)
		}

		if user.NickName == "" {
			user.NickName = strings.Split(user.Email, "@")[0]
		}

		sess := ctx.GetSession()
		sess.Values["oauthProvider"] = provider.GetProvider()
		sess.Values["oauthUserID"] = user.UserID
		sess.Values["oauthNickname"] = user.NickName
		sess.Values["oauthEmail"] = user.Email
		sess.Values["oauthAvatarURL"] = user.AvatarURL
		sess.Values["oauthIsAdmin"] = provider.IsAdmin()

		sess.Options.MaxAge = 10 * 60 // 10 minutes
		ctx.SaveSession(sess)

		return ctx.RedirectTo("/oauth/register")
	}

	// promote user to admin from oidc group
	if !userDB.IsAdmin && provider.IsAdmin() {
		userDB.IsAdmin = true
		if err = userDB.Update(); err != nil {
			return ctx.ErrorRes(500, "Cannot set user admin", err)
		}
	}

	sess := ctx.GetSession()
	sess.Values["user"] = userDB.ID
	ctx.SaveSession(sess)
	ctx.DeleteCsrfCookie()

	return ctx.RedirectTo("/")
}

func OauthRegister(ctx *context.Context) error {
	if ctx.GetData("DisableSignup") == true {
		ctx.AddFlash(ctx.Tr("error.signup-disabled"), "error")
		return ctx.Redirect(302, "/login")
	}

	sess := ctx.GetSession()

	ctx.SetData("title", ctx.TrH("auth.oauth.complete-registration"))
	ctx.SetData("htmlTitle", ctx.TrH("auth.oauth.complete-registration"))
	ctx.SetData("oauthProvider", config.C.OIDCProviderName)
	ctx.SetData("oauthNickname", sess.Values["oauthNickname"])
	ctx.SetData("oauthEmail", sess.Values["oauthEmail"])
	ctx.SetData("oauthAvatarURL", sess.Values["oauthAvatarURL"])

	return ctx.Html("oauth_register.html")
}

func ProcessOauthRegister(ctx *context.Context) error {
	if ctx.GetData("DisableSignup") == true {
		ctx.AddFlash(ctx.Tr("error.signup-disabled"), "error")
		return ctx.Redirect(302, "/login")
	}

	sess := ctx.GetSession()

	providerStr := sess.Values["oauthProvider"].(string)
	oauthUserID := sess.Values["oauthUserID"].(string)

	setOauthRegisterData := func(dto *db.OAuthRegisterDTO) {
		ctx.SetData("title", ctx.TrH("auth.oauth.complete-registration"))
		ctx.SetData("htmlTitle", ctx.TrH("auth.oauth.complete-registration"))
		ctx.SetData("oauthProvider", config.C.OIDCProviderName)
		if dto != nil {
			ctx.SetData("oauthNickname", dto.Username)
			ctx.SetData("oauthEmail", dto.Email)
		} else {
			ctx.SetData("oauthNickname", sess.Values["oauthNickname"])
			ctx.SetData("oauthEmail", sess.Values["oauthEmail"])
		}
		ctx.SetData("oauthAvatarURL", sess.Values["oauthAvatarURL"])
	}

	// Bind and validate form data
	dto := new(db.OAuthRegisterDTO)
	if err := ctx.Bind(dto); err != nil {
		return ctx.ErrorRes(400, ctx.Tr("error.cannot-bind-data"), err)
	}

	if err := ctx.Validate(dto); err != nil {
		ctx.AddFlash(validator.ValidationMessages(&err, ctx.GetData("locale").(*i18n.Locale)), "error")
		setOauthRegisterData(dto)
		return ctx.Html("oauth_register.html")
	}

	if exists, err := db.UserExists(dto.Username); err != nil || exists {
		ctx.AddFlash(ctx.Tr("flash.auth.username-exists"), "error")
		setOauthRegisterData(dto)
		return ctx.Html("oauth_register.html")
	}

	// Check if OAuth account is already linked to another user (race condition protection)
	if existingUser, err := db.GetUserByProvider(oauthUserID, providerStr); err == nil && existingUser != nil {
		ctx.AddFlash(ctx.Tr("flash.auth.oauth-already-linked", config.C.OIDCProviderName), "error")
		setOauthRegisterData(dto)
		return ctx.Html("oauth_register.html")
	}

	userDB := &db.User{
		Username: dto.Username,
		Email:    dto.Email,
	}
	if dto.Email != "" {
		userDB.MD5Hash = fmt.Sprintf("%x", md5.Sum([]byte(strings.ToLower(strings.TrimSpace(dto.Email)))))
	}

	nickname := ""
	if n, ok := sess.Values["oauthNickname"].(string); ok {
		nickname = n
	}
	avatarURL := ""
	if av, ok := sess.Values["oauthAvatarURL"].(string); ok {
		avatarURL = av
	}

	callbackProvider, err := oauth.NewCallbackProviderFromSession(providerStr, oauthUserID, nickname, dto.Email, avatarURL)
	if err != nil {
		return ctx.ErrorRes(500, "Cannot create provider", err)
	}
	callbackProvider.UpdateUserDB(userDB)

	if err := userDB.Create(); err != nil {
		if db.IsUniqueConstraintViolation(err) {
			ctx.AddFlash(ctx.Tr("flash.auth.username-exists"), "error")
			setOauthRegisterData(dto)
			return ctx.Html("oauth_register.html")
		}
		return ctx.ErrorRes(500, "Cannot create user", err)
	}

	if config.C.OIDCAdminGroup == "" && userDB.ID == 1 {
		if err := userDB.SetAdmin(); err != nil {
			return ctx.ErrorRes(500, "Cannot set user admin", err)
		}
	}

	if isAdmin, ok := sess.Values["oauthIsAdmin"].(bool); ok && isAdmin {
		userDB.IsAdmin = true
		_ = userDB.Update()
	}

	keys, err := callbackProvider.GetProviderUserSSHKeys()
	if err != nil {
		ctx.AddFlash(ctx.Tr("flash.auth.user-sshkeys-not-retrievable"), "error")
		log.Error().Err(err).Msg("Could not get user keys")
	} else {
		for _, key := range keys {
			sshKey := db.SSHKey{
				Title:   "Added from " + providerStr,
				Content: key,
				User:    *userDB,
			}
			if err = sshKey.Create(); err != nil {
				ctx.AddFlash(ctx.Tr("flash.auth.user-sshkeys-not-created"), "error")
				log.Error().Err(err).Msg("Could not create ssh key")
			}
		}
	}

	delete(sess.Values, "oauthProvider")
	delete(sess.Values, "oauthUserID")
	delete(sess.Values, "oauthNickname")
	delete(sess.Values, "oauthEmail")
	delete(sess.Values, "oauthAvatarURL")
	delete(sess.Values, "oauthIsAdmin")

	sess.Values["user"] = userDB.ID
	sess.Options.MaxAge = 60 * 60 * 24 * 365 // 1 year
	ctx.SaveSession(sess)
	ctx.DeleteCsrfCookie()

	return ctx.RedirectTo("/")
}

func OauthUnlink(ctx *context.Context) error {
	providerStr := ctx.Param("provider")
	provider, err := oauth.DefineProvider(ctx.Param("provider"), "")
	if err != nil {
		return ctx.ErrorRes(400, ctx.Tr("error.oauth-unsupported"), nil)
	}

	currUser := ctx.User

	if provider.UserHasProvider(currUser) {
		if err := currUser.DeleteProviderID(providerStr); err != nil {
			return ctx.ErrorRes(500, "Cannot unlink account from "+config.C.OIDCProviderName, err)
		}

		ctx.AddFlash(ctx.Tr("flash.auth.account-unlinked-oauth", config.C.OIDCProviderName), "success")
		return ctx.RedirectTo("/settings")
	}

	return ctx.RedirectTo("/settings")
}
