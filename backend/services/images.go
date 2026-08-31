package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"oneimg/backend/models"
	utilsBuckets "oneimg/backend/utils/buckets"
	"oneimg/backend/utils/ftp"
	"oneimg/backend/utils/publicurl"
	"oneimg/backend/utils/s3"
	"oneimg/backend/utils/webdav"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"gorm.io/gorm"
)

var ErrStorageDelete = errors.New("stored image could not be deleted")

type ImageService struct{ db *gorm.DB }

type ImageListQuery struct {
	Page         int
	PageSize     int
	Sort         string
	Order        string
	Search       string
	BucketID     *int
	TagIDs       []int
	Untagged     bool
	UploaderRole *int
	User         models.User
}

type ImageRecord struct {
	Image        models.Image
	Tags         []models.Tags
	UploaderRole int
}

func NewImageService(db *gorm.DB) *ImageService { return &ImageService{db: db} }

func (s *ImageService) List(input ImageListQuery) ([]ImageRecord, int64, error) {
	query := s.db.Model(&models.Image{}).Select("images.id")
	if input.User.Role != models.RoleAdmin {
		query = query.Where("images.user_id = ?", input.User.ID)
	}
	if input.BucketID != nil {
		query = query.Where("images.bucket_id = ?", *input.BucketID)
	}
	if strings.TrimSpace(input.Search) != "" {
		pattern := "%" + strings.TrimSpace(input.Search) + "%"
		query = query.Where("(images.file_name LIKE ? OR images.original_filename LIKE ? OR images.url LIKE ? OR images.content_hash LIKE ?)", pattern, pattern, pattern, pattern)
	}
	if input.UploaderRole != nil {
		query = query.Joins("JOIN users uploader ON uploader.id = images.user_id").Where("uploader.role = ?", *input.UploaderRole)
	}
	if input.Untagged {
		query = query.Where("NOT EXISTS (SELECT 1 FROM image_to_tags WHERE image_to_tags.image_id = images.id)")
	}
	if len(input.TagIDs) > 0 {
		query = query.Where("EXISTS (SELECT 1 FROM image_to_tags WHERE image_to_tags.image_id = images.id AND image_to_tags.tag_id IN ?)", input.TagIDs)
	}

	var total int64
	if err := query.Distinct("images.id").Count(&total).Error; err != nil {
		return nil, 0, err
	}
	sortFields := map[string]string{"created_at": "images.created_at", "file_size": "images.file_size", "filename": "images.file_name", "id": "images.id"}
	orderClause := sortFields[input.Sort] + " " + input.Order
	ids := make([]int, 0)
	if err := query.Distinct("images.id").Order(orderClause).Offset((input.Page - 1) * input.PageSize).Limit(input.PageSize).Find(&ids).Error; err != nil {
		return nil, 0, err
	}
	if len(ids) == 0 {
		return []ImageRecord{}, total, nil
	}

	var images []models.Image
	fetchOrderFields := map[string]string{"created_at": "created_at", "file_size": "file_size", "filename": "file_name", "id": "id"}
	if err := s.db.Where("id IN ?", ids).Order(fetchOrderFields[input.Sort] + " " + input.Order).Find(&images).Error; err != nil {
		return nil, 0, err
	}
	records, err := s.attachMetadata(images)
	if err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

func (s *ImageService) Get(id int) (ImageRecord, error) {
	var image models.Image
	if err := s.db.First(&image, id).Error; err != nil {
		return ImageRecord{}, err
	}
	items, err := s.attachMetadata([]models.Image{image})
	if err != nil {
		return ImageRecord{}, err
	}
	return items[0], nil
}

func (s *ImageService) GetMany(ids []int) ([]models.Image, error) {
	items := make([]models.Image, 0)
	err := s.db.Where("id IN ?", uniqueInts(ids)).Find(&items).Error
	if err == nil && len(items) != len(uniqueInts(ids)) {
		err = gorm.ErrRecordNotFound
	}
	return items, err
}

func (s *ImageService) attachMetadata(images []models.Image) ([]ImageRecord, error) {
	ids := make([]int, 0, len(images))
	userIDs := make([]int, 0, len(images))
	for _, image := range images {
		ids = append(ids, image.Id)
		userIDs = append(userIDs, image.UserId)
	}
	var links []models.ImageToTags
	if err := s.db.Where("image_id IN ?", ids).Find(&links).Error; err != nil {
		return nil, err
	}
	tagIDs := make([]int, 0, len(links))
	for _, link := range links {
		tagIDs = append(tagIDs, link.TagId)
	}
	var tags []models.Tags
	if len(tagIDs) > 0 {
		if err := s.db.Where("id IN ?", uniqueInts(tagIDs)).Find(&tags).Error; err != nil {
			return nil, err
		}
	}
	tagByID := make(map[int]models.Tags, len(tags))
	for _, tag := range tags {
		tagByID[tag.Id] = tag
	}
	tagsByImage := make(map[int][]models.Tags, len(images))
	for _, link := range links {
		if tag, ok := tagByID[link.TagId]; ok {
			tagsByImage[link.ImageId] = append(tagsByImage[link.ImageId], tag)
		}
	}
	var users []models.User
	if err := s.db.Select("id", "role").Where("id IN ?", uniqueInts(userIDs)).Find(&users).Error; err != nil {
		return nil, err
	}
	roleByUser := make(map[int]int, len(users))
	for _, user := range users {
		roleByUser[user.ID] = user.Role
	}
	var setting models.Settings
	if err := s.db.First(&setting).Error; err != nil {
		return nil, err
	}
	result := make([]ImageRecord, 0, len(images))
	for _, image := range images {
		image.Url = publicurl.BuildCDNForStorage(setting, image.Storage, publicurl.BuildForStorage(setting, image.Storage, image.BucketId, image.Url))
		image.Thumbnail = publicurl.BuildCDNForStorage(setting, image.Storage, publicurl.BuildForStorage(setting, image.Storage, image.BucketId, image.Thumbnail))
		itemTags := tagsByImage[image.Id]
		if itemTags == nil {
			itemTags = []models.Tags{}
		}
		result = append(result, ImageRecord{Image: image, Tags: itemTags, UploaderRole: roleByUser[image.UserId]})
	}
	return result, nil
}

func CanAccessImage(user models.User, image models.Image, permission string) bool {
	if image.UserId == models.SuperAdminID && user.ID != models.SuperAdminID {
		return false
	}
	if user.ID == models.SuperAdminID || image.UserId == user.ID {
		return true
	}
	return user.Role == models.RoleAdmin && (permission == "" || user.Permission.HasPermission(permission))
}

func (s *ImageService) Delete(ctx context.Context, id int) error {
	var image models.Image
	if err := s.db.First(&image, id).Error; err != nil {
		return err
	}
	var bucket models.Buckets
	if err := s.db.First(&bucket, image.BucketId).Error; err != nil {
		return err
	}
	if err := deleteStoredImage(ctx, image, bucket); err != nil {
		return fmt.Errorf("%w: %v", ErrStorageDelete, err)
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("image_id = ?", image.Id).Delete(&models.ImageToTags{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&image).Error; err != nil {
			return err
		}
		return decrementBucketUsage(tx, image.BucketId, imageStorageBytes(image))
	})
}

func deleteStoredImage(ctx context.Context, image models.Image, bucket models.Buckets) error {
	deletePair := func(deleteOne func(string) error) error {
		if err := deleteOne(image.Url); err != nil {
			return err
		}
		if strings.TrimSpace(image.Thumbnail) != "" {
			return deleteOne(image.Thumbnail)
		}
		return nil
	}
	switch image.Storage {
	case "default":
		return deletePair(func(value string) error {
			clean := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(value)), "/")
			if strings.HasPrefix(clean, "thumbnails/") {
				clean = filepath.Join("data", filepath.FromSlash(clean))
			} else {
				clean = filepath.Join("uploads", filepath.FromSlash(strings.TrimPrefix(clean, "uploads/")))
			}
			err := os.Remove(clean)
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		})
	case "s3", "r2":
		var setting models.Settings
		client, err := s3.NewS3Client(setting, bucket)
		if err != nil {
			return err
		}
		bucketName := utilsBuckets.ConvertToS3Bucket(bucket.Config).S3Bucket
		if image.Storage == "r2" {
			bucketName = utilsBuckets.ConvertToR2Bucket(bucket.Config).R2Bucket
		}
		return deletePair(func(value string) error {
			_, err := client.DeleteObject(ctx, &awss3.DeleteObjectInput{Bucket: aws.String(bucketName), Key: aws.String(strings.TrimPrefix(value, "/"))})
			return err
		})
	case "webdav":
		config := utilsBuckets.ConvertToWebDavBucket(bucket.Config)
		client := webdav.Client(webdav.Config{BaseURL: config.WebdavURL, Username: config.WebdavUser, Password: config.WebdavPass, Timeout: 30 * time.Second})
		return deletePair(func(value string) error { return client.WebDAVDelete(ctx, value) })
	case "ftp":
		config := utilsBuckets.ConvertToFTPBucket(bucket.Config)
		client := ftp.NewFTPUtil(ftp.FTPConfig{Host: config.FTPHost, Port: config.FTPPort, User: config.FTPUser, Password: config.FTPPass, Timeout: 30})
		defer client.Close()
		return deletePair(client.DeleteImage)
	default:
		return fmt.Errorf("unsupported storage type %q", image.Storage)
	}
}
