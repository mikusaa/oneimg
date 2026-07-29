package config

import (
	"crypto/rand"
	"encoding/base64"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	// 服务器配置
	Port   string
	AppURL string

	// SQLite 数据库
	SqlitePath string

	// 上传文件配置
	MaxFileSize  int64
	AllowedTypes []string

	// 默认用户
	DefaultUser string
	DefaultPass string

	// JWT配置
	JWTSecret string

	// Session配置
	SessionSecret string

	// 配置加密
	ConfigSecret string
}

// 全局配置实例
var App *Config

func envFilePath() string {
	if configured := strings.TrimSpace(os.Getenv("ONEIMG_ENV_FILE")); configured != "" {
		return configured
	}
	if _, err := os.Stat(".env"); err == nil {
		return ".env"
	}
	return filepath.Join("data", ".env")
}

// 检查.env文件/目录状态
func EnvExists() bool {
	envPath := envFilePath()
	info, err := os.Stat(envPath)
	if os.IsNotExist(err) {
		return false
	}
	// 如果存在但不是文件，先删除目录
	if err == nil && info.IsDir() {
		log.Printf("发现%s是目录，正在删除...", envPath)
		if err := os.RemoveAll(envPath); err != nil {
			log.Fatalf("删除.env目录失败：%v", err)
		}
		return false
	}
	return true
}

// 创建默认.env文件
func CreateDefaultEnv() {
	envPath := envFilePath()
	if err := os.MkdirAll(filepath.Dir(envPath), 0755); err != nil {
		log.Fatalf("创建配置目录失败：%v", err)
	}

	// 生成随机的SESSION_SECRET（32位base64编码）
	sessionSecret := generateRandomSecret(32)

	// 直接定义.env模板内容
	envTemplate := `# 服务器配置
SERVER_PORT=8080
APP_URL=http://localhost:8080

# SQLite 数据库
SQLITE_PATH=./data/data.db

# 文件上传配置
MAX_FILE_SIZE=10485760
ALLOWED_TYPES=image/jpeg,image/png,image/gif,image/webp,image/svg+xml

# 默认用户配置
DEFAULT_USER=admin
DEFAULT_PASS=123456

# Session配置
SESSION_SECRET=

# 配置加密密钥（用于敏感配置字段加密存储）
CONFIG_SECRET=
`

	configSecret := generateRandomSecret(32)

	// 替换模板中的密钥占位符
	envContent := strings.Replace(envTemplate, "SESSION_SECRET=", "SESSION_SECRET="+sessionSecret, 1)
	envContent = strings.Replace(envContent, "CONFIG_SECRET=", "CONFIG_SECRET="+configSecret, 1)

	// 写入.env文件
	// 确保目标路径不是目录
	if info, err := os.Stat(envPath); err == nil && info.IsDir() {
		log.Fatalf("无法写入.env文件：%s 是一个目录", envPath)
	}

	if err := os.WriteFile(envPath, []byte(envContent), 0600); err != nil {
		log.Fatalf("生成默认.env文件失败：%v", err)
	}

	absPath, err := filepath.Abs(envPath)
	if err != nil {
		absPath = envPath
	}
	log.Printf("✅ 首次启动：自动生成.env文件（路径：%s）", absPath)
}

// 生成指定长度的随机密钥（base64编码）
func generateRandomSecret(length int) string {
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		log.Fatalf("生成随机密钥失败：%v", err)
	}
	return base64.URLEncoding.EncodeToString(bytes)
}

// 初始化配置（优先加载外部.env，无则生成默认）
func NewConfig() {
	// 1. 检查.env文件，不存在则生成
	if !EnvExists() {
		CreateDefaultEnv()
	}

	// 2. 加载.env文件（此时必存在）
	envPath := envFilePath()
	err := godotenv.Load(envPath)
	if err != nil {
		log.Fatalf("加载.env文件失败：%v", err)
	}

	// 3. 解析配置项
	maxFileSize, _ := strconv.ParseInt(getEnv("MAX_FILE_SIZE", "10485760"), 10, 64)
	allowedTypes := strings.Split(getEnv("ALLOWED_TYPES", "image/jpeg,image/png,image/gif,image/webp"), ",")
	port := getEnv("SERVER_PORT", getEnv("PORT", "8080"))
	appURL := getEnv("APP_URL", "http://localhost:"+port)

	// SQLite 配置
	sqlitePath := getEnv("SQLITE_PATH", "./data/data.db")
	// 确保SQLite目录存在
	if err := os.MkdirAll(filepath.Dir(sqlitePath), 0755); err != nil {
		log.Printf("警告：创建SQLite目录失败：%v", err)
	}

	// 默认用户配置
	defaultUser := getEnv("DEFAULT_USER", "admin")
	defaultPass := getEnv("DEFAULT_PASS", "123456")

	// JWT配置（默认生成随机密钥，避免硬编码）
	jwtSecret := getEnv("JWT_SECRET", generateRandomSecret(32))

	// Session配置（读取.env中的值，无则生成）
	sessionSecret := getEnv("SESSION_SECRET", generateRandomSecret(32))
	configSecret := getEnv("CONFIG_SECRET", generateRandomSecret(32))

	// 初始化全局配置
	App = &Config{
		Port:          port,
		AppURL:        appURL,
		SqlitePath:    sqlitePath,
		MaxFileSize:   maxFileSize,
		AllowedTypes:  allowedTypes,
		DefaultUser:   defaultUser,
		DefaultPass:   defaultPass,
		JWTSecret:     jwtSecret,
		SessionSecret: sessionSecret,
		ConfigSecret:  configSecret,
	}

	log.Println("✅ 配置初始化完成")
}

// 获取环境变量
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
