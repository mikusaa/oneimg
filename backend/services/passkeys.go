package services

import (
	"bytes"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"oneimg/backend/models"
	passkeyutil "oneimg/backend/utils/passkeys"

	"github.com/go-webauthn/webauthn/webauthn"
	"gorm.io/gorm"
)

const MaxPasskeys = 10

var (
	ErrPasskeyLimit        = errors.New("passkey limit reached")
	ErrPasskeyNameInvalid  = errors.New("invalid passkey name")
	ErrPasskeyNameConflict = errors.New("passkey name already exists")
	ErrPasskeyCredential   = errors.New("passkey credential is invalid")
)

type PasskeyService struct {
	db       *gorm.DB
	accounts *AccountService
}

func NewPasskeyService(db *gorm.DB, accounts *AccountService) *PasskeyService {
	return &PasskeyService{db: db, accounts: accounts}
}

func (s *PasskeyService) List(userID int) ([]models.PasskeyCredential, error) {
	result := make([]models.PasskeyCredential, 0)
	err := s.db.Where("user_id = ?", userID).Order("id ASC").Find(&result).Error
	return result, err
}

func (s *PasskeyService) LoginUser(rawID, userHandle []byte) (*passkeyutil.User, error) {
	var stored models.PasskeyCredential
	if err := s.db.Where("credential_id = ?", passkeyutil.CredentialKey(rawID)).First(&stored).Error; err != nil {
		return nil, ErrPasskeyCredential
	}
	if !bytes.Equal(userHandle, passkeyutil.UserHandle(stored.UserID)) {
		return nil, ErrPasskeyCredential
	}
	return s.BuildUser(stored.UserID)
}

func (s *PasskeyService) BuildUser(userID int) (*passkeyutil.User, error) {
	account, err := s.accounts.GetActive(userID)
	if err != nil {
		return nil, err
	}
	stored, err := s.List(account.ID)
	if err != nil {
		return nil, err
	}
	return passkeyutil.BuildUser(account, stored)
}

func (s *PasskeyService) PrepareRegistration(userID int, name, currentPassword string) (*passkeyutil.User, string, error) {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 50 {
		return nil, "", ErrPasskeyNameInvalid
	}
	account, err := s.accounts.VerifyPassword(userID, currentPassword)
	if err != nil {
		return nil, "", err
	}
	stored, err := s.List(account.ID)
	if err != nil {
		return nil, "", err
	}
	if len(stored) >= MaxPasskeys {
		return nil, "", ErrPasskeyLimit
	}
	for _, item := range stored {
		if strings.EqualFold(item.Name, name) {
			return nil, "", ErrPasskeyNameConflict
		}
	}
	user, err := passkeyutil.BuildUser(account, stored)
	return user, name, err
}

func (s *PasskeyService) UpdateLoginCredential(userID int, credential *webauthn.Credential) error {
	credentialID, encrypted, err := passkeyutil.EncodeCredential(credential)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	updated := s.db.Model(&models.PasskeyCredential{}).
		Where("user_id = ? AND credential_id = ?", userID, credentialID).
		Updates(map[string]any{"credential_data": encrypted, "last_used_at": &now})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return ErrPasskeyCredential
	}
	return nil
}

func (s *PasskeyService) Create(userID int, name string, credential *webauthn.Credential) (models.PasskeyCredential, error) {
	credentialID, encrypted, err := passkeyutil.EncodeCredential(credential)
	if err != nil {
		return models.PasskeyCredential{}, err
	}
	item := models.PasskeyCredential{UserID: userID, Name: name, CredentialID: credentialID, CredentialData: encrypted}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.PasskeyCredential{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
			return err
		}
		if count >= MaxPasskeys {
			return ErrPasskeyLimit
		}
		return tx.Create(&item).Error
	})
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "unique") {
		return models.PasskeyCredential{}, ErrPasskeyNameConflict
	}
	return item, err
}

func (s *PasskeyService) Rename(userID, id int, name string) (models.PasskeyCredential, error) {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 50 {
		return models.PasskeyCredential{}, ErrPasskeyNameInvalid
	}
	var duplicate int64
	if err := s.db.Model(&models.PasskeyCredential{}).
		Where("user_id = ? AND id <> ? AND LOWER(name) = LOWER(?)", userID, id, name).
		Count(&duplicate).Error; err != nil {
		return models.PasskeyCredential{}, err
	}
	if duplicate > 0 {
		return models.PasskeyCredential{}, ErrPasskeyNameConflict
	}
	updated := s.db.Model(&models.PasskeyCredential{}).Where("id = ? AND user_id = ?", id, userID).Update("name", name)
	if updated.Error != nil {
		return models.PasskeyCredential{}, updated.Error
	}
	if updated.RowsAffected == 0 {
		return models.PasskeyCredential{}, gorm.ErrRecordNotFound
	}
	var item models.PasskeyCredential
	err := s.db.Where("id = ? AND user_id = ?", id, userID).First(&item).Error
	return item, err
}

func (s *PasskeyService) Revoke(userID, id int, currentPassword string) error {
	if _, err := s.accounts.VerifyPassword(userID, currentPassword); err != nil {
		return err
	}
	deleted := s.db.Where("id = ? AND user_id = ?", id, userID).Delete(&models.PasskeyCredential{})
	if deleted.Error != nil {
		return deleted.Error
	}
	if deleted.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
