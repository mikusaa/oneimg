package controllers

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"oneimg/backend/database"
	"oneimg/backend/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func resolveUploadBuckets(c *gin.Context, setting models.Settings) ([]models.Buckets, error) {
	db := database.GetDB()
	if db == nil || db.DB == nil {
		return nil, errors.New("数据库未初始化")
	}

	var allBuckets []models.Buckets
	if err := db.DB.Order("id ASC").Find(&allBuckets).Error; err != nil {
		return nil, err
	}

	role := c.GetInt("user_role")
	permission := models.Permission{Buckets: []int{}}
	if role != models.RoleAdmin {
		var user models.User
		err := db.DB.Select("id", "permission").First(&user, c.GetInt("user_id")).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if err == nil {
			permission = user.Permission
		}
	}

	result := make([]models.Buckets, 0, len(allBuckets))
	for _, bucket := range allBuckets {
		if bucket.Type == "telegram" {
			continue
		}
		if bucket.Id != setting.DefaultStorage {
			if bucket.Capacity > 0 && bucket.Usage >= bucket.Capacity {
				continue
			}
			if role != models.RoleAdmin && !models.IntSliceContains(permission.Buckets, bucket.Id) {
				continue
			}
		}
		result = append(result, bucket)
	}
	return result, nil
}

func canUseUploadBucket(c *gin.Context, setting models.Settings, bucketID int) (bool, error) {
	buckets, err := resolveUploadBuckets(c, setting)
	if err != nil {
		return false, err
	}
	for _, bucket := range buckets {
		if bucket.Id == bucketID {
			return true, nil
		}
	}
	return false, nil
}

func cleanupLocalUpload(image models.Image) {
	for _, publicPath := range []string{image.Url, image.Thumbnail} {
		path := strings.TrimSpace(publicPath)
		if path == "" || strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
			continue
		}
		cleanPath := filepath.Clean(filepath.FromSlash(strings.TrimPrefix(path, "/")))
		if cleanPath == "." || filepath.IsAbs(cleanPath) || cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
			continue
		}
		if cleanPath == "thumbnails" || strings.HasPrefix(cleanPath, "thumbnails"+string(filepath.Separator)) {
			cleanPath = filepath.Join("data", cleanPath)
		}
		_ = os.Remove(cleanPath)
	}
}
