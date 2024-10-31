package utils

import (
	"github.com/gorilla/securecookie"
	"github.com/rs/zerolog/log"
	"os"
)

// GenerateSecretKey generates a new secret key for sessions
// Returns the key and a boolean indicating if the key was generated
func GenerateSecretKey(filePath string) ([]byte, bool) {
	key, err := os.ReadFile(filePath)
	if err == nil {
		return key, false
	}

	key = securecookie.GenerateRandomKey(32)
	if key == nil {
		log.Fatal().Msg("Failed to generate a new key for sessions")
	}

	err = os.WriteFile(filePath, key, 0600)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to save the key to %s", filePath)
	}

	return key, true
}
