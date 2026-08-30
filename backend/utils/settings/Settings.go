package settings

import (
	"oneimg/backend/database"
	"oneimg/backend/models"
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

	return settings, nil
}
