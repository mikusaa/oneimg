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
	if err := applyImageSearch(db.Model(&models.Image{}).Where("images.user_id = ?", 2), "summer-photo").Find(&scoped).Error; err != nil {
		t.Fatalf("scoped Find() error = %v", err)
	}
	if len(scoped) != 0 {
		t.Fatalf("scoped search got %+v, want no images outside user scope", scoped)
	}
}

func TestApplyImageBucketFilterUsesPrimaryBucket(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&models.Image{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	images := []models.Image{
		{Url: "/local.webp", FileName: "local.webp", FileSize: 1, BucketId: 1, UserId: 1, Storage: "default"},
		{Url: "/remote.webp", FileName: "remote.webp", FileSize: 1, BucketId: 2, UserId: 1, Storage: "s3"},
	}
	if err := db.Create(&images).Error; err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	var got []models.Image
	if err := applyImageBucketFilter(db.Model(&models.Image{}), "2").Find(&got).Error; err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if len(got) != 1 || got[0].BucketId != 2 {
		t.Fatalf("bucket filter got %+v, want bucket 2 image", got)
	}
	if db.Migrator().HasTable("image_storages") {
		t.Fatal("bucket filtering must not require the legacy image_storages table")
	}
}

func TestFileNameFromURLUsesURLPath(t *testing.T) {
	got := fileNameFromURL("https://example.com/images/original.png?token=abc")
	if got != "original.png" {
		t.Fatalf("fileNameFromURL() = %q, want original.png", got)
	}
}
