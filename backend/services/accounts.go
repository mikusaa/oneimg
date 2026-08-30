package services

import (
	"errors"
	"strings"
	"unicode/utf8"

	"oneimg/backend/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrInvalidCredentials      = errors.New("invalid credentials")
	ErrAccountDisabled         = errors.New("account is disabled")
	ErrRegistrationDisabled    = errors.New("registration is disabled")
	ErrAccountFieldsRequired   = errors.New("at least one account field is required")
	ErrUsernameChangeForbidden = errors.New("username change is forbidden")
	ErrCurrentPassword         = errors.New("current password is incorrect")
)

type AccountService struct{ db *gorm.DB }

func NewAccountService(db *gorm.DB) *AccountService { return &AccountService{db: db} }

func (s *AccountService) Get(id int) (models.User, error) {
	var user models.User
	err := s.db.First(&user, id).Error
	return user, err
}

func (s *AccountService) GetActive(id int) (models.User, error) {
	user, err := s.Get(id)
	if err != nil {
		return models.User{}, err
	}
	if !activeRole(user.Role) {
		return models.User{}, ErrAccountDisabled
	}
	return user, nil
}

func (s *AccountService) Authenticate(username, password string) (models.User, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return models.User{}, ErrInvalidCredentials
	}
	var user models.User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		return models.User{}, ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		return models.User{}, ErrInvalidCredentials
	}
	if !activeRole(user.Role) {
		return models.User{}, ErrAccountDisabled
	}
	return user, nil
}

func (s *AccountService) Register(username, password string) (models.User, error) {
	var setting models.Settings
	if err := s.db.First(&setting).Error; err != nil {
		return models.User{}, err
	}
	if !setting.StartRegister {
		return models.User{}, ErrRegistrationDisabled
	}
	username = strings.TrimSpace(username)
	if utf8.RuneCountInString(username) < 3 || utf8.RuneCountInString(username) > 50 {
		return models.User{}, ErrUsernameInvalid
	}
	if len(password) < 6 || len(password) > 100 {
		return models.User{}, ErrPasswordInvalid
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return models.User{}, err
	}
	user := models.User{
		Username:   username,
		Password:   string(hash),
		Role:       models.RoleUser,
		Permission: models.Permission{Codes: []string{}, Buckets: []int{}},
	}
	if err := s.db.Create(&user).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return models.User{}, ErrUsernameConflict
		}
		return models.User{}, err
	}
	return user, nil
}

func (s *AccountService) VerifyPassword(userID int, password string) (models.User, error) {
	user, err := s.GetActive(userID)
	if err != nil {
		return models.User{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		return models.User{}, ErrCurrentPassword
	}
	return user, nil
}

func (s *AccountService) Update(userID int, currentPassword string, username, password *string) (models.User, error) {
	if username == nil && password == nil {
		return models.User{}, ErrAccountFieldsRequired
	}
	user, err := s.VerifyPassword(userID, currentPassword)
	if err != nil {
		return models.User{}, err
	}
	updates := map[string]any{}
	if username != nil {
		if user.Role != models.RoleAdmin {
			return models.User{}, ErrUsernameChangeForbidden
		}
		name := strings.TrimSpace(*username)
		if utf8.RuneCountInString(name) < 3 || utf8.RuneCountInString(name) > 50 {
			return models.User{}, ErrUsernameInvalid
		}
		updates["username"] = name
	}
	if password != nil {
		if len(*password) < 6 || len(*password) > 100 {
			return models.User{}, ErrPasswordInvalid
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
		if err != nil {
			return models.User{}, err
		}
		updates["password"] = string(hash)
	}
	if err := s.db.Model(&user).Updates(updates).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return models.User{}, ErrUsernameConflict
		}
		return models.User{}, err
	}
	if err := s.db.First(&user, userID).Error; err != nil {
		return models.User{}, err
	}
	return user, nil
}

func activeRole(role int) bool {
	return role == models.RoleAdmin || role == models.RoleUser
}
