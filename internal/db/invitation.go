package db

import (
	"fmt"
	"math/rand"
	"time"
)

type Invitation struct {
	ID        uint `gorm:"primaryKey"`
	Code      string
	ExpiresAt int64
	NbUsed    uint
	NbMax     uint
}

func GetAllInvitations() ([]*Invitation, error) {
	var invitations []*Invitation
	dialect := db.Dialector.Name()
	query := db.Model(&Invitation{})

	switch dialect {
	case "sqlite":
		query = query.Order("(((expires_at >= strftime('%s', 'now')) AND ((nb_max <= 0) OR (nb_used < nb_max)))) DESC")
	case "postgres":
		query = query.Order("(((expires_at >= EXTRACT(EPOCH FROM CURRENT_TIMESTAMP)) AND ((nb_max <= 0) OR (nb_used < nb_max)))) DESC")
	case "mysql":
		query = query.Order("(((expires_at >= UNIX_TIMESTAMP()) AND ((nb_max <= 0) OR (nb_used < nb_max)))) DESC")
	default:
		return nil, fmt.Errorf("unsupported database dialect: %s", dialect)
	}

	err := query.Order("id ASC").Find(&invitations).Error

	return invitations, err
}

func GetInvitationByID(id uint) (*Invitation, error) {
	invitation := new(Invitation)
	err := db.
		Where("id = ?", id).
		First(&invitation).Error
	return invitation, err
}

func GetInvitationByCode(code string) (*Invitation, error) {
	invitation := new(Invitation)
	err := db.
		Where("code = ?", code).
		First(&invitation).Error
	return invitation, err
}

func InvitationCodeExists(code string) (bool, error) {
	var count int64
	err := db.Model(&Invitation{}).Where("code = ?", code).Count(&count).Error
	return count > 0, err
}

func (i *Invitation) Create() error {
	i.Code = generateRandomCode()
	return db.Create(&i).Error
}

func (i *Invitation) Update() error {
	return db.Save(&i).Error
}

func (i *Invitation) Delete() error {
	return db.Delete(&i).Error
}

func (i *Invitation) IsExpired() bool {
	return i.ExpiresAt < time.Now().Unix()
}

func (i *Invitation) IsMaxedOut() bool {
	return i.NbMax > 0 && i.NbUsed >= i.NbMax
}

func (i *Invitation) IsUsable() bool {
	return !i.IsExpired() && !i.IsMaxedOut()
}

func (i *Invitation) Use() error {
	i.NbUsed++
	return i.Update()
}

func generateRandomCode() string {
	const charset = "0123456789ABCDEF"
	var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))
	result := make([]byte, 16)

	for i := range result {
		result[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(result)
}
