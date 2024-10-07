package db

import (
	"gorm.io/gorm"
)

type User struct {
	ID        uint   `gorm:"primaryKey"`
	Username  string `gorm:"uniqueIndex,size:191"`
	Password  string
	IsAdmin   bool
	CreatedAt int64
	Email     string
	MD5Hash   string // for gravatar, if no Email is specified, the value is random
	AvatarURL string
	GithubID  string
	GitlabID  string
	GiteaID   string
	OIDCID    string `gorm:"column:oidc_id"`

	Gists               []Gist               `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:UserID"`
	SSHKeys             []SSHKey             `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:UserID"`
	Liked               []Gist               `gorm:"many2many:likes;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	WebAuthnCredentials []WebAuthnCredential `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:UserID"`
}

func (user *User) BeforeDelete(tx *gorm.DB) error {
	// Decrement likes counter for all gists liked by this user
	// The likes will be automatically deleted by the foreign key constraint
	err := tx.Model(&Gist{}).
		Omit("updated_at").
		Where("id IN (?)", tx.
			Select("gist_id").
			Table("likes").
			Where("user_id = ?", user.ID),
		).
		UpdateColumn("nb_likes", gorm.Expr("nb_likes - 1")).
		Error
	if err != nil {
		return err
	}

	// Decrement forks counter for all gists forked by this user
	err = tx.Model(&Gist{}).
		Omit("updated_at").
		Where("id IN (?)", tx.
			Select("forked_id").
			Table("gists").
			Where("user_id = ?", user.ID),
		).
		UpdateColumn("nb_forks", gorm.Expr("nb_forks - 1")).
		Error
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

	// Delete all gists created by this user
	return tx.Where("user_id = ?", user.ID).Delete(&Gist{}).Error
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

func (user *User) HasMFA() (bool, error) {
	var exists bool
	err := db.Model(&WebAuthnCredential{}).Select("count(*) > 0").Where("user_id = ?", user.ID).Find(&exists).Error

	return exists, err
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
