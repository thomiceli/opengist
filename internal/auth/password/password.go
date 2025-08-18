package password

func HashPassword(code string) (string, error) {
	return Argon2id.Hash(code)
}

func VerifyPassword(code, hashedCode string) (bool, error) {
	return Argon2id.Verify(code, hashedCode)
}
