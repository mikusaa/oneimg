package controllers

import (
	"net/http"
	"oneimg/backend/config"
	"strings"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// 登录请求结构
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应结构
type LoginResponse struct {
	Token string       `json:"token,omitempty"`
	User  *models.User `json:"user,omitempty"`
}

func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(
			400,
			"请求参数错误",
		))
		return
	}

	// 获取数据库实例
	db := database.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "数据库连接失败"))
		return
	}

	var user models.User
	userInfo := db.DB.Where("username = ?", req.Username).First(&user)

	// 用户不存在
	if userInfo.Error != nil {
		c.JSON(http.StatusBadRequest, result.Error(401, "用户名或密码错误"))
		return
	}
	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(401, "用户名或密码错误"))
		return
	}
	if user.Role != models.RoleAdmin && user.Role != models.RoleUser {
		c.JSON(http.StatusForbidden, result.Error(403, "账户角色无效"))
		return
	}

	// 设置session
	session, err := SetSession(c, &user)
	if err != nil {
		return
	}

	// 返回结果去除密码
	user.Password = ""
	// 返回结果
	c.JSON(http.StatusOK, result.Success("登录成功", map[string]any{
		"token": session.ID(),
		"user":  user,
	}))
}

// 设置Session
func SetSession(c *gin.Context, user *models.User) (sessions.Session, error) {
	session, err := saveUserSession(c, user)
	if err != nil {
		errMsg := "session保存失败：" + err.Error()
		c.JSON(http.StatusInternalServerError, result.Error(500, errMsg))
		return nil, err
	}
	return session, nil
}

// saveUserSession 只保存会话，由调用方决定返回 JSON 还是重定向。
func saveUserSession(c *gin.Context, user *models.User) (sessions.Session, error) {
	// 获取session
	session := sessions.Default(c)

	// 设置session数据
	session.Set("user_id", user.ID)
	session.Set("user_role", user.Role)
	session.Set("username", user.Username)
	session.Set("logged_in", true)

	// 设置session选项
	session.Options(sessions.Options{
		MaxAge:   24 * 60 * 60, // 24小时，单位秒
		HttpOnly: true,         // 防止XSS攻击
		Secure:   strings.HasPrefix(strings.ToLower(config.App.AppURL), "https://"),
		SameSite: http.SameSiteStrictMode, // 防止CSRF攻击
		Path:     "/",                     // cookie路径
	})

	// 保存session
	if err := session.Save(); err != nil {
		return nil, err
	}

	return session, nil
}

// 退出登录
func Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "退出登录失败"))
		return
	}

	c.JSON(http.StatusOK, result.Success("退出登录成功", nil))
}
