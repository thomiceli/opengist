package db

type GistLanguage struct {
	GistID   uint   `gorm:"primaryKey"`
	Language string `gorm:"primaryKey;size:100"`
}
