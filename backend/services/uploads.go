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

type UploadService struct{ db *gorm.DB }

type UploadFileResult struct {
	FileName string
	ImageID  int
	Result   *interfaces.ImageUploadResult
	Error    error
}

func NewUploadService(db *gorm.DB) *UploadService { return &UploadService{db: db} }

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
	uploader, err := uploads.GetStorageUploader(&bucket)
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
			image = models.Image{Url: uploaded.URL, Thumbnail: uploaded.ThumbnailURL, FileName: uploaded.FileName,
				OriginalFileName: uploaded.OriginalFileName, OriginalFileSize: uploaded.OriginalFileSize,
				FileSize: uploaded.FileSize, MimeType: uploaded.MimeType, Width: uploaded.Width, Height: uploaded.Height,
				Storage: uploaded.Storage, BucketId: bucketID, UserId: user.ID, ContentHash: uploaded.ContentHash}
			if err := s.db.Create(&image).Error; err != nil {
				_ = deleteStoredImage(context.Background(), image, bucket)
				item.Error = fmt.Errorf("save image record: %w", err)
				results = append(results, item)
				continue
			}
			uploaded.ID = image.Id
			if uploaded.Storage != "default" {
				size := uint64(uploaded.FileSize + uploaded.ThumbnailSize)
				updated := s.db.Model(&models.Buckets{}).Where("id = ? AND (capacity = 0 OR usage + ? <= capacity)", bucketID, size).UpdateColumn("usage", gorm.Expr("usage + ?", size))
				if updated.Error != nil || updated.RowsAffected == 0 {
					_ = deleteStoredImage(context.Background(), image, bucket)
					_ = s.db.Delete(&image).Error
					item.Error = errors.New("storage capacity exceeded")
					results = append(results, item)
					continue
				}
			}
		}
		if len(tagIDs) > 0 {
			links := make([]models.ImageToTags, 0, len(tagIDs))
			for _, tagID := range tagIDs {
				links = append(links, models.ImageToTags{ImageId: image.Id, TagId: tagID})
			}
			if err := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&links).Error; err != nil {
				item.Error = fmt.Errorf("save image tags: %w", err)
				results = append(results, item)
				continue
			}
		}
		item.ImageID, item.Result = image.Id, uploaded
		results = append(results, item)
	}
	return results, nil
}

var ErrBucketAccess = errors.New("storage bucket access denied")
