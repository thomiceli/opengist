package web

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/gitea"
	"github.com/markbates/goth/providers/github"
	"github.com/rs/zerolog/log"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gorm.io/gorm"
	"io"
	"net/http"
	"opengist/internal/config"
	"opengist/internal/models"
	"strings"
)

var title = cases.Title(language.English)

func register(ctx echo.Context) error {
	setData(ctx, "title", "New account")
	setData(ctx, "htmlTitle", "New account")
	return html(ctx, "auth_form.html")
}

func processRegister(ctx echo.Context) error {
	if getData(ctx, "DisableSignup") == true {
		return errorRes(403, "Signing up is disabled", nil)
	}

	setData(ctx, "title", "New account")
	setData(ctx, "htmlTitle", "New account")

	sess := getSession(ctx)

	var dto = new(models.UserDTO)
	if err := ctx.Bind(dto); err != nil {
		return errorRes(400, "Cannot bind data", err)
	}

	if err := ctx.Validate(dto); err != nil {
		addFlash(ctx, validationMessages(&err), "error")
		return html(ctx, "auth_form.html")
	}

	if exists, err := models.UserExists(dto.Username); err != nil || exists {
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
	setData(ctx, "title", "Login")
	setData(ctx, "htmlTitle", "Login")
	return html(ctx, "auth_form.html")
}

func processLogin(ctx echo.Context) error {
	var err error
	sess := getSession(ctx)

	dto := &models.UserDTO{}
	if err = ctx.Bind(dto); err != nil {
		return errorRes(400, "Cannot bind data", err)
	}
	password := dto.Password

	var user *models.User

	if user, err = models.GetUserByUsername(dto.Username); err != nil {
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
		// if user is logged in, link account to user
		switch user.Provider {
		case "github":
			currUser.GithubID = user.UserID
		case "gitea":
			currUser.GiteaID = user.UserID
		}

		if err = currUser.Update(); err != nil {
			return errorRes(500, "Cannot update user "+title.String(user.Provider)+" id", err)
		}

		addFlash(ctx, "Account linked to "+title.String(user.Provider), "success")
		return redirect(ctx, "/settings")
	}

	// if user is not in database, create it
	userDB, err := models.GetUserByProvider(user.UserID, user.Provider)
	if err != nil {
		if getData(ctx, "DisableSignup") == true {
			return errorRes(403, "Signing up is disabled", nil)
		}

		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return errorRes(500, "Cannot get user", err)
		}

		userDB = &models.User{
			Username: user.NickName,
			Email:    user.Email,
			MD5Hash:  fmt.Sprintf("%x", md5.Sum([]byte(strings.ToLower(strings.TrimSpace(user.Email))))),
		}

		switch user.Provider {
		case "github":
			userDB.GithubID = user.UserID
		case "gitea":
			userDB.GiteaID = user.UserID
		}

		if err = userDB.Create(); err != nil {
			if models.IsUniqueConstraintViolation(err) {
				addFlash(ctx, "Username "+user.NickName+" already exists in Opengist", "error")
				return redirect(ctx, "/login")
			}

			return errorRes(500, "Cannot create user", err)
		}

		var resp *http.Response
		switch user.Provider {
		case "github":
			resp, err = http.Get("https://github.com/" + user.NickName + ".keys")
		case "gitea":
			resp, err = http.Get(trimGiteaUrl() + "/" + user.NickName + ".keys")
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
				sshKey := models.SSHKey{
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

	giteaUrl := trimGiteaUrl()

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
				opengistUrl+"/oauth/github/callback"),
		)

	case "gitea":
		goth.UseProviders(
			gitea.NewCustomisedURL(
				config.C.GiteaClientKey,
				config.C.GiteaSecret,
				opengistUrl+"/oauth/gitea/callback",
				giteaUrl+"/login/oauth/authorize",
				giteaUrl+"/login/oauth/access_token",
				giteaUrl+"/api/v1/user"),
		)
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
	if provider != "github" && provider != "gitea" {
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

func trimGiteaUrl() string {
	giteaUrl := config.C.GiteaUrl
	// remove trailing slash
	if giteaUrl[len(giteaUrl)-1] == '/' {
		giteaUrl = giteaUrl[:len(giteaUrl)-1]
	}

	return giteaUrl
}
