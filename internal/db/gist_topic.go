package db

type GistTopic struct {
	GistID uint   `gorm:"primaryKey"`
	Topic  string `gorm:"primaryKey;size:50"`
}
