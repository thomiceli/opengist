package models

import (
	"errors"
	"github.com/mattn/go-sqlite3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

func Setup(dbpath string) error {
	var err error

	if db, err = gorm.Open(sqlite.Open(dbpath+"?_fk=true"), &gorm.Config{
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
	})
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
