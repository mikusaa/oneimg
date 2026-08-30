package uploads

import (
	"fmt"

	"oneimg/backend/interfaces"
	"oneimg/backend/models"
)

const (
	MaxUploadFiles     = 10
	DefaultStorageType = "default"
)

// GetStorageUploader selects an existing storage implementation without
// coupling storage code to an HTTP response format.
func GetStorageUploader(bucket *models.Buckets) (interfaces.StorageUploader, error) {
	switch bucket.Type {
	case "default":
		return &DefaultUploader{}, nil
	case "r2":
		return &R2Uploader{}, nil
	case "s3":
		return &S3Uploader{}, nil
	case "webdav":
		return &WebDAVUploader{}, nil
	case "ftp":
		return &FTPUploader{}, nil
	default:
		return nil, fmt.Errorf("不支持的存储类型：%s", bucket.Type)
	}
}
