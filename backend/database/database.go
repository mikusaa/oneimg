package database

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"oneimg/backend/config"
	"oneimg/backend/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Database 数据库操作类
type Database struct {
	DB *gorm.DB
}

var db *Database

// NewDB 创建新的数据库连接
func NewDB(dialector gorm.Dialector) (*Database, error) {
	gormConfig := &gorm.Config{
		SkipDefaultTransaction:                   true,
		DisableForeignKeyConstraintWhenMigrating: true,
	}

	gormDB, err := gorm.Open(dialector, gormConfig)
	if err != nil {
		return nil, fmt.Errorf("gorm连接失败: %w", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("获取SQL连接失败: %w", err)
	}

	// 验证连接
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("连接验证失败: %w", err)
	}

	return &Database{DB: gormDB}, nil
}

// GetDB 获取数据库实例
func GetDB() *Database {
	return db
}

// InitDB 初始化数据库连接
func InitDB(cfg *config.Config) {
	ensureDirExists(cfg.SqlitePath)

	var err error
	db, err = NewDB(sqlite.Open(cfg.SqlitePath))
	if err != nil {
		log.Fatalf("❌ SQLite 数据库初始化失败: %v", err)
	}
	log.Printf("✅ SQLite 数据库连接成功: %s", cfg.SqlitePath)

	// 自动迁移数据表
	dropLegacyUserRoleUniqueIndex(db.DB)
	err = db.DB.AutoMigrate(
		&models.Tags{},
		&models.User{},
		&models.Image{},
		&models.ImageStorage{},
		&models.Settings{},
		&models.ImageToTags{},
		&models.Buckets{},
	)
	if err != nil {
		log.Fatalf("❌ 数据库表迁移失败: %v", err)
	}
	log.Println("✅ 数据库表迁移完成")
}

func dropLegacyUserRoleUniqueIndex(gormDB *gorm.DB) {
	for _, indexName := range []string{"unique_idx", "idx_users_role"} {
		if gormDB.Migrator().HasIndex(&models.User{}, indexName) {
			if err := gormDB.Migrator().DropIndex(&models.User{}, indexName); err != nil {
				log.Printf("⚠️ 删除旧用户角色唯一索引失败(%s): %v", indexName, err)
			}
		}
	}
}

func ensureDirExists(path string) {
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0755)
	}
}
