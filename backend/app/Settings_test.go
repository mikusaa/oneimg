package app

import (
	"testing"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/internal/testdb"
)

func settingsDB(t *testing.T) *database.Database {
	t.Helper()
	db := testdb.New(t)
	return db
}

func TestInitSettingsUsesOneImgDefaults(t *testing.T) {
	db := settingsDB(t)
	if err := InitSettings(db); err != nil {
		t.Fatal(err)
	}

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
	if err := InitSettings(db); err != nil {
		t.Fatal(err)
	}

	var settings []models.Settings
	if err := db.DB.Find(&settings).Error; err != nil {
		t.Fatal(err)
	}
	if len(settings) != 1 || settings[0].SEOTitle != "初春图床" {
		t.Fatalf("existing settings changed: %+v", settings)
	}
}
