package app

import (
	"errors"
	"log"
	"strings"

	"oneimg/backend/models"
	"oneimg/backend/utils/publicurl"

	"gorm.io/gorm"
)

// MigratePublicImageDomain 把旧的全局直链域名移到它实际生效的那个存储：升级前的系统默认存储，且必须是 S3/R2。
// 旧字段随后清空，避免以后切换默认存储时再被复制；列本身保留，回滚时可以手动填回。
func MigratePublicImageDomain(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var setting models.Settings
		if err := tx.First(&setting).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		legacy := strings.TrimSpace(setting.PublicImageDomain)
		if legacy == "" {
			return nil
		}

		domain, normalizeErr := publicurl.NormalizeDomain(legacy)
		var bucket models.Buckets
		findErr := tx.Where("id = ?", setting.DefaultStorage).Limit(1).Find(&bucket).Error
		if findErr != nil {
			return findErr
		}
		switch {
		case normalizeErr != nil:
			log.Printf("旧图片直链域名 %q 格式无效，未迁移，已清空", legacy)
		case bucket.Id == 0 || !publicurl.SupportsStorage(bucket.Type):
			log.Printf("旧图片直链域名 %q 在升级前未生效（默认存储不是 S3/R2），未迁移，已清空；需要时请在存储页为对应存储设置", legacy)
		case publicurl.BucketDomain(bucket) != "":
			log.Printf("存储「%s」已设置直链域名，保留现有值，旧图片直链域名 %q 已清空", bucket.Name, legacy)
		default:
			config := make(map[string]any, len(bucket.Config)+1)
			for key, value := range bucket.Config {
				config[key] = value
			}
			config[publicurl.ConfigKey] = domain
			if err := tx.Model(&bucket).Select("Config").Updates(models.Buckets{Config: config}).Error; err != nil {
				return err
			}
			log.Printf("图片直链域名 %s 已迁移到存储「%s」", domain, bucket.Name)
		}
		return tx.Model(&models.Settings{}).Where("id = ?", setting.ID).Update("public_image_domain", "").Error
	})
}
