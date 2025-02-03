package db

type GistLanguage struct {
	GistID   uint   `gorm:"primaryKey"`
	Language string `gorm:"primaryKey;size:100"`
}

func GetGistLanguagesForUser(fromUserId, currentUserId uint) ([]struct {
	Language string
	Count    int64
}, error) {
	var results []struct {
		Language string
		Count    int64
	}

	err := db.Model(&GistLanguage{}).
		Select("language, count(*) as count").
		Joins("JOIN gists ON gists.id = gist_languages.gist_id").
		Joins("JOIN users ON gists.user_id = users.id").
		Where("((gists.private = 0) or (gists.private > 0 and gists.user_id = ?))", currentUserId).
		Where("users.id = ?", fromUserId).
		Group("language").
		Order("count DESC").
		Limit(15).
		Find(&results).Error

	return results, err
}
