package db

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/dustin/go-humanize"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/index"
	"gorm.io/gorm"
)

type Visibility int

const (
	PublicVisibility Visibility = iota
	UnlistedVisibility
	PrivateVisibility
)

func (v Visibility) String() string {
	switch v {
	case PublicVisibility:
		return "public"
	case UnlistedVisibility:
		return "unlisted"
	case PrivateVisibility:
		return "private"
	default:
		return "???"
	}
}

func (v Visibility) Uint() uint {
	return uint(v)
}

func (v Visibility) Next() Visibility {
	switch v {
	case PublicVisibility:
		return UnlistedVisibility
	case UnlistedVisibility:
		return PrivateVisibility
	default:
		return PublicVisibility
	}
}

func ParseVisibility[T string | int](v T) Visibility {
	switch s := fmt.Sprint(v); s {
	case "0", "public":
		return PublicVisibility
	case "1", "unlisted":
		return UnlistedVisibility
	case "2", "private":
		return PrivateVisibility
	default:
		return PublicVisibility
	}
}

type Gist struct {
	ID              uint `gorm:"primaryKey"`
	Uuid            string
	Title           string
	URL             string
	Preview         string
	PreviewFilename string
	PreviewMimeType string
	Description     string
	Private         Visibility // 0: public, 1: unlisted, 2: private
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

	Topics    []GistTopic    `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Languages []GistLanguage `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
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
	err := db.Preload("User").Preload("Forked.User").Preload("Topics").
		Where("(gists.uuid like ? OR gists.url = ?) AND users.username like ?", gistUuid+"%", gistUuid, user).
		Joins("join users on gists.user_id = users.id").
		First(&gist).Error

	return gist, err
}

func GetGistByID(gistId string) (*Gist, error) {
	gist := new(Gist)
	err := db.Preload("User").Preload("Forked.User").Preload("Topics").
		Where("gists.id = ?", gistId).
		First(&gist).Error

	return gist, err
}

func GetAllGistsForCurrentUser(currentUserId uint, offset int, sort string, order string) ([]*Gist, error) {
	var gists []*Gist
	err := db.Preload("User").
		Preload("Forked.User").
		Preload("Topics").
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

func GetAllGistsFromSearch(currentUserId uint, query string, offset int, sort string, order string, topic string) ([]*Gist, error) {
	var gists []*Gist
	tx := db.Preload("User").Preload("Forked.User").Preload("Topics").
		Where("((gists.private = 0) or (gists.private > 0 and gists.user_id = ?))", currentUserId).
		Where("gists.title like ? or gists.description like ?", "%"+query+"%", "%"+query+"%")

	if topic != "" {
		tx = tx.Joins("join gist_topics on gists.id = gist_topics.gist_id").
			Where("gist_topics.topic = ?", topic)
	}

	err := tx.Limit(11).
		Offset(offset * 10).
		Order("gists." + sort + "_at " + order).
		Find(&gists).Error

	return gists, err
}

func gistsFromUserStatement(fromUserId uint, currentUserId uint) *gorm.DB {
	return db.Preload("User").Preload("Forked.User").Preload("Topics").
		Where("((gists.private = 0) or (gists.private > 0 and gists.user_id = ?))", currentUserId).
		Where("users.id = ?", fromUserId).
		Joins("join users on gists.user_id = users.id")
}

func GetAllGistsFromUser(fromUserId uint, currentUserId uint, title string, language string, visibility string, topics []string, offset int, sort string, order string) ([]*Gist, int64, error) {
	var gists []*Gist
	var count int64

	baseQuery := gistsFromUserStatement(fromUserId, currentUserId).Model(&Gist{})

	if title != "" {
		baseQuery = baseQuery.Where("gists.title like ?", "%"+title+"%")
	}

	if language != "" {
		baseQuery = baseQuery.Joins("join gist_languages on gists.id = gist_languages.gist_id").
			Where("gist_languages.language = ?", language)
	}

	if visibility != "" {
		baseQuery = baseQuery.Where("gists.private = ?", ParseVisibility(visibility))
	}

	if len(topics) > 0 {
		baseQuery = baseQuery.Joins("join gist_topics on gists.id = gist_topics.gist_id").
			Where("gist_topics.topic in ?", topics)
	}

	err := baseQuery.Count(&count).Error
	if err != nil {
		return nil, 0, err
	}

	err = baseQuery.Limit(11).
		Offset(offset * 10).
		Order("gists." + sort + "_at " + order).
		Find(&gists).Error

	return gists, count, err
}

func CountAllGistsFromUser(fromUserId uint, currentUserId uint) (int64, error) {
	var count int64
	err := gistsFromUserStatement(fromUserId, currentUserId).Model(&Gist{}).Count(&count).Error
	return count, err
}

func likedStatement(fromUserId uint, currentUserId uint) *gorm.DB {
	return db.Preload("User").Preload("Forked.User").Preload("Topics").
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
	return db.Preload("User").Preload("Forked.User").Preload("Topics").
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

func GetAllGistsVisibleByUser(userId uint) ([]uint, error) {
	var gists []uint

	err := db.Table("gists").
		Where("gists.private = 0 or gists.user_id = ?", userId).
		Pluck("gists.id", &gists).Error

	return gists, err
}

func GetAllGistsByIds(ids []uint) ([]*Gist, error) {
	var gists []*Gist
	err := db.Preload("User").Preload("Forked.User").Preload("Topics").
		Where("id in ?", ids).
		Find(&gists).Error

	// keep order
	ordered := make([]*Gist, 0, len(ids))
	for _, wantedId := range ids {
		for _, gist := range gists {
			if gist.ID == wantedId {
				ordered = append(ordered, gist)
				break
			}
		}
	}

	return ordered, err
}

func (gist *Gist) Create() error {
	// avoids foreign key constraint error because the default value in the struct is 0
	return db.Omit("forked_id").Create(&gist).Error
}

func (gist *Gist) CreateForked() error {
	return db.Create(&gist).Error
}

func (gist *Gist) Update() error {
	// reset the topics
	err := db.Model(&GistTopic{}).Where("gist_id = ?", gist.ID).Delete(&GistTopic{}).Error
	if err != nil {
		return err
	}

	return db.Omit("forked_id").Save(&gist).Error
}

func (gist *Gist) UpdateNoTimestamps() error {
	return db.Omit("forked_id", "updated_at").Save(&gist).Error
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
	return user != nil && gist.UserID == user.ID
}

func (gist *Gist) InitRepository() error {
	return git.InitRepository(gist.User.Username, gist.Uuid)
}

func (gist *Gist) DeleteRepository() error {
	return git.DeleteRepository(gist.User.Username, gist.Uuid)
}

func (gist *Gist) Files(revision string, truncate bool) ([]*git.File, error) {
	filesCat, err := git.CatFileBatch(gist.User.Username, gist.Uuid, revision, truncate)
	if err != nil {
		// if the revision or the file do not exist
		if exiterr, ok := err.(*exec.ExitError); ok && exiterr.ExitCode() == 128 {
			return nil, &git.RevisionNotFoundError{}
		}
		return nil, err
	}

	var files []*git.File
	for _, fileCat := range filesCat {
		var shortContent string
		if len(fileCat.Content) > 512 {
			shortContent = fileCat.Content[:512]
		} else {
			shortContent = fileCat.Content
		}

		files = append(files, &git.File{
			Filename:  fileCat.Name,
			Size:      fileCat.Size,
			HumanSize: humanize.IBytes(fileCat.Size),
			Content:   fileCat.Content,
			Truncated: fileCat.Truncated,
			MimeType:  git.DetectMimeType([]byte(shortContent), filepath.Ext(fileCat.Name)),
		})
	}
	return files, err
}

func (gist *Gist) File(revision string, filename string, truncate bool) (*git.File, error) {
	content, truncated, err := git.GetFileContent(gist.User.Username, gist.Uuid, revision, filename, truncate)

	// if the revision or the file do not exist
	if exiterr, ok := err.(*exec.ExitError); ok && exiterr.ExitCode() == 128 {
		return nil, nil
	}

	var size uint64

	size, err = git.GetFileSize(gist.User.Username, gist.Uuid, revision, filename)
	if err != nil {
		return nil, err
	}

	var shortContent string
	if len(content) > 512 {
		shortContent = content[:512]
	} else {
		shortContent = content
	}

	return &git.File{
		Filename:  filename,
		Size:      size,
		HumanSize: humanize.IBytes(size),
		Content:   content,
		Truncated: truncated,
		MimeType:  git.DetectMimeType([]byte(shortContent), filepath.Ext(filename)),
	}, err
}

func (gist *Gist) FileNames(revision string) ([]string, error) {
	return git.GetFilesOfRepository(gist.User.Username, gist.Uuid, revision)
}

func (gist *Gist) Log(skip int) ([]*git.Commit, error) {
	return git.GetLog(gist.User.Username, gist.Uuid, skip)
}

func (gist *Gist) NbCommits() (string, error) {
	return git.CountCommits(gist.User.Username, gist.Uuid)
}

func (gist *Gist) AddAndCommitFiles(files *[]FileDTO) error {
	if err := git.CloneTmp(gist.User.Username, gist.Uuid, gist.Uuid, gist.User.Email, true); err != nil {
		return err
	}

	for _, file := range *files {
		if file.SourcePath != "" { // if it's an uploaded file
			if err := git.MoveFileToRepository(gist.Uuid, file.Filename, file.SourcePath); err != nil {
				return err
			}
		} else { // else it's a text editor file
			if err := git.SetFileContent(gist.Uuid, file.Filename, file.Content); err != nil {
				return err
			}
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

func (gist *Gist) AddAndCommitFile(file *FileDTO) error {
	if err := git.CloneTmp(gist.User.Username, gist.Uuid, gist.Uuid, gist.User.Email, false); err != nil {
		return err
	}

	if err := git.SetFileContent(gist.Uuid, file.Filename, file.Content); err != nil {
		return err
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

func (gist *Gist) UpdatePreviewAndCount(withTimestampUpdate bool) error {
	filesStr, err := git.GetFilesOfRepository(gist.User.Username, gist.Uuid, "HEAD")
	if err != nil {
		return err
	}
	gist.NbFiles = len(filesStr)

	if len(filesStr) == 0 {
		gist.Preview = ""
		gist.PreviewFilename = ""
		gist.PreviewMimeType = ""
	} else {
		for _, fileStr := range filesStr {
			file, err := gist.File("HEAD", fileStr, true)
			if err != nil {
				return err
			}
			if file == nil {
				continue
			}
			gist.Preview = ""
			gist.PreviewFilename = file.Filename
			gist.PreviewMimeType = file.MimeType.ContentType

			if !file.MimeType.CanBeEdited() {
				continue
			}

			split := strings.Split(file.Content, "\n")
			if len(split) > 10 {
				gist.Preview = strings.Join(split[:10], "\n")
			} else {
				gist.Preview = file.Content
			}
		}
	}

	if withTimestampUpdate {
		return gist.Update()
	}
	return gist.UpdateNoTimestamps()
}

func (gist *Gist) VisibilityStr() string {
	switch gist.Private {
	case PublicVisibility:
		return "public"
	case UnlistedVisibility:
		return "unlisted"
	case PrivateVisibility:
		return "private"
	default:
		return ""
	}
}

func (gist *Gist) Identifier() string {
	if gist.URL != "" {
		return gist.URL
	}
	return gist.Uuid
}

func (gist *Gist) GetLanguagesFromFiles() ([]string, error) {
	files, err := gist.Files("HEAD", true)
	if err != nil {
		return nil, err
	}

	languages := make([]string, 0, len(files))
	for _, file := range files {
		var lexer chroma.Lexer
		if lexer = lexers.Get(file.Filename); lexer == nil {
			lexer = lexers.Fallback
		}

		fileType := lexer.Config().Name
		if lexer.Config().Name == "fallback" || lexer.Config().Name == "plaintext" {
			fileType = "Text"
		}

		languages = append(languages, fileType)
	}

	return languages, nil
}

func (gist *Gist) GetTopics() ([]string, error) {
	var topics []string
	err := db.Model(&GistTopic{}).
		Where("gist_id = ?", gist.ID).
		Pluck("topic", &topics).Error
	return topics, err
}

func (gist *Gist) TopicsSlice() []string {
	topics := make([]string, 0, len(gist.Topics))
	for _, topic := range gist.Topics {
		topics = append(topics, topic.Topic)
	}
	return topics
}

func (gist *Gist) UpdateLanguages() {
	languages, err := gist.GetLanguagesFromFiles()
	if err != nil {
		log.Error().Err(err).Msgf("Cannot get languages for gist %d", gist.ID)
		return
	}

	slices.Sort(languages)
	languages = slices.Compact(languages)

	tx := db.Begin()
	if tx.Error != nil {
		log.Error().Err(tx.Error).Msgf("Cannot start transaction for gist %d", gist.ID)
		return
	}

	if err := tx.Where("gist_id = ?", gist.ID).Delete(&GistLanguage{}).Error; err != nil {
		tx.Rollback()
		log.Error().Err(err).Msgf("Cannot delete languages for gist %d", gist.ID)
		return
	}

	for _, language := range languages {
		gistLanguage := &GistLanguage{
			GistID:   gist.ID,
			Language: language,
		}
		if err := tx.Create(gistLanguage).Error; err != nil {
			tx.Rollback()
			log.Error().Err(err).Msgf("Cannot create gist language %s for gist %d", language, gist.ID)
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		log.Error().Err(err).Msgf("Cannot commit transaction for gist %d", gist.ID)
		return
	}
}

func (gist *Gist) ToDTO() (*GistDTO, error) {
	files, err := gist.Files("HEAD", false)
	if err != nil {
		return nil, err
	}

	fileDTOs := make([]FileDTO, 0, len(files))
	for _, file := range files {
		f := FileDTO{
			Filename: file.Filename,
		}
		if file.MimeType.CanBeEdited() {
			f.Content = file.Content
		} else {
			f.Binary = true
		}
		fileDTOs = append(fileDTOs, f)
	}

	return &GistDTO{
		Title:       gist.Title,
		Description: gist.Description,
		URL:         gist.URL,
		Files:       fileDTOs,
		VisibilityDTO: VisibilityDTO{
			Private: gist.Private,
		},
		Topics: strings.Join(gist.TopicsSlice(), " "),
	}, nil
}

// -- DTO -- //

type GistDTO struct {
	Title       string    `validate:"max=250" form:"title"`
	Description string    `validate:"max=1000" form:"description"`
	URL         string    `validate:"max=32,alphanumdashorempty" form:"url"`
	Files       []FileDTO `validate:"min=1,dive"`
	Name        []string  `form:"name"`
	Content     []string  `form:"content"`
	Topics      string    `validate:"gisttopics" form:"topics"`
	VisibilityDTO
}

func (dto *GistDTO) HasMetadata() bool {
	return dto.Title != "" || dto.Description != "" || dto.URL != "" || dto.Topics != ""
}

type VisibilityDTO struct {
	Private Visibility `validate:"number,min=0,max=2" form:"private"`
}

type FileDTO struct {
	Filename   string `validate:"excludes=\x2f,excludes=\x5c,max=255"`
	Content    string
	Binary     bool
	SourcePath string // Path to uploaded file, used instead of Content when present
}

func (dto *GistDTO) ToGist() *Gist {
	return &Gist{
		Title:       dto.Title,
		Description: dto.Description,
		Private:     dto.Private,
		URL:         dto.URL,
		Topics:      dto.TopicStrToSlice(),
	}
}

func (dto *GistDTO) ToExistingGist(gist *Gist) *Gist {
	gist.Title = dto.Title
	gist.Description = dto.Description
	gist.URL = dto.URL
	gist.Topics = dto.TopicStrToSlice()
	return gist
}

func (dto *GistDTO) TopicStrToSlice() []GistTopic {
	topics := strings.Fields(dto.Topics)
	gistTopics := make([]GistTopic, 0, len(topics))
	for _, topic := range topics {
		gistTopics = append(gistTopics, GistTopic{Topic: topic})
	}
	return gistTopics
}

// -- Index -- //

func (gist *Gist) ToIndexedGist() (*index.Gist, error) {
	files, err := gist.Files("HEAD", true)
	if err != nil {
		return nil, err
	}

	exts := make([]string, 0, len(files))
	wholeContent := ""
	for _, file := range files {
		wholeContent += file.Content
		if !strings.HasSuffix(wholeContent, "\n") {
			wholeContent += "\n"
		}
		exts = append(exts, filepath.Ext(file.Filename))
	}

	fileNames, err := gist.FileNames("HEAD")
	if err != nil {
		return nil, err
	}

	langs, err := gist.GetLanguagesFromFiles()
	if err != nil {
		return nil, err
	}

	topics, err := gist.GetTopics()
	if err != nil {
		return nil, err
	}

	indexedGist := &index.Gist{
		GistID:     gist.ID,
		UserID:     gist.UserID,
		Visibility: gist.Private.Uint(),
		Username:   gist.User.Username,
		Title:      gist.Title,
		Content:    wholeContent,
		Filenames:  fileNames,
		Extensions: exts,
		Languages:  langs,
		Topics:     topics,
		CreatedAt:  gist.CreatedAt,
		UpdatedAt:  gist.UpdatedAt,
	}

	return indexedGist, nil
}

func (gist *Gist) AddInIndex() {
	if !index.IndexEnabled() {
		return
	}

	go func() {
		indexedGist, err := gist.ToIndexedGist()
		if err != nil {
			log.Error().Err(err).Msgf("Cannot convert gist %d to indexed gist", gist.ID)
			return
		}
		err = index.AddInIndex(indexedGist)
		if err != nil {
			log.Error().Err(err).Msgf("Error adding gist %d to index", gist.ID)
		}
	}()
}

func (gist *Gist) RemoveFromIndex() {
	if !index.IndexEnabled() {
		return
	}

	go func() {
		err := index.RemoveFromIndex(gist.ID)
		if err != nil {
			log.Error().Err(err).Msgf("Error remove gist %d from index", gist.ID)
		}
	}()
}
