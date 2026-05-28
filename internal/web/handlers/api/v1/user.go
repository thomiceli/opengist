package v1

import (
	"crypto/md5"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/web/context"
	"github.com/thomiceli/opengist/internal/web/handlers/api/v1/types"
)

// GetUser handles GET /api/v1/user.
// Returns the authenticated caller's own user record.
func GetUser(ctx *context.Context) error {
	return ctx.JSON(200, ctx.User.ToPrivateAPI())
}

// UpdateUser handles PATCH /api/v1/user.
// Updates the authenticated caller's username and/or email. Both fields are
// optional - only fields present in the body are touched. Returns the
// updated user on success (200), 422 on validation failures, 409 if the
// requested username is taken.
func UpdateUser(ctx *context.Context) error {
	var req types.UpdateUserRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.ErrorJson(422, "could not bind data", nil)
	}
	if req.Username == nil && req.Email == nil {
		return ctx.ErrorJson(422, "at least one of username or email must be set", nil)
	}

	user := ctx.User

	if req.Username != nil && !strings.EqualFold(*req.Username, user.Username) {
		// Same validator the web settings page uses (max=24, alphanumdash,
		// notreserved).
		dto := &db.UserUsernameDTO{Username: *req.Username}
		if err := ctx.Validate(dto); err != nil {
			return ctx.ErrorJson(422, err.Error(), nil)
		}

		exists, err := db.UserExists(dto.Username)
		if err != nil {
			return ctx.ErrorJson(500, "failed to check username uniqueness", err)
		}
		if exists {
			return ctx.ErrorJson(409, "username already taken", nil)
		}

		// Rename the user's repos directory on disk so subsequent git
		// operations resolve correctly under the new name.
		sourceDir := filepath.Join(config.GetHomeDir(), git.ReposDirectory, strings.ToLower(user.Username))
		destinationDir := filepath.Join(config.GetHomeDir(), git.ReposDirectory, strings.ToLower(dto.Username))
		if sourceDir != destinationDir {
			if _, err := os.Stat(sourceDir); !os.IsNotExist(err) {
				if err := os.Rename(sourceDir, destinationDir); err != nil {
					return ctx.ErrorJson(500, "failed to rename user directory", err)
				}
			}
		}
		user.Username = dto.Username
	}

	if req.Email != nil {
		email := strings.ToLower(strings.TrimSpace(*req.Email))
		user.Email = email
		// Gravatar key: hash of the lowercased email (or a random value when
		// no email is set, matching EmailProcess).
		if email == "" {
			user.MD5Hash = fmt.Sprintf("%x", md5.Sum([]byte(time.Now().String())))
		} else {
			user.MD5Hash = fmt.Sprintf("%x", md5.Sum([]byte(email)))
		}
	}

	if err := user.Update(); err != nil {
		return ctx.ErrorJson(500, "failed to update user", err)
	}
	return ctx.JSON(200, user.ToPrivateAPI())
}

// GetUserByID handles GET /api/v1/user/:id.
// Looks up a user by numeric ID and returns the SimpleUser shape (no
// private fields like email or admin flag). Anonymous-readable.
func GetUserByID(ctx *context.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return ctx.ErrorJson(400, "Invalid user id", nil)
	}
	u, err := db.GetUserById(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.ErrorJson(404, "User not found", nil)
		}
		return ctx.ErrorJson(500, "failed to look up user", err)
	}
	return ctx.JSON(200, u.ToSimpleAPI())
}

// GetUserByUsername handles GET /api/v1/users/:username.
// Looks up a user by username and returns the SimpleUser shape.
// Anonymous-readable.
func GetUserByUsername(ctx *context.Context) error {
	u, err := db.GetUserByUsername(ctx.Param("username"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.ErrorJson(404, "User not found", nil)
		}
		return ctx.ErrorJson(500, "failed to look up user", err)
	}
	return ctx.JSON(200, u.ToSimpleAPI())
}

// userVisibleAs resolves the `currentUserId` parameter the visibility
// OR-clauses on gist queries use. Returns target.ID only when the caller IS
// the target AND their token has gist:read; otherwise 0 (public-only). Same
// soft-scope policy as the /gists endpoint family.
func userVisibleAs(ctx *context.Context, target *db.User) uint {
	if ctx.User != nil && ctx.User.ID == target.ID {
		if tok, ok := ctx.GetData("accessToken").(*db.AccessToken); ok && tok.HasGistReadPermission() {
			return target.ID
		}
	}
	return 0
}

// ListUserLikedGists handles GET /api/v1/users/:username/liked.
// Lists gists liked by :username, filtered to what the caller is allowed
// to see. The target user's own private/unlisted liked gists only surface
// when the caller IS that user AND holds gist:read.
func ListUserLikedGists(ctx *context.Context) error {
	target, err := db.GetUserByUsername(ctx.Param("username"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.ErrorJson(404, "User not found", nil)
		}
		return ctx.ErrorJson(500, "failed to look up user", err)
	}
	visibleAs := userVisibleAs(ctx, target)
	return listGistsCommon(ctx, func(since *time.Time, offset, limit, perPage int, sort, order string) ([]*db.Gist, int64, error) {
		gists, err := db.GetAllGistsLikedByUser(target.ID, visibleAs, since, offset, sort, order, limit, perPage)
		if err != nil {
			return nil, 0, err
		}
		total, err := db.CountAllGistsLikedByUserSince(target.ID, visibleAs, since)
		return gists, total, err
	})
}

// ListUserForkedGists handles GET /api/v1/users/:username/forked.
// Lists gists forked by :username. Same caller-visibility rule as
// ListUserLikedGists.
func ListUserForkedGists(ctx *context.Context) error {
	target, err := db.GetUserByUsername(ctx.Param("username"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.ErrorJson(404, "User not found", nil)
		}
		return ctx.ErrorJson(500, "failed to look up user", err)
	}
	visibleAs := userVisibleAs(ctx, target)
	return listGistsCommon(ctx, func(since *time.Time, offset, limit, perPage int, sort, order string) ([]*db.Gist, int64, error) {
		gists, err := db.GetAllGistsForkedByUser(target.ID, visibleAs, since, offset, sort, order, limit, perPage)
		if err != nil {
			return nil, 0, err
		}
		total, err := db.CountAllGistsForkedByUserSince(target.ID, visibleAs, since)
		return gists, total, err
	})
}

// ListUserGists handles GET /api/v1/users/:username/gists.
// Returns the named user's gists with visibility filtering:
//
//   - Anonymous, or any caller other than the named user → only public
//     gists.
//   - Caller is the named user AND token has gist:read → all of their
//     gists (public + own private/unlisted).
//   - Caller is the named user but token lacks gist:read → public only,
//     matching the soft-scope rule used by /gists.
//
// Supports `page`, `per_page` (default 30, cap 100), and `since`
// (RFC 3339). Pagination via the Link header.
func ListUserGists(ctx *context.Context) error {
	username := ctx.Param("username")
	target, err := db.GetUserByUsername(username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.ErrorJson(404, "User not found", nil)
		}
		return ctx.ErrorJson(500, "failed to look up user", err)
	}

	visibleAs := userVisibleAs(ctx, target)
	return listGistsCommon(ctx, func(since *time.Time, offset, limit, perPage int, sort, order string) ([]*db.Gist, int64, error) {
		gists, err := db.GetAllGistsFromUserVisibleTo(target.ID, visibleAs, since, offset, sort, order, limit, perPage)
		if err != nil {
			return nil, 0, err
		}
		total, err := db.CountAllGistsFromUserVisibleTo(target.ID, visibleAs, since)
		return gists, total, err
	})
}
