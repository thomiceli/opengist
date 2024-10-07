package db

import (
	"encoding/hex"
	"github.com/go-webauthn/webauthn/webauthn"
	"time"
)

type WebAuthnCredential struct {
	ID                 uint `gorm:"primaryKey"`
	Name               string
	UserID             uint
	User               User
	CredentialID       binaryData `gorm:"type:binary_data"`
	PublicKey          binaryData `gorm:"type:binary_data"`
	AttestationType    string
	AAGUID             binaryData `gorm:"type:binary_data"`
	SignCount          uint32
	CloneWarning       bool
	FlagUserPresent    bool
	FlagUserVerified   bool
	FlagBackupEligible bool
	FlagBackupState    bool
	CreatedAt          int64
	LastUsedAt         int64
}

func (*WebAuthnCredential) TableName() string {
	return "webauthn"
}

func GetAllWACredentialsForUser(userID uint) ([]webauthn.Credential, error) {
	var creds []WebAuthnCredential
	err := db.Where("user_id = ?", userID).Find(&creds).Error
	if err != nil {
		return nil, err
	}
	webCreds := make([]webauthn.Credential, len(creds))
	for i, cred := range creds {
		webCreds[i] = webauthn.Credential{
			ID:              cred.CredentialID,
			PublicKey:       cred.PublicKey,
			AttestationType: cred.AttestationType,
			Authenticator: webauthn.Authenticator{
				AAGUID:       cred.AAGUID,
				SignCount:    cred.SignCount,
				CloneWarning: cred.CloneWarning,
			},
			Flags: webauthn.CredentialFlags{
				UserPresent:    cred.FlagUserPresent,
				UserVerified:   cred.FlagUserVerified,
				BackupEligible: cred.FlagBackupEligible,
				BackupState:    cred.FlagBackupState,
			},
		}
	}
	return webCreds, nil
}

func GetAllCredentialsForUser(userID uint) ([]WebAuthnCredential, error) {
	var creds []WebAuthnCredential
	err := db.Where("user_id = ?", userID).Find(&creds).Error
	return creds, err
}

func GetUserByCredentialID(credID binaryData) (*User, error) {
	var credential WebAuthnCredential
	var err error

	switch db.Dialector.Name() {
	case "postgres":
		hexCredID := hex.EncodeToString(credID)
		if err = db.Preload("User").Where("credential_id = decode(?, 'hex')", hexCredID).First(&credential).Error; err != nil {
			return nil, err
		}
	case "mysql":
	case "sqlite":
		hexCredID := hex.EncodeToString(credID)
		if err = db.Preload("User").Where("credential_id = unhex(?)", hexCredID).First(&credential).Error; err != nil {
			return nil, err
		}
	}

	return &credential.User, err
}

func GetCredentialByIDDB(id uint) (*WebAuthnCredential, error) {
	var cred WebAuthnCredential
	err := db.Where("id = ?", id).First(&cred).Error
	return &cred, err
}

func GetCredentialByID(id binaryData) (*WebAuthnCredential, error) {
	var cred WebAuthnCredential
	var err error

	switch db.Dialector.Name() {
	case "postgres":
		hexCredID := hex.EncodeToString(id)
		if err = db.Where("credential_id = decode(?, 'hex')", hexCredID).First(&cred).Error; err != nil {
			return nil, err
		}
	case "mysql":
	case "sqlite":
		hexCredID := hex.EncodeToString(id)
		if err = db.Where("credential_id = unhex(?)", hexCredID).First(&cred).Error; err != nil {
			return nil, err
		}
	}

	return &cred, err
}

func CreateFromCrendential(userID uint, name string, cred *webauthn.Credential) (*WebAuthnCredential, error) {
	credDb := &WebAuthnCredential{
		UserID:             userID,
		Name:               name,
		CredentialID:       cred.ID,
		PublicKey:          cred.PublicKey,
		AttestationType:    cred.AttestationType,
		AAGUID:             cred.Authenticator.AAGUID,
		SignCount:          cred.Authenticator.SignCount,
		CloneWarning:       cred.Authenticator.CloneWarning,
		FlagUserPresent:    cred.Flags.UserPresent,
		FlagUserVerified:   cred.Flags.UserVerified,
		FlagBackupEligible: cred.Flags.BackupEligible,
		FlagBackupState:    cred.Flags.BackupState,
	}
	err := db.Create(credDb).Error
	return credDb, err
}

func (w *WebAuthnCredential) UpdateSignCount() error {
	return db.Model(w).Update("sign_count", w.SignCount).Error
}

func (w *WebAuthnCredential) UpdateLastUsedAt() error {
	return db.Model(w).Update("last_used_at", time.Now().Unix()).Error
}

func (w *WebAuthnCredential) Delete() error {
	return db.Delete(w).Error
}

// -- DTO -- //

type CrendentialDTO struct {
	PasskeyName string `json:"passkeyname" validate:"max=50"`
}
