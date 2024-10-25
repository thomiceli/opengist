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
		{"1.8", v1_8_0},
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
	oldPath := filepath.Join(GetHomeDir(), "sessions", "session-auth.key")
	newPath := filepath.Join(GetHomeDir(), "opengist-secret.key")
	err := os.Rename(oldPath, newPath)
	if err == nil {
		fmt.Printf("Automatically moved %s to %s\n", oldPath, newPath)
	}

	oldPath = filepath.Join(GetHomeDir(), "sessions", "session-encrypt.key")
	newPath = filepath.Join(GetHomeDir(), "session-encrypt.key")
	err = os.Rename(oldPath, newPath)
	if err == nil {
		fmt.Printf("Automatically moved %s to %s\n", oldPath, newPath)
	}

	return nil
}
