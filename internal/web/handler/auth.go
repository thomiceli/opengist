package handler

import (
	"bytes"
	gocontext "context"
	"crypto/md5"
	gojson "encoding/json"
	"errors"
	"fmt"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/gitea"
	"github.com/markbates/goth/providers/github"
	"github.com/markbates/goth/providers/gitlab"
	"github.com/markbates/goth/providers/openidConnect"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/auth/totp"
	"github.com/thomiceli/opengist/internal/auth/webauthn"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/i18n"
	"github.com/thomiceli/opengist/internal/utils"
	"github.com/thomiceli/opengist/internal/web/context"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gorm.io/gorm"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	GitHubProvider = "github"
	GitLabProvider = "gitlab"
	GiteaProvider  = "gitea"
	OpenIDConnect  = "openid-connect"
)

func register(ctx *context.OGContext) error {
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
	return ctx.HTML_("auth_form.html")
}

func processRegister(ctx *context.OGContext) error {
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
		ctx.AddFlash(utils.ValidationMessages(&err, ctx.GetData("locale").(*i18n.Locale)), "error")
		return ctx.HTML_("auth_form.html")
	}

	if exists, err := db.UserExists(dto.Username); err != nil || exists {
		ctx.AddFlash(ctx.Tr("flash.auth.username-exists"), "error")
		return ctx.HTML_("auth_form.html")
	}

	user := dto.ToUser()

	password, err := utils.Argon2id.Hash(user.Password)
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

func login(ctx *context.OGContext) error {
	ctx.SetData("title", ctx.TrH("auth.login"))
	ctx.SetData("htmlTitle", ctx.TrH("auth.login"))
	ctx.SetData("disableForm", ctx.GetData("DisableLoginForm"))
	ctx.SetData("isLoginPage", true)
	return ctx.HTML_("auth_form.html")
}

func processLogin(ctx *context.OGContext) error {
	if ctx.GetData("DisableLoginForm") == true {
		return ctx.ErrorRes(403, ctx.Tr("error.login-disabled-form"), nil)
	}

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

	if ok, err := utils.Argon2id.Verify(password, user.Password); !ok {
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

func mfa(ctx *context.OGContext) error {
	var err error

	user := db.User{ID: ctx.GetSession().Values["mfaID"].(uint)}

	var hasWebauthn, hasTotp bool
	if hasWebauthn, hasTotp, err = user.HasMFA(); err != nil {
		return ctx.ErrorRes(500, "Cannot check for user MFA", err)
	}

	ctx.SetData("hasWebauthn", hasWebauthn)
	ctx.SetData("hasTotp", hasTotp)

	return ctx.HTML_("mfa.html")
}

func oauthCallback(ctx *context.OGContext) error {
	user, err := gothic.CompleteUserAuth(ctx.Response(), ctx.Request())
	if err != nil {
		return ctx.ErrorRes(400, ctx.Tr("error.complete-oauth-login", err.Error()), err)
	}

	currUser := ctx.User
	if currUser != nil {
		// if user is logged in, link account to user and update its avatar URL
		updateUserProviderInfo(currUser, user.Provider, user)

		if err = currUser.Update(); err != nil {
			return ctx.ErrorRes(500, "Cannot update user "+cases.Title(language.English).String(user.Provider)+" id", err)
		}

		ctx.AddFlash(ctx.Tr("flash.auth.account-linked-oauth", cases.Title(language.English).String(user.Provider)), "success")
		return ctx.RedirectTo("/settings")
	}

	// if user is not in database, create it
	userDB, err := db.GetUserByProvider(user.UserID, user.Provider)
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
		updateUserProviderInfo(userDB, user.Provider, user)

		if err = userDB.Create(); err != nil {
			if db.IsUniqueConstraintViolation(err) {
				ctx.AddFlash(ctx.Tr("flash.auth.username-exists"), "error")
				return ctx.RedirectTo("/login")
			}

			return ctx.ErrorRes(500, "Cannot create user", err)
		}

		if userDB.ID == 1 {
			if err = userDB.SetAdmin(); err != nil {
				return ctx.ErrorRes(500, "Cannot set user admin", err)
			}
		}

		var resp *http.Response
		switch user.Provider {
		case GitHubProvider:
			resp, err = http.Get("https://github.com/" + user.NickName + ".keys")
		case GitLabProvider:
			resp, err = http.Get(urlJoin(config.C.GitlabUrl, user.NickName+".keys"))
		case GiteaProvider:
			resp, err = http.Get(urlJoin(config.C.GiteaUrl, user.NickName+".keys"))
		case OpenIDConnect:
			err = errors.New("cannot get keys from OIDC provider")
		}

		if err == nil {
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				ctx.AddFlash(ctx.Tr("flash.auth.user-sshkeys-not-retrievable"), "error")
				log.Error().Err(err).Msg("Could not get user keys")
			}

			keys := strings.Split(string(body), "\n")
			if len(keys[len(keys)-1]) == 0 {
				keys = keys[:len(keys)-1]
			}
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

	sess := ctx.GetSession()
	sess.Values["user"] = userDB.ID
	ctx.SaveSession(sess)
	ctx.DeleteCsrfCookie()

	return ctx.RedirectTo("/")
}

func oauth(ctx *context.OGContext) error {
	provider := ctx.Param("provider")

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

	switch provider {
	case GitHubProvider:
		goth.UseProviders(
			github.New(
				config.C.GithubClientKey,
				config.C.GithubSecret,
				urlJoin(opengistUrl, "/oauth/github/callback"),
			),
		)

	case GitLabProvider:
		goth.UseProviders(
			gitlab.NewCustomisedURL(
				config.C.GitlabClientKey,
				config.C.GitlabSecret,
				urlJoin(opengistUrl, "/oauth/gitlab/callback"),
				urlJoin(config.C.GitlabUrl, "/oauth/authorize"),
				urlJoin(config.C.GitlabUrl, "/oauth/token"),
				urlJoin(config.C.GitlabUrl, "/api/v4/user"),
			),
		)

	case GiteaProvider:
		goth.UseProviders(
			gitea.NewCustomisedURL(
				config.C.GiteaClientKey,
				config.C.GiteaSecret,
				urlJoin(opengistUrl, "/oauth/gitea/callback"),
				urlJoin(config.C.GiteaUrl, "/login/oauth/authorize"),
				urlJoin(config.C.GiteaUrl, "/login/oauth/access_token"),
				urlJoin(config.C.GiteaUrl, "/api/v1/user"),
			),
		)
	case OpenIDConnect:
		oidcProvider, err := openidConnect.New(
			config.C.OIDCClientKey,
			config.C.OIDCSecret,
			urlJoin(opengistUrl, "/oauth/openid-connect/callback"),
			config.C.OIDCDiscoveryUrl,
			"openid",
			"email",
			"profile",
		)

		if err != nil {
			return ctx.ErrorRes(500, "Cannot create OIDC provider", err)
		}

		goth.UseProviders(oidcProvider)
	}

	ctxValue := gocontext.WithValue(ctx.Request().Context(), gothic.ProviderParamKey, provider)
	ctx.SetRequest(ctx.Request().WithContext(ctxValue))
	if provider != GitHubProvider && provider != GitLabProvider && provider != GiteaProvider && provider != OpenIDConnect {
		return ctx.ErrorRes(400, ctx.Tr("error.oauth-unsupported"), nil)
	}

	gothic.BeginAuthHandler(ctx.Response(), ctx.Request())
	return nil
}

func oauthUnlink(ctx *context.OGContext) error {
	provider := ctx.Param("provider")

	currUser := ctx.User
	// Map each provider to a function that checks the relevant ID in currUser
	providerIDCheckMap := map[string]func() bool{
		GitHubProvider: func() bool { return currUser.GithubID != "" },
		GitLabProvider: func() bool { return currUser.GitlabID != "" },
		GiteaProvider:  func() bool { return currUser.GiteaID != "" },
		OpenIDConnect:  func() bool { return currUser.OIDCID != "" },
	}

	if checkFunc, exists := providerIDCheckMap[provider]; exists && checkFunc() {
		if err := currUser.DeleteProviderID(provider); err != nil {
			return ctx.ErrorRes(500, "Cannot unlink account from "+cases.Title(language.English).String(provider), err)
		}

		ctx.AddFlash(ctx.Tr("flash.auth.account-unlinked-oauth", cases.Title(language.English).String(provider)), "success")
		return ctx.RedirectTo("/settings")
	}

	return ctx.RedirectTo("/settings")
}

func beginWebAuthnBinding(ctx *context.OGContext) error {
	credsCreation, jsonWaSession, err := webauthn.BeginBinding(ctx.User)
	if err != nil {
		return ctx.ErrorRes(500, "Cannot begin WebAuthn registration", err)
	}

	sess := ctx.GetSession()
	sess.Values["webauthn_registration_session"] = jsonWaSession
	sess.Options.MaxAge = 5 * 60 // 5 minutes
	ctx.SaveSession(sess)

	return ctx.JSON(200, credsCreation)
}

func finishWebAuthnBinding(ctx *context.OGContext) error {
	sess := ctx.GetSession()
	jsonWaSession, ok := sess.Values["webauthn_registration_session"].([]byte)
	if !ok {
		return ctx.JsonErrorRes(401, "Cannot get WebAuthn registration session", nil)
	}

	user := ctx.User

	// extract passkey name from request
	body, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		return ctx.JsonErrorRes(400, "Failed to read request body", err)
	}
	ctx.Request().Body.Close()
	ctx.Request().Body = io.NopCloser(bytes.NewBuffer(body))

	dto := new(db.CrendentialDTO)
	_ = gojson.Unmarshal(body, &dto)

	if err = ctx.Validate(dto); err != nil {
		return ctx.JsonErrorRes(400, "Invalid request", err)
	}
	passkeyName := dto.PasskeyName
	if passkeyName == "" {
		passkeyName = "WebAuthn"
	}

	waCredential, err := webauthn.FinishBinding(user, jsonWaSession, ctx.Request())
	if err != nil {
		return ctx.JsonErrorRes(403, "Failed binding attempt for passkey", err)
	}

	if _, err = db.CreateFromCrendential(user.ID, passkeyName, waCredential); err != nil {
		return ctx.JsonErrorRes(500, "Cannot create WebAuthn credential on database", err)
	}

	delete(sess.Values, "webauthn_registration_session")
	ctx.SaveSession(sess)

	ctx.AddFlash(ctx.Tr("flash.auth.passkey-registred", passkeyName), "success")
	return ctx.JSON_([]string{"OK"})
}

func beginWebAuthnLogin(ctx *context.OGContext) error {
	credsCreation, jsonWaSession, err := webauthn.BeginDiscoverableLogin()
	if err != nil {
		return ctx.JsonErrorRes(401, "Cannot begin WebAuthn login", err)
	}

	sess := ctx.GetSession()
	sess.Values["webauthn_login_session"] = jsonWaSession
	sess.Options.MaxAge = 5 * 60 // 5 minutes
	ctx.SaveSession(sess)

	return ctx.JSON_(credsCreation)
}

func finishWebAuthnLogin(ctx *context.OGContext) error {
	sess := ctx.GetSession()
	sessionData, ok := sess.Values["webauthn_login_session"].([]byte)
	if !ok {
		return ctx.JsonErrorRes(401, "Cannot get WebAuthn login session", nil)
	}

	userID, err := webauthn.FinishDiscoverableLogin(sessionData, ctx.Request())
	if err != nil {
		return ctx.JsonErrorRes(403, "Failed authentication attempt for passkey", err)
	}

	sess.Values["user"] = userID
	sess.Options.MaxAge = 60 * 60 * 24 * 365 // 1 year

	delete(sess.Values, "webauthn_login_session")
	ctx.SaveSession(sess)

	return ctx.JSON_([]string{"OK"})
}

func beginWebAuthnAssertion(ctx *context.OGContext) error {
	sess := ctx.GetSession()

	ogUser, err := db.GetUserById(sess.Values["mfaID"].(uint))
	if err != nil {
		return ctx.JsonErrorRes(500, "Cannot get user", err)
	}

	credsCreation, jsonWaSession, err := webauthn.BeginLogin(ogUser)
	if err != nil {
		return ctx.JsonErrorRes(401, "Cannot begin WebAuthn login", err)
	}

	sess.Values["webauthn_assertion_session"] = jsonWaSession
	sess.Options.MaxAge = 5 * 60 // 5 minutes
	ctx.SaveSession(sess)

	return ctx.JSON_(credsCreation)
}

func finishWebAuthnAssertion(ctx *context.OGContext) error {
	sess := ctx.GetSession()
	sessionData, ok := sess.Values["webauthn_assertion_session"].([]byte)
	if !ok {
		return ctx.JsonErrorRes(401, "Cannot get WebAuthn assertion session", nil)
	}

	userId := sess.Values["mfaID"].(uint)

	ogUser, err := db.GetUserById(userId)
	if err != nil {
		return ctx.JsonErrorRes(500, "Cannot get user", err)
	}

	if err = webauthn.FinishLogin(ogUser, sessionData, ctx.Request()); err != nil {
		return ctx.JsonErrorRes(403, "Failed authentication attempt for passkey", err)
	}

	sess.Values["user"] = userId
	sess.Options.MaxAge = 60 * 60 * 24 * 365 // 1 year

	delete(sess.Values, "webauthn_assertion_session")
	delete(sess.Values, "mfaID")
	ctx.SaveSession(sess)

	return ctx.JSON_([]string{"OK"})
}

func beginTotp(ctx *context.OGContext) error {
	user := ctx.User

	if _, hasTotp, err := user.HasMFA(); err != nil {
		return ctx.ErrorRes(500, "Cannot check for user MFA", err)
	} else if hasTotp {
		ctx.AddFlash(ctx.Tr("auth.totp.already-enabled"), "error")
		return ctx.RedirectTo("/settings")
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

	return ctx.HTML_("totp.html")

}

func finishTotp(ctx *context.OGContext) error {
	user := ctx.User

	if _, hasTotp, err := user.HasMFA(); err != nil {
		return ctx.ErrorRes(500, "Cannot check for user MFA", err)
	} else if hasTotp {
		ctx.AddFlash(ctx.Tr("auth.totp.already-enabled"), "error")
		return ctx.RedirectTo("/settings")
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
	return ctx.HTML_("totp.html")
}

func assertTotp(ctx *context.OGContext) error {
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
		redirectUrl = "/settings"
	}

	sess.Values["user"] = userId
	sess.Options.MaxAge = 60 * 60 * 24 * 365 // 1 year
	delete(sess.Values, "mfaID")
	ctx.SaveSession(sess)

	return ctx.RedirectTo(redirectUrl)
}

func disableTotp(ctx *context.OGContext) error {
	user := ctx.User
	userTotp, err := db.GetTOTPByUserID(user.ID)
	if err != nil {
		return ctx.ErrorRes(500, "Cannot get TOTP by UID", err)
	}

	if err = userTotp.Delete(); err != nil {
		return ctx.ErrorRes(500, "Cannot delete TOTP", err)
	}

	ctx.AddFlash(ctx.Tr("auth.totp.disabled"), "success")
	return ctx.RedirectTo("/settings")
}

func regenerateTotpRecoveryCodes(ctx *context.OGContext) error {
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
	return ctx.HTML_("totp.html")
}

func logout(ctx *context.OGContext) error {
	ctx.DeleteSession()
	ctx.DeleteCsrfCookie()
	return ctx.RedirectTo("/all")
}

func urlJoin(base string, elem ...string) string {
	joined, err := url.JoinPath(base, elem...)
	if err != nil {
		log.Error().Err(err).Msg("Cannot join url")
	}

	return joined
}

func updateUserProviderInfo(userDB *db.User, provider string, user goth.User) {
	userDB.AvatarURL = getAvatarUrlFromProvider(provider, user.UserID)
	switch provider {
	case GitHubProvider:
		userDB.GithubID = user.UserID
	case GitLabProvider:
		userDB.GitlabID = user.UserID
	case GiteaProvider:
		userDB.GiteaID = user.UserID
	case OpenIDConnect:
		userDB.OIDCID = user.UserID
		userDB.AvatarURL = user.AvatarURL
	}
}

func getAvatarUrlFromProvider(provider string, identifier string) string {
	switch provider {
	case GitHubProvider:
		return "https://avatars.githubusercontent.com/u/" + identifier + "?v=4"
	case GitLabProvider:
		return urlJoin(config.C.GitlabUrl, "/uploads/-/system/user/avatar/", identifier, "/avatar.png") + "?width=400"
	case GiteaProvider:
		resp, err := http.Get(urlJoin(config.C.GiteaUrl, "/api/v1/users/", identifier))
		if err != nil {
			log.Error().Err(err).Msg("Cannot get user from Gitea")
			return ""
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Error().Err(err).Msg("Cannot read Gitea response body")
			return ""
		}

		var result map[string]interface{}
		err = gojson.Unmarshal(body, &result)
		if err != nil {
			log.Error().Err(err).Msg("Cannot unmarshal Gitea response body")
			return ""
		}

		field, ok := result["avatar_url"]
		if !ok {
			log.Error().Msg("Field 'avatar_url' not found in Gitea JSON response")
			return ""
		}
		return field.(string)
	}
	return ""
}

type ContextAuthInfo struct {
	context *context.OGContext
}

func (auth ContextAuthInfo) RequireLogin() (bool, error) {
	return auth.context.GetData("RequireLogin") == true, nil
}

func (auth ContextAuthInfo) AllowGistsWithoutLogin() (bool, error) {
	return auth.context.GetData("AllowGistsWithoutLogin") == true, nil
}
