package controllers

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	"oneimg/backend/database"
	"oneimg/backend/models"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ChangeAccountInfoRequest 修改登录信息请求结构
type ChangeAccountInfoRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"omitempty,min=6,max=100"`
	NewUsername     string `json:"new_username" binding:"omitempty,min=3,max=64"`
}

// AccountResponse 账户响应结构
type AccountResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Success bool   `json:"success"`
}

var uuidRegex = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$")

func ChangeAccountInfo(c *gin.Context) {
	var req ChangeAccountInfoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, AccountResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
			Success: false,
		})
		return
	}

	userID := c.GetInt("user_id")
	role := c.GetInt("user_role")
	if role == models.RoleGuest {
		c.JSON(http.StatusForbidden, AccountResponse{Code: 403, Message: "游客不能修改账户信息", Success: false})
		return
	}
	if role != models.RoleAdmin && strings.TrimSpace(req.NewUsername) != "" {
		c.JSON(http.StatusForbidden, AccountResponse{Code: 403, Message: "普通用户不能修改用户名", Success: false})
		return
	}
	req.NewUsername = strings.TrimSpace(req.NewUsername)
	if req.NewUsername == "" && req.NewPassword == "" {
		c.JSON(http.StatusBadRequest, AccountResponse{Code: 400, Message: "请至少修改一项", Success: false})
		return
	}
	if req.NewUsername != "" && isTouristUsername(req.NewUsername) {
		c.JSON(http.StatusBadRequest, AccountResponse{Code: 400, Message: "游客保留用户名", Success: false})
		return
	}

	errCurrentPassword := errors.New("当前密码错误")
	errUsernameExists := errors.New("用户名已存在")
	db := database.GetDB().DB
	err := db.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.First(&user, userID).Error; err != nil {
			return err
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)); err != nil {
			return errCurrentPassword
		}
		updates := map[string]any{}
		if req.NewUsername != "" {
			var count int64
			if err := tx.Model(&models.User{}).Where("username = ? AND id <> ?", req.NewUsername, userID).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return errUsernameExists
			}
			updates["username"] = req.NewUsername
		}
		if req.NewPassword != "" {
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
			if err != nil {
				return err
			}
			updates["password"] = string(hashedPassword)
		}
		return tx.Model(&user).Updates(updates).Error
	})
	if err != nil {
		status := http.StatusInternalServerError
		message := "账户信息修改失败"
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			status, message = http.StatusNotFound, "用户不存在"
		case errors.Is(err, errCurrentPassword):
			status, message = http.StatusBadRequest, errCurrentPassword.Error()
		case errors.Is(err, errUsernameExists):
			status, message = http.StatusConflict, errUsernameExists.Error()
		}
		c.JSON(status, AccountResponse{Code: status, Message: message, Success: false})
		return
	}

	session := sessions.Default(c)
	session.Clear()
	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, AccountResponse{
			Code:    500,
			Message: "会话失效失败: " + err.Error(),
			Success: false,
		})
		return
	}

	c.JSON(http.StatusOK, AccountResponse{
		Code:    200,
		Message: "修改成功",
		Success: true,
	})
}

// isTouristUsername 辅助函数，检查是否为游客账号
func isTouristUsername(username string) bool {
	return strings.HasPrefix(username, "guest_") || username == "guest" || uuidRegex.MatchString(username)
}

// ClearAllSessions 清除所有会话
func ClearAllSessions(c *gin.Context) {
	// 获取当前session
	session := sessions.Default(c)
	session.Clear()
	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, AccountResponse{
			Code:    500,
			Message: "清除会话失败",
			Success: false,
		})
		return
	}

	c.JSON(http.StatusOK, AccountResponse{
		Code:    200,
		Message: "所有会话已清除",
		Success: true,
	})
}

// 辅助函数，获取用户UUID
func GetUUID(c *gin.Context) string {
	uuidRegex := regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$")
	if uuidRegex.MatchString(c.GetString("username")) {
		return c.GetString("username")
	} else if c.GetString("username") == "00000000-0000-0000-0000-000000000000" {
		return "00000000-0000-0000-0000-000000000000"
	} else {
		return c.GetString("username")
	}
}
