package models

import "time"

// 图片模型
type Image struct {
	Id               int       `json:"id" gorm:"type:integer;primaryKey;autoIncrement"`
	Url              string    `json:"url" gorm:"not null"`
	Thumbnail        string    `json:"thumbnail"`
	FileName         string    `json:"filename" gorm:"not null"`
	OriginalFileName string    `json:"original_filename" gorm:"column:original_filename;default:''"`
	FileSize         int64     `json:"file_size" gorm:"not null"`
	MimeType         string    `json:"mimeType"`
	Width            int       `json:"width"`
	Height           int       `json:"height"`
	Storage          string    `json:"storage" gorm:"default:default"`
	BucketId         int       `json:"bucket_id" gorm:"not null;default:1"`
	UserId           int       `json:"user_id" gorm:"not null;default:1"`
	MD5              string    `json:"md5"`
	ContentHash      string    `json:"content_hash" gorm:"column:content_hash;index"`
	UUID             string    `json:"uuid" gorm:"not null;default:'00000000-0000-0000-0000-000000000000'"`
	CreatedAt        time.Time `json:"created_at"`
}
