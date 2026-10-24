package app

import (
	"fmt"
	"log"
	"os"

	"oneimg/backend/config"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/services"
	"oneimg/backend/utils/images"
	"oneimg/backend/utils/passkeys"

	"gorm.io/gorm"
)

type System struct {
	Config   *config.Config
	Database *database.Database
	Services *services.Services
}

func Init() *System {
	config.NewConfig()
	result, err := Initialize(config.App)
	if err != nil {
		log.Fatal(err)
	}
	return result
}

func Initialize(cfg *config.Config) (*System, error) {
	db, err := database.Open(cfg)
	if err != nil {
		return nil, err
	}
	return InitializeDatabase(cfg, db)
}

// InitializeDatabase shares schema and business initialization with disposable
// integration databases. Ownership of the connection transfers to the System.
func InitializeDatabase(cfg *config.Config, db *database.Database) (result *System, err error) {
	defer func() {
		if err != nil {
			_ = db.Close()
		}
	}()
	if err := database.MigrateSchema(db.DB); err != nil {
		return nil, fmt.Errorf("database schema initialization failed (%s)", database.ErrorCode(err))
	}
	database.SetDB(db)
	config.App = cfg
	if err := os.MkdirAll(cfg.StateDirectory(), 0700); err != nil {
		return nil, fmt.Errorf("cannot create local state directory")
	}

	// 初始化图片服务
	images.InitImageService()
	if err := images.ConfigureBudget(cfg.ImageMaxPixels, cfg.ImageMaxDimension); err != nil {
		return nil, err
	}
	if err := passkeys.Init(cfg); err != nil {
		log.Printf("Passkey 功能已禁用: %v", err)
	} else {
		log.Printf("Passkey 功能已启用 (RP ID: %s)", passkeys.RPID())
	}

	// 初始化默认用户
	if err := InitDefaultUser(cfg, db); err != nil {
		return nil, fmt.Errorf("管理员初始化失败 (%s)", database.ErrorCode(err))
	}
	if err := MigrateAuthenticationState(db.DB, cfg); err != nil {
		return nil, fmt.Errorf("认证状态迁移失败 (%s)", database.ErrorCode(err))
	}
	if err := MigrateLegacyUserPermissions(db.DB); err != nil {
		return nil, fmt.Errorf("用户权限兼容迁移失败 (%s)", database.ErrorCode(err))
	}

	// 初始化系统设置
	if err := InitSettings(db); err != nil {
		return nil, err
	}
	if err := MigrateImageTypes(db.DB); err != nil {
		return nil, fmt.Errorf("图像类型迁移失败 (%s)", database.ErrorCode(err))
	}
	if err := InvalidateLegacyAPIAccess(db.DB); err != nil {
		return nil, fmt.Errorf("旧 API Token 清理失败 (%s)", database.ErrorCode(err))
	}

	// 初始化默认存储配置
	if err := InitDefaultStorage(db); err != nil {
		return nil, err
	}
	if err := services.MigrateStorageAccounting(db.DB, "uploads", "data"); err != nil {
		return nil, fmt.Errorf("存储计量迁移失败 (%s)", database.ErrorCode(err))
	}

	if err := ResetRemovedStorageDefaults(db.DB); err != nil {
		return nil, fmt.Errorf("已移除存储的默认配置修复失败 (%s)", database.ErrorCode(err))
	}
	if err := MigratePublicImageDomain(db.DB); err != nil {
		return nil, fmt.Errorf("图片直链域名迁移失败 (%s)", database.ErrorCode(err))
	}
	if count, err := services.CountSharedPhysicalObjects(db.DB); err != nil {
		return nil, fmt.Errorf("历史对象引用检查失败 (%s)", database.ErrorCode(err))
	} else if count > 0 {
		log.Printf("发现 %d 个历史共享物理对象，仅报告，未改动文件", count)
	}

	r := &System{
		Config:   cfg,
		Database: db,
		Services: services.New(db, cfg),
	}

	return r, nil
}

// LocalStorageName 是新安装时内置本地存储的名称。旧版本叫「本地默认存储」，
// 容易和「默认存储」（上传时默认选中的存储）混淆。已有安装保留原名，不做迁移。
const LocalStorageName = "本地存储"

// InitDefaultStorage 初始化默认存储配置
func InitDefaultStorage(db *database.Database) error {
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		return initDefaultStorage(&database.Database{DB: tx})
	})
	if err != nil {
		return fmt.Errorf("default storage initialization failed (%s)", database.ErrorCode(err))
	}
	return nil
}

func initDefaultStorage(db *database.Database) error {
	// 默认使用本地存储
	storageType := "default"
	// 默认路径
	storagePath := "/uploads"

	// 查询是否已存在存储配置
	var count int64
	if err := db.DB.Model(&models.Buckets{}).Where("type = ?", storageType).Count(&count).Error; err != nil {
		return fmt.Errorf("default storage lookup failed (%s)", database.ErrorCode(err))
	}

	if count > 0 {
		log.Println("存储配置已存在，跳过默认存储初始化")
		return database.SyncSequences(db.DB, nil)
	}

	Config := map[string]any{
		"storagePath": storagePath,
	}

	// 创建默认存储配置
	storage := models.Buckets{
		Id:       1,
		Name:     LocalStorageName,
		Type:     storageType,
		Capacity: 0, // 无限容量
		Config:   Config,
		Usage:    0,
	}

	result := db.DB.Create(&storage)
	if result.Error != nil {
		return fmt.Errorf("default storage creation failed (%s)", database.ErrorCode(result.Error))
	}

	// 版本存储迁移
	if err := Migrate(db); err != nil {
		return err
	}
	if err := database.SyncSequences(db.DB, nil); err != nil {
		return err
	}
	log.Printf("默认存储配置创建成功 - 存储类型: %s, 存储路径: %s", storage.Type, storagePath)
	return nil
}

func InitSettings(db *database.Database) error {
	var count int64
	if err := db.DB.Model(&models.Settings{}).Count(&count).Error; err != nil {
		return fmt.Errorf("settings lookup failed (%s)", database.ErrorCode(err))
	}
	if count > 0 {
		log.Println("系统配置已存在，跳过系统配置初始化")
		return nil
	}
	settings := models.NewSettings()
	result := db.DB.Create(&settings)
	if result.Error != nil {
		return fmt.Errorf("settings creation failed (%s)", database.ErrorCode(result.Error))
	}
	log.Printf("系统配置创建成功")
	return nil
}
