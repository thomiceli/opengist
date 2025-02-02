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

	err := gistsFromUserStatement(fromUserId, currentUserId).Model(&GistLanguage{}).
		Select("language, count(*) as count").
		Joins("JOIN gists ON gists.id = gist_languages.gist_id").
		Where("gists.user_id = ?", fromUserId).
		Group("language").
		Order("count DESC").
		Limit(15). // Added limit of 15
		Find(&results).Error

	return results, err
}
