package utils

import (
	"github.com/gorilla/securecookie"
	"github.com/rs/zerolog/log"
	"os"
)

func ReadKey(filePath string) []byte {
	key, err := os.ReadFile(filePath)
	if err == nil {
		return key
	}

	key = securecookie.GenerateRandomKey(32)
	if key == nil {
		log.Fatal().Msg("Failed to generate a new key for sessions")
	}

	err = os.WriteFile(filePath, key, 0600)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to save the key to %s", filePath)
	}

	return key
}
