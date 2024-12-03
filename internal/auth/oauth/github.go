package oauth

import (
	gocontext "context"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/github"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
	"net/http"
)

type GitHubProvider struct {
	Provider
	URL string
}

func (p *GitHubProvider) RegisterProvider() error {
	goth.UseProviders(
		github.New(
			config.C.GithubClientKey,
			config.C.GithubSecret,
			urlJoin(p.URL, "/oauth/github/callback"),
		),
	)

	return nil
}

func (p *GitHubProvider) BeginAuthHandler(ctx *context.Context) {
	ctxValue := gocontext.WithValue(ctx.Request().Context(), gothic.ProviderParamKey, GitHubProviderString)
	ctx.SetRequest(ctx.Request().WithContext(ctxValue))

	gothic.BeginAuthHandler(ctx.Response(), ctx.Request())
}

func (p *GitHubProvider) UserHasProvider(user *db.User) bool {
	return user.GithubID != ""
}

func NewGitHubProvider(url string) *GitHubProvider {
	return &GitHubProvider{
		URL: url,
	}
}

type GitHubCallbackProvider struct {
	CallbackProvider
	User *goth.User
}

func (p *GitHubCallbackProvider) GetProvider() string {
	return GitHubProviderString
}

func (p *GitHubCallbackProvider) GetProviderUser() *goth.User {
	return p.User
}

func (p *GitHubCallbackProvider) GetProviderUserID(user *db.User) bool {
	return user.GithubID != ""
}

func (p *GitHubCallbackProvider) GetProviderUserSSHKeys() ([]string, error) {
	resp, err := http.Get("https://github.com/" + p.User.NickName + ".keys")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return readKeys(resp)
}

func (p *GitHubCallbackProvider) UpdateUserDB(user *db.User) {
	user.GithubID = p.User.UserID
	user.AvatarURL = "https://avatars.githubusercontent.com/u/" + p.User.UserID + "?v=4"
}

func NewGitHubCallbackProvider(user *goth.User) CallbackProvider {
	return &GitHubCallbackProvider{
		User: user,
	}
}
