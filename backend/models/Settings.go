package models

// Settings 系统配置模型（全局唯一配置）
// 注意：该表应只有一条记录（ID=1），所有配置项存储在同一条记录中
type Settings struct {
	ID               int    `gorm:"type:integer;primarykey;column:id;autoIncrement" json:"id"`
	OriginalImage    bool   `gorm:"column:original_image;default:false" json:"original_image"`         // 是否保存原图（默认保存）
	SaveWebp         bool   `gorm:"column:save_webp;default:true" json:"save_webp"`                    // 是否保存webp格式（默认保存）
	Thumbnail        bool   `gorm:"column:thumbnail;default:true" json:"thumbnail"`                    // 是否生成缩略图（默认生成）
	StartRegister    bool   `gorm:"column:start_register;default:false" json:"start_register"`         // 是否开放普通用户注册
	StartAPI         bool   `gorm:"column:start_api;default:false" json:"-"`                           // 仅保留用于破坏性升级清理
	APIToken         string `gorm:"column:api_token;default:''" json:"-"`                              // 仅保留用于破坏性升级清理
	APITokenHash     string `gorm:"column:api_token_hash;default:''" json:"-"`                         // 仅保留用于破坏性升级清理
	SaveOriginalName bool   `gorm:"column:save_original_name;default:false" json:"save_original_name"` // 是否保存原文件名（默认不保存）

	// 默认存储
	DefaultStorage int `gorm:"column:default_storage;default:1" json:"default_storage"` // 默认存储

	VideoUploadEnabled bool `gorm:"column:video_upload_enabled;default:false" json:"video_upload_enabled"`
	MaxVideoFileSize   int  `gorm:"column:max_video_file_size;default:104857600" json:"max_video_file_size"`

	// 默认上传配置
	MaxFileSize        int    `gorm:"column:max_file_size;default:10485760" json:"max_file_size"` // 文件最大上传大小
	AllowedTypes       string `gorm:"column:allowed_types;default:'image/jpeg,image/png,image/gif,image/webp'" json:"allowed_types"`
	MainImageQuality   int    `gorm:"column:main_image_quality;default:85" json:"main_image_quality"`                                         // 主图WebP质量
	SkipCompressFormat string `gorm:"column:skip_compress_formats;default:'image/gif,image/svg+xml,image/webp'" json:"skip_compress_formats"` // 跳过主图压缩/转换的格式
	DefaultPath        string `gorm:"column:default_path;default:'/uploads/{year}/{moon}'" json:"default_path"`                               // 默认上传路径，支持时间、随机值、UUID、角色等魔法变量
	FileName           string `gorm:"column:file_name;default:'{random}'" json:"file_name"`                                                   // 上传文件名称，魔法变量 {random} 随机数 {year} 年 {month} 月 {day} 日 {hour} 小时 {minute} 分钟 {second} 秒

	// 图片直链设置
	PublicImageDomain string `gorm:"column:public_image_domain;default:''" json:"public_image_domain"` // 图片直链域名（用于非本地存储直接访问）
	CDNDomain         string `gorm:"column:cdn_domain;default:''" json:"cdn_domain"`                   // 本地存储CDN域名（根路径指向uploads目录）

	// 来源白名单设置
	RefererWhiteEnable bool   `gorm:"column:referer_white_enable;default:false" json:"referer_white_enable"` // 是否启用白名单
	RefererWhiteList   string `gorm:"column:referer_white_list;default:''" json:"referer_white_list"`        // 白名单（多个用逗号分隔）

	// SEO 设置。新安装的标题、描述和关键词由 NewSettings 写入；这里的 default 是早期版本建表时的列默认值，
	// 保持不变，否则启动迁移会在已有安装上重建 settings 表
	SEOTitle       string `gorm:"column:seo_title;default:'初春图床'" json:"seo_title"`                             // SEO标题
	SEODescription string `gorm:"column:seo_description;default:'初春图床，一个免费、稳定、高效的图床服务'" json:"seo_description"` // SEO描述
	SEOKeywords    string `gorm:"column:seo_keywords;default:'初春网络,雾创岛,初春图床,图床,免费,稳定,高效'" json:"seo_keywords"`  // SEO关键词
	SEOICP         string `gorm:"column:seo_icp;default:''" json:"seo_icp"`                                     // SEO ICP备案（默认为空）
	PublicSecurity string `gorm:"column:public_security;default:''" json:"public_security"`                     // SEO 公安备案（默认为空）
	SEOicon        string `gorm:"column:seo_icon;default:''" json:"seo_icon"`                                   // SEO ICON（默认为空）
}

// 新安装的默认站点信息；已有安装保留原值
const (
	DefaultSEOTitle       = "OneImg"
	DefaultSEODescription = "OneImg：自托管的图片与视频外链服务"
	DefaultSEOKeywords    = "OneImg,图床,图片托管,视频托管"
)

// NewSettings 返回新安装使用的系统配置，其余字段使用列默认值
func NewSettings() Settings {
	return Settings{
		SEOTitle:       DefaultSEOTitle,
		SEODescription: DefaultSEODescription,
		SEOKeywords:    DefaultSEOKeywords,
	}
}

// TableName 指定表名（避免GORM自动复数）
func (Settings) TableName() string {
	return "settings"
}
