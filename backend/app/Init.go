package app

import (
	"log"

	"oneimg/backend/config"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/services"
	"oneimg/backend/utils/images"
	"oneimg/backend/utils/passkeys"
)

type System struct {
	Config   *config.Config
	Database *database.Database
	Services *services.Services
}

func Init() *System {
	// 创建配置实例
	config.NewConfig()
	cfg := config.App
	// 初始化数据库
	database.InitDB(cfg)
	// 获取数据库实例
	db := database.GetDB()

	// 初始化图片服务
	images.InitImageService()
	if err := passkeys.Init(cfg); err != nil {
		log.Printf("Passkey 功能已禁用: %v", err)
	} else {
		log.Printf("Passkey 功能已启用 (RP ID: %s)", passkeys.RPID())
	}

	// 初始化默认用户
	if err := InitDefaultUser(cfg, db); err != nil {
		log.Fatalf("管理员初始化失败: %v", err)
	}
	if err := MigrateAuthenticationState(db.DB, cfg); err != nil {
		log.Fatalf("认证状态迁移失败: %v", err)
	}
	if err := MigrateLegacyUserPermissions(db.DB); err != nil {
		log.Fatalf("用户权限兼容迁移失败: %v", err)
	}

	// 初始化系统设置
	InitSettings(db)
	if err := InvalidateLegacyAPIAccess(db.DB); err != nil {
		log.Fatalf("旧 API Token 清理失败: %v", err)
	}

	// 初始化默认存储配置
	InitDefaultStorage(db)
	if err := services.MigrateStorageAccounting(db.DB, "uploads", "data"); err != nil {
		log.Fatalf("存储计量迁移失败: %v", err)
	}

	if err := ResetRemovedStorageDefaults(db.DB); err != nil {
		log.Fatalf("已移除存储的默认配置修复失败: %v", err)
	}

	r := &System{
		Config:   cfg,
		Database: db,
		Services: services.New(db, cfg),
	}

	return r
}

// InitDefaultStorage 初始化默认存储配置
func InitDefaultStorage(db *database.Database) {
	// 默认使用本地存储
	storageType := "default"
	// 默认路径
	storagePath := "/uploads"

	// 查询是否已存在存储配置
	var count int64
	db.DB.Model(&models.Buckets{}).Where("type = ?", storageType).Count(&count)

	if count > 0 {
		log.Println("存储配置已存在，跳过默认存储初始化")
		return
	}

	Config := map[string]any{
		"storagePath": storagePath,
	}

	// 创建默认存储配置
	storage := models.Buckets{
		Id:       1,
		Name:     "本地默认存储",
		Type:     storageType,
		Capacity: 0, // 无限容量
		Config:   Config,
		Usage:    0,
	}

	result := db.DB.Create(&storage)
	if result.Error != nil {
		log.Fatal("创建默认存储配置失败:", result.Error)
	}

	log.Printf("默认存储配置创建成功 - 存储类型: %s, 存储路径: %s", storage.Type, storagePath)

	// 版本存储迁移
	Migrate(db)
}

func InitSettings(db *database.Database) {
	var count int64
	db.DB.Model(&models.Settings{}).Count(&count)
	if count > 0 {
		log.Println("系统配置已存在，跳过系统配置初始化")
		return
	}
	result := db.DB.Create(&models.Settings{})
	if result.Error != nil {
		log.Fatal("创建系统配置失败:", result.Error)
	}
	log.Printf("系统配置创建成功")
}
