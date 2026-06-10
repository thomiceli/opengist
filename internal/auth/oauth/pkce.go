package oauth

import (
	"github.com/markbates/goth"
	"github.com/markbates/goth/providers/openidConnect"
	"github.com/thomiceli/opengist/internal/web/context"
	"golang.org/x/oauth2"
)

const codeVerifierSessionKey = "oauth_code_verifier"

func enablePKCE(ctx *context.Context, providerName string) error {
	gothProvider, err := goth.GetProvider(providerName)
	if err != nil {
		return err
	}

	oidcProvider, ok := gothProvider.(*openidConnect.Provider)
	if !ok {
		return nil
	}

	verifier := oauth2.GenerateVerifier()

	sess := ctx.GetSession()
	sess.Values[codeVerifierSessionKey] = verifier
	ctx.SaveSession(sess)

	oidcProvider.SetAuthCodeOptions(map[string]string{
		"code_challenge":        oauth2.S256ChallengeFromVerifier(verifier),
		"code_challenge_method": "S256",
	})

	return nil
}

func injectCodeVerifier(ctx *context.Context) {
	sess := ctx.GetSession()
	verifier, ok := sess.Values[codeVerifierSessionKey].(string)
	if !ok || verifier == "" {
		return
	}

	req := ctx.Request()
	q := req.URL.Query()
	q.Set("code_verifier", verifier)
	req.URL.RawQuery = q.Encode()

	delete(sess.Values, codeVerifierSessionKey)
	ctx.SaveSession(sess)
}
