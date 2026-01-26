package db

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

const (
	NoPermission        = 0
	ReadPermission      = 1
	ReadWritePermission = 2
)

type AccessToken struct {
	ID         uint `gorm:"primaryKey"`
	Name       string
	TokenHash  string `gorm:"uniqueIndex"` // SHA-256 hash of the token
	CreatedAt  int64
	ExpiresAt  int64 // 0 means no expiration
	LastUsedAt int64
	UserID     uint
	User       User `validate:"-"`

	ScopeGist uint // 0 = none, 1 = read, 2 = read+write
}

// GenerateToken creates a new random token and returns the plain text token.
// The token hash is stored in the AccessToken struct.
// The plain text token should be shown to the user once and never stored.
func (t *AccessToken) GenerateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	plainToken := "og_" + hex.EncodeToString(bytes)

	hash := sha256.Sum256([]byte(plainToken))
	t.TokenHash = hex.EncodeToString(hash[:])

	return plainToken, nil
}

func GetAccessTokenByID(tokenID uint) (*AccessToken, error) {
	token := new(AccessToken)
	err := db.
		Where("id = ?", tokenID).
		First(&token).Error
	return token, err
}

func GetAccessTokenByToken(plainToken string) (*AccessToken, error) {
	hash := sha256.Sum256([]byte(plainToken))
	tokenHash := hex.EncodeToString(hash[:])

	token := new(AccessToken)
	err := db.
		Preload("User").
		Where("token_hash = ?", tokenHash).
		First(&token).Error
	return token, err
}

func GetAccessTokensByUserID(userID uint) ([]*AccessToken, error) {
	var tokens []*AccessToken
	err := db.
		Where("user_id = ?", userID).
		Order("created_at desc").
		Find(&tokens).Error
	return tokens, err
}

func (t *AccessToken) Create() error {
	t.CreatedAt = time.Now().Unix()
	return db.Create(t).Error
}

func (t *AccessToken) Delete() error {
	return db.Delete(t).Error
}

func (t *AccessToken) UpdateLastUsed() error {
	return db.Model(t).Update("last_used_at", time.Now().Unix()).Error
}

func (t *AccessToken) IsExpired() bool {
	if t.ExpiresAt == 0 {
		return false
	}
	return time.Now().Unix() > t.ExpiresAt
}

func (t *AccessToken) HasGistReadPermission() bool {
	return t.ScopeGist >= ReadPermission
}

func (t *AccessToken) HasGistWritePermission() bool {
	return t.ScopeGist >= ReadWritePermission
}

// -- DTO -- //

type AccessTokenDTO struct {
	Name      string `form:"name" validate:"required,max=255"`
	ScopeGist uint   `form:"scope_gist" validate:"min=0,max=2"`
	ExpiresAt string `form:"expires_at"` // empty means no expiration, otherwise date format (YYYY-MM-DD)
}

func (dto *AccessTokenDTO) ToAccessToken() *AccessToken {
	var expiresAt int64
	if dto.ExpiresAt != "" {
		// date input format: 2006-01-02, expires at end of day
		if t, err := time.ParseInLocation("2006-01-02", dto.ExpiresAt, time.Local); err == nil {
			expiresAt = t.Add(24*time.Hour - time.Second).Unix()
		}
	}

	return &AccessToken{
		Name:      dto.Name,
		ScopeGist: dto.ScopeGist,
		ExpiresAt: expiresAt,
	}
}
