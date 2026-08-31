package database

import (
	"path/filepath"
	"testing"

	"oneimg/backend/config"
	"oneimg/backend/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestInitDBAddsOriginalFileSizeWithoutGuessingLegacyValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-image.db")
	legacy, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := legacy.Exec(`CREATE TABLE images (
		id integer PRIMARY KEY AUTOINCREMENT,
		url text NOT NULL,
		file_name text NOT NULL,
		file_size integer NOT NULL,
		bucket_id integer NOT NULL DEFAULT 1,
		user_id integer NOT NULL DEFAULT 1
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := legacy.Exec(`INSERT INTO images (url, file_name, file_size) VALUES (?, ?, ?)`, "/uploads/old.webp", "old.webp", 12345).Error; err != nil {
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
	if !db.DB.Migrator().HasColumn(&models.Image{}, "original_file_size") {
		t.Fatal("original_file_size column was not added")
	}
	if !db.DB.Migrator().HasColumn(&models.Image{}, "storage_bytes") {
		t.Fatal("storage_bytes column was not added")
	}
	var image models.Image
	if err := db.DB.First(&image).Error; err != nil {
		t.Fatal(err)
	}
	if image.OriginalFileSize != 0 {
		t.Fatalf("legacy original file size = %d, want unknown value 0", image.OriginalFileSize)
	}
	if image.StorageBytes != nil {
		t.Fatalf("legacy storage_bytes = %v, want unknown value", image.StorageBytes)
	}
}
