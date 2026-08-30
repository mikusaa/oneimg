package v1

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"oneimg/backend/models"
	"oneimg/backend/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type tokenDTO struct {
	ID         uint              `json:"id"`
	Name       string            `json:"name"`
	Prefix     string            `json:"prefix"`
	Scopes     models.StringList `json:"scopes"`
	ExpiresAt  *time.Time        `json:"expires_at"`
	LastUsedAt *time.Time        `json:"last_used_at"`
	RevokedAt  *time.Time        `json:"revoked_at"`
	CreatedAt  time.Time         `json:"created_at"`
}

func toTokenDTO(token models.PersonalAccessToken) tokenDTO {
	return tokenDTO{ID: token.ID, Name: token.Name, Prefix: token.Prefix, Scopes: token.Scopes,
		ExpiresAt: utcTimePointer(token.ExpiresAt), LastUsedAt: utcTimePointer(token.LastUsedAt),
		RevokedAt: utcTimePointer(token.RevokedAt), CreatedAt: token.CreatedAt.UTC()}
}

func utcTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	result := value.UTC()
	return &result
}

func (s *Server) listTokens(c *gin.Context) {
	user, _ := currentUser(c)
	tokens, err := s.services.Tokens.List(user.ID)
	if err != nil {
		writeProblem(c, http.StatusInternalServerError, "token_list_failed", "无法读取个人 Token")
		return
	}
	items := make([]tokenDTO, 0, len(tokens))
	for _, token := range tokens {
		items = append(items, toTokenDTO(token))
	}
	writeData(c, http.StatusOK, items, nil)
}

func (s *Server) createToken(c *gin.Context) {
	var input struct {
		Name            string   `json:"name"`
		Scopes          []string `json:"scopes"`
		ExpirationDays  *int     `json:"expiration_days"`
		CurrentPassword string   `json:"current_password"`
	}
	if !bindJSON(c, &input) {
		return
	}
	user, _ := currentUser(c)
	created, err := s.services.Tokens.Create(user.ID, services.CreateTokenInput{
		Name: input.Name, Scopes: input.Scopes, ExpirationDays: input.ExpirationDays,
		CurrentPassword: input.CurrentPassword,
	})
	if err != nil {
		tokenServiceError(c, err)
		return
	}
	c.Header("Location", "/api/v1/me/tokens/"+strconv.FormatUint(uint64(created.Token.ID), 10))
	writeData(c, http.StatusCreated, gin.H{"token": created.Plain, "record": toTokenDTO(created.Token)}, nil)
}

func (s *Server) revokeToken(c *gin.Context) {
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
	if err := s.services.Tokens.Revoke(user.ID, uint(id), input.CurrentPassword); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeProblem(c, http.StatusNotFound, "token_not_found", "Token 不存在或已撤销")
			return
		}
		tokenServiceError(c, err)
		return
	}
	writeNoContent(c)
}
