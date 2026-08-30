package services

import (
	"errors"
	"strings"

	"oneimg/backend/models"
	"oneimg/backend/utils/publicurl"

	"gorm.io/gorm"
)

var ErrSettingsValidation = errors.New("invalid settings value")

type SettingsService struct{ db *gorm.DB }

type SettingsPatch struct {
	OriginalImage       *bool
	SaveWebP            *bool
	Thumbnail           *bool
	RegistrationEnabled *bool
	SaveOriginalName    *bool
	DefaultStorage      *int
	MaxFileSize         *int
	AllowedTypes        *string
	MainImageQuality    *int
	SkipCompressFormats *string
	DefaultPath         *string
	FileName            *string
	PublicImageDomain   *string
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
				return ErrSettingsValidation
			}
			var count int64
			if err := tx.Model(&models.Buckets{}).Where("id = ?", *input.DefaultStorage).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return ErrBucketNotFound
			}
			put("default_storage", *input.DefaultStorage)
		}
		if input.MaxFileSize != nil {
			if *input.MaxFileSize < 1024 || *input.MaxFileSize > 1024*1024*1024 {
				return ErrSettingsValidation
			}
			put("max_file_size", *input.MaxFileSize)
		}
		if input.AllowedTypes != nil {
			value := strings.TrimSpace(*input.AllowedTypes)
			if value == "" {
				return ErrSettingsValidation
			}
			put("allowed_types", value)
		}
		if input.MainImageQuality != nil {
			if *input.MainImageQuality < 0 || *input.MainImageQuality > 100 {
				return ErrSettingsValidation
			}
			put("main_image_quality", *input.MainImageQuality)
		}
		if input.SkipCompressFormats != nil {
			value := strings.TrimSpace(*input.SkipCompressFormats)
			if value == "" {
				return ErrSettingsValidation
			}
			put("skip_compress_formats", value)
		}
		if input.DefaultPath != nil {
			put("default_path", strings.TrimSpace(*input.DefaultPath))
		}
		if input.FileName != nil {
			put("file_name", strings.TrimSpace(*input.FileName))
		}
		if input.PublicImageDomain != nil {
			value, err := publicurl.NormalizeDomain(*input.PublicImageDomain)
			if err != nil {
				return errors.Join(ErrSettingsValidation, err)
			}
			put("public_image_domain", value)
		}
		if input.CDNDomain != nil {
			value, err := publicurl.NormalizeDomain(*input.CDNDomain)
			if err != nil {
				return errors.Join(ErrSettingsValidation, err)
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
