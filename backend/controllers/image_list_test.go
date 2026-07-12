package controllers

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"oneimg/backend/models"
)

func TestApplyImageSearchMatchesSupportedFields(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&models.Image{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	images := []models.Image{
		{
			Url:              "/uploads/2026/07/stored.webp",
			FileName:         "stored.webp",
			OriginalFileName: "summer-photo.png",
			FileSize:         1,
			BucketId:         1,
			UserId:           1,
			Storage:          "default",
			ContentHash:      "hash-original",
			UUID:             "admin",
		},
		{
			Url:              "/uploads/2026/07/another.webp",
			FileName:         "another.webp",
			OriginalFileName: "other.png",
			FileSize:         1,
			BucketId:         1,
			UserId:           1,
			Storage:          "default",
			ContentHash:      "hash-other",
			UUID:             "admin",
		},
	}
	if err := db.Create(&images).Error; err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	tests := []struct {
		name   string
		search string
		want   string
	}{
		{name: "original filename", search: "summer-photo", want: "stored.webp"},
		{name: "stored filename", search: "stored", want: "stored.webp"},
		{name: "url", search: "2026/07/stored", want: "stored.webp"},
		{name: "content hash", search: "hash-original", want: "stored.webp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []models.Image
			if err := applyImageSearch(db.Model(&models.Image{}), tt.search).Find(&got).Error; err != nil {
				t.Fatalf("Find() error = %v", err)
			}
			if len(got) != 1 || got[0].FileName != tt.want {
				t.Fatalf("search %q got %+v, want one %s", tt.search, got, tt.want)
			}
		})
	}

	var scoped []models.Image
	if err := applyImageSearch(db.Model(&models.Image{}).Where("images.uuid = ?", "guest"), "summer-photo").Find(&scoped).Error; err != nil {
		t.Fatalf("scoped Find() error = %v", err)
	}
	if len(scoped) != 0 {
		t.Fatalf("scoped search got %+v, want no images outside uuid scope", scoped)
	}
}

func TestFileNameFromURLUsesURLPath(t *testing.T) {
	got := fileNameFromURL("https://example.com/images/original.png?token=abc")
	if got != "original.png" {
		t.Fatalf("fileNameFromURL() = %q, want original.png", got)
	}
}
