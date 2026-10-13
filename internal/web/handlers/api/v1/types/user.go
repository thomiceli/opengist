package types

import "time"

// SimpleUser is the public user shape used as the `owner` field on gist
// responses and by the public user-lookup endpoints. It carries no private
// fields (no email). Fields whose underlying feature doesn't exist in Opengist
// (followers, repos, etc.) are still populated with the spec-shaped URLs so
// clients parse cleanly.
type SimpleUser struct {
	ID        uint      `json:"id"`
	Login     string    `json:"login"`
	Username  string    `json:"username"`
	AvatarURL string    `json:"avatar_url"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
}

// PrivateUser is the self shape returned only by the authenticated-user
// endpoints (GET/PATCH /user). It extends SimpleUser with the caller's own
// email, which is never exposed on public endpoints.
type PrivateUser struct {
	SimpleUser
	Email string `json:"email"`
}
