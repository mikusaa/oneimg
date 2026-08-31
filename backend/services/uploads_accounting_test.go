package services

import (
	"context"
	"errors"
	"mime/multipart"
	"testing"

	"oneimg/backend/interfaces"
	"oneimg/backend/models"

	"github.com/gin-gonic/gin"
)

type accountingUploader struct {
	storage       string
	fileSize      int64
	thumbnailSize int64
}

func (u accountingUploader) Upload(_ *gin.Context, _ *models.Settings, _ *models.Buckets, file *multipart.FileHeader) (*interfaces.ImageUploadResult, error) {
	return &interfaces.ImageUploadResult{
		Success: true, URL: "/" + file.Filename, ThumbnailURL: "/thumb-" + file.Filename,
		Storage: u.storage, FileName: file.Filename, FileSize: u.fileSize, ThumbnailSize: u.thumbnailSize,
	}, nil
}

func TestUploadAccountingIsTransactionalForDefaultAndRemoteBuckets(t *testing.T) {
	for _, test := range []struct {
		name, storage string
		bucketID      int
	}{
		{name: "default", storage: "default", bucketID: 1},
		{name: "remote", storage: "s3", bucketID: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := newServiceTestDB(t)
			if err := db.Create(&models.Settings{DefaultStorage: 1}).Error; err != nil {
				t.Fatal(err)
			}
			buckets := []models.Buckets{{Id: 1, Name: "local", Type: "default", Capacity: 50, Config: map[string]any{}}}
			if test.bucketID == 2 {
				buckets = append(buckets, models.Buckets{Id: 2, Name: "remote", Type: "s3", Capacity: 50, Config: map[string]any{}})
			}
			if err := db.Create(&buckets).Error; err != nil {
				t.Fatal(err)
			}
			tag := models.Tags{Name: "tag"}
			if err := db.Create(&tag).Error; err != nil {
				t.Fatal(err)
			}
			cleanupCalls := 0
			service := NewUploadService(db)
			service.storageUploader = func(*models.Buckets) (interfaces.StorageUploader, error) {
				return accountingUploader{storage: test.storage, fileSize: 40, thumbnailSize: 10}, nil
			}
			service.deleteUploaded = func(context.Context, models.Image, models.Buckets) error {
				cleanupCalls++
				return nil
			}
			user := models.User{ID: 1, Role: models.RoleAdmin}
			files := []*multipart.FileHeader{{Filename: "first.webp"}, {Filename: "second.webp"}}
			results, err := service.UploadBatch(&gin.Context{}, user, files, test.bucketID, []int{tag.Id})
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != 2 || results[0].Error != nil || !errors.Is(results[1].Error, ErrStorageCapacityExceeded) {
				t.Fatalf("results = %#v", results)
			}
			if cleanupCalls != 1 {
				t.Fatalf("cleanup calls = %d, want 1", cleanupCalls)
			}
			var bucket models.Buckets
			if err := db.First(&bucket, test.bucketID).Error; err != nil {
				t.Fatal(err)
			}
			if bucket.Usage != 50 {
				t.Fatalf("bucket usage = %d, want 50", bucket.Usage)
			}
			var images []models.Image
			if err := db.Where("bucket_id = ?", test.bucketID).Find(&images).Error; err != nil {
				t.Fatal(err)
			}
			if len(images) != 1 || images[0].StorageBytes == nil || *images[0].StorageBytes != 50 {
				t.Fatalf("stored images = %#v", images)
			}
			var links int64
			if err := db.Model(&models.ImageToTags{}).Where("image_id = ?", images[0].Id).Count(&links).Error; err != nil {
				t.Fatal(err)
			}
			if links != 1 {
				t.Fatalf("tag links = %d, want 1", links)
			}
		})
	}
}

func TestUploadWithoutStoredThumbnailCountsOnlyMainImage(t *testing.T) {
	db := newServiceTestDB(t)
	if err := db.Create(&models.Settings{DefaultStorage: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Buckets{Id: 1, Name: "local", Type: "default", Config: map[string]any{}}).Error; err != nil {
		t.Fatal(err)
	}
	service := NewUploadService(db)
	service.storageUploader = func(*models.Buckets) (interfaces.StorageUploader, error) {
		return accountingUploader{storage: "default", fileSize: 40, thumbnailSize: 0}, nil
	}
	service.deleteUploaded = func(context.Context, models.Image, models.Buckets) error { return nil }
	results, err := service.UploadBatch(&gin.Context{}, models.User{ID: 1, Role: models.RoleAdmin}, []*multipart.FileHeader{{Filename: "main.webp"}}, 1, nil)
	if err != nil || len(results) != 1 || results[0].Error != nil {
		t.Fatalf("results = %#v, error = %v", results, err)
	}
	var image models.Image
	if err := db.First(&image).Error; err != nil {
		t.Fatal(err)
	}
	if image.StorageBytes == nil || *image.StorageBytes != 40 {
		t.Fatalf("storage_bytes = %v, want 40", image.StorageBytes)
	}
}
