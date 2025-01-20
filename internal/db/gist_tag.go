package db

type GistTag struct {
	GistID    uint   `gorm:"primaryKey"`
	Tag       string `gorm:"primaryKey;size:50"`
	CreatedAt int64
}
