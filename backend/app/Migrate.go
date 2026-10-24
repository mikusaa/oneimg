package app

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"math"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/buckets"
)

func Migrate(db *database.Database) error {
	return db.DB.Transaction(func(tx *gorm.DB) error {
		var images []models.Image
		if err := tx.Find(&images).Error; err != nil {
			return fmt.Errorf("legacy storage lookup failed (%s)", database.ErrorCode(err))
		}
		for _, item := range []struct {
			kind string
			id   int
			name string
		}{
			{"default", 1, LocalStorageName}, {"s3", 2, "S3对象存储"}, {"r2", 3, "Cloudflare R2存储"}, {"ftp", 4, "FTP文件存储"}, {"webdav", 5, "WebDAV存储"},
		} {
			var usage uint64
			var count int
			for _, image := range images {
				if image.Storage == item.kind {
					if image.FileSize < 0 || uint64(image.FileSize) > math.MaxInt64-usage {
						return errors.New("legacy storage size out of range")
					}
					usage += uint64(image.FileSize)
					count++
				}
			}
			if count == 0 {
				continue
			}
			if item.kind != "default" {
				bucket := models.Buckets{Id: item.id, Name: item.name, Type: item.kind, Capacity: 1099511627776, Usage: usage, Config: getBucketConfig(item.kind)}
				if err := tx.Create(&bucket).Error; err != nil {
					return fmt.Errorf("legacy storage creation failed (%s)", database.ErrorCode(err))
				}
			}
			if err := tx.Model(&models.Image{}).Where("storage = ?", item.kind).Update("bucket_id", item.id).Error; err != nil {
				return err
			}
		}
		return tx.Model(&models.User{}).Where("id = ? AND role = 0", models.SuperAdminID).Update("role", models.RoleAdmin).Error
	})
}

func getBucketConfig(storageType string) map[string]any {
	switch storageType {
	case "s3":
		return buckets.S3BucketToMap(models.S3Bucket{})
	case "r2":
		return buckets.R2BucketToMap(models.R2Bucket{})
	case "ftp":
		return buckets.FTPBucketToMap(models.FTPBucket{})
	case "webdav":
		return buckets.WebDavBucketToMap(models.WebDavBucket{})
	default:
		return make(map[string]any)
	}
}
