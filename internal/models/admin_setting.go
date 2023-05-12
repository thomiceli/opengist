package models

import (
	"gorm.io/gorm/clause"
)

type AdminSetting struct {
	Key   string `gorm:"uniqueIndex"`
	Value string
}

const (
	SettingDisableSignup    = "disable-signup"
	SettingRequireLogin     = "require-login"
	SettingDisableLoginForm = "disable-login-form"
)

func GetSetting(key string) (string, error) {
	var setting AdminSetting
	err := db.Where("key = ?", key).First(&setting).Error
	return setting.Value, err
}

func GetSettings() (map[string]string, error) {
	var settings []AdminSetting
	err := db.Find(&settings).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, setting := range settings {
		result[setting.Key] = setting.Value
	}

	return result, nil
}

func UpdateSetting(key string, value string) error {
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}}, // key column
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&AdminSetting{
		Key:   key,
		Value: value,
	}).Error
}

func setSetting(key string, value string) error {
	return db.Create(&AdminSetting{Key: key, Value: value}).Error
}

func initAdminSettings(settings map[string]string) error {
	for key, value := range settings {
		if err := setSetting(key, value); err != nil {
			if !IsUniqueConstraintViolation(err) {
				return err
			}
		}
	}

	return nil
}
