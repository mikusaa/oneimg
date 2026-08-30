package v1

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"oneimg/backend/models"
	"oneimg/backend/services"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type userDTO struct {
	ID         int           `json:"id"`
	Username   string        `json:"username"`
	Role       int           `json:"role"`
	Permission permissionDTO `json:"permission"`
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
}

type permissionDTO struct {
	Codes     []string `json:"codes"`
	BucketIDs []int    `json:"bucket_ids"`
}

func toUserDTO(user models.User) userDTO {
	return userDTO{
		ID: user.ID, Username: user.Username, Role: user.Role,
		Permission: permissionDTO{Codes: user.Permission.Codes, BucketIDs: user.Permission.Buckets},
		CreatedAt:  user.CreatedAt.UTC(), UpdatedAt: user.UpdatedAt.UTC(),
	}
}

func (s *Server) publicConfig(c *gin.Context) {
	setting, err := s.services.Settings.Get()
	if err != nil {
		writeProblem(c, http.StatusInternalServerError, "settings_unavailable", "无法读取公开配置")
		return
	}
	writeData(c, http.StatusOK, gin.H{
		"registration_enabled": setting.StartRegister,
		"passkey_available":    passkeysAvailable(),
		"site": gin.H{
			"title":           setting.SEOTitle,
			"description":     setting.SEODescription,
			"keywords":        setting.SEOKeywords,
			"icp":             setting.SEOICP,
			"public_security": setting.PublicSecurity,
			"icon":            setting.SEOicon,
		},
	}, nil)
}

func (s *Server) randomImagesPlaceholder(c *gin.Context) {
	writeProblem(c, http.StatusNotImplemented, "random_images_not_implemented", "随机图片能力将在后续版本实现")
}

func (s *Server) login(c *gin.Context) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !bindJSON(c, &input) {
		return
	}
	if input.Username == "" || input.Password == "" {
		writeProblem(c, http.StatusUnprocessableEntity, "validation_error", "用户名和密码不能为空")
		return
	}
	user, err := s.services.Accounts.Authenticate(input.Username, input.Password)
	if errors.Is(err, services.ErrInvalidCredentials) {
		writeProblem(c, http.StatusUnauthorized, "invalid_credentials", "用户名或密码错误")
		return
	}
	if errors.Is(err, services.ErrAccountDisabled) {
		writeProblem(c, http.StatusForbidden, "invalid_account_role", "账户角色无效")
		return
	}
	if err != nil {
		writeProblem(c, http.StatusInternalServerError, "account_read_failed", "无法读取账户")
		return
	}
	if err := s.saveSession(c, &user); err != nil {
		writeProblem(c, http.StatusInternalServerError, "session_save_failed", "无法保存登录会话")
		return
	}
	if err := s.issueCSRFCookie(c); err != nil {
		writeProblem(c, http.StatusInternalServerError, "csrf_token_failed", "无法初始化安全会话")
		return
	}
	writeData(c, http.StatusOK, toUserDTO(user), nil)
}

func (s *Server) register(c *gin.Context) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !bindJSON(c, &input) {
		return
	}
	user, err := s.services.Accounts.Register(input.Username, input.Password)
	switch {
	case errors.Is(err, services.ErrRegistrationDisabled):
		writeProblem(c, http.StatusForbidden, "registration_disabled", "当前未开放注册")
		return
	case errors.Is(err, services.ErrUsernameInvalid):
		writeProblem(c, http.StatusUnprocessableEntity, "validation_error", "用户名长度必须为 3-50 个字符")
		return
	case errors.Is(err, services.ErrPasswordInvalid):
		writeProblem(c, http.StatusUnprocessableEntity, "validation_error", "密码长度必须为 6-100 个字符")
		return
	case errors.Is(err, services.ErrUsernameConflict):
		writeProblem(c, http.StatusConflict, "username_conflict", "用户名已存在")
		return
	case err != nil:
		writeProblem(c, http.StatusInternalServerError, "account_create_failed", "无法创建账户")
		return
	}
	c.Header("Location", "/api/v1/users/"+itoa(user.ID))
	writeData(c, http.StatusCreated, toUserDTO(user), nil)
}

func (s *Server) logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	if err := session.Save(); err != nil {
		writeProblem(c, http.StatusInternalServerError, "session_clear_failed", "无法退出登录")
		return
	}
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(csrfCookieName, "", -1, "/", "", strings.HasPrefix(strings.ToLower(s.services.Config.AppURL), "https://"), false)
	writeNoContent(c)
}

func (s *Server) me(c *gin.Context) {
	user, err := currentUser(c)
	if err != nil {
		writeProblem(c, http.StatusUnauthorized, "authentication_required", "请先登录")
		return
	}
	if principal, _ := principalFrom(c); principal != nil && principal.IsSession() {
		if _, err := c.Cookie(csrfCookieName); err != nil {
			_ = s.issueCSRFCookie(c)
		}
	}
	writeData(c, http.StatusOK, toUserDTO(*user), nil)
}

func (s *Server) updateMe(c *gin.Context) {
	principal, _ := principalFrom(c)
	if principal == nil || !principal.IsSession() {
		writeProblem(c, http.StatusForbidden, "session_required", "账户凭据只能通过浏览器会话修改")
		return
	}
	var input struct {
		CurrentPassword string  `json:"current_password"`
		Username        *string `json:"username"`
		Password        *string `json:"password"`
	}
	if !bindJSON(c, &input) {
		return
	}
	updated, err := s.services.Accounts.Update(principal.User.ID, input.CurrentPassword, input.Username, input.Password)
	switch {
	case errors.Is(err, services.ErrAccountFieldsRequired):
		writeProblem(c, http.StatusUnprocessableEntity, "validation_error", "至少需要修改一个字段")
		return
	case errors.Is(err, services.ErrCurrentPassword):
		writeProblem(c, http.StatusUnauthorized, "current_password_invalid", "当前密码错误")
		return
	case errors.Is(err, services.ErrUsernameChangeForbidden):
		writeProblem(c, http.StatusForbidden, "username_change_forbidden", "普通用户不能修改用户名")
		return
	case errors.Is(err, services.ErrUsernameInvalid):
		writeProblem(c, http.StatusUnprocessableEntity, "validation_error", "用户名长度必须为 3-50 个字符")
		return
	case errors.Is(err, services.ErrPasswordInvalid):
		writeProblem(c, http.StatusUnprocessableEntity, "validation_error", "密码长度必须为 6-100 个字符")
		return
	case errors.Is(err, services.ErrUsernameConflict):
		writeProblem(c, http.StatusConflict, "username_conflict", "用户名已存在")
		return
	case errors.Is(err, gorm.ErrRecordNotFound):
		writeProblem(c, http.StatusNotFound, "user_not_found", "用户不存在")
		return
	case err != nil:
		writeProblem(c, http.StatusInternalServerError, "account_update_failed", "无法更新账户")
		return
	}
	session := sessions.Default(c)
	session.Clear()
	_ = session.Save()
	writeData(c, http.StatusOK, gin.H{"user": toUserDTO(updated), "reauthentication_required": true}, nil)
}

func (s *Server) saveSession(c *gin.Context, user *models.User) error {
	session := sessions.Default(c)
	session.Set("user_id", user.ID)
	session.Set("user_role", user.Role)
	session.Set("username", user.Username)
	session.Set("logged_in", true)
	session.Options(sessions.Options{MaxAge: 24 * 60 * 60, HttpOnly: true,
		Secure:   strings.HasPrefix(strings.ToLower(s.services.Config.AppURL), "https://"),
		SameSite: http.SameSiteStrictMode, Path: "/"})
	return session.Save()
}
