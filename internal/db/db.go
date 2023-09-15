package db

import (
	"errors"
	"github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/utils"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"strings"
)

var db *gorm.DB

func Setup(dbPath string, sharedCache bool) error {
	var err error
	journalMode := strings.ToUpper(config.C.SqliteJournalMode)

	if !utils.SliceContains([]string{"DELETE", "TRUNCATE", "PERSIST", "MEMORY", "WAL", "OFF"}, journalMode) {
		log.Warn().Msg("Invalid SQLite journal mode: " + journalMode)
	}

	sharedCacheStr := ""
	if sharedCache {
		sharedCacheStr = "&cache=shared"
	}

	if db, err = gorm.Open(sqlite.Open(dbPath+"?_fk=true&_journal_mode="+journalMode+sharedCacheStr), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}); err != nil {
		return err
	}

	if err = db.SetupJoinTable(&Gist{}, "Likes", &Like{}); err != nil {
		return err
	}

	if err = db.SetupJoinTable(&User{}, "Liked", &Like{}); err != nil {
		return err
	}

	if err = db.AutoMigrate(&User{}, &Gist{}, &SSHKey{}, &AdminSetting{}); err != nil {
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

func Close() error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func CountAll(table interface{}) (int64, error) {
	var count int64
	err := db.Model(table).Count(&count).Error
	return count, err
}

func IsUniqueConstraintViolation(err error) bool {
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
		return true
	}
	return false
}
