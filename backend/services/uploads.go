package services

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"

	"oneimg/backend/interfaces"
	"oneimg/backend/models"
	"oneimg/backend/utils/uploads"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UploadService struct {
	db              *gorm.DB
	storageUploader func(*models.Buckets) (interfaces.StorageUploader, error)
	deleteUploaded  func(context.Context, models.Image, models.Buckets) error
}

type UploadFileResult struct {
	FileName string
	ImageID  int
	Result   *interfaces.ImageUploadResult
	Error    error
}

func NewUploadService(db *gorm.DB) *UploadService {
	return &UploadService{db: db, storageUploader: uploads.GetStorageUploader, deleteUploaded: deleteStoredImage}
}

func (s *UploadService) UploadBatch(c *gin.Context, user models.User, files []*multipart.FileHeader, bucketID int, tagIDs []int) ([]UploadFileResult, error) {
	var setting models.Settings
	if err := s.db.First(&setting).Error; err != nil {
		return nil, err
	}
	if bucketID == 0 {
		bucketID = setting.DefaultStorage
	}
	_, allowedBuckets, err := NewStorageService(s.db).UploadOptions(user)
	if err != nil {
		return nil, err
	}
	allowed := false
	for _, bucket := range allowedBuckets {
		if bucket.Id == bucketID {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, ErrBucketAccess
	}
	var bucket models.Buckets
	if err := s.db.First(&bucket, bucketID).Error; err != nil {
		return nil, err
	}
	tagIDs = uniqueInts(tagIDs)
	if len(tagIDs) > 0 {
		var count int64
		if err := s.db.Model(&models.Tags{}).Where("id IN ?", tagIDs).Count(&count).Error; err != nil {
			return nil, err
		}
		if count != int64(len(tagIDs)) {
			return nil, gorm.ErrRecordNotFound
		}
	}
	uploader, err := s.storageUploader(&bucket)
	if err != nil {
		return nil, err
	}
	results := make([]UploadFileResult, 0, len(files))
	for _, file := range files {
		item := UploadFileResult{FileName: file.Filename}
		uploaded, err := uploader.Upload(c, &setting, &bucket, file)
		if err != nil {
			item.Error = err
			results = append(results, item)
			continue
		}
		image := models.Image{Id: uploaded.ID}
		if !uploaded.Duplicate {
			storageBytes := uploaded.FileSize
			if storageBytes < 0 {
				storageBytes = 0
			}
			if uploaded.ThumbnailSize > 0 {
				storageBytes += uploaded.ThumbnailSize
			}
			image = models.Image{Url: uploaded.URL, Thumbnail: uploaded.ThumbnailURL, FileName: uploaded.FileName,
				OriginalFileName: uploaded.OriginalFileName, OriginalFileSize: uploaded.OriginalFileSize,
				FileSize: uploaded.FileSize, MimeType: uploaded.MimeType, Width: uploaded.Width, Height: uploaded.Height,
				Storage: uploaded.Storage, BucketId: bucketID, UserId: user.ID, ContentHash: uploaded.ContentHash,
				StorageBytes: &storageBytes}
			err := s.db.Transaction(func(tx *gorm.DB) error {
				updated := tx.Model(&models.Buckets{}).
					Where("id = ? AND (capacity = 0 OR usage + ? <= capacity)", bucketID, storageBytes).
					UpdateColumn("usage", gorm.Expr("usage + ?", storageBytes))
				if updated.Error != nil {
					return updated.Error
				}
				if updated.RowsAffected == 0 {
					return ErrStorageCapacityExceeded
				}
				if err := tx.Create(&image).Error; err != nil {
					return fmt.Errorf("save image record: %w", err)
				}
				if err := createImageTagLinks(tx, image.Id, tagIDs); err != nil {
					return fmt.Errorf("save image tags: %w", err)
				}
				return nil
			})
			if err != nil {
				_ = s.deleteUploaded(context.Background(), image, bucket)
				item.Error = err
				results = append(results, item)
				continue
			}
			uploaded.ID = image.Id
		} else if err := s.db.Transaction(func(tx *gorm.DB) error {
			return createImageTagLinks(tx, image.Id, tagIDs)
		}); err != nil {
			item.Error = fmt.Errorf("save image tags: %w", err)
			results = append(results, item)
			continue
		}
		item.ImageID, item.Result = image.Id, uploaded
		results = append(results, item)
	}
	return results, nil
}

func createImageTagLinks(tx *gorm.DB, imageID int, tagIDs []int) error {
	if len(tagIDs) == 0 {
		return nil
	}
	links := make([]models.ImageToTags, 0, len(tagIDs))
	for _, tagID := range tagIDs {
		links = append(links, models.ImageToTags{ImageId: imageID, TagId: tagID})
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&links).Error
}

var (
	ErrBucketAccess            = errors.New("storage bucket access denied")
	ErrStorageCapacityExceeded = errors.New("storage capacity exceeded")
)
