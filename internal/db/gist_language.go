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

// GetGistLanguages returns the most common languages across every gist visible
// to currentUserId (public gists plus their own). Used to populate the language
// facet on the explore "All gists" filter.
func GetGistLanguages(currentUserId uint) ([]struct {
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
		Where("((gists.private = 0) or (gists.private > 0 and gists.user_id = ?))", currentUserId).
		Group("language").
		Order("count DESC").
		Limit(15).
		Find(&results).Error

	return results, err
}

// GetGistLanguagesByTopic returns the most common languages across the gists
// carrying `topic` that are visible to currentUserId. Used to populate the
// language facet on the /-/topics/{topic} filter so it only lists languages
// present among that topic's gists.
func GetGistLanguagesByTopic(currentUserId uint, topic string) ([]struct {
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
		Where("((gists.private = 0) or (gists.private > 0 and gists.user_id = ?))", currentUserId).
		Where("exists (select 1 from gist_topics where gist_topics.gist_id = gists.id and gist_topics.topic = ?)", topic).
		Group("language").
		Order("count DESC").
		Limit(15).
		Find(&results).Error

	return results, err
}
