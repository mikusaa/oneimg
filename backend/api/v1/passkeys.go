package v1

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"oneimg/backend/models"
	"oneimg/backend/services"
	passkeyutil "oneimg/backend/utils/passkeys"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/webauthn"
	"gorm.io/gorm"
)

const (
	passkeyLoginKey        = "v1_passkey_login"
	passkeyRegistrationKey = "v1_passkey_registration"
)

type passkeyCeremony struct {
	Session webauthn.SessionData `json:"session"`
	UserID  int                  `json:"user_id,omitempty"`
	Name    string               `json:"name,omitempty"`
}

type passkeyDTO struct {
	ID         uint       `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

func toPasskeyDTO(item models.PasskeyCredential) passkeyDTO {
	return passkeyDTO{ID: item.ID, Name: item.Name, CreatedAt: item.CreatedAt.UTC(), LastUsedAt: utcTimePointer(item.LastUsedAt)}
}

func (s *Server) passkeyLoginOptions(c *gin.Context) {
	client, err := passkeyutil.Client()
	if err != nil {
		writeProblem(c, http.StatusServiceUnavailable, "passkey_unavailable", "Passkey 登录暂不可用")
		return
	}
	options, data, err := client.BeginDiscoverableLogin()
	if err != nil {
		writeProblem(c, http.StatusInternalServerError, "passkey_login_options_failed", "无法生成 Passkey 登录挑战")
		return
	}
	if err := savePasskeyCeremony(c, passkeyLoginKey, passkeyCeremony{Session: *data}); err != nil {
		writeProblem(c, http.StatusInternalServerError, "passkey_session_failed", "无法保存 Passkey 登录挑战")
		return
	}
	writeData(c, http.StatusOK, gin.H{"options": options.Response}, nil)
}

func (s *Server) passkeyLoginVerify(c *gin.Context) {
	client, err := passkeyutil.Client()
	if err != nil {
		writeProblem(c, http.StatusServiceUnavailable, "passkey_unavailable", "Passkey 登录暂不可用")
		return
	}
	if !prepareJSONBody(c) {
		return
	}
	ceremony, err := consumePasskeyCeremony(c, passkeyLoginKey)
	if err != nil {
		writeProblem(c, http.StatusUnauthorized, "passkey_login_failed", "Passkey 登录请求已过期")
		return
	}
	user, credential, err := client.FinishPasskeyLogin(func(rawID, userHandle []byte) (webauthn.User, error) {
		return s.services.Passkeys.LoginUser(rawID, userHandle)
	}, ceremony.Session, c.Request)
	if err != nil || credential == nil {
		log.Printf("Passkey 登录失败: %v", err)
		writeProblem(c, http.StatusUnauthorized, "passkey_login_failed", "Passkey 登录失败")
		return
	}
	passkeyUser, ok := user.(*passkeyutil.User)
	if !ok || credential.Authenticator.CloneWarning {
		writeProblem(c, http.StatusUnauthorized, "passkey_login_failed", "Passkey 登录失败")
		return
	}
	if err := s.services.Passkeys.UpdateLoginCredential(passkeyUser.Account.ID, credential); err != nil {
		writeProblem(c, http.StatusUnauthorized, "passkey_login_failed", "Passkey 登录失败")
		return
	}
	if err := s.saveSession(c, &passkeyUser.Account); err != nil {
		writeProblem(c, http.StatusInternalServerError, "session_save_failed", "无法保存登录会话")
		return
	}
	if err := s.issueCSRFCookie(c); err != nil {
		writeProblem(c, http.StatusInternalServerError, "csrf_token_failed", "无法初始化安全会话")
		return
	}
	writeData(c, http.StatusOK, toUserDTO(passkeyUser.Account), nil)
}

func (s *Server) listPasskeys(c *gin.Context) {
	user, _ := currentUser(c)
	items, err := s.services.Passkeys.List(user.ID)
	if err != nil {
		writeProblem(c, http.StatusInternalServerError, "passkey_list_failed", "读取 Passkey 失败")
		return
	}
	result := make([]passkeyDTO, 0, len(items))
	for _, item := range items {
		result = append(result, toPasskeyDTO(item))
	}
	writeData(c, http.StatusOK, result, nil)
}

func (s *Server) passkeyRegistrationOptions(c *gin.Context) {
	client, err := passkeyutil.Client()
	if err != nil {
		writeProblem(c, http.StatusServiceUnavailable, "passkey_unavailable", "Passkey 暂不可用")
		return
	}
	var input struct {
		Name            string `json:"name"`
		CurrentPassword string `json:"current_password"`
	}
	if !bindJSON(c, &input) {
		return
	}
	user, _ := currentUser(c)
	passkeyUser, name, err := s.services.Passkeys.PrepareRegistration(user.ID, input.Name, input.CurrentPassword)
	if handlePasskeyServiceError(c, err) {
		return
	}
	options, data, err := client.BeginRegistration(passkeyUser, webauthn.WithExclusions(webauthn.Credentials(passkeyUser.Credentials).CredentialDescriptors()))
	if err != nil {
		writeProblem(c, http.StatusInternalServerError, "passkey_options_failed", "无法生成 Passkey 注册挑战")
		return
	}
	if err := savePasskeyCeremony(c, passkeyRegistrationKey, passkeyCeremony{Session: *data, UserID: user.ID, Name: name}); err != nil {
		writeProblem(c, http.StatusInternalServerError, "passkey_session_failed", "无法保存 Passkey 注册挑战")
		return
	}
	writeData(c, http.StatusOK, gin.H{"options": options.Response}, nil)
}

func (s *Server) passkeyRegistrationVerify(c *gin.Context) {
	client, err := passkeyutil.Client()
	if err != nil {
		writeProblem(c, http.StatusServiceUnavailable, "passkey_unavailable", "Passkey 暂不可用")
		return
	}
	if !prepareJSONBody(c) {
		return
	}
	ceremony, err := consumePasskeyCeremony(c, passkeyRegistrationKey)
	user, _ := currentUser(c)
	if err != nil || ceremony.UserID != user.ID {
		writeProblem(c, http.StatusBadRequest, "passkey_registration_expired", "Passkey 注册请求已过期")
		return
	}
	passkeyUser, err := s.services.Passkeys.BuildUser(user.ID)
	if err != nil {
		writeProblem(c, http.StatusInternalServerError, "passkey_registration_failed", "读取 Passkey 失败")
		return
	}
	credential, err := client.FinishRegistration(passkeyUser, ceremony.Session, c.Request)
	if err != nil {
		writeProblem(c, http.StatusUnprocessableEntity, "passkey_registration_failed", "Passkey 注册验证失败")
		return
	}
	item, err := s.services.Passkeys.Create(user.ID, ceremony.Name, credential)
	if err != nil {
		if errors.Is(err, services.ErrPasskeyLimit) || errors.Is(err, services.ErrPasskeyNameConflict) {
			writeProblem(c, http.StatusConflict, "passkey_conflict", "Passkey 已存在或已达到数量限制")
			return
		}
		writeProblem(c, http.StatusInternalServerError, "passkey_save_failed", "保存 Passkey 失败")
		return
	}
	c.Header("Location", "/api/v1/me/passkeys/"+itoa(int(item.ID)))
	writeData(c, http.StatusCreated, toPasskeyDTO(item), nil)
}

func (s *Server) renamePasskey(c *gin.Context) {
	id, ok := parsePositiveID(c, "id")
	if !ok {
		return
	}
	var input struct {
		Name string `json:"name"`
	}
	if !bindJSON(c, &input) {
		return
	}
	user, _ := currentUser(c)
	item, err := s.services.Passkeys.Rename(user.ID, id, input.Name)
	if errors.Is(err, services.ErrPasskeyNameInvalid) {
		writeProblem(c, http.StatusUnprocessableEntity, "validation_error", "设备名称长度必须为 1-50 个字符")
		return
	}
	if errors.Is(err, services.ErrPasskeyNameConflict) {
		writeProblem(c, http.StatusConflict, "passkey_name_conflict", "设备名称已存在")
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeProblem(c, http.StatusNotFound, "passkey_not_found", "Passkey 不存在")
		return
	}
	if err != nil {
		writeProblem(c, http.StatusInternalServerError, "passkey_update_failed", "更新 Passkey 失败")
		return
	}
	writeData(c, http.StatusOK, toPasskeyDTO(item), nil)
}

func (s *Server) revokePasskey(c *gin.Context) {
	id, ok := parsePositiveID(c, "id")
	if !ok {
		return
	}
	var input struct {
		CurrentPassword string `json:"current_password"`
	}
	if !bindJSON(c, &input) {
		return
	}
	user, _ := currentUser(c)
	err := s.services.Passkeys.Revoke(user.ID, id, input.CurrentPassword)
	if errors.Is(err, services.ErrCurrentPassword) {
		writeProblem(c, http.StatusUnauthorized, "current_password_invalid", "当前密码错误")
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeProblem(c, http.StatusNotFound, "passkey_not_found", "Passkey 不存在")
		return
	}
	if err != nil {
		writeProblem(c, http.StatusInternalServerError, "passkey_revoke_failed", "撤销 Passkey 失败")
		return
	}
	writeNoContent(c)
}

func handlePasskeyServiceError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, services.ErrPasskeyNameInvalid):
		writeProblem(c, http.StatusUnprocessableEntity, "validation_error", "设备名称长度必须为 1-50 个字符")
	case errors.Is(err, services.ErrCurrentPassword):
		writeProblem(c, http.StatusUnauthorized, "current_password_invalid", "当前密码错误")
	case errors.Is(err, services.ErrPasskeyLimit):
		writeProblem(c, http.StatusConflict, "passkey_limit_reached", "每个账户最多绑定 10 个 Passkey")
	case errors.Is(err, services.ErrPasskeyNameConflict):
		writeProblem(c, http.StatusConflict, "passkey_name_conflict", "设备名称已存在")
	case errors.Is(err, gorm.ErrRecordNotFound), errors.Is(err, services.ErrAccountDisabled):
		writeProblem(c, http.StatusUnauthorized, "invalid_session", "用户不存在或已停用")
	default:
		writeProblem(c, http.StatusInternalServerError, "passkey_options_failed", "读取 Passkey 失败")
	}
	return true
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
	raw, ok := value.(string)
	if !ok || raw == "" {
		return passkeyCeremony{}, errors.New("ceremony not found")
	}
	var result passkeyCeremony
	err := json.Unmarshal([]byte(raw), &result)
	return result, err
}
