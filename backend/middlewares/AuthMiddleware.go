package middlewares

import (
	"net/http"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/secureconfig"
	"oneimg/backend/utils/settings"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// AuthResponse 认证失败响应结构
type AuthResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// AuthMiddleware Session认证中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		setting, _ := settings.GetSettings()
		apiToken := ""
		if setting.StartAPI {
			// 获取请求头中的token
			authHeader := c.Request.Header.Get("Authorization")
			parts := strings.SplitN(authHeader, "=", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[0]) == "oneimg_token" {
				apiToken = strings.TrimSpace(parts[1])
			}

			if validateToken(setting, apiToken) {
				apiUser := &models.User{
					ID:       models.SuperAdminID,
					Role:     models.RoleAdmin,
					Username: "api_admin",
					Permission: models.Permission{
						Codes:   []string{"*"},
						Buckets: []int{},
					},
				}
				c.Set("user_id", apiUser.ID)
				c.Set("user_role", apiUser.Role)
				c.Set("username", apiUser.Username)
				c.Set("current_user", apiUser)
				c.Next()
				return
			}
		}

		// 获取session
		session := sessions.Default(c)

		// 检查是否已登录
		loggedIn := session.Get("logged_in")
		if (loggedIn == nil || loggedIn != true) && apiToken == "" {
			c.JSON(http.StatusUnauthorized, AuthResponse{
				Code:    401,
				Message: "用户未登录",
			})
			c.Abort()
			return
		}

		// 获取用户信息
		userID := session.Get("user_id")
		userRole := session.Get("user_role")
		username := session.Get("username")

		if userID == nil || username == nil {
			c.JSON(http.StatusUnauthorized, AuthResponse{
				Code:    401,
				Message: "会话信息无效",
			})
			c.Abort()
			return
		}

		userIDValue, userIDOK := userID.(int)
		userRoleValue, userRoleOK := userRole.(int)
		usernameValue, usernameOK := username.(string)
		if !userIDOK || !userRoleOK || !usernameOK {
			c.JSON(http.StatusUnauthorized, AuthResponse{Code: 401, Message: "会话信息无效"})
			c.Abort()
			return
		}

		// 游客是虚拟账号；其他会话每次核对用户，使删除/角色变更立即生效。
		var currentUser models.User
		if userRoleValue != models.RoleGuest {
			db := database.GetDB()
			if db == nil || db.DB.Select("id", "role", "username", "permission").First(&currentUser, userIDValue).Error != nil {
				session.Clear()
				_ = session.Save()
				c.JSON(http.StatusUnauthorized, AuthResponse{Code: 401, Message: "用户不存在或已被禁用"})
				c.Abort()
				return
			}
			userRoleValue = currentUser.Role
			usernameValue = currentUser.Username
			session.Set("user_role", userRoleValue)
			session.Set("username", usernameValue)
		} else {
			currentUser = models.User{
				ID:         userIDValue,
				Role:       models.RoleGuest,
				Username:   usernameValue,
				Permission: models.Permission{Codes: []string{}, Buckets: []int{}},
			}
		}

		// 将用户信息存储到上下文中，供后续处理使用
		session.Set("logged_in", true)

		c.Set("user_id", userIDValue)
		c.Set("user_role", userRoleValue)
		c.Set("username", usernameValue)
		c.Set("current_user", &currentUser)

		// 继续处理请求
		c.Next()
	}
}

func RequirePermission(code string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := GetCurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, AuthResponse{Code: 401, Message: "用户信息获取失败"})
			c.Abort()
			return
		}
		if user.ID == models.SuperAdminID {
			c.Next()
			return
		}
		if user.Role != models.RoleAdmin || !user.Permission.HasPermission(code) {
			c.JSON(http.StatusForbidden, AuthResponse{
				Code:    403,
				Message: "无操作权限，需要权限: [" + models.PermissionName(code) + "]",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

func RequireAnyPermission(codes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := GetCurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, AuthResponse{Code: 401, Message: "用户信息获取失败"})
			c.Abort()
			return
		}
		if user.ID == models.SuperAdminID {
			c.Next()
			return
		}
		if user.Role == models.RoleAdmin {
			for _, code := range codes {
				if user.Permission.HasPermission(code) {
					c.Next()
					return
				}
			}
		}
		c.JSON(http.StatusForbidden, AuthResponse{Code: 403, Message: "无操作权限"})
		c.Abort()
	}
}

func HasPermission(c *gin.Context, code string) bool {
	user, ok := GetCurrentUser(c)
	if !ok {
		return false
	}
	return user.ID == models.SuperAdminID || (user.Role == models.RoleAdmin && user.Permission.HasPermission(code))
}

func validateToken(setting models.Settings, token string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	if secureconfig.CompareSecretHash(setting.APITokenHash, token) {
		return true
	}
	return setting.APIToken != "" && secureconfig.ConstantTimeEqual(setting.APIToken, token)
}

func AdminOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取用户ID
		userRole := c.GetInt("user_role")

		if userRole != models.RoleAdmin {
			c.JSON(http.StatusForbidden, AuthResponse{
				Code:    403,
				Message: "无权访问",
			})
			c.Abort()
			return
		}

		// 继续处理请求
		c.Next()
	}
}

// OptionalAuthMiddleware 可选认证中间件（不强制要求认证）
func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取session
		session := sessions.Default(c)

		// 检查是否已登录
		loggedIn := session.Get("logged_in")
		if loggedIn != nil && loggedIn == true {
			// 获取用户信息
			userID := session.Get("user_id")
			username := session.Get("username")

			if userID != nil && username != nil {
				// 将用户信息存储到上下文中
				c.Set("user_id", userID)
				c.Set("username", username)
			}
		}

		// 继续处理请求（无论是否登录）
		c.Next()
	}
}

func GetCurrentUser(c *gin.Context) (*models.User, bool) {
	value, exists := c.Get("current_user")
	if !exists {
		return nil, false
	}
	user, ok := value.(*models.User)
	return user, ok
}
