package app

import (
	"path/filepath"
	"testing"

	"oneimg/backend/database"
	"oneimg/backend/models"

	"gorm.io/driver/sqlite"
)

func settingsDB(t *testing.T) *database.Database {
	t.Helper()
	db, err := database.NewDB(sqlite.Open(filepath.Join(t.TempDir(), "data.db")))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.DB.AutoMigrate(&models.Settings{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestInitSettingsUsesOneImgDefaults(t *testing.T) {
	db := settingsDB(t)
	InitSettings(db)

	var setting models.Settings
	if err := db.DB.First(&setting).Error; err != nil {
		t.Fatal(err)
	}
	if setting.SEOTitle != models.DefaultSEOTitle || setting.SEODescription != models.DefaultSEODescription || setting.SEOKeywords != models.DefaultSEOKeywords {
		t.Fatalf("site defaults = %q / %q / %q", setting.SEOTitle, setting.SEODescription, setting.SEOKeywords)
	}
	// 其他字段仍使用列默认值
	if setting.DefaultStorage != 1 || setting.MainImageQuality != 85 {
		t.Fatalf("column defaults not applied: default_storage=%d quality=%d", setting.DefaultStorage, setting.MainImageQuality)
	}
}

func TestInitSettingsKeepsExistingSiteTitle(t *testing.T) {
	db := settingsDB(t)
	existing := models.Settings{SEOTitle: "初春图床"}
	if err := db.DB.Create(&existing).Error; err != nil {
		t.Fatal(err)
	}
	InitSettings(db)

	var settings []models.Settings
	if err := db.DB.Find(&settings).Error; err != nil {
		t.Fatal(err)
	}
	if len(settings) != 1 || settings[0].SEOTitle != "初春图床" {
		t.Fatalf("existing settings changed: %+v", settings)
	}
}
