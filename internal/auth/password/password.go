package password

import "github.com/thomiceli/opengist/internal/auth"

func HashPassword(code string) (string, error) {
	return auth.Argon2id.Hash(code)
}

func VerifyPassword(code, hashedCode string) (bool, error) {
	return auth.Argon2id.Verify(code, hashedCode)
}
