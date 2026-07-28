package app

import (
	"oneimg/backend/models"

	"gorm.io/gorm"
)

// ResetRemovedStorageDefaults keeps upgrades usable without deleting legacy
// bucket or image records belonging to removed storage implementations.
func ResetRemovedStorageDefaults(db *gorm.DB) error {
	removedBucketIDs := db.Model(&models.Buckets{}).
		Select("id").
		Where("type = ?", "telegram")
	return db.Model(&models.Settings{}).
		Where("default_storage IN (?)", removedBucketIDs).
		Update("default_storage", 1).Error
}
