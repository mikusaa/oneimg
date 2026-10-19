package app

import (
	"path/filepath"
	"testing"

	"oneimg/backend/database"
	"oneimg/backend/models"

	"gorm.io/driver/sqlite"
)

func storageInitDB(t *testing.T) *database.Database {
	t.Helper()
	db, err := database.NewDB(sqlite.Open(filepath.Join(t.TempDir(), "data.db")))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.DB.AutoMigrate(&models.Buckets{}, &models.Image{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestInitDefaultStorageNamesNewLocalStorage(t *testing.T) {
	db := storageInitDB(t)
	InitDefaultStorage(db)

	var buckets []models.Buckets
	if err := db.DB.Find(&buckets).Error; err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 1 || buckets[0].Id != 1 || buckets[0].Type != "default" || buckets[0].Name != LocalStorageName {
		t.Fatalf("new install storage = %+v", buckets)
	}
}

// 已有安装保留原来的名称（包括旧默认名「本地默认存储」和管理员改过的名称）
func TestInitDefaultStorageKeepsExistingName(t *testing.T) {
	db := storageInitDB(t)
	existing := models.Buckets{Id: 1, Name: "本地默认存储", Type: "default", Config: map[string]any{"storagePath": "/uploads"}}
	if err := db.DB.Create(&existing).Error; err != nil {
		t.Fatal(err)
	}
	InitDefaultStorage(db)

	var buckets []models.Buckets
	if err := db.DB.Find(&buckets).Error; err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 1 || buckets[0].Name != "本地默认存储" {
		t.Fatalf("existing storage changed: %+v", buckets)
	}
}
