package settings

import (
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/secureconfig"
)

func GetSettings() (models.Settings, error) {
	// 获取数据库实例
	db := database.GetDB()
	if db == nil {
		return models.Settings{}, nil
	}
	var settings models.Settings
	err := db.DB.First(&settings).Error
	if err != nil {
		return settings, err
	}

	if changed, migrateErr := secureconfig.TryMigrateSettingsSecrets(&settings); migrateErr == nil && changed {
		_ = db.DB.Model(&settings).Updates(map[string]any{
			"api_token":      settings.APIToken,
			"api_token_hash": settings.APITokenHash,
		}).Error
	}

	return settings, nil
}
