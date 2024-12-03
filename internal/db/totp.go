package db

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/thomiceli/opengist/internal/auth"
	"github.com/thomiceli/opengist/internal/auth/password"
	ogtotp "github.com/thomiceli/opengist/internal/auth/totp"
	"github.com/thomiceli/opengist/internal/config"
	"slices"
)

type TOTP struct {
	ID            uint `gorm:"primaryKey"`
	UserID        uint `gorm:"uniqueIndex"`
	User          User
	Secret        string
	RecoveryCodes jsonData `gorm:"type:json"`
	CreatedAt     int64
	LastUsedAt    int64
}

func GetTOTPByUserID(userID uint) (*TOTP, error) {
	var totp TOTP
	err := db.Where("user_id = ?", userID).First(&totp).Error
	return &totp, err
}

func (totp *TOTP) StoreSecret(secret string) error {
	secretBytes := []byte(secret)
	encrypted, err := auth.AESEncrypt(config.SecretKey, secretBytes)
	if err != nil {
		return err
	}

	totp.Secret = base64.URLEncoding.EncodeToString(encrypted)
	return nil
}

func (totp *TOTP) ValidateCode(code string) (bool, error) {
	ciphertext, err := base64.URLEncoding.DecodeString(totp.Secret)
	if err != nil {
		return false, err
	}

	secretBytes, err := auth.AESDecrypt(config.SecretKey, ciphertext)
	if err != nil {
		return false, err
	}

	return ogtotp.Validate(code, string(secretBytes)), nil
}

func (totp *TOTP) ValidateRecoveryCode(code string) (bool, error) {
	var hashedCodes []string
	if err := json.Unmarshal(totp.RecoveryCodes, &hashedCodes); err != nil {
		return false, err
	}

	for i, hashedCode := range hashedCodes {
		ok, err := password.VerifyPassword(code, hashedCode)
		if err != nil {
			return false, err
		}
		if ok {
			codesJson, _ := json.Marshal(slices.Delete(hashedCodes, i, i+1))
			totp.RecoveryCodes = codesJson
			return true, db.Model(&totp).Updates(TOTP{RecoveryCodes: codesJson}).Error
		}
	}
	return false, nil
}

func (totp *TOTP) GenerateRecoveryCodes() ([]string, error) {
	codes, plainCodes, err := generateRandomCodes()
	if err != nil {
		return nil, err
	}

	codesJson, _ := json.Marshal(codes)
	totp.RecoveryCodes = codesJson

	return plainCodes, db.Model(&totp).Updates(TOTP{RecoveryCodes: codesJson}).Error
}

func (totp *TOTP) Create() error {
	return db.Create(&totp).Error
}

func (totp *TOTP) Delete() error {
	return db.Delete(&totp).Error
}

func generateRandomCodes() ([]string, []string, error) {
	const count = 5
	const length = 10
	codes := make([]string, count)
	plainCodes := make([]string, count)
	for i := 0; i < count; i++ {
		bytes := make([]byte, (length+1)/2)
		if _, err := rand.Read(bytes); err != nil {
			return nil, nil, err
		}
		hexCode := hex.EncodeToString(bytes)
		code := fmt.Sprintf("%s-%s", hexCode[:length/2], hexCode[length/2:])
		plainCodes[i] = code
		hashed, err := password.HashPassword(code)
		if err != nil {
			return nil, nil, err
		}
		codes[i] = hashed
	}
	return codes, plainCodes, nil
}

// -- DTO -- //

type TOTPDTO struct {
	Code string `form:"code" validate:"max=50"`
}
