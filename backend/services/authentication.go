package services

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"oneimg/backend/models"
)

var (
	ErrAuthVersion            = errors.New("authentication version is no longer current")
	ErrPasswordChangeRequired = errors.New("password change required")
	ErrInvalidBootstrapCode   = errors.New("invalid bootstrap code")
)

func rotatePassword(tx *gorm.DB, user *models.User, hash string, updates map[string]any) error {
	updates["password"] = hash
	updates["auth_version"] = gorm.Expr("auth_version + 1")
	result := tx.Model(&models.User{}).Where("id = ? AND auth_version = ?", user.ID, user.AuthVersion).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrAuthVersion
	}
	now := time.Now().UTC()
	return tx.Model(&models.PersonalAccessToken{}).Where("user_id = ? AND revoked_at IS NULL", user.ID).Update("revoked_at", now).Error
}
