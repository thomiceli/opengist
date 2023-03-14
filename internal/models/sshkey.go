package models

import "time"

type SSHKey struct {
	ID         uint   `gorm:"primaryKey"`
	Title      string `form:"title" validate:"required,max=50"`
	Content    string `form:"content" validate:"required"`
	SHA        string
	CreatedAt  int64
	LastUsedAt int64
	UserID     uint
	User       User `validate:"-" `
}

func GetSSHKeysByUserID(userId uint) ([]*SSHKey, error) {
	var sshKeys []*SSHKey
	err := db.
		Where("user_id = ?", userId).
		Order("created_at asc").
		Find(&sshKeys).Error

	return sshKeys, err
}

func GetSSHKeyByID(sshKeyId uint) (*SSHKey, error) {
	sshKey := new(SSHKey)
	err := db.
		Where("id = ?", sshKeyId).
		First(&sshKey).Error

	return sshKey, err
}

func GetSSHKeyByContent(sshKeyContent string) (*SSHKey, error) {
	sshKey := new(SSHKey)
	err := db.
		Where("content like ?", sshKeyContent+"%").
		First(&sshKey).Error

	return sshKey, err
}

func AddSSHKey(sshKey *SSHKey) error {
	return db.Create(&sshKey).Error
}

func RemoveSSHKey(sshKey *SSHKey) error {
	return db.Delete(&sshKey).Error
}

func SSHKeyLastUsedNow(sshKeyID uint) error {
	return db.Model(&SSHKey{}).
		Where("id = ?", sshKeyID).
		Update("last_used_at", time.Now().Unix()).Error
}
