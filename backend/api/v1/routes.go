package v1

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func (s *Server) Register(engine *gin.Engine) {
	v1 := engine.Group("/api/v1")

	public := v1.Group("/public")
	public.GET("/config", s.publicConfig)
	public.GET("/images/random", s.randomImagesPlaceholder)

	authPublic := v1.Group("/auth")
	authPublic.POST("/login", s.limitByIP("login", 10, 5*time.Minute), s.login)
	authPublic.POST("/register", s.limitByIP("register", 5, time.Hour), s.register)
	authPublic.POST("/passkeys/login/options", s.limitByIP("passkey-login", 10, 5*time.Minute), s.passkeyLoginOptions)
	authPublic.POST("/passkeys/login/verify", s.limitByIP("passkey-login", 10, 5*time.Minute), s.passkeyLoginVerify)

	auth := v1.Group("")
	auth.Use(s.requireAuth(), s.limitBearer, s.csrfProtection())
	auth.POST("/auth/logout", requireSession(), s.logout)
	auth.GET("/me", s.me)
	auth.PATCH("/me", requireSession(), s.updateMe)
	auth.GET("/me/passkeys", requireSession(), s.listPasskeys)
	auth.POST("/me/passkeys/registration/options", requireSession(), s.passkeyRegistrationOptions)
	auth.POST("/me/passkeys/registration/verify", requireSession(), s.passkeyRegistrationVerify)
	auth.PATCH("/me/passkeys/:id", requireSession(), s.renamePasskey)
	auth.POST("/me/passkeys/:id/revoke", requireSession(), s.revokePasskey)
	auth.GET("/me/tokens", requireSession(), s.listTokens)
	auth.POST("/me/tokens", requireSession(), s.createToken)
	auth.POST("/me/tokens/:id/revoke", requireSession(), s.revokeToken)

	auth.GET("/upload-options", requireScope("images:write"), s.uploadOptions)
	auth.GET("/images", requireScope("images:read"), s.listImages)
	auth.POST("/images", requireScope("images:write"), s.limitByPrincipal("upload", 30, time.Minute), s.uploadImages)
	auth.GET("/images/:id", requireScope("images:read"), s.getImage)
	auth.DELETE("/images/:id", requireScope("images:delete"), s.deleteImage)
	auth.POST("/image-imports", requireScope("images:write"), s.limitByPrincipal("upload", 30, time.Minute), s.importImage)
	auth.PUT("/images/:id/tags/:tag_id", requireScope("images:write"), s.putImageTag)
	auth.DELETE("/images/:id/tags/:tag_id", requireScope("images:write"), s.deleteImageTag)
	auth.PATCH("/images/tags", requireScope("images:write"), s.patchImageTags)

	auth.GET("/tags", requireScope("tags:read"), s.listTags)
	auth.POST("/tags", requireScope("tags:write"), requirePermission("tag:create"), s.createTag)
	auth.PATCH("/tags/:id", requireScope("tags:write"), requirePermission("tag:update"), s.updateTag)
	auth.DELETE("/tags/:id", requireScope("tags:write"), requirePermission("tag:delete"), s.deleteTag)

	auth.GET("/storage-buckets", requireScope("storage:read"), requireAnyPermission("storage:create", "storage:update", "storage:delete"), s.listStorage)
	auth.POST("/storage-buckets", requireScope("storage:write"), requirePermission("storage:create"), s.createStorage)
	auth.GET("/storage-buckets/:id", requireScope("storage:read"), requireAnyPermission("storage:create", "storage:update", "storage:delete"), s.getStorage)
	auth.PATCH("/storage-buckets/:id", requireScope("storage:write"), requirePermission("storage:update"), s.updateStorage)
	auth.DELETE("/storage-buckets/:id", requireScope("storage:write"), requirePermission("storage:delete"), s.deleteStorage)
	auth.POST("/storage-connection-tests", requireScope("storage:write"), requireAnyPermission("storage:create", "storage:update"), s.testStorageConnection)

	auth.GET("/stats/dashboard", requireScope("stats:read"), s.dashboardStats)
	auth.GET("/stats/images", requireScope("stats:read"), s.imageStats)

	auth.GET("/users", requireScope("users:read"), requirePermission("user:list"), s.listUsers)
	auth.POST("/users", requireScope("users:write"), requirePermission("user:create"), s.createUser)
	auth.PATCH("/users/:id", requireScope("users:write"), requirePermission("user:role:update"), s.updateUser)
	auth.DELETE("/users/:id", requireScope("users:write"), requirePermission("user:delete"), s.deleteUser)
	auth.PUT("/users/:id/permissions", requireScope("users:write"), requirePermission("user:permission:update"), s.updateUserPermissions)
	auth.POST("/users/:id/password-reset", requireScope("users:write"), requirePermission("user:password:reset"), s.resetUserPassword)
	auth.POST("/users/:id/passkeys/revoke", requireScope("users:write"), requirePermission("user:passkey:reset"), s.revokeUserPasskeys)

	auth.GET("/settings", requireScope("settings:read"), s.getSettings)
	auth.PATCH("/settings", requireScope("settings:write"), s.patchSettings)
}

func (s *Server) NotFound(c *gin.Context) {
	writeProblem(c, http.StatusNotFound, "api_not_found", "API 路径不存在")
}

func (s *Server) MethodNotAllowed(c *gin.Context) {
	writeProblem(c, http.StatusMethodNotAllowed, "method_not_allowed", "该 API 不支持此 HTTP 方法")
}
