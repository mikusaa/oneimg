package services

import (
	"crypto/rand"
	"errors"
	"strings"
	"unicode/utf8"

	"oneimg/backend/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrUsernameInvalid  = errors.New("username must contain between 3 and 50 characters")
	ErrPasswordInvalid  = errors.New("password must contain between 6 and 100 characters")
	ErrUserRoleInvalid  = errors.New("invalid user role")
	ErrProtectedUser    = errors.New("protected user cannot be modified")
	ErrUsernameConflict = errors.New("username already exists")
	ErrBucketNotFound   = errors.New("storage bucket does not exist")
)

type UserService struct{ db *gorm.DB }

type UserListQuery struct {
	Page, PageSize int
	Username       string
	Role           *int
	ID             *int
	Sort, Order    string
}

type UserRecord struct {
	User         models.User
	PasskeyCount int64
}

func NewUserService(db *gorm.DB) *UserService { return &UserService{db: db} }

func (s *UserService) List(input UserListQuery) ([]UserRecord, int64, error) {
	query := s.db.Model(&models.User{})
	if input.ID != nil {
		query = query.Where("id = ?", *input.ID)
	}
	if strings.TrimSpace(input.Username) != "" {
		query = query.Where("username LIKE ?", "%"+strings.TrimSpace(input.Username)+"%")
	}
	if input.Role != nil {
		query = query.Where("role = ?", *input.Role)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var users []models.User
	orderFields := map[string]string{"id": "id", "username": "username", "created_at": "created_at"}
	if err := query.Order(orderFields[input.Sort] + " " + input.Order).Offset((input.Page - 1) * input.PageSize).Limit(input.PageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	ids := make([]int, 0, len(users))
	for _, user := range users {
		ids = append(ids, user.ID)
	}
	type row struct {
		UserID int
		Count  int64
	}
	var rows []row
	if len(ids) > 0 {
		if err := s.db.Model(&models.PasskeyCredential{}).Select("user_id, COUNT(*) count").Where("user_id IN ?", ids).Group("user_id").Scan(&rows).Error; err != nil {
			return nil, 0, err
		}
	}
	counts := make(map[int]int64, len(rows))
	for _, item := range rows {
		counts[item.UserID] = item.Count
	}
	result := make([]UserRecord, 0, len(users))
	for _, user := range users {
		result = append(result, UserRecord{User: user, PasskeyCount: counts[user.ID]})
	}
	return result, total, nil
}

func validateAccount(username, password string, role int) (string, error) {
	username = strings.TrimSpace(username)
	if utf8.RuneCountInString(username) < 3 || utf8.RuneCountInString(username) > 50 {
		return "", ErrUsernameInvalid
	}
	if len(password) < 6 || len(password) > 100 {
		return "", ErrPasswordInvalid
	}
	if role != models.RoleAdmin && role != models.RoleUser {
		return "", ErrUserRoleInvalid
	}
	return username, nil
}

func (s *UserService) Create(username, password string, role int) (models.User, error) {
	username, err := validateAccount(username, password, role)
	if err != nil {
		return models.User{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return models.User{}, err
	}
	user := models.User{Username: username, Password: string(hash), Role: role, Permission: models.Permission{Codes: []string{}, Buckets: []int{}}}
	if err := s.db.Create(&user).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return models.User{}, ErrUsernameConflict
		}
		return models.User{}, err
	}
	return user, nil
}

func (s *UserService) Update(actorID, id int, username *string, role *int) (models.User, error) {
	if id == models.SuperAdminID || id == actorID {
		return models.User{}, ErrProtectedUser
	}
	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		return models.User{}, err
	}
	updates := map[string]any{}
	if username != nil {
		name := strings.TrimSpace(*username)
		if utf8.RuneCountInString(name) < 3 || utf8.RuneCountInString(name) > 50 {
			return models.User{}, ErrUsernameInvalid
		}
		updates["username"] = name
	}
	if role != nil {
		if *role != models.RoleAdmin && *role != models.RoleUser {
			return models.User{}, ErrUserRoleInvalid
		}
		updates["role"] = *role
		if *role == models.RoleUser {
			user.Permission.Codes = []string{}
			updates["permission"] = user.Permission
		}
	}
	if len(updates) == 0 {
		return models.User{}, errors.New("no user fields supplied")
	}
	if err := s.db.Model(&user).Updates(updates).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return models.User{}, ErrUsernameConflict
		}
		return models.User{}, err
	}
	if err := s.db.First(&user, id).Error; err != nil {
		return models.User{}, err
	}
	return user, nil
}

func (s *UserService) Delete(actorID, id int) error {
	if id == models.SuperAdminID || id == actorID {
		return ErrProtectedUser
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.First(&user, id).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", id).Delete(&models.PasskeyCredential{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", id).Delete(&models.PersonalAccessToken{}).Error; err != nil {
			return err
		}
		return tx.Delete(&user).Error
	})
}

func (s *UserService) SetPermissions(actorID, id int, permission models.Permission) (models.User, error) {
	if id == models.SuperAdminID || id == actorID {
		return models.User{}, ErrProtectedUser
	}
	if err := models.ValidatePermissionCodes(permission.Codes); err != nil {
		return models.User{}, err
	}
	permission.Codes = models.FilterPermissionCodes(permission.Codes)
	permission.Buckets = uniqueInts(permission.Buckets)
	if len(permission.Buckets) > 0 {
		var count int64
		if err := s.db.Model(&models.Buckets{}).Where("id IN ?", permission.Buckets).Count(&count).Error; err != nil {
			return models.User{}, err
		}
		if count != int64(len(permission.Buckets)) {
			return models.User{}, ErrBucketNotFound
		}
	}
	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		return models.User{}, err
	}
	if user.Role != models.RoleAdmin {
		permission.Codes = []string{}
	}
	if err := s.db.Model(&user).Update("permission", permission).Error; err != nil {
		return models.User{}, err
	}
	user.Permission = permission
	return user, nil
}

func (s *UserService) ResetPassword(actorID, id int) (string, error) {
	if id == models.SuperAdminID || id == actorID {
		return "", ErrProtectedUser
	}
	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		return "", err
	}
	plain, err := randomAlphaNumeric(16)
	if err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	if err := s.db.Model(&user).Update("password", string(hash)).Error; err != nil {
		return "", err
	}
	return plain, nil
}

func (s *UserService) RevokePasskeys(actorID, id int) error {
	if id == models.SuperAdminID || id == actorID {
		return ErrProtectedUser
	}
	var count int64
	if err := s.db.Model(&models.User{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return gorm.ErrRecordNotFound
	}
	return s.db.Where("user_id = ?", id).Delete(&models.PasskeyCredential{}).Error
}

func randomAlphaNumeric(length int) (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	for i := range bytes {
		bytes[i] = alphabet[int(bytes[i])%len(alphabet)]
	}
	return string(bytes), nil
}
