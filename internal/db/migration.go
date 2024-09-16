package db

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

type MigrationVersion struct {
	ID      uint `gorm:"primaryKey"`
	Version uint
}

func applyMigrations(db *gorm.DB, dbInfo *databaseInfo) error {
	switch dbInfo.Type {
	case SQLite:
		return applySqliteMigrations(db)
	case PostgreSQL, MySQL:
		return nil
	default:
		return fmt.Errorf("unknown database type: %s", dbInfo.Type)
	}

}

func applySqliteMigrations(db *gorm.DB) error {
	// Create migration table if it doesn't exist
	if err := db.AutoMigrate(&MigrationVersion{}); err != nil {
		log.Fatal().Err(err).Msg("Error creating migration version table")
		return err
	}

	// Get the current migration version
	var currentVersion MigrationVersion
	db.First(&currentVersion)

	// Define migrations
	migrations := []struct {
		Version uint
		Func    func(*gorm.DB) error
	}{
		{1, v1_modifyConstraintToSSHKeys},
		{2, v2_lowercaseEmails},
		// Add more migrations here as needed
	}

	// Apply migrations
	for _, m := range migrations {
		if m.Version > currentVersion.Version {
			tx := db.Begin()
			if err := tx.Error; err != nil {
				log.Fatal().Err(err).Msg("Error starting transaction")
				return err
			}

			if err := m.Func(db); err != nil {
				log.Fatal().Err(err).Msg(fmt.Sprintf("Error applying migration %d:", m.Version))
				tx.Rollback()
				return err
			} else {
				if err = tx.Commit().Error; err != nil {
					log.Fatal().Err(err).Msg(fmt.Sprintf("Error committing migration %d:", m.Version))
					return err
				}
				currentVersion.Version = m.Version
				db.Save(&currentVersion)
				log.Info().Msg(fmt.Sprintf("Migration %d applied successfully", m.Version))
			}
		}
	}

	return nil
}

// Modify the constraint on the ssh_keys table to use ON DELETE CASCADE
func v1_modifyConstraintToSSHKeys(db *gorm.DB) error {
	createSQL := `
	CREATE TABLE ssh_keys_temp (
		id integer primary key,
		title text,
		content text,
		sha text,
		created_at integer,
		last_used_at integer,
		user_id integer
		constraint fk_users_ssh_keys references users(id) on update cascade on delete cascade
	);
	`

	if err := db.Exec(createSQL).Error; err != nil {
		return err
	}

	// Copy data from the old table to the new table
	copySQL := `INSERT INTO ssh_keys_temp SELECT * FROM ssh_keys;`
	if err := db.Exec(copySQL).Error; err != nil {
		return err
	}

	// Drop the old table
	dropSQL := `DROP TABLE ssh_keys;`
	if err := db.Exec(dropSQL).Error; err != nil {
		return err
	}

	// Rename the new table to the original table name
	renameSQL := `ALTER TABLE ssh_keys_temp RENAME TO ssh_keys;`
	return db.Exec(renameSQL).Error
}

func v2_lowercaseEmails(db *gorm.DB) error {
	// Copy the lowercase emails into the new column
	copySQL := `UPDATE users SET email = lower(email);`
	return db.Exec(copySQL).Error
}
