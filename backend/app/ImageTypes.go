package app

import (
	"gorm.io/gorm"
	"oneimg/backend/models"
	"strings"
)

func MigrateImageTypes(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var settings []models.Settings
		if err := tx.Find(&settings).Error; err != nil {
			return err
		}
		for _, setting := range settings {
			var types []string
			for _, item := range strings.Split(setting.AllowedTypes, ",") {
				item = strings.ToLower(strings.TrimSpace(item))
				if item == "image/jpeg" || item == "image/png" || item == "image/gif" || item == "image/webp" {
					types = append(types, item)
				}
			}
			if len(types) == 0 {
				types = []string{"image/jpeg", "image/png", "image/gif", "image/webp"}
			}
			if err := tx.Model(&setting).Update("allowed_types", strings.Join(types, ",")).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
