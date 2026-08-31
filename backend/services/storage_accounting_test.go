package services

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"oneimg/backend/models"

	"gorm.io/gorm"
)

func storageBytesPointer(value int64) *int64 { return &value }

func TestMigrateStorageAccountingBackfillsLocalFilesAndIsIdempotent(t *testing.T) {
	db := newServiceTestDB(t)
	uploadsRoot := filepath.Join(t.TempDir(), "uploads")
	dataRoot := filepath.Join(t.TempDir(), "data")
	writeAccountingFile(t, filepath.Join(uploadsRoot, "2026", "photo.webp"), []byte("main"))
	writeAccountingFile(t, filepath.Join(dataRoot, "thumbnails", "2026", "photo.webp"), []byte("thumb"))

	bucket := models.Buckets{Id: 1, Name: "local", Type: "default", Usage: 999, Config: map[string]any{}}
	image := models.Image{
		Url: "/uploads/2026/photo.webp", Thumbnail: "/thumbnails/2026/photo.webp",
		FileName: "photo.webp", FileSize: 400, Storage: "default", BucketId: bucket.Id, UserId: 1,
	}
	if err := db.Create(&bucket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&image).Error; err != nil {
		t.Fatal(err)
	}

	for range 2 {
		if err := MigrateStorageAccounting(db, uploadsRoot, dataRoot); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.First(&image, image.Id).Error; err != nil {
		t.Fatal(err)
	}
	if image.StorageBytes == nil || *image.StorageBytes != 9 {
		t.Fatalf("storage_bytes = %v, want 9", image.StorageBytes)
	}
	if err := db.First(&bucket, bucket.Id).Error; err != nil {
		t.Fatal(err)
	}
	if bucket.Usage != 9 {
		t.Fatalf("bucket usage = %d, want 9", bucket.Usage)
	}
}

func TestMigrateStorageAccountingKeepsMissingAndRemoteObjectsEstimated(t *testing.T) {
	db := newServiceTestDB(t)
	buckets := []models.Buckets{
		{Id: 1, Name: "local", Type: "default", Usage: 5, Config: map[string]any{}},
		{Id: 2, Name: "remote", Type: "s3", Usage: 100, Config: map[string]any{}},
	}
	if err := db.Create(&buckets).Error; err != nil {
		t.Fatal(err)
	}
	images := []models.Image{
		{Url: "/uploads/missing.webp", FileName: "missing.webp", FileSize: 11, Storage: "default", BucketId: 1, UserId: 1},
		{Url: "/remote-old.webp", FileName: "remote-old.webp", FileSize: 60, Storage: "s3", BucketId: 2, UserId: 1},
		{Url: "/remote-new.webp", FileName: "remote-new.webp", FileSize: 70, StorageBytes: storageBytesPointer(70), Storage: "s3", BucketId: 2, UserId: 1},
	}
	if err := db.Create(&images).Error; err != nil {
		t.Fatal(err)
	}

	if err := MigrateStorageAccounting(db, filepath.Join(t.TempDir(), "uploads"), filepath.Join(t.TempDir(), "data")); err != nil {
		t.Fatal(err)
	}
	for _, image := range images[:2] {
		var stored models.Image
		if err := db.First(&stored, image.Id).Error; err != nil {
			t.Fatal(err)
		}
		if stored.StorageBytes != nil {
			t.Fatalf("image %d was incorrectly marked exact", image.Id)
		}
	}
	for id, want := range map[int]uint64{1: 11, 2: 130} {
		var bucket models.Buckets
		if err := db.First(&bucket, id).Error; err != nil {
			t.Fatal(err)
		}
		if bucket.Usage != want {
			t.Fatalf("bucket %d usage = %d, want %d", id, bucket.Usage, want)
		}
	}
}

func TestDeletingLastUnknownRecordMakesBucketExact(t *testing.T) {
	db := newServiceTestDB(t)
	bucket := models.Buckets{Id: 2, Name: "remote", Type: "s3", Usage: 100, Config: map[string]any{}}
	unknown := models.Image{FileName: "old.webp", FileSize: 60, Storage: "s3", BucketId: 2, UserId: 1}
	exact := models.Image{FileName: "new.webp", FileSize: 20, StorageBytes: storageBytesPointer(25), Storage: "s3", BucketId: 2, UserId: 1}
	if err := db.Create(&bucket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]models.Image{unknown, exact}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("file_name = ?", unknown.FileName).First(&unknown).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&unknown).Error; err != nil {
			return err
		}
		return decrementBucketUsage(tx, bucket.Id, imageStorageBytes(unknown))
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&bucket, bucket.Id).Error; err != nil {
		t.Fatal(err)
	}
	if bucket.Usage != 25 {
		t.Fatalf("bucket usage = %d, want exact 25", bucket.Usage)
	}
}

func TestStorageSummariesAreReadOnlyAndFilesystemFailureDegrades(t *testing.T) {
	db := newServiceTestDB(t)
	buckets := []models.Buckets{
		{Id: 1, Name: "local", Type: "default", Usage: 999, Config: map[string]any{}},
		{Id: 2, Name: "remote", Type: "s3", Usage: 80, Config: map[string]any{}},
	}
	if err := db.Create(&buckets).Error; err != nil {
		t.Fatal(err)
	}
	images := []models.Image{
		{FileName: "local.webp", FileSize: 10, StorageBytes: storageBytesPointer(12), Storage: "default", BucketId: 1, UserId: 1},
		{FileName: "remote.webp", FileSize: 50, Storage: "s3", BucketId: 2, UserId: 1},
	}
	if err := db.Create(&images).Error; err != nil {
		t.Fatal(err)
	}
	service := NewStorageService(db)
	service.filesystemMetrics = func(path string) (*FilesystemMetrics, error) {
		if path != "uploads" {
			t.Fatalf("filesystem path = %q, want uploads", path)
		}
		return &FilesystemMetrics{TotalBytes: 1000, UsedBytes: 400, AvailableBytes: 550}, nil
	}
	items, err := service.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].UsageBytes != 12 || !items[0].UsageExact || items[0].Filesystem == nil {
		t.Fatalf("local summary = %#v", items[0])
	}
	if items[1].UsageBytes != 80 || items[1].UsageExact || items[1].Filesystem != nil {
		t.Fatalf("remote summary = %#v", items[1])
	}
	var stored models.Buckets
	if err := db.First(&stored, 1).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Usage != 999 {
		t.Fatalf("GET-style summary changed quota counter to %d", stored.Usage)
	}

	service.filesystemMetrics = func(string) (*FilesystemMetrics, error) { return nil, errors.New("stat failed") }
	item, err := service.Get(1)
	if err != nil {
		t.Fatal(err)
	}
	if item.Filesystem != nil {
		t.Fatalf("filesystem failure should be omitted: %#v", item.Filesystem)
	}
}

func TestUploadOptionsDoesNotCollectFilesystemMetrics(t *testing.T) {
	db := newServiceTestDB(t)
	if err := db.Create(&models.Settings{DefaultStorage: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Buckets{Id: 1, Name: "local", Type: "default", Config: map[string]any{}}).Error; err != nil {
		t.Fatal(err)
	}
	called := false
	service := NewStorageService(db)
	service.filesystemMetrics = func(string) (*FilesystemMetrics, error) {
		called = true
		return nil, errors.New("must not be called")
	}
	if _, _, err := service.UploadOptions(models.User{ID: 1, Role: models.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("upload options collected filesystem metrics")
	}
}

func writeAccountingFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}
