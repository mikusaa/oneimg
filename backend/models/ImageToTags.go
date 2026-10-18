package models

type ImageToTags struct {
	ImageId int `json:"image_id" gorm:"primaryKey"`
	// 主键以 image_id 开头，按标签统计和筛选需要单独的 tag_id 索引
	TagId int `json:"tag_id" gorm:"primaryKey;index"`
}
