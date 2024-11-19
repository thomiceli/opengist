package auth

import (
	"errors"
	"github.com/rs/zerolog/log"
	passwordpkg "github.com/thomiceli/opengist/internal/auth/password"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/i18n"
	"github.com/thomiceli/opengist/internal/ldap"
	"github.com/thomiceli/opengist/internal/validator"
	"github.com/thomiceli/opengist/internal/web/context"
	"gorm.io/gorm"
)

func Register(ctx *context.Context) error {
	disableSignup := ctx.GetData("DisableSignup")
	disableForm := ctx.GetData("DisableLoginForm")

	code := ctx.QueryParam("code")
	if code != "" {
		if invitation, err := db.GetInvitationByCode(code); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.ErrorRes(500, "Cannot check for invitation code", err)
		} else if invitation != nil && invitation.IsUsable() {
			disableSignup = false
		}
	}

	ctx.SetData("title", ctx.TrH("auth.new-account"))
	ctx.SetData("htmlTitle", ctx.TrH("auth.new-account"))
	ctx.SetData("disableForm", disableForm)
	ctx.SetData("disableSignup", disableSignup)
	ctx.SetData("isLoginPage", false)
	return ctx.Html("auth_form.html")
}

func ProcessRegister(ctx *context.Context) error {
	disableSignup := ctx.GetData("DisableSignup")

	code := ctx.QueryParam("code")
	invitation, err := db.GetInvitationByCode(code)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return ctx.ErrorRes(500, "Cannot check for invitation code", err)
	} else if invitation.ID != 0 && invitation.IsUsable() {
		disableSignup = false
	}

	if disableSignup == true {
		return ctx.ErrorRes(403, ctx.Tr("error.signup-disabled"), nil)
	}

	if ctx.GetData("DisableLoginForm") == true {
		return ctx.ErrorRes(403, ctx.Tr("error.signup-disabled-form"), nil)
	}

	ctx.SetData("title", ctx.TrH("auth.new-account"))
	ctx.SetData("htmlTitle", ctx.TrH("auth.new-account"))

	sess := ctx.GetSession()

	dto := new(db.UserDTO)
	if err := ctx.Bind(dto); err != nil {
		return ctx.ErrorRes(400, ctx.Tr("error.cannot-bind-data"), err)
	}

	if err := ctx.Validate(dto); err != nil {
		ctx.AddFlash(validator.ValidationMessages(&err, ctx.GetData("locale").(*i18n.Locale)), "error")
		return ctx.Html("auth_form.html")
	}

	if exists, err := db.UserExists(dto.Username); err != nil || exists {
		ctx.AddFlash(ctx.Tr("flash.auth.username-exists"), "error")
		return ctx.Html("auth_form.html")
	}

	user := dto.ToUser()

	password, err := passwordpkg.HashPassword(user.Password)
	if err != nil {
		return ctx.ErrorRes(500, "Cannot hash password", err)
	}
	user.Password = password

	if err = user.Create(); err != nil {
		return ctx.ErrorRes(500, "Cannot create user", err)
	}

	if user.ID == 1 {
		if err = user.SetAdmin(); err != nil {
			return ctx.ErrorRes(500, "Cannot set user admin", err)
		}
	}

	if invitation.ID != 0 {
		if err := invitation.Use(); err != nil {
			return ctx.ErrorRes(500, "Cannot use invitation", err)
		}
	}

	sess.Values["user"] = user.ID
	ctx.SaveSession(sess)

	return ctx.RedirectTo("/")
}

func Login(ctx *context.Context) error {
	ctx.SetData("title", ctx.TrH("auth.login"))
	ctx.SetData("htmlTitle", ctx.TrH("auth.login"))
	ctx.SetData("disableForm", ctx.GetData("DisableLoginForm"))
	ctx.SetData("isLoginPage", true)
	return ctx.Html("auth_form.html")
}

func ProcessLogin(ctx *context.Context) error {
	if ctx.GetData("DisableLoginForm") == true {
		return ctx.ErrorRes(403, ctx.Tr("error.login-disabled-form"), nil)
	}

	enableLDAP := ctx.GetData("EnableLdap")

	var err error
	sess := ctx.GetSession()

	dto := &db.UserDTO{}
	if err = ctx.Bind(dto); err != nil {
		return ctx.ErrorRes(400, ctx.Tr("error.cannot-bind-data"), err)
	}
	password := dto.Password

	var user *db.User

	if user, err = db.GetUserByUsername(dto.Username); err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.ErrorRes(500, "Cannot get user", err)
		}
		log.Warn().Msg("Invalid HTTP authentication attempt from " + ctx.RealIP())
		ctx.AddFlash(ctx.Tr("flash.auth.invalid-credentials"), "error")
		return ctx.RedirectTo("/login")
	}

	if enableLDAP == true {
		ok, err := ldap.Authenticate(user.Username, password)
		if err != nil {
			log.Warn().Msgf("Cannot check for LDAP password: %v", err)
			log.Info().Msg("LDAP authentication failed for user: " + user.Username)
		}

		if !ok {
			log.Warn().Msg("Invalid LDAP authentication attempt from " + ctx.RealIP())
			log.Info().Msg("LDAP authentication failed for user: " + user.Username)
			ctx.AddFlash(ctx.Tr("flash.auth.invalid-credentials"), "error")
		}

		if ok {
			user.Password, err = passwordpkg.HashPassword(password)
			if err != nil {
				return ctx.ErrorRes(500, "Cannot hash password for user", err)
			}

			err = user.Update()
			if err != nil {
				log.Info().Msg("LDAP authentication failed for " + user.Username)
				return ctx.ErrorRes(500, "Cannot update LDAP user "+user.Username, err)
			}

			log.Info().Msg("Synced local password from LDAP for user: " + user.Username)
			log.Info().Msg("LDAP authentication succeeded for user: " + user.Username)
		}
	}

	if ok, err := passwordpkg.VerifyPassword(password, user.Password); !ok {
		if err != nil {
			return ctx.ErrorRes(500, "Cannot check for password", err)
		}
		log.Warn().Msg("Invalid HTTP authentication attempt from " + ctx.RealIP())
		ctx.AddFlash(ctx.Tr("flash.auth.invalid-credentials"), "error")
		return ctx.RedirectTo("/login")
	}

	// handle MFA
	var hasWebauthn, hasTotp bool
	if hasWebauthn, hasTotp, err = user.HasMFA(); err != nil {
		return ctx.ErrorRes(500, "Cannot check for user MFA", err)
	}
	if hasWebauthn || hasTotp {
		sess.Values["mfaID"] = user.ID
		sess.Options.MaxAge = 5 * 60 // 5 minutes
		ctx.SaveSession(sess)
		return ctx.RedirectTo("/mfa")
	}

	sess.Values["user"] = user.ID
	sess.Options.MaxAge = 60 * 60 * 24 * 365 // 1 year
	ctx.SaveSession(sess)
	ctx.DeleteCsrfCookie()

	return ctx.RedirectTo("/")
}

func Logout(ctx *context.Context) error {
	ctx.DeleteSession()
	ctx.DeleteCsrfCookie()
	return ctx.RedirectTo("/all")
}
