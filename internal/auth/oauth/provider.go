package oauth

import (
	"errors"
	"fmt"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	GitHubProviderString = "github"
	GitLabProviderString = "gitlab"
	GiteaProviderString  = "gitea"
	OpenIDConnectString  = "openid-connect"
)

type Provider interface {
	RegisterProvider() error
	BeginAuthHandler(ctx *context.Context)
	UserHasProvider(user *db.User) bool
}

type CallbackProvider interface {
	GetProvider() string
	GetProviderUser() *goth.User
	GetProviderUserID(user *db.User) bool
	GetProviderUserSSHKeys() ([]string, error)
	UpdateUserDB(user *db.User)
}

func DefineProvider(provider string, url string) (Provider, error) {
	switch provider {
	case GitHubProviderString:
		return NewGitHubProvider(url), nil
	case GitLabProviderString:
		return NewGitLabProvider(url), nil
	case GiteaProviderString:
		return NewGiteaProvider(url), nil
	case OpenIDConnectString:
		return NewOIDCProvider(url), nil
	}

	return nil, errors.New(fmt.Sprintf("unsupported provider %s", provider))
}

func CompleteUserAuth(ctx *context.Context) (CallbackProvider, error) {
	user, err := gothic.CompleteUserAuth(ctx.Response(), ctx.Request())
	if err != nil {
		return nil, err
	}

	switch user.Provider {
	case GitHubProviderString:
		return NewGitHubCallbackProvider(&user), nil
	case GitLabProviderString:
		return NewGitLabCallbackProvider(&user), nil
	case GiteaProviderString:
		return NewGiteaCallbackProvider(&user), nil
	case OpenIDConnectString:
		return NewOIDCCallbackProvider(&user), nil
	}

	return nil, errors.New(fmt.Sprintf("unsupported provider %s", user.Provider))
}

func urlJoin(base string, elem ...string) string {
	joined, err := url.JoinPath(base, elem...)
	if err != nil {
		log.Error().Err(err).Msg("Cannot join url")
	}

	return joined
}

func readKeys(response *http.Response) ([]string, error) {
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("could not get user keys %v", err)
	}

	keys := strings.Split(string(body), "\n")
	if len(keys[len(keys)-1]) == 0 {
		keys = keys[:len(keys)-1]
	}

	return keys, nil
}
