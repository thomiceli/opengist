package web

import (
	"errors"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
	"opengist/internal/models"
)

func register(ctx echo.Context) error {
	setData(ctx, "title", "New account")
	setData(ctx, "htmlTitle", "New account")
	return html(ctx, "auth_form.html")
}

func processRegister(ctx echo.Context) error {
	if getData(ctx, "signupDisabled") == true {
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

func logout(ctx echo.Context) error {
	deleteSession(ctx)
	deleteCsrfCookie(ctx)
	return redirect(ctx, "/all")
}
