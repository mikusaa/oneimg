package database

import (
	"path/filepath"
	"testing"

	"oneimg/backend/config"
	"oneimg/backend/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestInitDBDoesNotCreateStorageSyncSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.db")
	InitDB(&config.Config{SqlitePath: path})

	if db.DB.Migrator().HasTable("image_storages") {
		t.Fatal("new database should not create image_storages")
	}
	if db.DB.Migrator().HasColumn(&models.Settings{}, "multi_storage_sync") {
		t.Fatal("new database should not create multi_storage_sync")
	}
}

func TestInitDBPreservesLegacyStorageSyncSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := legacy.Exec("CREATE TABLE settings (id integer PRIMARY KEY, multi_storage_sync numeric DEFAULT 0)").Error; err != nil {
		t.Fatal(err)
	}
	if err := legacy.Exec("CREATE TABLE image_storages (id integer PRIMARY KEY, image_id integer, bucket_id integer)").Error; err != nil {
		t.Fatal(err)
	}
	sqlDB, err := legacy.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}

	InitDB(&config.Config{SqlitePath: path})
	if !db.DB.Migrator().HasTable("image_storages") {
		t.Fatal("legacy image_storages table should be preserved")
	}
	if !db.DB.Migrator().HasColumn(&models.Settings{}, "multi_storage_sync") {
		t.Fatal("legacy multi_storage_sync column should be preserved")
	}
}

func TestInitDBAddsTagIndexWithoutRebuildingLinks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tags.db")
	legacy, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	// 旧版本的关联表只有 (image_id, tag_id) 主键
	if err := legacy.Exec("CREATE TABLE image_to_tags (image_id integer, tag_id integer, PRIMARY KEY (image_id, tag_id))").Error; err != nil {
		t.Fatal(err)
	}
	if err := legacy.Exec("INSERT INTO image_to_tags (image_id, tag_id) VALUES (1, 2), (3, 2), (3, 4)").Error; err != nil {
		t.Fatal(err)
	}
	sqlDB, err := legacy.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}

	InitDB(&config.Config{SqlitePath: path})
	if !db.DB.Migrator().HasIndex(&models.ImageToTags{}, "idx_image_to_tags_tag_id") {
		t.Fatal("image_to_tags should have a tag_id index")
	}
	var links int64
	if err := db.DB.Model(&models.ImageToTags{}).Count(&links).Error; err != nil {
		t.Fatal(err)
	}
	if links != 3 {
		t.Fatalf("image_to_tags rows = %d, want 3", links)
	}
	if err := db.DB.Create(&models.ImageToTags{ImageId: 1, TagId: 2}).Error; err == nil {
		t.Fatal("composite primary key should still reject duplicate links")
	}
}
