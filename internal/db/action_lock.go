package db

import (
	"time"

	"gorm.io/gorm/clause"
)

// ActionLock is a DB-backed lease used to single-flight an action across
// multiple Opengist instances sharing the same database. Each lock is one row
// keyed by Action (the action's identifier); LockedUntil holds the Unix
// timestamp the current lease expires at (0 = free).
type ActionLock struct {
	Action      int `gorm:"primaryKey;autoIncrement:false"`
	LockedUntil int64
}

func (ActionLock) TableName() string {
	return "action_lock"
}

// AcquireLock atomically grabs the lock for action when it is free or its lease
// has expired, extending the lease by leaseTTL. It returns true only for the
// single caller that won the row. The conditional UPDATE is what makes this
// safe across SQLite/PostgreSQL/MySQL: concurrent writers serialize on the row
// (SQLite serializes all writes), so at most one re-evaluates the
// `locked_until < now` predicate to true. leaseTTL only needs to outlast a
// normal run; it's a safety net so a crashed holder doesn't block future runs.
func AcquireLock(action int, leaseTTL time.Duration) (bool, error) {
	now := time.Now().Unix()

	if err := db.Clauses(clause.OnConflict{DoNothing: true}).
		Create(&ActionLock{Action: action, LockedUntil: 0}).Error; err != nil {
		return false, err
	}

	res := db.Model(&ActionLock{}).
		Where("action = ? AND locked_until < ?", action, now).
		Update("locked_until", time.Now().Add(leaseTTL).Unix())
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// ReleaseLock frees the lock for action so the next run can acquire it
// immediately instead of waiting for the lease to expire.
func ReleaseLock(action int) error {
	return db.Model(&ActionLock{}).
		Where("action = ?", action).
		Update("locked_until", 0).Error
}
