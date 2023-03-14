package web

import (
	"github.com/labstack/echo/v4"
	"opengist/internal/config"
	"opengist/internal/models"
)

func register(ctx echo.Context) error {
	setData(ctx, "title", "New account")
	setData(ctx, "htmlTitle", "New account")
	return html(ctx, "auth_form.html")
}

func processRegister(ctx echo.Context) error {
	if config.C.DisableSignup {
		return errorRes(403, "Signing up is disabled", nil)
	}

	setData(ctx, "title", "New account")
	setData(ctx, "htmlTitle", "New account")

	sess := getSession(ctx)

	var user = new(models.User)
	if err := ctx.Bind(user); err != nil {
		return errorRes(400, "Cannot bind data", err)
	}

	if err := ctx.Validate(user); err != nil {
		addFlash(ctx, validationMessages(&err), "error")
		return html(ctx, "auth_form.html")
	}

	password, err := argon2id.hash(user.Password)
	if err != nil {
		return errorRes(500, "Cannot hash password", err)
	}
	user.Password = password

	var count int64
	if err = models.DoesUserExists(user.Username, &count); err != nil || count >= 1 {
		addFlash(ctx, "Username already exists", "error")
		return html(ctx, "auth_form.html")
	}

	if err = models.CreateUser(user); err != nil {
		return errorRes(500, "Cannot create user", err)
	}

	if user.ID == 1 {
		user.IsAdmin = true
		if err = models.SetAdminUser(user); err != nil {
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
	sess := getSession(ctx)

	user := &models.User{}
	if err := ctx.Bind(user); err != nil {
		return errorRes(400, "Cannot bind data", err)
	}
	password := user.Password

	if err := models.GetLoginUser(user); err != nil {
		addFlash(ctx, "Invalid credentials", "error")
		return redirect(ctx, "/login")
	}

	if ok, err := argon2id.verify(password, user.Password); !ok {
		if err != nil {
			return errorRes(500, "Cannot check for password", err)
		}
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
