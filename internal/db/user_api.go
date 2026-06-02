package db

import (
	"time"

	"github.com/thomiceli/opengist/internal/web/handlers/api/v1/types"
)

// ToSimpleAPI returns the public simple-user shape used inside gist responses
// and by the public user-lookup endpoints. It carries no private fields (no
// email). Fields whose underlying feature doesn't exist in Opengist
// (followers, repos, ...) are still populated with the spec-shaped URLs so
// clients can parse cleanly.
func (u *User) ToSimpleAPI() types.SimpleUser {
	return types.SimpleUser{
		ID:        u.ID,
		Login:     u.Username,
		Username:  u.Username,
		AvatarURL: u.AvatarURL,
		Type:      "User",
		CreatedAt: time.Unix(u.CreatedAt, 0).UTC(),
	}
}

// ToPrivateAPI returns the self shape for the authenticated-user endpoints
// (GET/PATCH /user): the public fields plus the caller's own email.
func (u *User) ToPrivateAPI() types.PrivateUser {
	return types.PrivateUser{
		SimpleUser: u.ToSimpleAPI(),
		Email:      u.Email,
	}
}
