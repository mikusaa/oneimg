package app

import (
	"testing"

	"oneimg/backend/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestResetRemovedStorageDefaults(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&models.Buckets{}, &models.Settings{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	localBucket := models.Buckets{Id: 1, Name: "local", Type: "default", Config: map[string]any{}}
	if err := db.Create(&localBucket).Error; err != nil {
		t.Fatalf("create local bucket: %v", err)
	}
	removedBucket := models.Buckets{Name: "removed", Type: "telegram", Config: map[string]any{}}
	if err := db.Create(&removedBucket).Error; err != nil {
		t.Fatalf("create removed bucket: %v", err)
	}
	setting := models.Settings{DefaultStorage: removedBucket.Id}
	if err := db.Create(&setting).Error; err != nil {
		t.Fatalf("create settings: %v", err)
	}

	if err := ResetRemovedStorageDefaults(db); err != nil {
		t.Fatalf("ResetRemovedStorageDefaults() error = %v", err)
	}
	if err := db.First(&setting, setting.ID).Error; err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if setting.DefaultStorage != 1 {
		t.Fatalf("default storage = %d, want 1", setting.DefaultStorage)
	}
	var removedBucketCount int64
	if err := db.Model(&models.Buckets{}).Where("id = ?", removedBucket.Id).Count(&removedBucketCount).Error; err != nil {
		t.Fatalf("count removed bucket: %v", err)
	}
	if removedBucketCount != 1 {
		t.Fatal("legacy bucket should be retained")
	}
}
