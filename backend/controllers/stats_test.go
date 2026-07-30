package controllers

import (
	"testing"
	"time"

	"oneimg/backend/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCountImagesCreatedBetweenUsesLocalTimeBounds(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Image{}); err != nil {
		t.Fatal(err)
	}

	location := time.FixedZone("UTC+8", 8*60*60)
	images := []models.Image{
		{Url: "/early.webp", FileName: "early.webp", FileSize: 1, CreatedAt: time.Date(2026, 7, 30, 0, 30, 0, 0, location)},
		{Url: "/late.webp", FileName: "late.webp", FileSize: 1, CreatedAt: time.Date(2026, 7, 30, 23, 30, 0, 0, location)},
		{Url: "/previous.webp", FileName: "previous.webp", FileSize: 1, CreatedAt: time.Date(2026, 7, 29, 23, 30, 0, 0, location)},
	}
	if err := db.Create(&images).Error; err != nil {
		t.Fatal(err)
	}

	dayStart := time.Date(2026, 7, 30, 0, 0, 0, 0, location)
	if count := countImagesCreatedBetween(db.Model(&models.Image{}), dayStart, dayStart.AddDate(0, 0, 1)); count != 2 {
		t.Fatalf("today count = %d, want 2", count)
	}
}
