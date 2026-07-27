package db

type GistTopic struct {
	GistID uint   `gorm:"primaryKey"`
	Topic  string `gorm:"primaryKey;size:50"`
}

// TopicCount is a topic together with the number of gists tagged with it.
type TopicCount struct {
	Topic string
	Count int64
}

// GetTopicsWithCount returns topics in use, together with how many gists carry
// each, ordered from most to least used and paginated (limit is typically
// perPage+1 so callers can detect a following page). Only topics on gists
// visible to currentUserId are counted (public gists, plus the user's own
// private ones).
func GetTopicsWithCount(currentUserId uint, offset int, limit int, perPage int) ([]*TopicCount, error) {
	var topics []*TopicCount
	err := db.Model(&GistTopic{}).
		Select("gist_topics.topic as topic, count(*) as count").
		Joins("join gists on gists.id = gist_topics.gist_id").
		Where("gists.private = 0 or gists.user_id = ?", currentUserId).
		Group("gist_topics.topic").
		Order("count desc, gist_topics.topic asc").
		Limit(limit).
		Offset(offset * perPage).
		Find(&topics).Error
	return topics, err
}
