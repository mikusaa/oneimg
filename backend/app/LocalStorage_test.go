package app

import (
	"testing"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/internal/testdb"
)

func storageInitDB(t *testing.T) *database.Database {
	t.Helper()
	db := testdb.New(t)
	return db
}

func TestInitDefaultStorageNamesNewLocalStorage(t *testing.T) {
	db := storageInitDB(t)
	if err := InitDefaultStorage(db); err != nil {
		t.Fatal(err)
	}

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
	if err := InitDefaultStorage(db); err != nil {
		t.Fatal(err)
	}

	var buckets []models.Buckets
	if err := db.DB.Find(&buckets).Error; err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 1 || buckets[0].Name != "本地默认存储" {
		t.Fatalf("existing storage changed: %+v", buckets)
	}
}

func TestInitDefaultStorageFailureCanBeRetried(t *testing.T) {
	db := storageInitDB(t)
	occupied := models.Buckets{Id: 2, Name: "occupied", Type: "webdav"}
	if err := db.DB.Create(&occupied).Error; err != nil {
		t.Fatal(err)
	}
	image := models.Image{FileName: "legacy", Storage: "s3", FileSize: 10}
	if err := db.DB.Create(&image).Error; err != nil {
		t.Fatal(err)
	}
	if err := InitDefaultStorage(db); err == nil {
		t.Fatal("fixed ID collision was ignored")
	}
	var defaults int64
	if err := db.DB.Model(&models.Buckets{}).Where("type='default'").Count(&defaults).Error; err != nil || defaults != 0 {
		t.Fatal("failed initialization left a default that would skip the retry", err)
	}
	if err := db.DB.Delete(&occupied).Error; err != nil {
		t.Fatal(err)
	}
	if err := InitDefaultStorage(db); err != nil {
		t.Fatal("initialization could not be retried", err)
	}
	if err := db.DB.First(&image, image.Id).Error; err != nil || image.BucketId != 2 {
		t.Fatal("legacy storage assignment was skipped", err)
	}
}
