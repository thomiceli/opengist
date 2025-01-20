package auth

import (
	"bytes"
	gojson "encoding/json"
	"github.com/thomiceli/opengist/internal/auth/webauthn"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
	"io"
)

func BeginWebAuthnBinding(ctx *context.Context) error {
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

func FinishWebAuthnBinding(ctx *context.Context) error {
	sess := ctx.GetSession()
	jsonWaSession, ok := sess.Values["webauthn_registration_session"].([]byte)
	if !ok {
		return ctx.ErrorRes(401, "Cannot get WebAuthn registration session", nil)
	}

	user := ctx.User

	// extract passkey name from request
	body, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		return ctx.ErrorRes(400, "Failed to read request body", err)
	}
	ctx.Request().Body.Close()
	ctx.Request().Body = io.NopCloser(bytes.NewBuffer(body))

	dto := new(db.CrendentialDTO)
	_ = gojson.Unmarshal(body, &dto)

	if err = ctx.Validate(dto); err != nil {
		return ctx.ErrorRes(400, "Invalid request", err)
	}
	passkeyName := dto.PasskeyName
	if passkeyName == "" {
		passkeyName = "WebAuthn"
	}

	waCredential, err := webauthn.FinishBinding(user, jsonWaSession, ctx.Request())
	if err != nil {
		return ctx.ErrorRes(403, "Failed binding attempt for passkey", err)
	}

	if _, err = db.CreateFromCrendential(user.ID, passkeyName, waCredential); err != nil {
		return ctx.ErrorRes(500, "Cannot create WebAuthn credential on database", err)
	}

	delete(sess.Values, "webauthn_registration_session")
	ctx.SaveSession(sess)

	ctx.AddFlash(ctx.Tr("flash.auth.passkey-registred", passkeyName), "success")
	return ctx.Json([]string{"OK"})
}

func BeginWebAuthnLogin(ctx *context.Context) error {
	credsCreation, jsonWaSession, err := webauthn.BeginDiscoverableLogin()
	if err != nil {
		return ctx.ErrorRes(401, "Cannot begin WebAuthn login", err)
	}

	sess := ctx.GetSession()
	sess.Values["webauthn_login_session"] = jsonWaSession
	sess.Options.MaxAge = 5 * 60 // 5 minutes
	ctx.SaveSession(sess)

	return ctx.Json(credsCreation)
}

func FinishWebAuthnLogin(ctx *context.Context) error {
	sess := ctx.GetSession()
	sessionData, ok := sess.Values["webauthn_login_session"].([]byte)
	if !ok {
		return ctx.ErrorRes(401, "Cannot get WebAuthn login session", nil)
	}

	userID, err := webauthn.FinishDiscoverableLogin(sessionData, ctx.Request())
	if err != nil {
		return ctx.ErrorRes(403, "Failed authentication attempt for passkey", err)
	}

	sess.Values["user"] = userID
	sess.Options.MaxAge = 60 * 60 * 24 * 365 // 1 year

	delete(sess.Values, "webauthn_login_session")
	ctx.SaveSession(sess)

	return ctx.Json([]string{"OK"})
}

func BeginWebAuthnAssertion(ctx *context.Context) error {
	sess := ctx.GetSession()

	ogUser, err := db.GetUserById(sess.Values["mfaID"].(uint))
	if err != nil {
		return ctx.ErrorRes(500, "Cannot get user", err)
	}

	credsCreation, jsonWaSession, err := webauthn.BeginLogin(ogUser)
	if err != nil {
		return ctx.ErrorRes(401, "Cannot begin WebAuthn login", err)
	}

	sess.Values["webauthn_assertion_session"] = jsonWaSession
	sess.Options.MaxAge = 5 * 60 // 5 minutes
	ctx.SaveSession(sess)

	return ctx.Json(credsCreation)
}

func FinishWebAuthnAssertion(ctx *context.Context) error {
	sess := ctx.GetSession()
	sessionData, ok := sess.Values["webauthn_assertion_session"].([]byte)
	if !ok {
		return ctx.ErrorRes(401, "Cannot get WebAuthn assertion session", nil)
	}

	userId := sess.Values["mfaID"].(uint)

	ogUser, err := db.GetUserById(userId)
	if err != nil {
		return ctx.ErrorRes(500, "Cannot get user", err)
	}

	if err = webauthn.FinishLogin(ogUser, sessionData, ctx.Request()); err != nil {
		return ctx.ErrorRes(403, "Failed authentication attempt for passkey", err)
	}

	sess.Values["user"] = userId
	sess.Options.MaxAge = 60 * 60 * 24 * 365 // 1 year

	delete(sess.Values, "webauthn_assertion_session")
	delete(sess.Values, "mfaID")
	ctx.SaveSession(sess)

	return ctx.Json([]string{"OK"})
}
