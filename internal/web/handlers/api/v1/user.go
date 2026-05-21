package v1

import (
	"time"

	"github.com/thomiceli/opengist/internal/web/context"
)

// GetUser handles GET /api/v1/user
func GetUser(ctx *context.Context) error {
	u := ctx.User
	resp := UserResponse{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		IsAdmin:   u.IsAdmin,
		CreatedAt: time.Unix(u.CreatedAt, 0).UTC(),
	}
	return ctx.JSON(200, resp)
}
