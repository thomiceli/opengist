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
	"github.com/thomiceli/opengist/internal/config"
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
	URLNormalized   string
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
	ExpiresAt       int64 // 0: never expires

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

func (gist *Gist) BeforeSave(_ *gorm.DB) error {
	gist.URLNormalized = strings.ToLower(gist.URL)
	return nil
}

func (gist *Gist) BeforeDelete(tx *gorm.DB) error {
	gist.DeleteRepository()
	gist.RemoveFromIndex()
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
		Where("(gists.uuid LIKE ? OR gists.url_normalized = ?) AND users.username_normalized = ?",
			strings.ToLower(gistUuid)+"%", strings.ToLower(gistUuid), strings.ToLower(user)).
		Joins("join users on gists.user_id = users.id").
		First(&gist).Error

	return gist, err
}

func GetGistByUUID(uuid string) (*Gist, error) {
	gist := new(Gist)
	err := db.Preload("User").Preload("Forked.User").Where("uuid = ?", uuid).First(gist).Error
	return gist, err
}

func GetGistByID(gistId string) (*Gist, error) {
	gist := new(Gist)
	err := db.Preload("User").Preload("Forked.User").Preload("Topics").
		Where("gists.id = ?", gistId).
		First(&gist).Error

	return gist, err
}

// GetAllGistsForCurrentUser returns gists visible to currentUserId - all public
// gists plus the user's own private/unlisted ones - ordered by sort/order and
// paginated to one extra row (the 11th is the peek-next sentinel).
// `since`, when non-nil, restricts results to gists updated at or after that
// instant (used by the API; the web handler passes nil).
func GetAllGistsForCurrentUser(currentUserId uint, since *time.Time, offset int, sort string, order string, limit int, perPage int) ([]*Gist, error) {
	var gists []*Gist
	query := db.Preload("User").
		Preload("Forked.User").
		Preload("Topics").
		Where("gists.private = 0 or gists.user_id = ?", currentUserId)
	if since != nil {
		query = query.Where("gists.updated_at >= ?", since.Unix())
	}
	err := query.
		Limit(limit).
		Offset(offset * perPage).
		Order(sort + "_at " + order).
		Find(&gists).Error

	return gists, err
}

// GetAllGistsFromUserVisibleTo returns gists owned by fromUserId, filtered
// to what currentUserId is allowed to see (public always; private/unlisted
// only when currentUserId == fromUserId). Same pagination/since shape as
// the other API list helpers. Pass currentUserId=0 to force the
// public-only subset.
func GetAllGistsFromUserVisibleTo(fromUserId uint, currentUserId uint, since *time.Time, offset int, sort string, order string, limit int, perPage int) ([]*Gist, error) {
	var gists []*Gist
	query := gistsFromUserStatement(fromUserId, currentUserId)
	if since != nil {
		query = query.Where("gists.updated_at >= ?", since.Unix())
	}
	err := query.
		Limit(limit).
		Offset(offset * perPage).
		Order("gists." + sort + "_at " + order).
		Find(&gists).Error

	return gists, err
}

// GetAllGistsOfUser returns every gist owned by userID - public, unlisted,
// and private - with the same pagination/since semantics as GetAllGistsForCurrentUser.
// Used by the API list endpoint for callers whose
// token holds gist:read: they see all of their own content but nothing from
// other users (others' public gists live under /gists/public).
func GetAllGistsOfUser(userID uint, since *time.Time, offset int, sort string, order string, limit int, perPage int) ([]*Gist, error) {
	var gists []*Gist
	query := db.Preload("User").
		Preload("Forked.User").
		Preload("Topics").
		Where("gists.user_id = ?", userID)
	if since != nil {
		query = query.Where("gists.updated_at >= ?", since.Unix())
	}
	err := query.
		Limit(limit).
		Offset(offset * perPage).
		Order(sort + "_at " + order).
		Find(&gists).Error

	return gists, err
}

// GetAllPublicGistsOfUser returns only the public gists owned by userID, with
// the same pagination/since semantics as GetAllGistsForCurrentUser. Used by
// the API list endpoint for callers that authenticate but whose token lacks
// gist:read - they get only their own public gists.
func GetAllPublicGistsOfUser(userID uint, since *time.Time, offset int, sort string, order string, limit int, perPage int) ([]*Gist, error) {
	var gists []*Gist
	query := db.Preload("User").
		Preload("Forked.User").
		Preload("Topics").
		Where("gists.private = 0 AND gists.user_id = ?", userID)
	if since != nil {
		query = query.Where("gists.updated_at >= ?", since.Unix())
	}
	err := query.
		Limit(limit).
		Offset(offset * perPage).
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

// GetAllGistsLikedByUser returns gists that fromUserId has starred, filtered
// to what currentUserId is allowed to see. `since`, when non-nil, restricts
// results to gists updated at or after that instant (used by the API; the web
// handler passes nil for both since and the explicit pagination args).
func GetAllGistsLikedByUser(fromUserId uint, currentUserId uint, since *time.Time, offset int, sort string, order string, limit int, perPage int) ([]*Gist, error) {
	var gists []*Gist
	query := likedStatement(fromUserId, currentUserId)
	if since != nil {
		query = query.Where("gists.updated_at >= ?", since.Unix())
	}
	err := query.
		Limit(limit).
		Offset(offset * perPage).
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

// GetAllGistsForkedByUser returns gists forked by fromUserId, filtered to
// what currentUserId is allowed to see. `since`, when non-nil, restricts
// results to gists updated at or after that instant (used by the API; the
// web handler passes nil for both since and the explicit pagination args).
func GetAllGistsForkedByUser(fromUserId uint, currentUserId uint, since *time.Time, offset int, sort string, order string, limit int, perPage int) ([]*Gist, error) {
	var gists []*Gist
	query := forkedStatement(fromUserId, currentUserId)
	if since != nil {
		query = query.Where("gists.updated_at >= ?", since.Unix())
	}
	err := query.
		Limit(limit).
		Offset(offset * perPage).
		Order("gists." + sort + "_at " + order).
		Find(&gists).Error
	return gists, err
}

func CountAllGistsForkedByUser(fromUserId uint, currentUserId uint) (int64, error) {
	var count int64
	err := forkedStatement(fromUserId, currentUserId).Model(&Gist{}).Count(&count).Error
	return count, err
}

// applySince narrows a gist query to rows updated at or after `since` when it
// is non-nil, matching the filter the API list queries apply. Kept separate so
// the count helpers stay in sync with their Find counterparts.
func applySince(q *gorm.DB, since *time.Time) *gorm.DB {
	if since != nil {
		return q.Where("gists.updated_at >= ?", since.Unix())
	}
	return q
}

// The Count* helpers below mirror the API list queries (including the optional
// `since` filter) so list responses can report a total. They're separate from
// the web UI's CountAll* helpers above, which don't take `since`.

func CountAllGistsForCurrentUser(currentUserId uint, since *time.Time) (int64, error) {
	var count int64
	q := applySince(db.Model(&Gist{}).Where("gists.private = 0 or gists.user_id = ?", currentUserId), since)
	err := q.Count(&count).Error
	return count, err
}

func CountAllGistsFromUserVisibleTo(fromUserId uint, currentUserId uint, since *time.Time) (int64, error) {
	var count int64
	err := applySince(gistsFromUserStatement(fromUserId, currentUserId).Model(&Gist{}), since).Count(&count).Error
	return count, err
}

func CountAllGistsOfUser(userID uint, since *time.Time) (int64, error) {
	var count int64
	q := applySince(db.Model(&Gist{}).Where("gists.user_id = ?", userID), since)
	err := q.Count(&count).Error
	return count, err
}

func CountAllPublicGistsOfUser(userID uint, since *time.Time) (int64, error) {
	var count int64
	q := applySince(db.Model(&Gist{}).Where("gists.private = 0 AND gists.user_id = ?", userID), since)
	err := q.Count(&count).Error
	return count, err
}

func CountAllGistsLikedByUserSince(fromUserId uint, currentUserId uint, since *time.Time) (int64, error) {
	var count int64
	err := applySince(likedStatement(fromUserId, currentUserId).Model(&Gist{}), since).Count(&count).Error
	return count, err
}

func CountAllGistsForkedByUserSince(fromUserId uint, currentUserId uint, since *time.Time) (int64, error) {
	var count int64
	err := applySince(forkedStatement(fromUserId, currentUserId).Model(&Gist{}), since).Count(&count).Error
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

// GetForks returns gists that fork this gist, filtered to what
// currentUserId is allowed to see. `offset` is the page index (0-based);
// `limit` caps the returned slice (pass perPage+1 for the peek-next
// sentinel). `perPage` is the slice size used for the offset arithmetic
// (offset * perPage rows are skipped).
func (gist *Gist) GetForks(currentUserId uint, offset int, limit int, perPage int) ([]*Gist, error) {
	var gists []*Gist
	err := db.Model(&gist).Preload("User").
		Where("forked_id = ?", gist.ID).
		Where("(gists.private = 0) or (gists.private > 0 and gists.user_id = ?)", currentUserId).
		Limit(limit).
		Offset(offset * perPage).
		Order("updated_at desc").
		Find(&gists).Error

	return gists, err
}

// CountForks returns the number of forks of this gist visible to currentUserId,
// using the same visibility filter as GetForks (pass 0 for the public subset).
func (gist *Gist) CountForks(currentUserId uint) (int64, error) {
	var count int64
	err := db.Model(&Gist{}).
		Where("forked_id = ?", gist.ID).
		Where("(gists.private = 0) or (gists.private > 0 and gists.user_id = ?)", currentUserId).
		Count(&count).Error
	return count, err
}

func (gist *Gist) CanWrite(user *User) bool {
	return user != nil && gist.UserID == user.ID
}

func (gist *Gist) InitRepository() error {
	return git.InitRepository(gist.User.Username, gist.Uuid)
}

func (gist *Gist) DeleteRepository() {
	err := git.DeleteRepository(gist.User.Username, gist.Uuid)
	if err != nil {
		log.Warn().Err(err).Msgf("Could not delete repository %s/%s", gist.User.Username, gist.Uuid)
	}
}

func (gist *Gist) Files(revision string, truncate bool) ([]*git.File, bool, error) {
	filesCat, gistTruncated, err := git.CatFileBatch(gist.User.Username, gist.Uuid, revision, truncate)
	if err != nil {
		// if the revision or the file do not exist
		if exiterr, ok := err.(*exec.ExitError); ok && exiterr.ExitCode() == 128 {
			return nil, false, &git.RevisionNotFoundError{}
		}
		return nil, false, err
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
	return files, gistTruncated, err
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

// GistCommit pairs a raw git commit with the Opengist account whose email
// matches the commit's AuthorEmail (when one exists). The git.Commit pointer
// is embedded so callers/templates can read AuthorName, Hash, Timestamp,
// Files, etc. directly. User is nil when no account matches - callers can
// fall back to the embedded AuthorName/AuthorEmail.
type GistCommit struct {
	*git.Commit
	User *User
}

// Log returns the gist's commit history starting from `revision` (pass
// "HEAD" for the full history or a SHA to walk from a specific commit
// downward), with each commit's author resolved to an Opengist user via a
// single bulk email lookup. Lookup is case-insensitive on both sides -
// matches the historical web behavior even when the DB stores mixed-case
// emails. `skip` is the number of commits to skip from the top of the walk
// (use offset*per_page for paging); `limit` caps the returned slice (pass
// per_page+1 to enable the peek-next sentinel trick).
func (gist *Gist) Log(revision string, skip int, limit int) ([]*GistCommit, error) {
	raw, err := git.GetLog(gist.User.Username, gist.Uuid, revision, skip, limit)
	if err != nil {
		return nil, err
	}

	// Collect distinct lowercased author emails.
	loweredSet := make(map[string]struct{}, len(raw))
	for _, c := range raw {
		if c.AuthorEmail == "" {
			continue
		}
		loweredSet[strings.ToLower(c.AuthorEmail)] = struct{}{}
	}

	// One IN query, then re-key by lowercased email so we can look up
	// case-insensitively even if the DB column holds a mixed-case value.
	byDBEmail, _ := GetUsersFromEmails(loweredSet)
	byLowered := make(map[string]*User, len(byDBEmail))
	for e, u := range byDBEmail {
		byLowered[strings.ToLower(e)] = u
	}

	out := make([]*GistCommit, len(raw))
	for i, c := range raw {
		out[i] = &GistCommit{
			Commit: c,
			User:   byLowered[strings.ToLower(c.AuthorEmail)],
		}
	}
	return out, nil
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

// HTTPCloneURL returns the HTTPS clone URL (`{baseURL}/{user}/{identifier}.git`).
// Returns "" when HTTP git access is disabled (config.HttpGit == false).
func (gist *Gist) HTTPCloneURL(baseURL string) string {
	if !config.C.HttpGit {
		return ""
	}
	return baseURL + "/" + gist.User.Username + "/" + gist.Identifier() + ".git"
}

// SSHCloneURL returns the SSH clone URL. `fallbackHost` is the request's Host
// header (or any host:port-shaped string) used when SshExternalDomain isn't
// configured — only its hostname part is kept. Returns "" when SSH git access
// is disabled (ssh.git-enabled = disabled).
func (gist *Gist) SSHCloneURL(fallbackHost string) string {
	if !config.C.SshEnabled() {
		return ""
	}
	sshDomain := config.C.SshExternalDomain
	if sshDomain == "" {
		sshDomain = strings.Split(fallbackHost, ":")[0]
	}
	var user string
	if config.C.SshUsername != "" {
		user = config.C.SshUsername + "@"
	}
	path := gist.User.Username + "/" + gist.Identifier() + ".git"
	if config.C.SshPort == "22" {
		return user + sshDomain + ":" + path
	}
	return "ssh://" + user + sshDomain + ":" + config.C.SshPort + "/" + path
}

func (gist *Gist) GetLanguagesFromFiles() ([]string, error) {
	files, _, err := gist.Files("HEAD", true)
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
	files, _, err := gist.Files("HEAD", false)
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
	Title              string         `validate:"max=250" form:"title"`
	Description        string         `validate:"max=1000" form:"description"`
	URL                string         `validate:"max=32,alphanumdashorempty" form:"url"`
	Files              []FileDTO      `validate:"min=1,dive"`
	Name               []string       `form:"name"`
	Content            []string       `form:"content"`
	Topics             string         `validate:"gisttopics" form:"topics"`
	UploadedFilesUUID  []string       `validate:"omitempty,dive,required,uuid" form:"uploadedfile_uuid"`
	UploadedFilesNames []string       `validate:"omitempty,dive,required" form:"uploadedfile_filename"`
	BinaryFileOldName  []string       `form:"binary_old_name"`
	BinaryFileNewName  []string       `form:"binary_new_name"`
	Expire             ExpirationType `validate:"omitempty,oneof=never 1hour 12hours 1day 7days 15days custom" form:"expire"`
	ExpireAt           string         `validate:"expirationdate" form:"expire_at"`
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
	files, _, err := gist.Files("HEAD", true)
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
		GistID:      gist.ID,
		UserID:      gist.UserID,
		Visibility:  gist.Private.Uint(),
		Username:    gist.User.Username,
		Description: gist.Description,
		Title:       gist.Title,
		Content:     wholeContent,
		Filenames:   fileNames,
		Extensions:  exts,
		Languages:   langs,
		Topics:      topics,
		CreatedAt:   gist.CreatedAt,
		UpdatedAt:   gist.UpdatedAt,
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
