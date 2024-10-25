package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// auto migration for newer versions of Opengist
func migrateConfig() error {
	configMigrations := []struct {
		Version string
		Func    func() error
	}{
		{"1.8.0", v1_8_0},
	}

	for _, fn := range configMigrations {
		err := fn.Func()
		if err != nil {
			return err
		}
	}

	return nil
}

func v1_8_0() error {
	homeDir := GetHomeDir()
	sessionsDir := filepath.Join(homeDir, "sessions")

	moves := []struct {
		oldName string
		newName string
	}{
		{
			oldName: filepath.Join(sessionsDir, "session-auth.key"),
			newName: filepath.Join(homeDir, "opengist-secret.key"),
		},
		{
			oldName: filepath.Join(sessionsDir, "session-encrypt.key"),
			newName: filepath.Join(homeDir, "session-encrypt.key"),
		},
	}

	for _, move := range moves {
		moveFile(move.oldName, move.newName)
	}

	return nil
}

func moveFile(oldPath, newPath string) {
	if _, err := os.Stat(oldPath); err != nil {
		return
	}

	if err := os.Rename(oldPath, newPath); err == nil {
		fmt.Printf("Automatically moved %s to %s\n", oldPath, newPath)
	}
}
