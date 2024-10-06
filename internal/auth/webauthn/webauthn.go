package webauthn

import (
	"encoding/json"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"net/http"
	"net/url"
)

var webAuthn *webauthn.WebAuthn

func Init(urlStr string) error {
	var rpid, rporigin string
	var err error

	if urlStr == "" {
		log.Info().Msg("External URL is not set, passkeys RP ID and Origins will be set to localhost")
		rpid = "localhost"
		rporigin = "http://localhost" + ":" + config.C.HttpPort
	} else {
		urlStruct, err := url.Parse(urlStr)
		if err != nil {
			return err
		}

		rpid = urlStruct.Hostname()
		rporigin, err = protocol.FullyQualifiedOrigin(urlStr)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get fully qualified origin from external URL")
		}
	}

	webAuthn, err = webauthn.New(&webauthn.Config{
		RPDisplayName: "Opengist",
		RPID:          rpid,
		RPOrigins:     []string{rporigin},
	})
	return err
}

func BeginBinding(dbUser *db.User) (credCreation *protocol.CredentialCreation, jsonSession []byte, err error) {
	waUser := &user{User: dbUser}
	credCreation, session, err := webAuthn.BeginRegistration(waUser, webauthn.WithAuthenticatorSelection(
		protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementRequired,
			UserVerification: protocol.VerificationRequired,
		},
	), webauthn.WithAppIdExcludeExtension("Opengist"), webauthn.WithExclusions(waUser.Exclusions()))
	if err != nil {
		return nil, nil, err
	}

	jsonSession, _ = json.Marshal(session)
	return
}

func FinishBinding(dbUser *db.User, jsonSession []byte, response *http.Request) (*webauthn.Credential, error) {
	waUser := &user{User: dbUser}

	var session webauthn.SessionData
	_ = json.Unmarshal(jsonSession, &session)

	return webAuthn.FinishRegistration(waUser, session, response)
}

func BeginDiscoverableLogin() (credCreation *protocol.CredentialAssertion, jsonSession []byte, err error) {
	credCreation, session, err := webAuthn.BeginDiscoverableLogin(
		webauthn.WithUserVerification(protocol.VerificationPreferred),
	)

	jsonSession, _ = json.Marshal(session)
	return
}

func FinishDiscoverableLogin(jsonSession []byte, response *http.Request) (uint, error) {
	var session webauthn.SessionData
	_ = json.Unmarshal(jsonSession, &session)

	parsedResponse, err := protocol.ParseCredentialRequestResponse(response)
	if err != nil {
		return 0, err
	}

	waUser, cred, err := webAuthn.ValidatePasskeyLogin(discoverUser, session, parsedResponse)
	if err != nil {
		return 0, err
	}

	dbCredential, err := db.GetCredentialByID(cred.ID)
	if err != nil {
		return 0, err
	}
	if err = dbCredential.UpdateSignCount(); err != nil {
		return 0, err
	}
	if err = dbCredential.UpdateLastUsedAt(); err != nil {
		return 0, err
	}

	return waUser.(*user).User.ID, nil
}

func BeginLogin(dbUser *db.User) (credCreation *protocol.CredentialAssertion, jsonSession []byte, err error) {
	waUser := &user{User: dbUser}
	credCreation, session, err := webAuthn.BeginLogin(waUser)

	jsonSession, _ = json.Marshal(session)
	return
}

func FinishLogin(dbUser *db.User, jsonSession []byte, response *http.Request) error {
	waUser := &user{User: dbUser}

	var session webauthn.SessionData
	_ = json.Unmarshal(jsonSession, &session)

	cred, err := webAuthn.FinishLogin(waUser, session, response)
	if err != nil {
		return err
	}

	dbCredential, err := db.GetCredentialByID(cred.ID)
	if err != nil {
		return err
	}
	if err = dbCredential.UpdateSignCount(); err != nil {
		return err
	}
	if err = dbCredential.UpdateLastUsedAt(); err != nil {
		return err
	}

	return err
}
