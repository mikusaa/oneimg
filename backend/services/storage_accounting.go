package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"oneimg/backend/models"

	"gorm.io/gorm"
)

type storageAggregate struct {
	BucketID      int
	UnknownCount  int64
	ExactBytes    int64
	FallbackBytes int64
}

// MigrateStorageAccounting backfills only local objects that can be verified
// without contacting a storage provider, then reconciles each quota counter.
func MigrateStorageAccounting(db *gorm.DB, uploadsRoot, dataRoot string) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	var images []models.Image
	if err := db.Where("storage_bytes IS NULL AND storage = ?", "default").Find(&images).Error; err != nil {
		return err
	}
	for index := range images {
		storageBytes, ok := localStoredBytes(images[index], uploadsRoot, dataRoot)
		if !ok {
			continue
		}
		if err := db.Model(&models.Image{}).Where("id = ? AND storage_bytes IS NULL", images[index].Id).
			Update("storage_bytes", storageBytes).Error; err != nil {
			return err
		}
	}

	return db.Transaction(func(tx *gorm.DB) error {
		var buckets []models.Buckets
		if err := tx.Find(&buckets).Error; err != nil {
			return err
		}
		aggregates, err := loadStorageAggregates(tx)
		if err != nil {
			return err
		}
		for _, bucket := range buckets {
			aggregate := aggregates[bucket.Id]
			usage := nonNegativeBytes(aggregate.ExactBytes)
			if aggregate.UnknownCount > 0 {
				usage = maxUint64(bucket.Usage, nonNegativeBytes(aggregate.FallbackBytes))
			}
			if bucket.Usage != usage {
				if err := tx.Model(&models.Buckets{}).Where("id = ?", bucket.Id).UpdateColumn("usage", usage).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func loadStorageAggregates(db *gorm.DB) (map[int]storageAggregate, error) {
	rows := make([]storageAggregate, 0)
	err := db.Model(&models.Image{}).
		Select(`bucket_id,
			SUM(CASE WHEN storage_bytes IS NULL THEN 1 ELSE 0 END) AS unknown_count,
			COALESCE(SUM(CASE WHEN storage_bytes IS NOT NULL THEN storage_bytes ELSE 0 END), 0) AS exact_bytes,
			COALESCE(SUM(CASE WHEN storage_bytes IS NOT NULL THEN storage_bytes WHEN file_size > 0 THEN file_size ELSE 0 END), 0) AS fallback_bytes`).
		Group("bucket_id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[int]storageAggregate, len(rows))
	for _, row := range rows {
		result[row.BucketID] = row
	}
	return result, nil
}

func localStoredBytes(image models.Image, uploadsRoot, dataRoot string) (int64, bool) {
	mainPath, ok := storedPath(uploadsRoot, image.Url, "uploads")
	if !ok {
		return 0, false
	}
	mainInfo, err := os.Stat(mainPath)
	if err != nil || !mainInfo.Mode().IsRegular() {
		return 0, false
	}
	total := mainInfo.Size()
	if strings.TrimSpace(image.Thumbnail) == "" {
		return total, true
	}
	thumbnailRoot := filepath.Join(dataRoot, "thumbnails")
	thumbnailPath, ok := storedPath(thumbnailRoot, image.Thumbnail, "thumbnails")
	if !ok {
		return 0, false
	}
	thumbnailInfo, err := os.Stat(thumbnailPath)
	if err != nil || !thumbnailInfo.Mode().IsRegular() {
		return 0, false
	}
	return total + thumbnailInfo.Size(), true
}

func storedPath(root, value, prefix string) (string, bool) {
	clean := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(strings.TrimSpace(value))), "/")
	clean = strings.TrimPrefix(clean, strings.Trim(prefix, "/")+"/")
	if clean == "" || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	rootPath, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}
	candidate, err := filepath.Abs(filepath.Join(rootPath, filepath.FromSlash(clean)))
	if err != nil {
		return "", false
	}
	relative, err := filepath.Rel(rootPath, candidate)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false
	}
	return candidate, true
}

func nonNegativeBytes(value int64) uint64 {
	if value <= 0 {
		return 0
	}
	return uint64(value)
}

func maxUint64(left, right uint64) uint64 {
	if left > right {
		return left
	}
	return right
}

func imageStorageBytes(image models.Image) uint64 {
	if image.StorageBytes != nil {
		return nonNegativeBytes(*image.StorageBytes)
	}
	return nonNegativeBytes(image.FileSize)
}

func decrementBucketUsage(tx *gorm.DB, bucketID int, amount uint64) error {
	if err := tx.Model(&models.Buckets{}).Where("id = ?", bucketID).
		UpdateColumn("usage", gorm.Expr("CASE WHEN usage >= ? THEN usage - ? ELSE 0 END", amount, amount)).Error; err != nil {
		return err
	}
	var unknownCount int64
	if err := tx.Model(&models.Image{}).Where("bucket_id = ? AND storage_bytes IS NULL", bucketID).Count(&unknownCount).Error; err != nil {
		return err
	}
	if unknownCount != 0 {
		return nil
	}
	var exactBytes int64
	if err := tx.Model(&models.Image{}).Where("bucket_id = ?", bucketID).
		Select("COALESCE(SUM(storage_bytes), 0)").Scan(&exactBytes).Error; err != nil {
		return err
	}
	return tx.Model(&models.Buckets{}).Where("id = ?", bucketID).UpdateColumn("usage", nonNegativeBytes(exactBytes)).Error
}
