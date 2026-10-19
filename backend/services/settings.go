package services

import (
	"errors"
	"strings"

	"oneimg/backend/models"
	"oneimg/backend/utils/images"
	"oneimg/backend/utils/publicurl"

	"gorm.io/gorm"
)

var ErrSettingsValidation = errors.New("invalid settings value")

// SettingsFieldError 描述单个设置字段校验失败的原因，可用 errors.Is 匹配 ErrSettingsValidation。
type SettingsFieldError struct {
	Field   string
	Message string
}

func (e *SettingsFieldError) Error() string { return e.Field + ": " + e.Message }
func (e *SettingsFieldError) Unwrap() error { return ErrSettingsValidation }

func fieldError(field, message string) error { return &SettingsFieldError{Field: field, Message: message} }

type SettingsService struct{ db *gorm.DB }

type SettingsPatch struct {
	OriginalImage       *bool
	SaveWebP            *bool
	Thumbnail           *bool
	RegistrationEnabled *bool
	SaveOriginalName    *bool
	DefaultStorage      *int
	MaxFileSize         *int
	VideoUploadEnabled  *bool
	MaxVideoFileSize    *int
	AllowedTypes        *string
	MainImageQuality    *int
	SkipCompressFormats *string
	DefaultPath         *string
	FileName            *string
	CDNDomain           *string
	RefererWhiteEnable  *bool
	RefererWhiteList    *string
	SEOTitle            *string
	SEODescription      *string
	SEOKeywords         *string
	SEOICP              *string
	PublicSecurity      *string
	SEOIcon             *string
}

func NewSettingsService(db *gorm.DB) *SettingsService { return &SettingsService{db: db} }

func (s *SettingsService) Get() (models.Settings, error) {
	var item models.Settings
	err := s.db.First(&item).Error
	return item, err
}

func (s *SettingsService) Update(input SettingsPatch) (models.Settings, error) {
	var result models.Settings
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&result).Error; err != nil {
			return err
		}
		updates := map[string]any{}
		put := func(key string, value any) { updates[key] = value }
		if input.OriginalImage != nil {
			put("original_image", *input.OriginalImage)
		}
		if input.SaveWebP != nil {
			put("save_webp", *input.SaveWebP)
		}
		if input.Thumbnail != nil {
			put("thumbnail", *input.Thumbnail)
		}
		if input.RegistrationEnabled != nil {
			put("start_register", *input.RegistrationEnabled)
		}
		if input.SaveOriginalName != nil {
			put("save_original_name", *input.SaveOriginalName)
		}
		if input.DefaultStorage != nil {
			if *input.DefaultStorage <= 0 {
				return fieldError("default_storage", "请选择一个存储")
			}
			var target models.Buckets
			if err := tx.Select("id", "disabled").First(&target, *input.DefaultStorage).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return fieldError("default_storage", "存储不存在")
				}
				return err
			}
			if target.Disabled {
				return ErrStorageDisabled
			}
			put("default_storage", *input.DefaultStorage)
		}
		if input.MaxFileSize != nil {
			if *input.MaxFileSize < 1024 || *input.MaxFileSize > 1024*1024*1024 {
				return fieldError("max_file_size", "图片大小上限需要在 1 KiB 到 1 GiB 之间")
			}
			put("max_file_size", *input.MaxFileSize)
		}
		if input.VideoUploadEnabled != nil {
			put("video_upload_enabled", *input.VideoUploadEnabled)
		}
		if input.MaxVideoFileSize != nil {
			if *input.MaxVideoFileSize < 1<<20 || *input.MaxVideoFileSize > 1<<30 {
				return fieldError("max_video_file_size", "视频大小上限需要在 1 MiB 到 1 GiB 之间")
			}
			put("max_video_file_size", *input.MaxVideoFileSize)
		}
		if input.AllowedTypes != nil {
			value := strings.TrimSpace(*input.AllowedTypes)
			if value == "" {
				return fieldError("allowed_types", "至少保留一种图片类型")
			}
			for _, mime := range strings.Split(value, ",") {
				mime = strings.ToLower(strings.TrimSpace(mime))
				if mime != "image/jpeg" && mime != "image/png" && mime != "image/gif" && mime != "image/webp" {
					return fieldError("allowed_types", "只支持 image/jpeg、image/png、image/gif、image/webp")
				}
			}
			put("allowed_types", strings.ToLower(strings.ReplaceAll(value, " ", "")))
		}
		if input.MainImageQuality != nil {
			if *input.MainImageQuality < 0 || *input.MainImageQuality > 100 {
				return fieldError("main_image_quality", "压缩质量需要在 0 到 100 之间")
			}
			put("main_image_quality", *input.MainImageQuality)
		}
		if input.SkipCompressFormats != nil {
			value := strings.TrimSpace(*input.SkipCompressFormats)
			if value == "" {
				return fieldError("skip_compress_formats", "跳过压缩的格式不能为空")
			}
			put("skip_compress_formats", value)
		}
		// 只校验发生变化的模板：升级前保存的旧值即使不合规，也不阻止保存其他设置。
		if input.DefaultPath != nil {
			value := strings.TrimSpace(*input.DefaultPath)
			if value != result.DefaultPath {
				if err := images.ValidatePathTemplate(value); err != nil {
					return fieldError("default_path", err.Error())
				}
			}
			put("default_path", value)
		}
		if input.FileName != nil {
			value := strings.TrimSpace(*input.FileName)
			if value != result.FileName {
				if err := images.ValidateNameTemplate(value); err != nil {
					return fieldError("file_name", err.Error())
				}
			}
			put("file_name", value)
		}
		if input.CDNDomain != nil {
			value, err := publicurl.NormalizeDomain(*input.CDNDomain)
			if err != nil {
				return fieldError("cdn_domain", err.Error())
			}
			put("cdn_domain", value)
		}
		if input.RefererWhiteEnable != nil {
			put("referer_white_enable", *input.RefererWhiteEnable)
		}
		if input.RefererWhiteList != nil {
			put("referer_white_list", strings.TrimSpace(*input.RefererWhiteList))
		}
		if input.SEOTitle != nil {
			put("seo_title", strings.TrimSpace(*input.SEOTitle))
		}
		if input.SEODescription != nil {
			put("seo_description", strings.TrimSpace(*input.SEODescription))
		}
		if input.SEOKeywords != nil {
			put("seo_keywords", strings.TrimSpace(*input.SEOKeywords))
		}
		if input.SEOICP != nil {
			put("seo_icp", strings.TrimSpace(*input.SEOICP))
		}
		if input.PublicSecurity != nil {
			put("public_security", strings.TrimSpace(*input.PublicSecurity))
		}
		if input.SEOIcon != nil {
			put("seo_icon", strings.TrimSpace(*input.SEOIcon))
		}
		if len(updates) == 0 {
			return ErrSettingsValidation
		}
		if err := tx.Model(&result).Updates(updates).Error; err != nil {
			return err
		}
		return tx.First(&result, result.ID).Error
	})
	return result, err
}
