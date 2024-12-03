package oauth

import (
	gocontext "context"
	gojson "encoding/json"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/gitea"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
	"io"
	"net/http"
)

type GiteaProvider struct {
	Provider
	URL string
}

func (p *GiteaProvider) RegisterProvider() error {
	goth.UseProviders(
		gitea.NewCustomisedURL(
			config.C.GiteaClientKey,
			config.C.GiteaSecret,
			urlJoin(p.URL, "/oauth/gitea/callback"),
			urlJoin(config.C.GiteaUrl, "/login/oauth/authorize"),
			urlJoin(config.C.GiteaUrl, "/login/oauth/access_token"),
			urlJoin(config.C.GiteaUrl, "/api/v1/user"),
		),
	)

	return nil
}

func (p *GiteaProvider) BeginAuthHandler(ctx *context.Context) {
	ctxValue := gocontext.WithValue(ctx.Request().Context(), gothic.ProviderParamKey, GiteaProviderString)
	ctx.SetRequest(ctx.Request().WithContext(ctxValue))

	gothic.BeginAuthHandler(ctx.Response(), ctx.Request())
}

func (p *GiteaProvider) UserHasProvider(user *db.User) bool {
	return user.GiteaID != ""
}

func NewGiteaProvider(url string) *GiteaProvider {
	return &GiteaProvider{
		URL: url,
	}
}

type GiteaCallbackProvider struct {
	CallbackProvider
	User *goth.User
}

func (p *GiteaCallbackProvider) GetProvider() string {
	return GiteaProviderString
}

func (p *GiteaCallbackProvider) GetProviderUser() *goth.User {
	return p.User
}

func (p *GiteaCallbackProvider) GetProviderUserID(user *db.User) bool {
	return user.GiteaID != ""
}

func (p *GiteaCallbackProvider) GetProviderUserSSHKeys() ([]string, error) {
	resp, err := http.Get(urlJoin(config.C.GiteaUrl, p.User.NickName+".keys"))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return readKeys(resp)
}

func (p *GiteaCallbackProvider) UpdateUserDB(user *db.User) {
	user.GiteaID = p.User.UserID

	resp, err := http.Get(urlJoin(config.C.GiteaUrl, "/api/v1/users/", p.User.UserID))
	if err != nil {
		log.Error().Err(err).Msg("Cannot get user from Gitea")
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().Err(err).Msg("Cannot read Gitea response body")
		return
	}

	var result map[string]interface{}
	err = gojson.Unmarshal(body, &result)
	if err != nil {
		log.Error().Err(err).Msg("Cannot unmarshal Gitea response body")
		return
	}

	field, ok := result["avatar_url"]
	if !ok {
		log.Error().Msg("Field 'avatar_url' not found in Gitea JSON response")
		return
	}

	user.AvatarURL = field.(string)
}

func NewGiteaCallbackProvider(user *goth.User) CallbackProvider {
	return &GiteaCallbackProvider{
		User: user,
	}
}
