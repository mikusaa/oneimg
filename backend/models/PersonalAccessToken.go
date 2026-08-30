package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// StringList stores a JSON string array in SQLite.
type StringList []string

func (s StringList) Value() (driver.Value, error) {
	if s == nil {
		s = StringList{}
	}
	return json.Marshal(s)
}

func (s *StringList) Scan(src any) error {
	if src == nil {
		*s = StringList{}
		return nil
	}
	var raw []byte
	switch value := src.(type) {
	case []byte:
		raw = value
	case string:
		raw = []byte(value)
	default:
		return errors.New("invalid JSON source for StringList")
	}
	if len(raw) == 0 || string(raw) == "null" {
		*s = StringList{}
		return nil
	}
	return json.Unmarshal(raw, s)
}

type PersonalAccessToken struct {
	ID         uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID     int        `json:"user_id" gorm:"not null;index"`
	Name       string     `json:"name" gorm:"not null;size:50"`
	Prefix     string     `json:"prefix" gorm:"not null;size:32;uniqueIndex"`
	SecretHash string     `json:"-" gorm:"not null;size:64"`
	Scopes     StringList `json:"scopes" gorm:"type:text;not null"`
	ExpiresAt  *time.Time `json:"expires_at" gorm:"index"`
	LastUsedAt *time.Time `json:"last_used_at"`
	RevokedAt  *time.Time `json:"revoked_at" gorm:"index"`
	CreatedAt  time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (t PersonalAccessToken) HasScope(scope string) bool {
	for _, current := range t.Scopes {
		if current == scope {
			return true
		}
	}
	return false
}
