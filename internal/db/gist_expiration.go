package db

import (
	"strings"
	"time"

	"github.com/thomiceli/opengist/internal/validator"
)

// ExpirationType is the expiration choice a user can pick when creating a
// gist: either a fixed-duration preset, "custom" (paired with an explicit
// date), or "never". The empty value and "never" both mean "never expires".
type ExpirationType string

const (
	ExpiryNever       ExpirationType = "never"
	ExpiryOneHour     ExpirationType = "1hour"
	ExpiryTwelveHours ExpirationType = "12hours"
	ExpiryOneDay      ExpirationType = "1day"
	ExpirySevenDays   ExpirationType = "7days"
	ExpiryFifteenDays ExpirationType = "15days"
	ExpiryCustom      ExpirationType = "custom"
)

// Duration returns the time span of a fixed-duration preset, or 0 for
// "never", "custom", and any unknown value (those resolve their timestamp
// elsewhere).
func (e ExpirationType) Duration() time.Duration {
	switch e {
	case ExpiryOneHour:
		return time.Hour
	case ExpiryTwelveHours:
		return 12 * time.Hour
	case ExpiryOneDay:
		return 24 * time.Hour
	case ExpirySevenDays:
		return 7 * 24 * time.Hour
	case ExpiryFifteenDays:
		return 15 * 24 * time.Hour
	default:
		return 0
	}
}

// ExpiresAtTimestamp converts a fixed-duration preset into an absolute Unix
// expiration timestamp relative to now, returning 0 for "never"/"custom".
// Use GistDTO.ExpiresAtTimestamp to resolve a custom date.
func (e ExpirationType) ExpiresAtTimestamp() int64 {
	d := e.Duration()
	if d == 0 {
		return 0
	}
	return time.Now().Add(d).Unix()
}

// ExpiresAtTimestamp resolves the gist's absolute expiration time (Unix
// seconds) from the chosen preset, or from the custom date when Expire is
// "custom". Returns 0 when the gist never expires or the custom date can't be
// parsed - callers validate the DTO (the `expirationdate` rule) beforehand, so
// an unparseable value here only happens on the non-validated push-option path,
// where "never" is the safe fallback.
func (dto *GistDTO) ExpiresAtTimestamp() int64 {
	if dto.Expire != ExpiryCustom {
		return dto.Expire.ExpiresAtTimestamp()
	}

	t, err := validator.ParseDateTime(strings.TrimSpace(dto.ExpireAt))
	if err != nil {
		return 0
	}
	return t.Unix()
}

func (gist *Gist) IsExpired() bool {
	return gist.ExpiresAt > 0 && gist.ExpiresAt <= time.Now().Unix()
}

func DeleteExpiredGists() ([]*Gist, error) {
	var gists []*Gist
	err := db.Preload("User").
		Where("expires_at > 0 AND expires_at <= ?", time.Now().Unix()).
		Find(&gists).Error
	if err != nil {
		return nil, err
	}

	if len(gists) == 0 {
		return nil, nil
	}

	if err := db.Delete(&gists).Error; err != nil {
		return nil, err
	}

	return gists, nil
}
