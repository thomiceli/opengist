package oauth

import (
	gocontext "context"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/gitlab"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
	"net/http"
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
	user.AvatarURL = urlJoin(config.C.GitlabUrl, "/uploads/-/system/user/avatar/", p.User.UserID, "/avatar.png") + "?width=400"
}

func NewGitLabCallbackProvider(user *goth.User) CallbackProvider {
	return &GitLabCallbackProvider{
		User: user,
	}
}
