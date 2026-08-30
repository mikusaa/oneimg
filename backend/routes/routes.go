package routes

import (
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"time"

	apiV1 "oneimg/backend/api/v1"
	"oneimg/backend/app"
	"oneimg/backend/media"
	"oneimg/backend/middlewares"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/goccy/go-yaml"
)

// 设置路由
func SetupRoutes(frontendFS fs.FS, system *app.System) *gin.Engine {
	cfg := system.Config

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.HandleMethodNotAllowed = true
	v1Server := apiV1.NewServer(system.Services)

	// 基础中间件
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middlewares.SessionMiddleware(cfg))
	r.Use(apiV1.RequestIDMiddleware())
	r.Use(v1Server.OriginProtection())

	// 跨域配置
	r.Use(cors.New(cors.Config{
		AllowOriginFunc: func(origin string) bool {
			return allowedBrowserOrigin(origin, cfg.AppURL)
		},
		AllowMethods:     []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With", "X-OneImg-CSRF", "X-Request-ID"},
		ExposeHeaders:    []string{"Content-Length", "Location", "Retry-After", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	distFS, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		panic("加载前端文件失败：" + err.Error())
	}
	assetsFS, _ := fs.Sub(distFS, "assets")
	r.StaticFS("/assets", http.FS(assetsFS))

	// 静态资源
	r.StaticFile("/favicon.ico", "./frontend/dist/favicon.ico")

	v1Server.Register(r)
	r.GET("/api/openapi.yaml", func(c *gin.Context) {
		content, err := fs.ReadFile(frontendFS, "api/openapi.yaml")
		if err != nil {
			v1Server.NotFound(c)
			return
		}
		c.Data(http.StatusOK, "application/yaml; charset=utf-8", content)
	})
	r.GET("/api/openapi.json", func(c *gin.Context) {
		content, err := fs.ReadFile(frontendFS, "api/openapi.yaml")
		if err != nil {
			v1Server.NotFound(c)
			return
		}
		jsonContent, err := yaml.YAMLToJSON(content)
		if err != nil {
			c.String(http.StatusInternalServerError, "OpenAPI 文档转换失败")
			return
		}
		c.Data(http.StatusOK, "application/json; charset=utf-8", jsonContent)
	})
	r.GET("/api/docs", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(`<!doctype html><html><head><meta charset="utf-8"><title>OneImg API</title><link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css"></head><body><div id="swagger-ui"></div><script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script><script>SwaggerUIBundle({url:'/api/openapi.yaml',dom_id:'#swagger-ui',deepLinking:true,persistAuthorization:true})</script></body></html>`))
	})
	r.NoMethod(v1Server.MethodNotAllowed)

	// 前端SPA路由与图片代理逻辑集成
	r.NoRoute(func(c *gin.Context) {
		// 1. API路径返回404
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			v1Server.NotFound(c)
			return
		}

		// 2. 尝试通过图片代理识别图片路径
		if media.ImageProxy(c) {
			return
		}

		// 3. 回退到前端 SPA 页面
		indexContent, err := fs.ReadFile(distFS, "index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "加载前端页面失败：%s", err)
			return
		}

		// 返回HTML内容
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, string(indexContent))
	})

	return r
}

func allowedBrowserOrigin(origin, appURL string) bool {
	if strings.TrimSpace(origin) == "" {
		return true
	}
	parsedOrigin, originErr := url.Parse(strings.TrimSpace(origin))
	parsedApp, appErr := url.Parse(strings.TrimSpace(appURL))
	if originErr == nil && appErr == nil && strings.EqualFold(parsedOrigin.Scheme, parsedApp.Scheme) && strings.EqualFold(parsedOrigin.Host, parsedApp.Host) {
		return true
	}
	return originErr == nil && isLoopbackHTTPOrigin(parsedOrigin) && isLoopbackHTTPOrigin(parsedApp)
}

func isLoopbackHTTPOrigin(parsed *url.URL) bool {
	host := strings.ToLower(parsed.Hostname())
	return (parsed.Scheme == "http" || parsed.Scheme == "https") && (host == "localhost" || host == "127.0.0.1" || host == "::1")
}
