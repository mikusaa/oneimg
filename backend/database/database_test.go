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
