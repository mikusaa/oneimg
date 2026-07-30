package models

import "time"

// PasskeyCredential stores lookup metadata and an encrypted WebAuthn credential.
type PasskeyCredential struct {
	ID             uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID         int        `json:"user_id" gorm:"not null;index;uniqueIndex:idx_passkey_user_name"`
	Name           string     `json:"name" gorm:"not null;size:50;uniqueIndex:idx_passkey_user_name,collate:nocase"`
	CredentialID   string     `json:"-" gorm:"not null;uniqueIndex;size:1024"`
	CredentialData string     `json:"-" gorm:"not null;type:text"`
	CreatedAt      time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
	LastUsedAt     *time.Time `json:"last_used_at"`
}
