package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"oneimg/backend/models"
	utilsBuckets "oneimg/backend/utils/buckets"
	"oneimg/backend/utils/ftp"
	"oneimg/backend/utils/s3"
	"oneimg/backend/utils/secureconfig"
	"oneimg/backend/utils/webdav"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrStorageTypeInvalid   = errors.New("invalid storage type")
	ErrStorageConfigInvalid = errors.New("invalid storage configuration")
	ErrDefaultStorage       = errors.New("default storage cannot be modified")
	ErrStorageCapacity      = errors.New("storage capacity is invalid")
)

type StorageService struct{ db *gorm.DB }

type StorageInput struct {
	Name          string
	Type          string
	CapacityBytes uint64
	Config        map[string]any
}

func NewStorageService(db *gorm.DB) *StorageService { return &StorageService{db: db} }

func (s *StorageService) List() ([]models.Buckets, error) {
	items := make([]models.Buckets, 0)
	if err := s.db.Where("type <> ?", "telegram").Order("id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	for index := range items {
		items[index].Config = secureconfig.MaskBucketConfigValues(items[index].Config)
	}
	return items, nil
}

func (s *StorageService) UploadOptions(user models.User) (models.Settings, []models.Buckets, error) {
	var setting models.Settings
	if err := s.db.First(&setting).Error; err != nil {
		return setting, nil, err
	}
	var buckets []models.Buckets
	if err := s.db.Where("type <> ?", "telegram").Order("id ASC").Find(&buckets).Error; err != nil {
		return setting, nil, err
	}
	result := make([]models.Buckets, 0, len(buckets))
	for _, bucket := range buckets {
		if bucket.Id != setting.DefaultStorage {
			if bucket.Capacity > 0 && bucket.Usage >= bucket.Capacity {
				continue
			}
			if user.Role != models.RoleAdmin && !models.IntSliceContains(user.Permission.Buckets, bucket.Id) {
				continue
			}
		}
		bucket.Config = nil
		result = append(result, bucket)
	}
	return setting, result, nil
}

func (s *StorageService) Get(id int) (models.Buckets, error) {
	var item models.Buckets
	if err := s.db.First(&item, id).Error; err != nil {
		return item, err
	}
	item.Config = secureconfig.MaskBucketConfigValues(item.Config)
	return item, nil
}

func validateStorageInput(input StorageInput, allowDefault bool) (StorageInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Type = strings.ToLower(strings.TrimSpace(input.Type))
	valid := map[string]bool{"s3": true, "r2": true, "ftp": true, "webdav": true}
	if allowDefault {
		valid["default"] = true
	}
	if input.Name == "" || !valid[input.Type] {
		return input, ErrStorageTypeInvalid
	}
	if input.Type != "default" && input.CapacityBytes == 0 {
		return input, ErrStorageCapacity
	}
	required := map[string][]string{
		"s3":     {"s3_endpoint", "s3_access_key", "s3_secret_key", "s3_bucket"},
		"r2":     {"r2_endpoint", "r2_access_key", "r2_secret_key", "r2_bucket"},
		"ftp":    {"ftp_host", "ftp_port", "ftp_user", "ftp_pass"},
		"webdav": {"webdav_url", "webdav_user", "webdav_pass"},
	}
	for _, key := range required[input.Type] {
		if key == "ftp_port" {
			port := secureconfig.GetInt(input.Config, key)
			if port < 1 || port > 65535 {
				return input, ErrStorageConfigInvalid
			}
			continue
		}
		if strings.TrimSpace(secureconfig.GetString(input.Config, key)) == "" {
			return input, ErrStorageConfigInvalid
		}
	}
	return input, nil
}

func (s *StorageService) Create(input StorageInput) (models.Buckets, error) {
	input, err := validateStorageInput(input, false)
	if err != nil {
		return models.Buckets{}, err
	}
	config, err := secureconfig.EncryptBucketConfigValues(input.Config)
	if err != nil {
		return models.Buckets{}, err
	}
	item := models.Buckets{Name: input.Name, Type: input.Type, Capacity: input.CapacityBytes, Config: config}
	if err := s.db.Create(&item).Error; err != nil {
		return models.Buckets{}, err
	}
	item.Config = secureconfig.MaskBucketConfigValues(item.Config)
	return item, nil
}

func (s *StorageService) Update(id int, input StorageInput) (models.Buckets, error) {
	if id == 1 {
		return models.Buckets{}, ErrDefaultStorage
	}
	var existing models.Buckets
	if err := s.db.First(&existing, id).Error; err != nil {
		return existing, err
	}
	if input.Type != existing.Type {
		return existing, ErrStorageTypeInvalid
	}
	decrypted, err := secureconfig.DecryptBucketConfigValues(existing.Config)
	if err != nil {
		return existing, err
	}
	for key, value := range input.Config {
		if secureconfig.IsBucketSensitiveKey(key) && strings.TrimSpace(fmt.Sprint(value)) == "" {
			continue
		}
		decrypted[key] = value
	}
	input.Config = decrypted
	input, err = validateStorageInput(input, false)
	if err != nil {
		return existing, err
	}
	if input.CapacityBytes < existing.Usage {
		return existing, ErrStorageCapacity
	}
	encrypted, err := secureconfig.EncryptBucketConfigValues(input.Config)
	if err != nil {
		return existing, err
	}
	if err := s.db.Model(&existing).Updates(map[string]any{"name": input.Name, "capacity": input.CapacityBytes, "config": encrypted}).Error; err != nil {
		return existing, err
	}
	existing.Name, existing.Capacity, existing.Config = input.Name, input.CapacityBytes, secureconfig.MaskBucketConfigValues(encrypted)
	return existing, nil
}

func (s *StorageService) Delete(ctx context.Context, id int) error {
	if id == 1 {
		return ErrDefaultStorage
	}
	var bucket models.Buckets
	if err := s.db.First(&bucket, id).Error; err != nil {
		return err
	}
	var images []models.Image
	if err := s.db.Where("bucket_id = ?", id).Find(&images).Error; err != nil {
		return err
	}
	for _, image := range images {
		if err := deleteStoredImage(ctx, image, bucket); err != nil {
			return fmt.Errorf("%w: %v", ErrStorageDelete, err)
		}
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("image_id IN (SELECT id FROM images WHERE bucket_id = ?)", id).Delete(&models.ImageToTags{}).Error; err != nil {
			return err
		}
		if err := tx.Where("bucket_id = ?", id).Delete(&models.Image{}).Error; err != nil {
			return err
		}
		var users []models.User
		if err := tx.Find(&users).Error; err != nil {
			return err
		}
		for _, user := range users {
			filtered := make([]int, 0, len(user.Permission.Buckets))
			for _, bucketID := range user.Permission.Buckets {
				if bucketID != id {
					filtered = append(filtered, bucketID)
				}
			}
			if len(filtered) != len(user.Permission.Buckets) {
				user.Permission.Buckets = filtered
				if err := tx.Model(&user).Update("permission", user.Permission).Error; err != nil {
					return err
				}
			}
		}
		if err := tx.Model(&models.Settings{}).Where("default_storage = ?", id).Update("default_storage", 1).Error; err != nil {
			return err
		}
		return tx.Delete(&bucket).Error
	})
}

func (s *StorageService) TestConnection(ctx context.Context, input StorageInput, existingID *int) (string, error) {
	if existingID != nil {
		var existing models.Buckets
		if err := s.db.First(&existing, *existingID).Error; err != nil {
			return "", err
		}
		decrypted, err := secureconfig.DecryptBucketConfigValues(existing.Config)
		if err != nil {
			return "", err
		}
		for key, value := range input.Config {
			if secureconfig.IsBucketSensitiveKey(key) && strings.TrimSpace(fmt.Sprint(value)) == "" {
				continue
			}
			decrypted[key] = value
		}
		if input.Name == "" {
			input.Name = existing.Name
		}
		if input.Type == "" {
			input.Type = existing.Type
		}
		if input.CapacityBytes == 0 {
			input.CapacityBytes = existing.Capacity
		}
		input.Config = decrypted
	}
	input, err := validateStorageInput(input, true)
	if err != nil {
		return "", err
	}
	bucket := models.Buckets{Name: input.Name, Type: input.Type, Capacity: input.CapacityBytes, Config: input.Config}
	switch bucket.Type {
	case "default":
		if err := os.MkdirAll("uploads", 0755); err != nil {
			return "", err
		}
		file, err := os.CreateTemp("uploads", ".oneimg-storage-test-*")
		if err != nil {
			return "", err
		}
		name := file.Name()
		defer os.Remove(name)
		if _, err := file.WriteString("oneimg storage test"); err != nil {
			file.Close()
			return "", err
		}
		if err := file.Close(); err != nil {
			return "", err
		}
		return "local storage is writable", nil
	case "s3", "r2":
		client, err := s3.NewS3Client(models.Settings{}, bucket)
		if err != nil {
			return "", err
		}
		bucketName := utilsBuckets.ConvertToS3Bucket(bucket.Config).S3Bucket
		if bucket.Type == "r2" {
			bucketName = utilsBuckets.ConvertToR2Bucket(bucket.Config).R2Bucket
		}
		key := ".oneimg-connection-test/" + uuid.NewString() + ".txt"
		if _, err := client.PutObject(ctx, &awss3.PutObjectInput{Bucket: aws.String(bucketName), Key: aws.String(key), Body: bytes.NewReader([]byte("oneimg storage test"))}); err != nil {
			return "", err
		}
		if _, err := client.DeleteObject(ctx, &awss3.DeleteObjectInput{Bucket: aws.String(bucketName), Key: aws.String(key)}); err != nil {
			return "", err
		}
		return "object write and delete succeeded", nil
	case "ftp":
		config := utilsBuckets.ConvertToFTPBucket(bucket.Config)
		client := ftp.NewFTPUtil(ftp.FTPConfig{Host: config.FTPHost, Port: config.FTPPort, User: config.FTPUser, Password: config.FTPPass, Timeout: 10})
		defer client.Close()
		path := ".oneimg-connection-test-" + uuid.NewString() + ".txt"
		if err := client.UploadImage(path, []byte("oneimg storage test"), "text/plain"); err != nil {
			return "", err
		}
		if err := client.DeleteImage(path); err != nil {
			return "", err
		}
		return "file write and delete succeeded", nil
	case "webdav":
		config := utilsBuckets.ConvertToWebDavBucket(bucket.Config)
		client := webdav.Client(webdav.Config{BaseURL: config.WebdavURL, Username: config.WebdavUser, Password: config.WebdavPass, Timeout: 10 * time.Second})
		path := "/.oneimg-connection-test-" + uuid.NewString() + ".txt"
		if err := client.WebDAVUpload(ctx, path, bytes.NewReader([]byte("oneimg storage test"))); err != nil {
			return "", err
		}
		if err := client.WebDAVDelete(ctx, path); err != nil {
			return "", err
		}
		return "file write and delete succeeded", nil
	default:
		return "", ErrStorageTypeInvalid
	}
}

var _ = gorm.ErrRecordNotFound
