package db

import "errors"

var ErrInitGistAlreadyConsumed = errors.New("init gist already consumed")

// GistInitQueue tracks gists created by the "git push .../init" flow between the
// two HTTP requests git performs (info/refs then git-receive-pack). Each entry
// carries a Token that correlates those two requests so the receive-pack step
// targets the exact gist created by its matching info/refs step, instead of
// guessing from a per-user FIFO.
type GistInitQueue struct {
	GistID uint   `gorm:"primaryKey"`
	Gist   Gist   `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:GistID"`
	UserID uint   `gorm:"primaryKey"`
	User   User   `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:UserID"`
	Token  string `gorm:"uniqueIndex"`
}

func AddInitGistToQueue(gistID uint, userID uint, token string) error {
	queue := &GistInitQueue{
		GistID: gistID,
		UserID: userID,
		Token:  token,
	}
	return db.Create(&queue).Error
}

func GetInitGistByToken(token string) (*Gist, error) {
	queue := new(GistInitQueue)
	err := db.Preload("Gist").Preload("Gist.User").
		Where("token = ?", token).
		First(&queue).Error
	if err != nil {
		return nil, err
	}
	return &queue.Gist, nil
}

func PopInitGistByToken(token string) (*Gist, error) {
	queue := new(GistInitQueue)
	if err := db.Preload("Gist").Preload("Gist.User").
		Where("token = ?", token).
		First(&queue).Error; err != nil {
		return nil, err
	}

	res := db.Where("token = ?", token).Delete(&GistInitQueue{})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrInitGistAlreadyConsumed
	}

	return &queue.Gist, nil
}

func PopInitGistForUser(userID uint) (*Gist, error) {
	for {
		queue := new(GistInitQueue)
		if err := db.Preload("Gist").Preload("Gist.User").
			Where("user_id = ?", userID).
			Order("gist_id asc").
			First(&queue).Error; err != nil {
			return nil, err
		}

		res := db.Where("gist_id = ? AND user_id = ?", queue.GistID, userID).
			Delete(&GistInitQueue{})
		if res.Error != nil {
			return nil, res.Error
		}
		if res.RowsAffected > 0 {
			return &queue.Gist, nil
		}
		// Lost the race for this entry; try the next oldest one.
	}
}
