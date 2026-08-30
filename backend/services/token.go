package services

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"oneimg/backend/models"

	"gorm.io/gorm"
)

const (
	personalTokenPrefix       = "oneimg_pat_"
	personalTokenPrefixLength = 12
	personalTokenSecretLength = 43
)

var PersonalTokenScopes = []string{
	"images:read", "images:write", "images:delete",
	"tags:read", "tags:write",
	"storage:read", "storage:write",
	"users:read", "users:write",
	"settings:read", "settings:write",
	"stats:read",
}

type TokenService struct {
	db     *gorm.DB
	secret []byte
	now    func() time.Time
}

type CreateTokenInput struct {
	Name           string
	Scopes         []string
	ExpirationDays *int
}

type CreatedToken struct {
	Token models.PersonalAccessToken
	Plain string
}

func NewTokenService(db *gorm.DB, configSecret string) *TokenService {
	return &TokenService{db: db, secret: []byte(configSecret), now: time.Now}
}

func (s *TokenService) List(userID int) ([]models.PersonalAccessToken, error) {
	var tokens []models.PersonalAccessToken
	err := s.db.Where("user_id = ?", userID).Order("id DESC").Find(&tokens).Error
	return tokens, err
}

func (s *TokenService) Create(userID int, input CreateTokenInput) (CreatedToken, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || len([]rune(input.Name)) > 50 {
		return CreatedToken{}, ErrInvalidTokenName
	}
	scopes, err := normalizeScopes(input.Scopes)
	if err != nil {
		return CreatedToken{}, err
	}

	expirationDays := 90
	if input.ExpirationDays != nil {
		expirationDays = *input.ExpirationDays
	}
	if expirationDays != 0 && expirationDays != 30 && expirationDays != 90 && expirationDays != 365 {
		return CreatedToken{}, ErrInvalidTokenExpiration
	}

	prefix, err := randomURLString(9)
	if err != nil {
		return CreatedToken{}, err
	}
	secret, err := randomURLString(32)
	if err != nil {
		return CreatedToken{}, err
	}
	plain := personalTokenPrefix + prefix + "_" + secret
	var expiresAt *time.Time
	if expirationDays > 0 {
		value := s.now().UTC().AddDate(0, 0, expirationDays)
		expiresAt = &value
	}
	record := models.PersonalAccessToken{
		UserID:     userID,
		Name:       input.Name,
		Prefix:     prefix,
		SecretHash: s.hash(plain),
		Scopes:     models.StringList(scopes),
		ExpiresAt:  expiresAt,
	}
	if err := s.db.Create(&record).Error; err != nil {
		return CreatedToken{}, err
	}
	return CreatedToken{Token: record, Plain: plain}, nil
}

func (s *TokenService) Revoke(userID int, tokenID uint) error {
	now := s.now().UTC()
	result := s.db.Model(&models.PersonalAccessToken{}).
		Where("id = ? AND user_id = ? AND revoked_at IS NULL", tokenID, userID).
		Update("revoked_at", &now)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *TokenService) Authenticate(plain string) (*models.PersonalAccessToken, *models.User, error) {
	prefix, ok := parseTokenPrefix(plain)
	if !ok {
		return nil, nil, ErrInvalidToken
	}
	var token models.PersonalAccessToken
	if err := s.db.Where("prefix = ? AND revoked_at IS NULL", prefix).First(&token).Error; err != nil {
		return nil, nil, ErrInvalidToken
	}
	if token.ExpiresAt != nil && !token.ExpiresAt.After(s.now().UTC()) {
		return nil, nil, ErrExpiredToken
	}
	expected, err := hex.DecodeString(token.SecretHash)
	if err != nil {
		return nil, nil, ErrInvalidToken
	}
	actual := s.hashBytes(plain)
	if len(expected) != len(actual) || subtle.ConstantTimeCompare(expected, actual) != 1 {
		return nil, nil, ErrInvalidToken
	}
	var user models.User
	if err := s.db.First(&user, token.UserID).Error; err != nil {
		return nil, nil, ErrInvalidToken
	}
	if user.Role != models.RoleAdmin && user.Role != models.RoleUser {
		return nil, nil, ErrInvalidToken
	}
	now := s.now().UTC()
	_ = s.db.Model(&token).Update("last_used_at", &now).Error
	token.LastUsedAt = &now
	return &token, &user, nil
}

func (s *TokenService) hash(plain string) string {
	return hex.EncodeToString(s.hashBytes(plain))
}

func (s *TokenService) hashBytes(plain string) []byte {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(plain))
	return mac.Sum(nil)
}

func normalizeScopes(scopes []string) ([]string, error) {
	if len(scopes) == 0 {
		return nil, ErrTokenScopesRequired
	}
	seen := make(map[string]struct{}, len(scopes))
	result := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if !slices.Contains(PersonalTokenScopes, scope) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidTokenScope, scope)
		}
		if _, exists := seen[scope]; exists {
			continue
		}
		seen[scope] = struct{}{}
		result = append(result, scope)
	}
	return result, nil
}

func parseTokenPrefix(plain string) (string, bool) {
	if !strings.HasPrefix(plain, personalTokenPrefix) {
		return "", false
	}
	rest := strings.TrimPrefix(plain, personalTokenPrefix)
	if len(rest) != personalTokenPrefixLength+1+personalTokenSecretLength || rest[personalTokenPrefixLength] != '_' {
		return "", false
	}
	return rest[:personalTokenPrefixLength], true
}

func randomURLString(size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

var (
	ErrInvalidToken           = errors.New("invalid personal access token")
	ErrExpiredToken           = errors.New("personal access token expired")
	ErrInvalidTokenName       = errors.New("invalid token name")
	ErrInvalidTokenScope      = errors.New("invalid token scope")
	ErrTokenScopesRequired    = errors.New("at least one token scope is required")
	ErrInvalidTokenExpiration = errors.New("invalid token expiration")
)
