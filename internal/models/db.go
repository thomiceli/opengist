package models

import (
	"errors"
	"strings"

	"github.com/glebarez/sqlite"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	msqlite "modernc.org/sqlite"
)

var db *gorm.DB

func Setup(dbPath string) error {
	var err error
	journalMode := strings.ToUpper(config.C.SqliteJournalMode)

	if !utils.SliceContains([]string{"DELETE", "TRUNCATE", "PERSIST", "MEMORY", "WAL", "OFF"}, journalMode) {
		log.Warn().Msg("Invalid SQLite journal mode: " + journalMode)
	}

	if db, err = gorm.Open(sqlite.Open(dbPath+"?_fk=true&_journal_mode="+journalMode), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}); err != nil {
		return err
	}

	if err = db.AutoMigrate(&User{}, &SSHKey{}, &Gist{}, &AdminSetting{}); err != nil {
		return err
	}

	ApplyMigrations(db)

	// Default admin setting values
	return initAdminSettings(map[string]string{
		SettingDisableSignup:    "0",
		SettingRequireLogin:     "0",
		SettingDisableLoginForm: "0",
		SettingDisableGravatar:  "0",
	})
}

func CountAll(table interface{}) (int64, error) {
	var count int64
	err := db.Model(table).Count(&count).Error
	return count, err
}

func IsUniqueConstraintViolation(err error) bool {
	var sqliteErr *msqlite.Error
	if errors.As(err, &sqliteErr) && sqliteErr.Code() == 2067 {
		return true
	}
	return false
}
