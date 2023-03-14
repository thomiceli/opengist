package models

import (
	"time"
)

type Gist struct {
	ID              uint `gorm:"primaryKey"`
	Uuid            string
	Title           string `validate:"max=50" form:"title"`
	Preview         string
	PreviewFilename string
	Description     string `validate:"max=150" form:"description"`
	Private         bool   `form:"private"`
	UserID          uint
	User            User `validate:"-"`
	NbFiles         int
	NbLikes         int
	NbForks         int
	CreatedAt       int64
	UpdatedAt       int64

	Likes    []User `gorm:"many2many:likes;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Forked   *Gist  `gorm:"foreignKey:ForkedID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	ForkedID uint

	Files []File `gorm:"-" validate:"min=1,dive"`
}

type File struct {
	Filename    string `validate:"excludes=\x2f,excludes=\x5c,max=50"`
	OldFilename string `validate:"excludes=\x2f,excludes=\x5c,max=50"`
	Content     string `validate:"required"`
}

type Commit struct {
	Hash      string
	Author    string
	Timestamp string
	Changed   string
	Files     []File
}

func GetGist(user string, gistUuid string) (*Gist, error) {
	gist := new(Gist)
	err := db.Preload("User").Preload("Forked.User").
		Where("gists.uuid = ? AND users.username like ?", gistUuid, user).
		Joins("join users on gists.user_id = users.id").
		First(&gist).Error

	return gist, err
}

func GetGistByID(gistId string) (*Gist, error) {
	gist := new(Gist)
	err := db.Preload("User").Preload("Forked.User").
		Where("gists.id = ?", gistId).
		First(&gist).Error

	return gist, err
}

func GetAllGistsForCurrentUser(currentUserId uint, offset int, sort string, order string) ([]*Gist, error) {
	var gists []*Gist
	err := db.Preload("User").Preload("Forked.User").
		Where("gists.private = 0 or gists.user_id = ?", currentUserId).
		Limit(11).
		Offset(offset * 10).
		Order(sort + "_at " + order).
		Find(&gists).Error

	return gists, err
}

func GetAllGists(offset int) ([]*Gist, error) {
	var all []*Gist
	err := db.Preload("User").
		Limit(11).
		Offset(offset * 10).
		Order("id asc").
		Find(&all).Error

	return all, err
}

func GetAllGistsFromUser(fromUser string, currentUserId uint, offset int, sort string, order string) ([]*Gist, error) {
	var gists []*Gist
	err := db.Preload("User").Preload("Forked.User").
		Where("users.username = ? and ((gists.private = 0) or (gists.private = 1 and gists.user_id = ?))", fromUser, currentUserId).
		Joins("join users on gists.user_id = users.id").
		Limit(11).
		Offset(offset * 10).
		Order("gists." + sort + "_at " + order).
		Find(&gists).Error

	return gists, err
}

func CreateGist(gist *Gist) error {
	// avoids foreign key constraint error because the default value in the struct is 0
	return db.Omit("forked_id").Create(&gist).Error
}

func CreateForkedGist(gist *Gist) error {
	return db.Create(&gist).Error
}

func UpdateGist(gist *Gist) error {
	return db.Omit("forked_id").Save(&gist).Error
}

func DeleteGist(gist *Gist) error {
	return db.Delete(&gist).Error
}

func GistLastActiveNow(gistID uint) error {
	return db.Model(&Gist{}).
		Where("id = ?", gistID).
		Update("updated_at", time.Now().Unix()).Error
}

func AppendUserLike(gist *Gist, user *User) error {
	err := db.Model(&gist).Omit("updated_at").Update("nb_likes", gist.NbLikes+1).Error
	if err != nil {
		return err
	}

	return db.Model(&gist).Omit("updated_at").Association("Likes").Append(user)
}

func RemoveUserLike(gist *Gist, user *User) error {
	err := db.Model(&gist).Omit("updated_at").Update("nb_likes", gist.NbLikes-1).Error
	if err != nil {
		return err
	}

	return db.Model(&gist).Omit("updated_at").Association("Likes").Delete(user)
}

func IncrementGistForkCount(gist *Gist) error {
	return db.Model(&gist).Omit("updated_at").Update("nb_forks", gist.NbForks+1).Error
}

func DecrementGistForkCount(gist *Gist) error {
	return db.Model(&gist).Omit("updated_at").Update("nb_forks", gist.NbForks-1).Error
}

func GetForkedGist(gist *Gist, user *User) (*Gist, error) {
	fork := new(Gist)
	err := db.Preload("User").
		Where("forked_id = ? and user_id = ?", gist.ID, user.ID).
		First(&fork).Error
	return fork, err
}

func GetUsersLikesForGist(gist *Gist, offset int) ([]*User, error) {
	var users []*User
	err := db.Model(&gist).
		Where("gist_id = ?", gist.ID).
		Limit(31).
		Offset(offset * 30).
		Association("Likes").Find(&users)
	return users, err
}

func GetUsersForksForGist(gist *Gist, currentUserId uint, offset int) ([]*Gist, error) {
	var gists []*Gist
	err := db.Model(&gist).Preload("User").
		Where("forked_id = ?", gist.ID).
		Where("(gists.private = 0) or (gists.private = 1 and gists.user_id = ?)", currentUserId).
		Limit(11).
		Offset(offset * 10).
		Order("updated_at desc").
		Find(&gists).Error

	return gists, err
}

func UserCanWrite(user *User, gist *Gist) bool {
	return !(user == nil) && (gist.UserID == user.ID)
}
