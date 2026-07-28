package models

// Settings 系统配置模型（全局唯一配置）
// 注意：该表应只有一条记录（ID=1），所有配置项存储在同一条记录中
type Settings struct {
	ID               int    `gorm:"type:integer;primarykey;column:id;autoIncrement" json:"id"`
	OriginalImage    bool   `gorm:"column:original_image;default:false" json:"original_image"`         // 是否保存原图（默认保存）
	SaveWebp         bool   `gorm:"column:save_webp;default:true" json:"save_webp"`                    // 是否保存webp格式（默认保存）
	Thumbnail        bool   `gorm:"column:thumbnail;default:true" json:"thumbnail"`                    // 是否生成缩略图（默认生成）
	Tourist          bool   `gorm:"column:tourist;default:false" json:"tourist"`                       // 是否允许游客上传（默认允许）
	PowVerify        bool   `gorm:"column:pow_verify;default:false" json:"pow_verify"`                 // 是否启用POW验证（默认关闭）
	StartRegister    bool   `gorm:"column:start_register;default:false" json:"start_register"`         // 是否开放普通用户注册
	StartAPI         bool   `gorm:"column:start_api;default:false" json:"start_api"`                   // 是否启用API（默认关闭）
	APIToken         string `gorm:"column:api_token;default:''" json:"api_token"`                      // 兼容旧字段
	APITokenHash     string `gorm:"column:api_token_hash;default:''" json:"-"`                         // API Token哈希
	SaveOriginalName bool   `gorm:"column:save_original_name;default:false" json:"save_original_name"` // 是否保存原文件名（默认不保存）

	// 默认存储
	DefaultStorage   int  `gorm:"column:default_storage;default:1" json:"default_storage"`           // 单存储模式下的默认存储
	MultiStorageSync bool `gorm:"column:multi_storage_sync;default:false" json:"multi_storage_sync"` // 是否启用本机落盘后的多存储后台同步

	// 默认上传配置
	MaxFileSize        int    `gorm:"column:max_file_size;default:10485760" json:"max_file_size"` // 文件最大上传大小
	AllowedTypes       string `gorm:"column:allowed_types;default:'image/jpeg,image/png,image/gif,image/webp,image/svg+xml'" json:"allowed_types"`
	MainImageQuality   int    `gorm:"column:main_image_quality;default:85" json:"main_image_quality"`                                         // 主图WebP质量
	SkipCompressFormat string `gorm:"column:skip_compress_formats;default:'image/gif,image/svg+xml,image/webp'" json:"skip_compress_formats"` // 跳过主图压缩/转换的格式
	DefaultPath        string `gorm:"column:default_path;default:'/uploads/{year}/{moon}'" json:"default_path"`                               // 默认上传路径，魔法变量 {year} 年 {month} 月 {day} 日 {hour} 小时 {minute} 分钟 {random} 随机 {uuid} UUID {role} 角色（1 为管理员, 2 为游客）
	FileName           string `gorm:"column:file_name;default:'{random}'" json:"file_name"`                                                   // 上传文件名称，魔法变量 {random} 随机数 {year} 年 {month} 月 {day} 日 {hour} 小时 {minute} 分钟 {second} 秒

	// 图片直链设置
	PublicImageDomain string `gorm:"column:public_image_domain;default:''" json:"public_image_domain"` // 图片直链域名（用于非本地存储直接访问）
	CDNDomain         string `gorm:"column:cdn_domain;default:''" json:"cdn_domain"`                   // 本地存储CDN域名（根路径指向uploads目录）

	// 来源白名单设置
	RefererWhiteEnable bool   `gorm:"column:referer_white_enable;default:false" json:"referer_white_enable"` // 是否启用白名单
	RefererWhiteList   string `gorm:"column:referer_white_list;default:''" json:"referer_white_list"`        // 白名单（多个用逗号分隔）

	// SEO 设置
	SEOTitle       string `gorm:"column:seo_title;default:'初春图床'" json:"seo_title"`                             // SEO标题（默认为初春图床）
	SEODescription string `gorm:"column:seo_description;default:'初春图床，一个免费、稳定、高效的图床服务'" json:"seo_description"` // SEO描述（默认为初春图床，一个免费、稳定、高效的图床服务）
	SEOKeywords    string `gorm:"column:seo_keywords;default:'初春网络,雾创岛,初春图床,图床,免费,稳定,高效'" json:"seo_keywords"`  // SEO关键词（默认为初春网络,雾创岛,初春图床,图床,免费,稳定,高效）
	SEOICP         string `gorm:"column:seo_icp;default:''" json:"seo_icp"`                                     // SEO ICP备案（默认为空）
	PublicSecurity string `gorm:"column:public_security;default:''" json:"public_security"`                     // SEO 公安备案（默认为空）
	SEOicon        string `gorm:"column:seo_icon;default:''" json:"seo_icon"`                                   // SEO ICON（默认为空）
}

// TableName 指定表名（避免GORM自动复数）
func (Settings) TableName() string {
	return "settings"
}
