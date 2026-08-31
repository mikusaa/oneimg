package models

import "time"

// 图片模型
type Image struct {
	Id               int       `json:"id" gorm:"type:integer;primaryKey;autoIncrement"`
	Url              string    `json:"url" gorm:"not null"`
	Thumbnail        string    `json:"thumbnail"`
	FileName         string    `json:"filename" gorm:"not null"`
	OriginalFileName string    `json:"original_filename" gorm:"column:original_filename;default:''"`
	OriginalFileSize int64     `json:"original_file_size" gorm:"column:original_file_size;default:0"`
	FileSize         int64     `json:"file_size" gorm:"not null"`
	StorageBytes     *int64    `json:"storage_bytes,omitempty" gorm:"column:storage_bytes"` // 主图与成功保存的缩略图总字节数；nil 表示历史计量未知
	MimeType         string    `json:"mimeType"`
	Width            int       `json:"width"`
	Height           int       `json:"height"`
	Storage          string    `json:"storage" gorm:"default:default"`
	BucketId         int       `json:"bucket_id" gorm:"not null;default:1"`
	UserId           int       `json:"user_id" gorm:"not null;default:1"`
	ContentHash      string    `json:"content_hash" gorm:"column:content_hash;index"`
	CreatedAt        time.Time `json:"created_at"`
}
