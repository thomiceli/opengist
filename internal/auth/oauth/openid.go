package oauth

import (
	gocontext "context"
	"errors"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/openidConnect"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
)

type OIDCProvider struct {
	Provider
	URL string
}

func (p *OIDCProvider) RegisterProvider() error {
	oidcProvider, err := openidConnect.New(
		config.C.OIDCClientKey,
		config.C.OIDCSecret,
		urlJoin(p.URL, "/oauth/openid-connect/callback"),
		config.C.OIDCDiscoveryUrl,
		"openid",
		"email",
		"profile",
		config.C.OIDCGroupClaimName,
	)

	if err != nil {
		return errors.New("Cannot create OIDC provider: " + err.Error())
	}

	goth.UseProviders(oidcProvider)
	return nil
}

func (p *OIDCProvider) BeginAuthHandler(ctx *context.Context) {
	ctxValue := gocontext.WithValue(ctx.Request().Context(), gothic.ProviderParamKey, OpenIDConnectString)
	ctx.SetRequest(ctx.Request().WithContext(ctxValue))

	gothic.BeginAuthHandler(ctx.Response(), ctx.Request())
}

func (p *OIDCProvider) UserHasProvider(user *db.User) bool {
	return user.OIDCID != ""
}

func NewOIDCProvider(url string) *OIDCProvider {
	return &OIDCProvider{
		URL: url,
	}
}

type OIDCCallbackProvider struct {
	CallbackProvider
	User *goth.User
}

func (p *OIDCCallbackProvider) GetProvider() string {
	return OpenIDConnectString
}

func (p *OIDCCallbackProvider) GetProviderUser() *goth.User {
	return p.User
}

func (p *OIDCCallbackProvider) GetProviderUserID(user *db.User) bool {
	return user.OIDCID != ""
}

func (p *OIDCCallbackProvider) GetProviderUserSSHKeys() ([]string, error) {
	return nil, nil
}

func (p *OIDCCallbackProvider) UpdateUserDB(user *db.User) {
	user.OIDCID = p.User.UserID
	user.AvatarURL = p.User.AvatarURL
}

func NewOIDCCallbackProvider(user *goth.User) CallbackProvider {
	return &OIDCCallbackProvider{
		User: user,
	}
}
