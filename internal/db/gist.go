package db

import (
	"github.com/labstack/echo/v4"
	"github.com/thomiceli/opengist/internal/git"
	"gorm.io/gorm"
	"os/exec"
	"strings"
	"time"
)

type Gist struct {
	ID              uint `gorm:"primaryKey"`
	Uuid            string
	Title           string
	Preview         string
	PreviewFilename string
	Description     string
	Private         int // 0: public, 1: unlisted, 2: private
	UserID          uint
	User            User
	NbFiles         int
	NbLikes         int
	NbForks         int
	CreatedAt       int64
	UpdatedAt       int64

	Likes    []User `gorm:"many2many:likes;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Forked   *Gist  `gorm:"foreignKey:ForkedID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	ForkedID uint
}

type Like struct {
	UserID    uint `gorm:"primaryKey"`
	GistID    uint `gorm:"primaryKey"`
	CreatedAt int64
}

func (gist *Gist) BeforeDelete(tx *gorm.DB) error {
	// Decrement fork counter if the gist was forked
	err := tx.Model(&Gist{}).
		Omit("updated_at").
		Where("id = ?", gist.ForkedID).
		UpdateColumn("nb_forks", gorm.Expr("nb_forks - 1")).Error
	return err
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
	var gists []*Gist
	err := db.Preload("User").
		Limit(11).
		Offset(offset * 10).
		Order("id asc").
		Find(&gists).Error

	return gists, err
}

func GetAllGistsFromSearch(currentUserId uint, query string, offset int, sort string, order string) ([]*Gist, error) {
	var gists []*Gist
	err := db.Preload("User").Preload("Forked.User").
		Where("((gists.private = 0) or (gists.private > 0 and gists.user_id = ?))", currentUserId).
		Where("gists.title like ? or gists.description like ?", "%"+query+"%", "%"+query+"%").
		Limit(11).
		Offset(offset * 10).
		Order("gists." + sort + "_at " + order).
		Find(&gists).Error

	return gists, err
}

func gistsFromUserStatement(fromUserId uint, currentUserId uint) *gorm.DB {
	return db.Preload("User").Preload("Forked.User").
		Where("((gists.private = 0) or (gists.private > 0 and gists.user_id = ?))", currentUserId).
		Where("users.id = ?", fromUserId).
		Joins("join users on gists.user_id = users.id")
}

func GetAllGistsFromUser(fromUserId uint, currentUserId uint, offset int, sort string, order string) ([]*Gist, error) {
	var gists []*Gist
	err := gistsFromUserStatement(fromUserId, currentUserId).Limit(11).
		Offset(offset * 10).
		Order("gists." + sort + "_at " + order).
		Find(&gists).Error

	return gists, err
}

func CountAllGistsFromUser(fromUserId uint, currentUserId uint) (int64, error) {
	var count int64
	err := gistsFromUserStatement(fromUserId, currentUserId).Model(&Gist{}).Count(&count).Error
	return count, err
}

func likedStatement(fromUserId uint, currentUserId uint) *gorm.DB {
	return db.Preload("User").Preload("Forked.User").
		Where("((gists.private = 0) or (gists.private > 0 and gists.user_id = ?))", currentUserId).
		Where("likes.user_id = ?", fromUserId).
		Joins("join likes on gists.id = likes.gist_id").
		Joins("join users on likes.user_id = users.id")
}

func GetAllGistsLikedByUser(fromUserId uint, currentUserId uint, offset int, sort string, order string) ([]*Gist, error) {
	var gists []*Gist
	err := likedStatement(fromUserId, currentUserId).Limit(11).
		Offset(offset * 10).
		Order("gists." + sort + "_at " + order).
		Find(&gists).Error
	return gists, err
}

func CountAllGistsLikedByUser(fromUserId uint, currentUserId uint) (int64, error) {
	var count int64
	err := likedStatement(fromUserId, currentUserId).Model(&Gist{}).Count(&count).Error
	return count, err
}

func forkedStatement(fromUserId uint, currentUserId uint) *gorm.DB {
	return db.Preload("User").Preload("Forked.User").
		Where("gists.forked_id is not null and ((gists.private = 0) or (gists.private > 0 and gists.user_id = ?))", currentUserId).
		Where("gists.user_id = ?", fromUserId).
		Joins("join users on gists.user_id = users.id")
}

func GetAllGistsForkedByUser(fromUserId uint, currentUserId uint, offset int, sort string, order string) ([]*Gist, error) {
	var gists []*Gist
	err := forkedStatement(fromUserId, currentUserId).Limit(11).
		Offset(offset * 10).
		Order("gists." + sort + "_at " + order).
		Find(&gists).Error
	return gists, err
}

func CountAllGistsForkedByUser(fromUserId uint, currentUserId uint) (int64, error) {
	var count int64
	err := forkedStatement(fromUserId, currentUserId).Model(&Gist{}).Count(&count).Error
	return count, err
}

func GetAllGistsRows() ([]*Gist, error) {
	var gists []*Gist
	err := db.Table("gists").
		Preload("User").
		Find(&gists).Error

	return gists, err
}

func (gist *Gist) Create() error {
	// avoids foreign key constraint error because the default value in the struct is 0
	return db.Omit("forked_id").Create(&gist).Error
}

func (gist *Gist) CreateForked() error {
	return db.Create(&gist).Error
}

func (gist *Gist) Update() error {
	return db.Omit("forked_id").Save(&gist).Error
}

func (gist *Gist) Delete() error {
	err := gist.DeleteRepository()
	if err != nil {
		return err
	}

	return db.Delete(&gist).Error
}

func (gist *Gist) SetLastActiveNow() error {
	return db.Model(&Gist{}).
		Where("id = ?", gist.ID).
		Update("updated_at", time.Now().Unix()).Error
}

func (gist *Gist) AppendUserLike(user *User) error {
	err := db.Model(&gist).Omit("updated_at").Update("nb_likes", gist.NbLikes+1).Error
	if err != nil {
		return err
	}

	return db.Model(&gist).Omit("updated_at").Association("Likes").Append(user)
}

func (gist *Gist) RemoveUserLike(user *User) error {
	err := db.Model(&gist).Omit("updated_at").Update("nb_likes", gist.NbLikes-1).Error
	if err != nil {
		return err
	}

	return db.Model(&gist).Omit("updated_at").Association("Likes").Delete(user)
}

func (gist *Gist) IncrementForkCount() error {
	return db.Model(&gist).Omit("updated_at").Update("nb_forks", gist.NbForks+1).Error
}

func (gist *Gist) GetForkParent(user *User) (*Gist, error) {
	fork := new(Gist)
	err := db.Preload("User").
		Where("forked_id = ? and user_id = ?", gist.ID, user.ID).
		First(&fork).Error
	return fork, err
}

func (gist *Gist) GetUsersLikes(offset int) ([]*User, error) {
	var users []*User
	err := db.Model(&gist).
		Where("gist_id = ?", gist.ID).
		Limit(31).
		Offset(offset * 30).
		Association("Likes").Find(&users)
	return users, err
}

func (gist *Gist) GetForks(currentUserId uint, offset int) ([]*Gist, error) {
	var gists []*Gist
	err := db.Model(&gist).Preload("User").
		Where("forked_id = ?", gist.ID).
		Where("(gists.private = 0) or (gists.private > 0 and gists.user_id = ?)", currentUserId).
		Limit(11).
		Offset(offset * 10).
		Order("updated_at desc").
		Find(&gists).Error

	return gists, err
}

func (gist *Gist) CanWrite(user *User) bool {
	return !(user == nil) && (gist.UserID == user.ID)
}

func (gist *Gist) InitRepository() error {
	return git.InitRepository(gist.User.Username, gist.Uuid)
}

func (gist *Gist) InitRepositoryViaInit(ctx echo.Context) error {
	return git.InitRepositoryViaInit(gist.User.Username, gist.Uuid, ctx)
}

func (gist *Gist) DeleteRepository() error {
	return git.DeleteRepository(gist.User.Username, gist.Uuid)
}

func (gist *Gist) Files(revision string) ([]*git.File, error) {
	var files []*git.File
	filesStr, err := git.GetFilesOfRepository(gist.User.Username, gist.Uuid, revision)
	if err != nil {
		// if the revision or the file do not exist

		if exiterr, ok := err.(*exec.ExitError); ok && exiterr.ExitCode() == 128 {
			return nil, nil
		}

		return nil, err
	}

	for _, fileStr := range filesStr {
		file, err := gist.File(revision, fileStr, true)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, err
}

func (gist *Gist) File(revision string, filename string, truncate bool) (*git.File, error) {
	content, truncated, err := git.GetFileContent(gist.User.Username, gist.Uuid, revision, filename, truncate)

	// if the revision or the file do not exist
	if exiterr, ok := err.(*exec.ExitError); ok && exiterr.ExitCode() == 128 {
		return nil, nil
	}

	return &git.File{
		Filename:  filename,
		Content:   content,
		Truncated: truncated,
	}, err
}

func (gist *Gist) Log(skip int) ([]*git.Commit, error) {
	return git.GetLog(gist.User.Username, gist.Uuid, skip)
}

func (gist *Gist) NbCommits() (string, error) {
	return git.CountCommits(gist.User.Username, gist.Uuid)
}

func (gist *Gist) AddAndCommitFiles(files *[]FileDTO) error {
	if err := git.CloneTmp(gist.User.Username, gist.Uuid, gist.Uuid, gist.User.Email); err != nil {
		return err
	}

	for _, file := range *files {
		if err := git.SetFileContent(gist.Uuid, file.Filename, file.Content); err != nil {
			return err
		}
	}

	if err := git.AddAll(gist.Uuid); err != nil {
		return err
	}

	if err := git.CommitRepository(gist.Uuid, gist.User.Username, gist.User.Email); err != nil {
		return err
	}

	return git.Push(gist.Uuid)
}

func (gist *Gist) ForkClone(username string, uuid string) error {
	return git.ForkClone(gist.User.Username, gist.Uuid, username, uuid)
}

func (gist *Gist) UpdateServerInfo() error {
	return git.UpdateServerInfo(gist.User.Username, gist.Uuid)
}

func (gist *Gist) RPC(service string) ([]byte, error) {
	return git.RPC(gist.User.Username, gist.Uuid, service)
}

func (gist *Gist) UpdatePreviewAndCount() error {
	filesStr, err := git.GetFilesOfRepository(gist.User.Username, gist.Uuid, "HEAD")
	if err != nil {
		return err
	}
	gist.NbFiles = len(filesStr)

	if len(filesStr) == 0 {
		gist.Preview = ""
		gist.PreviewFilename = ""
	} else {
		file, err := gist.File("HEAD", filesStr[0], true)
		if err != nil {
			return err
		}

		split := strings.Split(file.Content, "\n")
		if len(split) > 10 {
			gist.Preview = strings.Join(split[:10], "\n")
		} else {
			gist.Preview = file.Content
		}

		gist.PreviewFilename = file.Filename
	}

	return gist.Update()
}

// -- DTO -- //

type GistDTO struct {
	Title       string    `validate:"max=250" form:"title"`
	Description string    `validate:"max=1000" form:"description"`
	Private     int       `validate:"number,min=0,max=2" form:"private"`
	Files       []FileDTO `validate:"min=1,dive"`
	Name        []string  `form:"name"`
	Content     []string  `form:"content"`
}

type FileDTO struct {
	Filename string `validate:"excludes=\x2f,excludes=\x5c,max=255"`
	Content  string `validate:"required"`
}

func (dto *GistDTO) ToGist() *Gist {
	return &Gist{
		Title:       dto.Title,
		Description: dto.Description,
		Private:     dto.Private,
	}
}

func (dto *GistDTO) ToExistingGist(gist *Gist) *Gist {
	gist.Title = dto.Title
	gist.Description = dto.Description
	return gist
}
