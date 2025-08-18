package db

type GistInitQueue struct {
	GistID uint `gorm:"primaryKey"`
	Gist   Gist `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:GistID"`
	UserID uint `gorm:"primaryKey"`
	User   User `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:UserID"`
}

func GetInitGistInQueueForUser(userID uint) (*Gist, error) {
	queue := new(GistInitQueue)
	err := db.Preload("Gist").Preload("Gist.User").
		Where("user_id = ?", userID).
		Order("gist_id asc").
		First(&queue).Error
	if err != nil {
		return nil, err
	}

	err = db.Delete(&queue).Error
	if err != nil {
		return nil, err
	}

	return &queue.Gist, nil
}

func AddInitGistToQueue(gistID uint, userID uint) error {
	queue := &GistInitQueue{
		GistID: gistID,
		UserID: userID,
	}
	return db.Create(&queue).Error
}
