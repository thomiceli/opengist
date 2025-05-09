package auth

import (
	"errors"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/auth/ldap"
	passwordpkg "github.com/thomiceli/opengist/internal/auth/password"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/i18n"
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

	var user *db.User
	var err error
	sess := ctx.GetSession()

	dto := &db.UserDTO{}
	if err = ctx.Bind(dto); err != nil {
		return ctx.ErrorRes(400, ctx.Tr("error.cannot-bind-data"), err)
	}

	if ldap.Enabled() {
		if user, err = tryLdapLogin(ctx, dto.Username, dto.Password); err != nil {
			return err
		}
	}
	if user == nil {
		if user, err = tryDbLogin(ctx, dto.Username, dto.Password); user == nil {
			return err
		}
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

func tryDbLogin(ctx *context.Context, username, password string) (user *db.User, err error) {
	if user, err = db.GetUserByUsername(username); err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ctx.ErrorRes(500, "Cannot get user", err)
		}

		log.Warn().Msg("Invalid HTTP authentication attempt from " + ctx.RealIP())
		ctx.AddFlash(ctx.Tr("flash.auth.invalid-credentials"), "error")
		return nil, ctx.RedirectTo("/login")
	}

	if ok, err := passwordpkg.VerifyPassword(password, user.Password); !ok {
		if err != nil {
			return nil, ctx.ErrorRes(500, "Cannot check for password", err)
		}
		log.Warn().Msg("Invalid HTTP authentication attempt from " + ctx.RealIP())
		ctx.AddFlash(ctx.Tr("flash.auth.invalid-credentials"), "error")
		return nil, ctx.RedirectTo("/login")
	}

	return user, nil
}

func tryLdapLogin(ctx *context.Context, username, password string) (user *db.User, err error) {
	ok, err := ldap.Authenticate(username, password)
	if err != nil {
		log.Info().Err(err).Msgf("LDAP authentication error")
		return nil, ctx.ErrorRes(500, "Cannot get user", err)
	}

	if !ok {
		log.Warn().Msg("Invalid LDAP authentication attempt from " + ctx.RealIP())
		return nil, nil
	}

	if user, err = db.GetUserByUsername(username); err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ctx.ErrorRes(500, "Cannot get user", err)
		}
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		user = &db.User{
			Username: username,
		}
		if err = user.Create(); err != nil {
			log.Warn().Err(err).Msg("Cannot create user after LDAP authentication")
			return nil, ctx.ErrorRes(500, "Cannot create user", err)
		}

		return user, nil
	}

	return user, nil
}
