package models

import (
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

	if err = db.AutoMigrate(&User{}, &SSHKey{}, &Gist{}); err != nil {
		return err
	}

	return nil
}

func CountAll(table interface{}) (int64, error) {
	var count int64
	err := db.Model(table).Count(&count).Error
	return count, err
}
