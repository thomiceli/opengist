package auth

import (
	"crypto/md5"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/auth/oauth"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
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
		return ctx.ErrorRes(400, ctx.Tr("error.oauth-unsupported"), nil)
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
		return ctx.ErrorRes(400, ctx.Tr("error.complete-oauth-login", err.Error()), err)
	}

	currUser := ctx.User
	// if user is logged in, link account to user and update its avatar URL
	if currUser != nil {
		provider.UpdateUserDB(currUser)

		if err = currUser.Update(); err != nil {
			return ctx.ErrorRes(500, "Cannot update user "+cases.Title(language.English).String(provider.GetProvider())+" id", err)
		}

		ctx.AddFlash(ctx.Tr("flash.auth.account-linked-oauth", cases.Title(language.English).String(provider.GetProvider())), "success")
		return ctx.RedirectTo("/settings")
	}

	user := provider.GetProviderUser()
	userDB, err := db.GetUserByProvider(user.UserID, provider.GetProvider())
	// if user is not in database, create it
	if err != nil {
		if ctx.GetData("DisableSignup") == true {
			return ctx.ErrorRes(403, ctx.Tr("error.signup-disabled"), nil)
		}

		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.ErrorRes(500, "Cannot get user", err)
		}

		if user.NickName == "" {
			user.NickName = strings.Split(user.Email, "@")[0]
		}

		userDB = &db.User{
			Username: user.NickName,
			Email:    user.Email,
			MD5Hash:  fmt.Sprintf("%x", md5.Sum([]byte(strings.ToLower(strings.TrimSpace(user.Email))))),
		}

		// set provider id and avatar URL
		provider.UpdateUserDB(userDB)

		if err = userDB.Create(); err != nil {
			if db.IsUniqueConstraintViolation(err) {
				ctx.AddFlash(ctx.Tr("flash.auth.username-exists"), "error")
				return ctx.RedirectTo("/login")
			}

			return ctx.ErrorRes(500, "Cannot create user", err)
		}

		// if oidc admin group is not configured set first user as admin
		if config.C.OIDCAdminGroup == "" && userDB.ID == 1 {
			if err = userDB.SetAdmin(); err != nil {
				return ctx.ErrorRes(500, "Cannot set user admin", err)
			}
		}

		keys, err := provider.GetProviderUserSSHKeys()
		if err != nil {
			ctx.AddFlash(ctx.Tr("flash.auth.user-sshkeys-not-retrievable"), "error")
			log.Error().Err(err).Msg("Could not get user keys")
		} else {
			for _, key := range keys {
				sshKey := db.SSHKey{
					Title:   "Added from " + user.Provider,
					Content: key,
					User:    *userDB,
				}

				if err = sshKey.Create(); err != nil {
					ctx.AddFlash(ctx.Tr("flash.auth.user-sshkeys-not-created"), "error")
					log.Error().Err(err).Msg("Could not create ssh key")
				}
			}
		}
	}

	// update is admin status from oidc group
	if config.C.OIDCAdminGroup != "" {
		groupClaimName := config.C.OIDCGroupClaimName
		if groupClaimName == "" {
			log.Error().Msg("No OIDC group claim name configured")
		} else if groups, ok := user.RawData[groupClaimName].([]interface{}); ok {
			var groupNames []string
			for _, group := range groups {
				if groupName, ok := group.(string); ok {
					groupNames = append(groupNames, groupName)
				}
			}
			isOIDCAdmin := slices.Contains(groupNames, config.C.OIDCAdminGroup)
			log.Debug().Bool("isOIDCAdmin", isOIDCAdmin).Str("user", user.Name).Msg("User is in admin group")

			if userDB.IsAdmin != isOIDCAdmin {
				userDB.IsAdmin = isOIDCAdmin
				if err = userDB.Update(); err != nil {
					return ctx.ErrorRes(500, "Cannot set user admin", err)
				}
			}
		} else {
			log.Error().Msg("No groups found in user data")
		}
	}

	sess := ctx.GetSession()
	sess.Values["user"] = userDB.ID
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
			return ctx.ErrorRes(500, "Cannot unlink account from "+cases.Title(language.English).String(providerStr), err)
		}

		ctx.AddFlash(ctx.Tr("flash.auth.account-unlinked-oauth", cases.Title(language.English).String(providerStr)), "success")
		return ctx.RedirectTo("/settings")
	}

	return ctx.RedirectTo("/settings")
}
