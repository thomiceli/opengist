package db

import (
	"crypto/sha256"
	"encoding/base64"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
	"time"
)

type SSHKey struct {
	ID         uint `gorm:"primaryKey"`
	Title      string
	Content    string
	SHA        string
	CreatedAt  int64
	LastUsedAt int64
	UserID     uint
	User       User `validate:"-" `
}

func (sshKey *SSHKey) BeforeCreate(*gorm.DB) error {
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(sshKey.Content))
	if err != nil {
		return err
	}
	sha := sha256.Sum256(pubKey.Marshal())
	sshKey.SHA = base64.StdEncoding.EncodeToString(sha[:])
	return nil
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

func SSHKeyDoesExists(sshKeyContent string) (bool, error) {
	var count int64
	err := db.Model(&SSHKey{}).
		Where("content = ?", sshKeyContent).
		Count(&count).Error
	return count > 0, err
}

func (sshKey *SSHKey) Create() error {
	return db.Create(&sshKey).Error
}

func (sshKey *SSHKey) Delete() error {
	return db.Delete(&sshKey).Error
}

func SSHKeyLastUsedNow(sshKeyContent string) error {
	return db.Model(&SSHKey{}).
		Where("content = ?", sshKeyContent).
		Update("last_used_at", time.Now().Unix()).Error
}

// -- DTO -- //

type SSHKeyDTO struct {
	Title   string `form:"title" validate:"required,max=50"`
	Content string `form:"content" validate:"required"`
}

func (dto *SSHKeyDTO) ToSSHKey() *SSHKey {
	return &SSHKey{
		Title:   dto.Title,
		Content: dto.Content,
	}
}
