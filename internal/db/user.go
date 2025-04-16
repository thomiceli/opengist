package db

import (
	"encoding/json"
	"github.com/thomiceli/opengist/internal/git"
	"gorm.io/gorm"
)

type User struct {
	ID               uint   `gorm:"primaryKey"`
	Username         string `gorm:"uniqueIndex,size:191"`
	Password         string
	IsAdmin          bool
	CreatedAt        int64
	Email            string
	MD5Hash          string // for gravatar, if no Email is specified, the value is random
	AvatarURL        string
	GithubID         string
	GitlabID         string
	GiteaID          string
	OIDCID           string `gorm:"column:oidc_id"`
	StylePreferences string

	Gists               []Gist               `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:UserID"`
	SSHKeys             []SSHKey             `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:UserID"`
	Liked               []Gist               `gorm:"many2many:likes;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	WebAuthnCredentials []WebAuthnCredential `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:UserID"`
}

func (user *User) BeforeDelete(tx *gorm.DB) error {
	// Decrement likes counter using derived table
	err := tx.Exec(`
		UPDATE gists 
		SET nb_likes = nb_likes - 1
		WHERE id IN (
			SELECT gist_id 
			FROM (
				SELECT gist_id 
				FROM likes 
				WHERE user_id = ?
			) AS derived_likes
		)
	`, user.ID).Error
	if err != nil {
		return err
	}

	// Decrement forks counter using derived table
	err = tx.Exec(`
		UPDATE gists 
		SET nb_forks = nb_forks - 1
		WHERE id IN (
			SELECT forked_id 
			FROM (
				SELECT forked_id 
				FROM gists 
				WHERE user_id = ? AND forked_id IS NOT NULL
			) AS derived_forks
		)
	`, user.ID).Error
	if err != nil {
		return err
	}

	err = tx.Where("user_id = ?", user.ID).Delete(&SSHKey{}).Error
	if err != nil {
		return err
	}

	err = tx.Where("user_id = ?", user.ID).Delete(&WebAuthnCredential{}).Error
	if err != nil {
		return err
	}

	err = tx.Where("user_id = ?", user.ID).Delete(&Gist{}).Error
	if err != nil {
		return err
	}

	// Delete user directory
	if err = git.DeleteUserDirectory(user.Username); err != nil {
		return err
	}

	return nil
}

func UserExists(username string) (bool, error) {
	var count int64
	err := db.Model(&User{}).Where("username like ?", username).Count(&count).Error
	return count > 0, err
}

func GetAllUsers(offset int) ([]*User, error) {
	var users []*User
	err := db.
		Limit(11).
		Offset(offset * 10).
		Order("id asc").
		Find(&users).Error

	return users, err
}

func GetUserByUsername(username string) (*User, error) {
	user := new(User)
	err := db.
		Where("username like ?", username).
		First(&user).Error
	return user, err
}

func GetUserById(userId uint) (*User, error) {
	user := new(User)
	err := db.
		Where("id = ?", userId).
		First(&user).Error
	return user, err
}

func GetUsersFromEmails(emailsSet map[string]struct{}) (map[string]*User, error) {
	var users []*User

	emails := make([]string, 0, len(emailsSet))
	for email := range emailsSet {
		emails = append(emails, email)
	}

	err := db.
		Where("email IN ?", emails).
		Find(&users).Error
	if err != nil {
		return nil, err
	}

	userMap := make(map[string]*User)
	for _, user := range users {
		userMap[user.Email] = user
	}

	return userMap, nil
}

func GetUserFromSSHKey(sshKey string) (*User, error) {
	user := new(User)
	err := db.
		Joins("JOIN ssh_keys ON users.id = ssh_keys.user_id").
		Where("ssh_keys.content = ?", sshKey).
		First(&user).Error
	return user, err
}

func SSHKeyExistsForUser(sshKey string, userId uint) (*SSHKey, error) {
	key := new(SSHKey)
	err := db.
		Where("content = ?", sshKey).
		Where("user_id = ?", userId).
		First(&key).Error

	return key, err
}

func GetUserByProvider(id string, provider string) (*User, error) {
	user := new(User)
	var err error
	switch provider {
	case "github":
		err = db.Where("github_id = ?", id).First(&user).Error
	case "gitlab":
		err = db.Where("gitlab_id = ?", id).First(&user).Error
	case "gitea":
		err = db.Where("gitea_id = ?", id).First(&user).Error
	case "openid-connect":
		err = db.Where("oidc_id = ?", id).First(&user).Error
	}

	return user, err
}

func (user *User) Create() error {
	return db.Create(&user).Error
}

func (user *User) Update() error {
	return db.Save(&user).Error
}

func (user *User) Delete() error {
	return db.Delete(&user).Error
}

func (user *User) SetAdmin() error {
	return db.Model(&user).Update("is_admin", true).Error
}

func (user *User) HasLiked(gist *Gist) (bool, error) {
	association := db.Model(&gist).Where("user_id = ?", user.ID).Association("Likes")
	if association.Error != nil {
		return false, association.Error
	}

	if association.Count() == 0 {
		return false, nil
	}
	return true, nil
}

func (user *User) DeleteProviderID(provider string) error {
	providerIDFields := map[string]string{
		"github":         "github_id",
		"gitlab":         "gitlab_id",
		"gitea":          "gitea_id",
		"openid-connect": "oidc_id",
	}

	if providerIDField, ok := providerIDFields[provider]; ok {
		return db.Model(&user).
			Update(providerIDField, nil).
			Update("avatar_url", nil).
			Error
	}

	return nil
}

func (user *User) HasMFA() (bool, bool, error) {
	var webauthn bool
	var totp bool
	err := db.Model(&WebAuthnCredential{}).Select("count(*) > 0").Where("user_id = ?", user.ID).Find(&webauthn).Error
	if err != nil {
		return false, false, err
	}

	err = db.Model(&TOTP{}).Select("count(*) > 0").Where("user_id = ?", user.ID).Find(&totp).Error

	return webauthn, totp, err
}

func (user *User) GetStyle() *UserStyleDTO {
	style := new(UserStyleDTO)
	err := json.Unmarshal([]byte(user.StylePreferences), style)
	if err != nil {
		return nil
	}
	return style
}

// -- DTO -- //

type UserDTO struct {
	Username string `form:"username" validate:"required,max=24,alphanumdash,notreserved"`
	Password string `form:"password" validate:"required"`
}

func (dto *UserDTO) ToUser() *User {
	return &User{
		Username: dto.Username,
		Password: dto.Password,
	}
}

type UserUsernameDTO struct {
	Username string `form:"username" validate:"required,max=24,alphanumdash,notreserved"`
}

type UserStyleDTO struct {
	SoftWrap         bool   `form:"softwrap" json:"soft_wrap"`
	RemovedLineColor string `form:"removedlinecolor" json:"removed_line_color" validate:"min=0,max=7"`
	AddedLineColor   string `form:"addedlinecolor" json:"added_line_color" validate:"min=0,max=7"`
	GitLineColor     string `form:"gitlinecolor" json:"git_line_color" validate:"min=0,max=7"`
}

func (dto *UserStyleDTO) ToJson() string {
	data, err := json.Marshal(dto)
	if err != nil {
		return "{}"
	}
	return string(data)
}
