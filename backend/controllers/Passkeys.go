package controllers

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/passkeys"
	"oneimg/backend/utils/result"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/webauthn"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	passkeyLoginSessionKey        = "passkey_login_session"
	passkeyRegistrationSessionKey = "passkey_registration_session"
	maxPasskeysPerUser            = 10
)

type passkeyCeremony struct {
	Session webauthn.SessionData `json:"session"`
	UserID  int                  `json:"user_id,omitempty"`
	Name    string               `json:"name,omitempty"`
}

type passkeyResponse struct {
	ID         uint       `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

func BeginPasskeyLogin(c *gin.Context) {
	client, err := passkeys.Client()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, result.Error(503, "Passkey 登录暂不可用"))
		return
	}

	options, sessionData, err := client.BeginDiscoverableLogin()
	if err != nil {
		log.Printf("生成 Passkey 登录挑战失败: %v", err)
		c.JSON(http.StatusInternalServerError, result.Error(500, "Passkey 登录暂不可用"))
		return
	}
	if err := savePasskeyCeremony(c, passkeyLoginSessionKey, passkeyCeremony{Session: *sessionData}); err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "Passkey 登录初始化失败"))
		return
	}
	c.JSON(http.StatusOK, result.Success("ok", map[string]any{"options": options.Response}))
}

func FinishPasskeyLogin(c *gin.Context) {
	client, err := passkeys.Client()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, result.Error(503, "Passkey 登录暂不可用"))
		return
	}
	ceremony, err := consumePasskeyCeremony(c, passkeyLoginSessionKey)
	if err != nil {
		passkeyLoginFailed(c, err)
		return
	}

	db := database.GetDB().DB
	user, credential, err := client.FinishPasskeyLogin(func(rawID, userHandle []byte) (webauthn.User, error) {
		var stored models.PasskeyCredential
		if err := db.Where("credential_id = ?", passkeys.CredentialKey(rawID)).First(&stored).Error; err != nil {
			return nil, errors.New("credential not found")
		}
		if !bytes.Equal(userHandle, passkeys.UserHandle(stored.UserID)) {
			return nil, errors.New("user handle mismatch")
		}

		var account models.User
		if err := db.First(&account, stored.UserID).Error; err != nil {
			return nil, errors.New("account not found")
		}
		if account.Role != models.RoleAdmin && account.Role != models.RoleUser {
			return nil, errors.New("account role invalid")
		}
		storedCredentials, err := loadStoredPasskeys(db, account.ID)
		if err != nil {
			return nil, err
		}
		return passkeys.BuildUser(account, storedCredentials)
	}, ceremony.Session, c.Request)
	if err != nil {
		passkeyLoginFailed(c, err)
		return
	}
	passkeyUser, ok := user.(*passkeys.User)
	if !ok || credential == nil {
		passkeyLoginFailed(c, errors.New("invalid passkey user"))
		return
	}
	if credential.Authenticator.CloneWarning {
		passkeyLoginFailed(c, errors.New("authenticator clone warning"))
		return
	}

	credentialID, encryptedData, err := passkeys.EncodeCredential(credential)
	if err != nil {
		passkeyLoginFailed(c, err)
		return
	}
	now := time.Now()
	update := db.Model(&models.PasskeyCredential{}).
		Where("user_id = ? AND credential_id = ?", passkeyUser.Account.ID, credentialID).
		Updates(map[string]any{"credential_data": encryptedData, "last_used_at": &now})
	if update.Error != nil || update.RowsAffected != 1 {
		if update.Error != nil {
			err = update.Error
		} else {
			err = errors.New("credential update did not match a row")
		}
		passkeyLoginFailed(c, err)
		return
	}

	session, err := SetSession(c, &passkeyUser.Account)
	if err != nil {
		return
	}
	passkeyUser.Account.Password = ""
	c.JSON(http.StatusOK, result.Success("登录成功", map[string]any{
		"token": session.ID(),
		"user":  passkeyUser.Account,
	}))
}

func BeginPasskeyRegistration(c *gin.Context) {
	client, err := passkeys.Client()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, result.Error(503, "Passkey 暂不可用"))
		return
	}
	var req struct {
		Name            string `json:"name" binding:"required"`
		CurrentPassword string `json:"current_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "请输入设备名称和当前密码"))
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if utf8.RuneCountInString(req.Name) < 1 || utf8.RuneCountInString(req.Name) > 50 {
		c.JSON(http.StatusBadRequest, result.Error(400, "设备名称长度必须在 1-50 个字符之间"))
		return
	}

	db := database.GetDB().DB
	var account models.User
	if err := db.First(&account, c.GetInt("user_id")).Error; err != nil {
		c.JSON(http.StatusUnauthorized, result.Error(401, "用户不存在"))
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(account.Password), []byte(req.CurrentPassword)) != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "当前密码错误"))
		return
	}

	storedCredentials, err := loadStoredPasskeys(db, account.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "读取 Passkey 失败"))
		return
	}
	if len(storedCredentials) >= maxPasskeysPerUser {
		c.JSON(http.StatusConflict, result.Error(409, "每个账户最多绑定 10 个 Passkey"))
		return
	}
	var duplicateName int64
	if err := db.Model(&models.PasskeyCredential{}).
		Where("user_id = ? AND LOWER(name) = LOWER(?)", account.ID, req.Name).
		Count(&duplicateName).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "检查设备名称失败"))
		return
	}
	if duplicateName > 0 {
		c.JSON(http.StatusConflict, result.Error(409, "设备名称已存在"))
		return
	}

	passkeyUser, err := passkeys.BuildUser(account, storedCredentials)
	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "读取 Passkey 失败"))
		return
	}
	options, sessionData, err := client.BeginRegistration(
		passkeyUser,
		webauthn.WithExclusions(webauthn.Credentials(passkeyUser.Credentials).CredentialDescriptors()),
	)
	if err != nil {
		log.Printf("生成 Passkey 注册挑战失败(user_id=%d): %v", account.ID, err)
		c.JSON(http.StatusInternalServerError, result.Error(500, "Passkey 注册初始化失败"))
		return
	}
	if err := savePasskeyCeremony(c, passkeyRegistrationSessionKey, passkeyCeremony{
		Session: *sessionData,
		UserID:  account.ID,
		Name:    req.Name,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "Passkey 注册初始化失败"))
		return
	}
	c.JSON(http.StatusOK, result.Success("ok", map[string]any{"options": options.Response}))
}

func FinishPasskeyRegistration(c *gin.Context) {
	client, err := passkeys.Client()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, result.Error(503, "Passkey 暂不可用"))
		return
	}
	ceremony, err := consumePasskeyCeremony(c, passkeyRegistrationSessionKey)
	if err != nil || ceremony.UserID != c.GetInt("user_id") {
		c.JSON(http.StatusBadRequest, result.Error(400, "Passkey 注册请求已过期，请重试"))
		return
	}

	db := database.GetDB().DB
	var account models.User
	if err := db.First(&account, ceremony.UserID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, result.Error(401, "用户不存在"))
		return
	}
	storedCredentials, err := loadStoredPasskeys(db, account.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "读取 Passkey 失败"))
		return
	}
	passkeyUser, err := passkeys.BuildUser(account, storedCredentials)
	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "读取 Passkey 失败"))
		return
	}
	credential, err := client.FinishRegistration(passkeyUser, ceremony.Session, c.Request)
	if err != nil {
		log.Printf("验证 Passkey 注册响应失败(user_id=%d): %v", account.ID, err)
		c.JSON(http.StatusBadRequest, result.Error(400, "Passkey 注册失败，请重试"))
		return
	}
	credentialID, encryptedData, err := passkeys.EncodeCredential(credential)
	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "保存 Passkey 失败"))
		return
	}

	stored := models.PasskeyCredential{
		UserID:         account.ID,
		Name:           ceremony.Name,
		CredentialID:   credentialID,
		CredentialData: encryptedData,
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		var count, duplicateName int64
		if err := tx.Model(&models.PasskeyCredential{}).Where("user_id = ?", account.ID).Count(&count).Error; err != nil {
			return err
		}
		if count >= maxPasskeysPerUser {
			return errors.New("passkey limit reached")
		}
		if err := tx.Model(&models.PasskeyCredential{}).
			Where("user_id = ? AND LOWER(name) = LOWER(?)", account.ID, ceremony.Name).
			Count(&duplicateName).Error; err != nil {
			return err
		}
		if duplicateName > 0 {
			return errors.New("passkey name exists")
		}
		return tx.Create(&stored).Error
	})
	if err != nil {
		status := http.StatusInternalServerError
		message := "保存 Passkey 失败"
		lowerError := strings.ToLower(err.Error())
		if strings.Contains(lowerError, "unique") || strings.Contains(lowerError, "passkey limit") || strings.Contains(lowerError, "name exists") {
			status = http.StatusConflict
			message = "该 Passkey 已存在或账户已达到数量限制"
		}
		c.JSON(status, result.Error(status, message))
		return
	}
	c.JSON(http.StatusOK, result.Success("Passkey 添加成功", passkeyListItem(stored)))
}

func GetPasskeys(c *gin.Context) {
	stored, err := loadStoredPasskeys(database.GetDB().DB, c.GetInt("user_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "获取 Passkey 失败"))
		return
	}
	items := make([]passkeyResponse, 0, len(stored))
	for _, item := range stored {
		items = append(items, passkeyListItem(item))
	}
	c.JSON(http.StatusOK, result.Success("ok", map[string]any{"passkeys": items, "count": len(items)}))
}

func RenamePasskey(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, result.Error(400, "Passkey ID 无效"))
		return
	}
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "请输入设备名称"))
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if utf8.RuneCountInString(req.Name) < 1 || utf8.RuneCountInString(req.Name) > 50 {
		c.JSON(http.StatusBadRequest, result.Error(400, "设备名称长度必须在 1-50 个字符之间"))
		return
	}

	db := database.GetDB().DB
	var duplicate int64
	if err := db.Model(&models.PasskeyCredential{}).
		Where("user_id = ? AND id <> ? AND LOWER(name) = LOWER(?)", c.GetInt("user_id"), id, req.Name).
		Count(&duplicate).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "检查设备名称失败"))
		return
	}
	if duplicate > 0 {
		c.JSON(http.StatusConflict, result.Error(409, "设备名称已存在"))
		return
	}
	updated := db.Model(&models.PasskeyCredential{}).
		Where("id = ? AND user_id = ?", id, c.GetInt("user_id")).
		Update("name", req.Name)
	if updated.Error != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "重命名 Passkey 失败"))
		return
	}
	if updated.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, result.Error(404, "Passkey 不存在"))
		return
	}
	c.JSON(http.StatusOK, result.Success("重命名成功", nil))
}

func DeletePasskey(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, result.Error(400, "Passkey ID 无效"))
		return
	}
	var req struct {
		CurrentPassword string `json:"current_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "请输入当前密码"))
		return
	}

	db := database.GetDB().DB
	var account models.User
	if err := db.First(&account, c.GetInt("user_id")).Error; err != nil {
		c.JSON(http.StatusUnauthorized, result.Error(401, "用户不存在"))
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(account.Password), []byte(req.CurrentPassword)) != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "当前密码错误"))
		return
	}
	deleted := db.Where("id = ? AND user_id = ?", id, account.ID).Delete(&models.PasskeyCredential{})
	if deleted.Error != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "删除 Passkey 失败"))
		return
	}
	if deleted.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, result.Error(404, "Passkey 不存在"))
		return
	}
	c.JSON(http.StatusOK, result.Success("Passkey 已删除", nil))
}

func RevokeUserPasskeys(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil || userID <= 0 {
		c.JSON(http.StatusBadRequest, result.Error(400, "用户 ID 无效"))
		return
	}
	if userID == models.SuperAdminID {
		c.JSON(http.StatusBadRequest, result.Error(400, "不能撤销超级管理员的 Passkey"))
		return
	}
	if userID == c.GetInt("user_id") {
		c.JSON(http.StatusBadRequest, result.Error(400, "请在账户设置中管理自己的 Passkey"))
		return
	}

	db := database.GetDB().DB
	var userCount int64
	if err := db.Model(&models.User{}).Where("id = ?", userID).Count(&userCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "查询用户失败"))
		return
	}
	if userCount == 0 {
		c.JSON(http.StatusNotFound, result.Error(404, "用户不存在"))
		return
	}
	deleted := db.Where("user_id = ?", userID).Delete(&models.PasskeyCredential{})
	if deleted.Error != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "撤销 Passkey 失败"))
		return
	}
	c.JSON(http.StatusOK, result.Success("Passkey 已撤销", map[string]any{"count": deleted.RowsAffected}))
}

func loadStoredPasskeys(db *gorm.DB, userID int) ([]models.PasskeyCredential, error) {
	var stored []models.PasskeyCredential
	err := db.Where("user_id = ?", userID).Order("id ASC").Find(&stored).Error
	return stored, err
}

func savePasskeyCeremony(c *gin.Context, key string, ceremony passkeyCeremony) error {
	data, err := json.Marshal(ceremony)
	if err != nil {
		return err
	}
	session := sessions.Default(c)
	session.Set(key, string(data))
	return session.Save()
}

func consumePasskeyCeremony(c *gin.Context, key string) (passkeyCeremony, error) {
	session := sessions.Default(c)
	value := session.Get(key)
	session.Delete(key)
	if err := session.Save(); err != nil {
		return passkeyCeremony{}, err
	}
	data, ok := value.(string)
	if !ok || strings.TrimSpace(data) == "" {
		return passkeyCeremony{}, errors.New("passkey ceremony not found")
	}
	var ceremony passkeyCeremony
	if err := json.Unmarshal([]byte(data), &ceremony); err != nil {
		return passkeyCeremony{}, err
	}
	return ceremony, nil
}

func passkeyLoginFailed(c *gin.Context, err error) {
	log.Printf("Passkey 登录失败: %v", err)
	c.JSON(http.StatusUnauthorized, result.Error(401, "Passkey 登录失败"))
}

func passkeyListItem(item models.PasskeyCredential) passkeyResponse {
	return passkeyResponse{
		ID:         item.ID,
		Name:       item.Name,
		CreatedAt:  item.CreatedAt,
		LastUsedAt: item.LastUsedAt,
	}
}
