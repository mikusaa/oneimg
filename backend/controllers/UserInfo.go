package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"oneimg/backend/middlewares"
	"oneimg/backend/utils/result"
)

func CheckLoginStatus(c *gin.Context) {
	user, ok := middlewares.GetCurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, result.Error(401, "用户信息无效"))
		return
	}

	// 使用统一返回格式
	c.JSON(http.StatusOK, result.Success(
		"已登录",
		map[string]any{
			"user_id":    user.ID,
			"username":   user.Username,
			"role":       user.Role,
			"user_role":  user.Role,
			"permission": user.Permission,
			"logged_in":  true,
		}))
}
