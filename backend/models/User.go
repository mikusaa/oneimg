package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// 用户模型
type User struct {
	ID         int        `json:"id" gorm:"type:integer;primaryKey;autoIncrement"`
	Role       int        `json:"role" gorm:"default:1"`
	Username   string     `json:"username" gorm:"unique;not null"`
	Password   string     `json:"-" gorm:"not null"`
	Permission Permission `json:"permission" gorm:"type:jsonb"`
	CreatedAt  time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

const (
	SuperAdminID = 1
	RoleAdmin    = 1
	RoleGuest    = 2
	RoleUser     = 3
)

// Permission 保存普通用户可使用的存储桶。
type Permission struct {
	Buckets []int `json:"buckets" gorm:"default:[]"`
}

func (p Permission) Value() (driver.Value, error) {
	return json.Marshal(p)
}

func (p *Permission) Scan(src any) error {
	if src == nil {
		p.Buckets = []int{}
		return nil
	}
	var data []byte
	switch v := src.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return errors.New("invalid json source for Permission")
	}
	if len(data) == 0 || string(data) == "null" || string(data) == "[]" {
		p.Buckets = []int{}
		return nil
	}
	return json.Unmarshal(data, p)
}

func IntSliceContains(arr []int, target int) bool {
	for _, v := range arr {
		if v == target {
			return true
		}
	}
	return false
}
