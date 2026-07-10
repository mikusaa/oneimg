package main

/**
 * 初春图床v3
 * 重构后端，标准化接口，支持更多存储方式
 */
import (
	"embed"
	"log"
	"os"

	"oneimg/backend/app"
	"oneimg/backend/routes"
	"oneimg/backend/utils/localimport"
	"oneimg/backend/utils/watermark"
)

// 导入静态资源
//
//go:embed frontend/dist/**
var fs embed.FS

// 导入水印字体资源
//
//go:embed frontend/src/assets/fonts/**
var fontFs embed.FS

func main() {
	system := app.Init()

	if len(os.Args) > 1 && os.Args[1] == "import-local" {
		os.Exit(localimport.RunCLI(os.Args[2:], system.Database.DB))
	}

	r := routes.SetupRoutes(fs)
	watermark.Init(fontFs)
	log.Println("应用初始化完成")

	port := system.Config.Port

	log.Printf("Server starting on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
