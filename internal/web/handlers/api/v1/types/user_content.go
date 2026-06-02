package types

// UpdateUserRequest is the PATCH /api/v1/user body. Pointer fields let the
// handler distinguish "absent" from "explicit empty" so partial updates
// don't accidentally clear other fields.
//
//   - username - change the caller's username. Goes through the same
//     validator the web settings page uses (max 24 chars, alphanumeric +
//     dashes, not a reserved word).
//   - email    - change the caller's email. Lowercased server-side and the
//     gravatar MD5 hash is recomputed.
type UpdateUserRequest struct {
	Username *string `json:"username,omitempty"`
	Email    *string `json:"email,omitempty"`
}
