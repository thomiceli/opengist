package oauth

import (
	gocontext "context"
	gojson "encoding/json"
	"io"
	"net/http"

	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/gitlab"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
)

type GitLabProvider struct {
	Provider
	URL string
}

func (p *GitLabProvider) RegisterProvider() error {
	goth.UseProviders(
		gitlab.NewCustomisedURL(
			config.C.GitlabClientKey,
			config.C.GitlabSecret,
			urlJoin(p.URL, "/oauth/gitlab/callback"),
			urlJoin(config.C.GitlabUrl, "/oauth/authorize"),
			urlJoin(config.C.GitlabUrl, "/oauth/token"),
			urlJoin(config.C.GitlabUrl, "/api/v4/user"),
		),
	)

	return nil
}

func (p *GitLabProvider) BeginAuthHandler(ctx *context.Context) {
	ctxValue := gocontext.WithValue(ctx.Request().Context(), gothic.ProviderParamKey, GitLabProviderString)
	ctx.SetRequest(ctx.Request().WithContext(ctxValue))

	gothic.BeginAuthHandler(ctx.Response(), ctx.Request())
}

func (p *GitLabProvider) UserHasProvider(user *db.User) bool {
	return user.GitlabID != ""
}

func NewGitLabProvider(url string) *GitLabProvider {
	return &GitLabProvider{
		URL: url,
	}
}

type GitLabCallbackProvider struct {
	CallbackProvider
	User *goth.User
}

func (p *GitLabCallbackProvider) GetProvider() string {
	return GitLabProviderString
}

func (p *GitLabCallbackProvider) GetProviderUser() *goth.User {
	return p.User
}

func (p *GitLabCallbackProvider) GetProviderUserID(user *db.User) bool {
	return user.GitlabID != ""
}

func (p *GitLabCallbackProvider) GetProviderUserSSHKeys() ([]string, error) {
	resp, err := http.Get(urlJoin(config.C.GitlabUrl, p.User.NickName+".keys"))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return readKeys(resp)
}

func (p *GitLabCallbackProvider) UpdateUserDB(user *db.User) {
	user.GitlabID = p.User.UserID

	resp, err := http.Get(urlJoin(config.C.GitlabUrl, "/api/v4/avatar?size=400&email=", p.User.Email))
	if err != nil {
		log.Error().Err(err).Msg("Cannot get user avatar from GitLab")
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().Err(err).Msg("Cannot read Gitlab response body")
		return
	}

	var result map[string]interface{}
	err = gojson.Unmarshal(body, &result)
	if err != nil {
		log.Error().Err(err).Msg("Cannot unmarshal Gitlab response body")
		return
	}

	field, ok := result["avatar_url"]
	if !ok {
		log.Error().Msg("Field 'avatar_url' not found in Gitlab JSON response")
		return
	}

	user.AvatarURL = field.(string)
}

func NewGitLabCallbackProvider(user *goth.User) CallbackProvider {
	return &GitLabCallbackProvider{
		User: user,
	}
}
