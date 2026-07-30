package passkeys

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"oneimg/backend/config"
	"oneimg/backend/models"
	"oneimg/backend/utils/secureconfig"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

const CeremonyTimeout = 5 * time.Minute

var (
	clientMu  sync.RWMutex
	client    *webauthn.WebAuthn
	clientErr error
)

type User struct {
	Account     models.User
	Credentials []webauthn.Credential
}

func (u *User) WebAuthnID() []byte {
	return UserHandle(u.Account.ID)
}

func (u *User) WebAuthnName() string {
	return u.Account.Username
}

func (u *User) WebAuthnDisplayName() string {
	return u.Account.Username
}

func (u *User) WebAuthnCredentials() []webauthn.Credential {
	return u.Credentials
}

func Init(cfg *config.Config) error {
	webAuthnConfig, err := BuildConfig(cfg)
	var initializedClient *webauthn.WebAuthn
	if err == nil {
		initializedClient, err = webauthn.New(webAuthnConfig)
	}

	clientMu.Lock()
	defer clientMu.Unlock()
	client = initializedClient
	clientErr = err
	return err
}

func Client() (*webauthn.WebAuthn, error) {
	clientMu.RLock()
	defer clientMu.RUnlock()
	if client == nil {
		if clientErr != nil {
			return nil, clientErr
		}
		return nil, errors.New("Passkey 服务未初始化")
	}
	return client, nil
}

func Available() bool {
	clientMu.RLock()
	defer clientMu.RUnlock()
	return client != nil && clientErr == nil
}

func RPID() string {
	clientMu.RLock()
	defer clientMu.RUnlock()
	if client == nil {
		return ""
	}
	return client.Config.RPID
}

func BuildConfig(cfg *config.Config) (*webauthn.Config, error) {
	if cfg == nil {
		return nil, errors.New("应用配置为空")
	}
	if strings.TrimSpace(cfg.ConfigSecret) == "" {
		return nil, errors.New("CONFIG_SECRET 不能为空")
	}

	appOrigin, appHost, err := normalizeOrigin(cfg.AppURL)
	if err != nil {
		return nil, fmt.Errorf("APP_URL 无法用于 Passkey: %w", err)
	}

	rpID := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(cfg.PasskeyRPID), "."))
	if rpID == "" {
		rpID = appHost
	}

	originValues := cfg.PasskeyOrigins
	if len(originValues) == 0 {
		originValues = []string{appOrigin}
	}
	origins := make([]string, 0, len(originValues))
	seenOrigins := make(map[string]struct{}, len(originValues))
	for _, value := range originValues {
		origin, host, err := normalizeOrigin(value)
		if err != nil {
			return nil, fmt.Errorf("PASSKEY_ORIGINS 包含无效来源 %q: %w", value, err)
		}
		if !hostMatchesRPID(host, rpID) {
			return nil, fmt.Errorf("来源 %q 不属于 RP ID %q", origin, rpID)
		}
		if _, exists := seenOrigins[origin]; exists {
			continue
		}
		seenOrigins[origin] = struct{}{}
		origins = append(origins, origin)
	}

	rpName := strings.TrimSpace(cfg.PasskeyRPName)
	if rpName == "" {
		rpName = "OneImg"
	}

	return &webauthn.Config{
		RPID:                  rpID,
		RPDisplayName:         rpName,
		RPOrigins:             origins,
		AttestationPreference: protocol.PreferNoAttestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			RequireResidentKey: protocol.ResidentKeyRequired(),
			ResidentKey:        protocol.ResidentKeyRequirementRequired,
			UserVerification:   protocol.VerificationRequired,
		},
		Timeouts: webauthn.TimeoutsConfig{
			Login: webauthn.TimeoutConfig{
				Enforce: true,
				Timeout: CeremonyTimeout,
			},
			Registration: webauthn.TimeoutConfig{
				Enforce: true,
				Timeout: CeremonyTimeout,
			},
		},
	}, nil
}

func UserHandle(userID int) []byte {
	handle := make([]byte, 8)
	binary.BigEndian.PutUint64(handle, uint64(userID))
	return handle
}

func CredentialKey(credentialID []byte) string {
	return base64.RawURLEncoding.EncodeToString(credentialID)
}

func EncodeCredential(credential *webauthn.Credential) (credentialID, encryptedData string, err error) {
	if credential == nil || len(credential.ID) == 0 {
		return "", "", errors.New("Passkey 凭据为空")
	}
	data, err := json.Marshal(credential)
	if err != nil {
		return "", "", fmt.Errorf("序列化 Passkey 凭据失败: %w", err)
	}
	encryptedData, err = secureconfig.EncryptBytes(data)
	if err != nil {
		return "", "", fmt.Errorf("加密 Passkey 凭据失败: %w", err)
	}
	return CredentialKey(credential.ID), encryptedData, nil
}

func DecodeCredential(stored models.PasskeyCredential) (webauthn.Credential, error) {
	data, err := secureconfig.DecryptBytes(stored.CredentialData)
	if err != nil {
		return webauthn.Credential{}, fmt.Errorf("解密 Passkey 凭据 %d 失败: %w", stored.ID, err)
	}
	var credential webauthn.Credential
	if err := json.Unmarshal(data, &credential); err != nil {
		return webauthn.Credential{}, fmt.Errorf("解析 Passkey 凭据 %d 失败: %w", stored.ID, err)
	}
	if CredentialKey(credential.ID) != stored.CredentialID {
		return webauthn.Credential{}, fmt.Errorf("Passkey 凭据 %d 的索引不一致", stored.ID)
	}
	return credential, nil
}

func BuildUser(account models.User, stored []models.PasskeyCredential) (*User, error) {
	credentials := make([]webauthn.Credential, 0, len(stored))
	for _, item := range stored {
		credential, err := DecodeCredential(item)
		if err != nil {
			return nil, err
		}
		credentials = append(credentials, credential)
	}
	return &User{Account: account, Credentials: credentials}, nil
}

func normalizeOrigin(rawValue string) (origin, host string, err error) {
	parsed, err := url.Parse(strings.TrimSpace(rawValue))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", "", errors.New("必须是包含协议和主机名的完整地址")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", "", errors.New("地址不能包含认证信息、路径、查询参数或片段")
	}
	scheme := strings.ToLower(parsed.Scheme)
	host = strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if scheme != "https" && !(scheme == "http" && isLoopbackHost(host)) {
		return "", "", errors.New("必须使用 HTTPS，localhost 本地开发除外")
	}
	return scheme + "://" + strings.ToLower(parsed.Host), host, nil
}

func hostMatchesRPID(host, rpID string) bool {
	if isLoopbackHost(rpID) {
		return host == rpID
	}
	return host == rpID || strings.HasSuffix(host, "."+rpID)
}

func isLoopbackHost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
