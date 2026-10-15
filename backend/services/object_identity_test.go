package services

import (
	"context"
	"os"
	"sync"
	"testing"

	"oneimg/backend/models"
)

func TestHistoricalCrossReferencesAndConcurrentDeletion(t *testing.T) {
	t.Chdir(t.TempDir())
	db := newServiceTestDB(t)
	if err := db.Create(&models.Buckets{Id: 1, Name: "local", Type: "default", Usage: 20}).Error; err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"uploads/shared.png", "uploads/b.png", "data/thumbnails/only-a.webp"} {
		writeAccountingFile(t, file, []byte("original"))
	}
	records := []models.Image{
		{Url: "/uploads/shared.png", Thumbnail: "/thumbnails/only-a.webp", Storage: "default", BucketId: 1, FileSize: 10},
		{Url: "uploads/b.png", Thumbnail: "uploads/shared.png", Storage: "default", BucketId: 1, FileSize: 10},
	}
	if err := db.Create(&records).Error; err != nil {
		t.Fatal(err)
	}
	if count, err := CountSharedPhysicalObjects(db); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	service := NewImageService(db)
	if err := service.Delete(context.Background(), records[0].Id); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("uploads/shared.png"); err != nil {
		t.Fatal("shared main/thumbnail removed")
	}
	if _, err := os.Stat("data/thumbnails/only-a.webp"); !os.IsNotExist(err) {
		t.Fatal("unreferenced thumbnail retained")
	}
	if err := service.Delete(context.Background(), records[1].Id); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("uploads/shared.png"); !os.IsNotExist(err) {
		t.Fatal("last reference not cleaned")
	}
	for i := range records {
		records[i].Id = 0
		records[i].Url = "/uploads/shared.png"
		records[i].Thumbnail = ""
	}
	writeAccountingFile(t, "uploads/shared.png", []byte("new"))
	if err := db.Create(&records).Error; err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, record := range records {
		wg.Add(1)
		go func(id int) { defer wg.Done(); errs <- service.Delete(context.Background(), id) }(record.Id)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	objectLocks.Lock()
	defer objectLocks.Unlock()
	if len(objectLocks.entries) != 0 {
		t.Fatal("object lock entries leaked")
	}
}

func TestPhysicalIdentityAcrossLogicalBuckets(t *testing.T) {
	db := newServiceTestDB(t)
	cfg := map[string]any{"s3_endpoint": "https://storage.example.test", "s3_bucket": "media"}
	a := models.Buckets{Id: 2, Name: "a", Type: "s3", Config: cfg}
	b := models.Buckets{Id: 3, Name: "b", Type: "s3", Config: cfg}
	if err := db.Create(&[]models.Buckets{a, b}).Error; err != nil {
		t.Fatal(err)
	}
	records := []models.Image{{Url: "/same.png", Storage: "s3", BucketId: 2}, {Thumbnail: "same.png", Storage: "s3", BucketId: 3}}
	if err := db.Create(&records).Error; err != nil {
		t.Fatal(err)
	}
	target, err := unreferencedImageObjects(db, records[0], a, []int{records[0].Id})
	if err != nil || target.Url != "" {
		t.Fatal("logical bucket alias defeated reference protection", err)
	}
}

func TestCustomLocalPathDeletionUsesActualObject(t *testing.T) {
	t.Chdir(t.TempDir())
	db := newServiceTestDB(t)
	if err := db.Create(&models.Buckets{Id: 1, Type: "default", Usage: 10}).Error; err != nil {
		t.Fatal(err)
	}
	writeAccountingFile(t, "custom/photo.png", []byte("actual object"))
	writeAccountingFile(t, "uploads/custom/photo.png", []byte("unrelated object"))
	record := models.Image{Url: "/custom/photo.png", Storage: "default", BucketId: 1, FileSize: 10}
	if err := db.Create(&record).Error; err != nil {
		t.Fatal(err)
	}
	if err := NewImageService(db).Delete(context.Background(), record.Id); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("custom/photo.png"); !os.IsNotExist(err) {
		t.Fatal("actual object was not deleted", err)
	}
	data, err := os.ReadFile("uploads/custom/photo.png")
	if err != nil || string(data) != "unrelated object" {
		t.Fatal("a different object was deleted", err)
	}
}
