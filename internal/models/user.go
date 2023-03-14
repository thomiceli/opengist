package models

type User struct {
	ID        uint   `gorm:"primaryKey"`
	Username  string `form:"username" gorm:"uniqueIndex" validate:"required,max=24,alphanum,notreserved"`
	Password  string `form:"password" validate:"required"`
	IsAdmin   bool
	CreatedAt int64

	Gists   []Gist   `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:UserID"`
	SSHKeys []SSHKey `gorm:"foreignKey:UserID"`
	Liked   []Gist   `gorm:"many2many:likes;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func DoesUserExists(userName string, count *int64) error {
	return db.Table("users").
		Where("username like ?", userName).
		Count(count).Error
}

func GetAllUsers(offset int) ([]*User, error) {
	var all []*User
	err := db.
		Limit(11).
		Offset(offset * 10).
		Order("id asc").
		Find(&all).Error

	return all, err
}

func GetLoginUser(user *User) error {
	return db.
		Where("username like ?", user.Username).
		First(&user).Error
}

func GetLoginUserById(user *User) error {
	return db.
		Where("id = ?", user.ID).
		First(&user).Error
}

func CreateUser(user *User) error {
	return db.Create(&user).Error
}

func DeleteUserByID(userid string) error {
	return db.Delete(&User{}, "id = ?", userid).Error
}

func SetAdminUser(user *User) error {
	return db.Model(&user).Update("is_admin", true).Error
}

func GetUserBySSHKeyID(sshKeyId uint) (*User, error) {
	user := new(User)
	err := db.
		Preload("SSHKeys").
		Joins("join ssh_keys on users.id = ssh_keys.user_id").
		Where("ssh_keys.id = ?", sshKeyId).
		First(&user).Error

	return user, err
}

func UserHasLikedGist(user *User, gist *Gist) (bool, error) {
	association := db.Model(&gist).Where("user_id = ?", user.ID).Association("Likes")
	if association.Error != nil {
		return false, association.Error
	}

	if association.Count() == 0 {
		return false, nil
	}
	return true, nil
}
