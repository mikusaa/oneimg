package controllers

import (
	"net/http"
	"strings"
	"time"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"
	settingsutil "oneimg/backend/utils/settings"

	"github.com/gin-gonic/gin"
)

func Register(c *gin.Context) {
	setting, err := settingsutil.GetSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Fail(500, "获取设置失败"))
		return
	}
	if !setting.StartRegister {
		c.JSON(http.StatusForbidden, result.Fail(403, "暂未开放注册"))
		return
	}

	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		PowToken string `json:"powToken"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, result.Fail(400, "请求参数错误"))
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if len(req.Username) < 3 || len(req.Username) > 50 {
		c.JSON(http.StatusBadRequest, result.Fail(400, "用户名长度必须在3-50个字符之间"))
		return
	}
	if len(req.Password) < 6 || len(req.Password) > 100 {
		c.JSON(http.StatusBadRequest, result.Fail(400, "密码长度必须在6-100个字符之间"))
		return
	}
	if isTouristUsername(req.Username) {
		c.JSON(http.StatusBadRequest, result.Fail(400, "该用户名不可注册"))
		return
	}
	if setting.PowVerify && !ValidatePowToken(req.PowToken) {
		c.JSON(http.StatusBadRequest, result.Fail(400, "人机验证失败，请重试"))
		return
	}

	db := database.GetDB().DB
	var existingCount int64
	if err := db.Model(&models.User{}).Where("username = ?", req.Username).Count(&existingCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Fail(500, "检查用户名失败"))
		return
	}
	if existingCount > 0 {
		c.JSON(http.StatusConflict, result.Fail(409, "用户名已存在"))
		return
	}
	hashedPassword, err := hashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Fail(500, "密码加密失败"))
		return
	}
	user := models.User{
		Username: req.Username,
		Password: hashedPassword,
		Role:     models.RoleUser,
		Permission: models.Permission{
			Codes:   []string{},
			Buckets: []int{},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.Create(&user).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") || strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			c.JSON(http.StatusConflict, result.Fail(409, "用户名已存在"))
			return
		}
		c.JSON(http.StatusInternalServerError, result.Fail(500, "注册失败"))
		return
	}
	c.JSON(http.StatusOK, result.Success("注册成功，请登录", map[string]any{
		"id": user.ID, "username": user.Username, "role": user.Role,
	}))
}
