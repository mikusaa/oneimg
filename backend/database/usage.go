package database

import (
	"errors"
	"math"

	"gorm.io/gorm"
	"oneimg/backend/models"
)

func ReserveUsage(tx *gorm.DB, bucketID int, amount int64) (bool, error) {
	if amount < 0 {
		return false, errors.New("storage size out of range")
	}
	result := tx.Model(&models.Buckets{}).
		Where("id = ? AND usage >= 0 AND usage <= ? AND (capacity = 0 OR capacity - usage >= ?)", bucketID, math.MaxInt64-amount, amount).
		UpdateColumn("usage", gorm.Expr("usage + ?", amount))
	return result.RowsAffected == 1, result.Error
}

func StorageBytes(main, thumbnail int64) (int64, error) {
	if main < 0 || thumbnail < 0 || main > math.MaxInt64-thumbnail {
		return 0, errors.New("storage size out of range")
	}
	return main + thumbnail, nil
}
