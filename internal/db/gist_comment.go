package db

import (
	"slices"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GistComment is a Markdown comment left by a user on a gist. Comments are
// stored in the database only, not in the gist's git repository, and are
// removed along with their gist or their author.
type GistComment struct {
	ID        uint `gorm:"primaryKey"`
	GistID    uint `gorm:"index"`
	UserID    uint `gorm:"index"`
	User      User `validate:"-"`
	Content   string
	CreatedAt int64
	UpdatedAt int64
}

// GetComments returns every comment posted on the gist, oldest first, with
// their author preloaded.
func (gist *Gist) GetComments() ([]*GistComment, error) {
	var comments []*GistComment
	err := db.Preload("User").
		Where("gist_id = ?", gist.ID).
		Order("created_at asc, id asc").
		Find(&comments).Error
	return comments, err
}

// GetCommentsPage returns a page of the gist's comments, oldest first.
// `offset` is the page index (0-based); `limit` caps the returned slice (pass
// perPage+1 for the peek-next sentinel) and `perPage` is the page size used
// for the offset arithmetic.
func (gist *Gist) GetCommentsPage(offset int, limit int, perPage int) ([]*GistComment, error) {
	var comments []*GistComment
	err := db.Preload("User").
		Where("gist_id = ?", gist.ID).
		Order("created_at asc, id asc").
		Limit(limit).
		Offset(offset * perPage).
		Find(&comments).Error
	return comments, err
}

// GetLastComments returns the last n comments posted on the gist, oldest
// first, with their author preloaded.
func (gist *Gist) GetLastComments(n int) ([]*GistComment, error) {
	var comments []*GistComment
	err := db.Preload("User").
		Where("gist_id = ?", gist.ID).
		Order("created_at desc, id desc").
		Limit(n).
		Find(&comments).Error
	slices.Reverse(comments)
	return comments, err
}

func (gist *Gist) CountComments() (int64, error) {
	var count int64
	err := db.Model(&GistComment{}).Where("gist_id = ?", gist.ID).Count(&count).Error
	return count, err
}

// Position returns how many comments were posted on the gist before this one,
// in the same oldest first order as GetCommentsPage.
func (comment *GistComment) Position() (int64, error) {
	var count int64
	err := db.Model(&GistComment{}).
		Where("gist_id = ? AND (created_at < ? OR (created_at = ? AND id < ?))",
			comment.GistID, comment.CreatedAt, comment.CreatedAt, comment.ID).
		Count(&count).Error
	return count, err
}

// GetGistComment returns the comment commentID if it belongs to the gist
// gistID, with its author preloaded.
func GetGistComment(gistID uint, commentID uint) (*GistComment, error) {
	comment := new(GistComment)
	err := db.Preload("User").
		Where("id = ? AND gist_id = ?", commentID, gistID).
		First(&comment).Error
	return comment, err
}

// Create inserts the comment and increments the gist's comment counter.
func (comment *GistComment) Create() error {
	// Same instant for both, so a fresh comment is never reported as edited
	now := time.Now().Unix()
	comment.CreatedAt = now
	comment.UpdatedAt = now

	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit(clause.Associations).Create(comment).Error; err != nil {
			return err
		}
		return tx.Model(&Gist{}).
			Where("id = ?", comment.GistID).
			UpdateColumn("nb_comments", gorm.Expr("nb_comments + 1")).Error
	})
}

// Update saves the comment's content, bumping its UpdatedAt.
func (comment *GistComment) Update() error {
	return db.Model(comment).Update("content", comment.Content).Error
}

// Delete removes the comment and decrements the gist's comment counter.
func (comment *GistComment) Delete() error {
	return db.Transaction(func(tx *gorm.DB) error {
		res := tx.Delete(&GistComment{}, comment.ID)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			// Already deleted by a concurrent request, which decremented the counter
			return nil
		}
		return tx.Model(&Gist{}).
			Where("id = ?", comment.GistID).
			UpdateColumn("nb_comments", gorm.Expr("nb_comments - 1")).Error
	})
}

// IsEdited reports whether the comment content was changed after posting.
func (comment *GistComment) IsEdited() bool {
	return comment.UpdatedAt > comment.CreatedAt
}

// CanEdit reports whether user may change the comment: only its author can.
func (comment *GistComment) CanEdit(user *User) bool {
	return user != nil && comment.UserID == user.ID
}

// CanDelete reports whether user may delete the comment: its author, the
// owner of the gist it was posted on, or an admin.
func (comment *GistComment) CanDelete(user *User, gist *Gist) bool {
	return user != nil && (comment.UserID == user.ID || gist.UserID == user.ID || user.IsAdmin)
}

// -- DTO -- //

type GistCommentDTO struct {
	Content string `validate:"required,max=3000" form:"content"`
}

// Normalize converts CRLF line endings (sent by browser textareas) to LF and
// trims leading blank lines and trailing whitespace, which don't change the
// rendered Markdown. Content made only of whitespace ends up empty, so that it
// fails the required rule.
func (dto *GistCommentDTO) Normalize() {
	content := strings.ReplaceAll(dto.Content, "\r\n", "\n")
	content = strings.TrimRight(content, " \t\r\n")
	content = strings.TrimLeft(content, "\n")
	if strings.TrimSpace(content) == "" {
		content = ""
	}
	dto.Content = content
}
