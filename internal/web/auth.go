package web

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/gitea"
	"github.com/markbates/goth/providers/github"
	"github.com/markbates/goth/providers/openidConnect"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gorm.io/gorm"
)

var title = cases.Title(language.English)

func register(ctx echo.Context) error {
	setData(ctx, "title", tr(ctx, "auth.new-account"))
	setData(ctx, "htmlTitle", "New account")
	setData(ctx, "disableForm", getData(ctx, "DisableLoginForm"))
	return html(ctx, "auth_form.html")
}

func processRegister(ctx echo.Context) error {
	if getData(ctx, "DisableSignup") == true {
		return errorRes(403, "Signing up is disabled", nil)
	}

	if getData(ctx, "DisableLoginForm") == true {
		return errorRes(403, "Signing up via registration form is disabled", nil)
	}

	setData(ctx, "title", "New account")
	setData(ctx, "htmlTitle", "New account")

	sess := getSession(ctx)

	dto := new(db.UserDTO)
	if err := ctx.Bind(dto); err != nil {
		return errorRes(400, "Cannot bind data", err)
	}

	if err := ctx.Validate(dto); err != nil {
		addFlash(ctx, validationMessages(&err), "error")
		return html(ctx, "auth_form.html")
	}

	if exists, err := db.UserExists(dto.Username); err != nil || exists {
		addFlash(ctx, "Username already exists", "error")
		return html(ctx, "auth_form.html")
	}

	user := dto.ToUser()

	password, err := argon2id.hash(user.Password)
	if err != nil {
		return errorRes(500, "Cannot hash password", err)
	}
	user.Password = password

	if err = user.Create(); err != nil {
		return errorRes(500, "Cannot create user", err)
	}

	if user.ID == 1 {
		if err = user.SetAdmin(); err != nil {
			return errorRes(500, "Cannot set user admin", err)
		}
	}

	sess.Values["user"] = user.ID
	saveSession(sess, ctx)

	return redirect(ctx, "/")
}

func login(ctx echo.Context) error {
	setData(ctx, "title", tr(ctx, "auth.login"))
	setData(ctx, "htmlTitle", "Login")
	setData(ctx, "disableForm", getData(ctx, "DisableLoginForm"))
	return html(ctx, "auth_form.html")
}

func processLogin(ctx echo.Context) error {
	if getData(ctx, "DisableLoginForm") == true {
		return errorRes(403, "Logging in via login form is disabled", nil)
	}

	var err error
	sess := getSession(ctx)

	dto := &db.UserDTO{}
	if err = ctx.Bind(dto); err != nil {
		return errorRes(400, "Cannot bind data", err)
	}
	password := dto.Password

	var user *db.User

	if user, err = db.GetUserByUsername(dto.Username); err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return errorRes(500, "Cannot get user", err)
		}
		log.Warn().Msg("Invalid HTTP authentication attempt from " + ctx.RealIP())
		addFlash(ctx, "Invalid credentials", "error")
		return redirect(ctx, "/login")
	}

	if ok, err := argon2id.verify(password, user.Password); !ok {
		if err != nil {
			return errorRes(500, "Cannot check for password", err)
		}
		log.Warn().Msg("Invalid HTTP authentication attempt from " + ctx.RealIP())
		addFlash(ctx, "Invalid credentials", "error")
		return redirect(ctx, "/login")
	}

	sess.Values["user"] = user.ID
	saveSession(sess, ctx)
	deleteCsrfCookie(ctx)

	return redirect(ctx, "/")
}

func oauthCallback(ctx echo.Context) error {
	user, err := gothic.CompleteUserAuth(ctx.Response(), ctx.Request())
	if err != nil {
		return errorRes(400, "Cannot complete user auth", err)
	}

	currUser := getUserLogged(ctx)
	if currUser != nil {
		// if user is logged in, link account to user and update its avatar URL
		switch user.Provider {
		case "github":
			currUser.GithubID = user.UserID
			currUser.AvatarURL = getAvatarUrlFromProvider("github", user.UserID)
		case "gitea":
			currUser.GiteaID = user.UserID
			currUser.AvatarURL = getAvatarUrlFromProvider("gitea", user.NickName)
		case "openid-connect":
			currUser.OIDCID = user.UserID
			currUser.AvatarURL = user.AvatarURL
		}

		if err = currUser.Update(); err != nil {
			return errorRes(500, "Cannot update user "+title.String(user.Provider)+" id", err)
		}

		addFlash(ctx, "Account linked to "+title.String(user.Provider), "success")
		return redirect(ctx, "/settings")
	}

	// if user is not in database, create it
	userDB, err := db.GetUserByProvider(user.UserID, user.Provider)
	if err != nil {
		if getData(ctx, "DisableSignup") == true {
			return errorRes(403, "Signing up is disabled", nil)
		}

		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return errorRes(500, "Cannot get user", err)
		}

		userDB = &db.User{
			Username: user.NickName,
			Email:    user.Email,
			MD5Hash:  fmt.Sprintf("%x", md5.Sum([]byte(strings.ToLower(strings.TrimSpace(user.Email))))),
		}

		// set provider id and avatar URL
		switch user.Provider {
		case "github":
			userDB.GithubID = user.UserID
			userDB.AvatarURL = getAvatarUrlFromProvider("github", user.UserID)
		case "gitea":
			userDB.GiteaID = user.UserID
			userDB.AvatarURL = getAvatarUrlFromProvider("gitea", user.NickName)
		case "openid-connect":
			userDB.OIDCID = user.UserID
			userDB.AvatarURL = user.AvatarURL
		}

		if err = userDB.Create(); err != nil {
			if db.IsUniqueConstraintViolation(err) {
				addFlash(ctx, "Username "+user.NickName+" already exists in Opengist", "error")
				return redirect(ctx, "/login")
			}

			return errorRes(500, "Cannot create user", err)
		}

		if userDB.ID == 1 {
			if err = userDB.SetAdmin(); err != nil {
				return errorRes(500, "Cannot set user admin", err)
			}
		}

		var resp *http.Response
		switch user.Provider {
		case "github":
			resp, err = http.Get("https://github.com/" + user.NickName + ".keys")
		case "gitea":
			resp, err = http.Get(urlJoin(config.C.GiteaUrl, user.NickName+".keys"))
		case "openid-connect":
			err = errors.New("cannot get keys from OIDC provider")
		}

		if err == nil {
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				addFlash(ctx, "Could not get user keys", "error")
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
					addFlash(ctx, "Could not create ssh key", "error")
					log.Error().Err(err).Msg("Could not create ssh key")
				}
			}
		}
	}

	sess := getSession(ctx)
	sess.Values["user"] = userDB.ID
	saveSession(sess, ctx)
	deleteCsrfCookie(ctx)

	return redirect(ctx, "/")
}

func oauth(ctx echo.Context) error {
	provider := ctx.Param("provider")

	httpProtocol := "http"
	if ctx.Request().TLS != nil || ctx.Request().Header.Get("X-Forwarded-Proto") == "https" {
		httpProtocol = "https"
	}

	var opengistUrl string
	if config.C.ExternalUrl != "" {
		opengistUrl = config.C.ExternalUrl
	} else {
		opengistUrl = httpProtocol + "://" + ctx.Request().Host
	}

	switch provider {
	case "github":
		goth.UseProviders(
			github.New(
				config.C.GithubClientKey,
				config.C.GithubSecret,
				urlJoin(opengistUrl, "/oauth/github/callback"),
			),
		)

	case "gitea":
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
	case "openid-connect":
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
			return errorRes(500, "Cannot create OIDC provider", err)
		}

		goth.UseProviders(oidcProvider)
	}

	currUser := getUserLogged(ctx)
	if currUser != nil {
		isDelete := false
		var err error
		switch provider {
		case "github":
			if currUser.GithubID != "" {
				isDelete = true
				err = currUser.DeleteProviderID(provider)
			}
		case "gitea":
			if currUser.GiteaID != "" {
				isDelete = true
				err = currUser.DeleteProviderID(provider)
			}
		case "openid-connect":
			if currUser.OIDCID != "" {
				isDelete = true
				err = currUser.DeleteProviderID(provider)
			}
		}

		if err != nil {
			return errorRes(500, "Cannot unlink account from "+title.String(provider), err)
		}

		if isDelete {
			addFlash(ctx, "Account unlinked from "+title.String(provider), "success")
			return redirect(ctx, "/settings")
		}
	}

	ctxValue := context.WithValue(ctx.Request().Context(), gothic.ProviderParamKey, provider)
	ctx.SetRequest(ctx.Request().WithContext(ctxValue))
	if provider != "github" && provider != "gitea" && provider != "openid-connect" {
		return errorRes(400, "Unsupported provider", nil)
	}

	gothic.BeginAuthHandler(ctx.Response(), ctx.Request())
	return nil
}

func logout(ctx echo.Context) error {
	deleteSession(ctx)
	deleteCsrfCookie(ctx)
	return redirect(ctx, "/all")
}

func urlJoin(base string, elem ...string) string {
	joined, err := url.JoinPath(base, elem...)
	if err != nil {
		log.Error().Err(err).Msg("Cannot join url")
	}

	return joined
}

func getAvatarUrlFromProvider(provider string, identifier string) string {
	switch provider {
	case "github":
		return "https://avatars.githubusercontent.com/u/" + identifier + "?v=4"
	case "gitea":
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
		err = json.Unmarshal(body, &result)
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
