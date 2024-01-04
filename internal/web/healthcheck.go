package web

import (
	"github.com/labstack/echo/v4"
	"github.com/thomiceli/opengist/internal/db"
	"time"
)

func healthcheck(ctx echo.Context) error {
	// Check database connection
	dbOk := "ok"
	httpStatus := 200

	err := db.Ping()
	if err != nil {
		dbOk = "ko"
		httpStatus = 503
	}

	return ctx.JSON(httpStatus, map[string]interface{}{
		"opengist": "ok",
		"database": dbOk,
		"time":     time.Now().Format(time.RFC3339),
	})
}
