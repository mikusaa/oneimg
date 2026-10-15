package app

import (
	"errors"
	"log"
	"path/filepath"
	"time"

	"oneimg/backend/config"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/localsecret"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func InitDefaultUser(cfg *config.Config, db *database.Database) error {
	return db.DB.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.User{}).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return nil
		}
		password := cfg.DefaultPass
		if password == "" || password == "123456" {
			var err error
			password, err = localsecret.Generate()
			if err != nil {
				return err
			}
			path := filepath.Join(filepath.Dir(cfg.SqlitePath), ".initial_admin_password")
			if err := localsecret.Write(path, password); err != nil {
				return err
			}
			log.Printf("初始管理员密码已保存到 %s，请首次登录后改密", path)
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		user := models.User{Username: cfg.DefaultUser, Password: string(hash), Role: models.RoleAdmin, AuthVersion: 1,
			PasswordChangeRequired: true, Permission: models.Permission{Codes: []string{}, Buckets: []int{}}}
		return tx.Create(&user).Error
	})
}

func MigrateAuthenticationState(db *gorm.DB, cfg *config.Config) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.PersonalAccessToken{}).Where("auth_version = 0").
			UpdateColumn("auth_version", gorm.Expr("(SELECT auth_version FROM users WHERE users.id = personal_access_tokens.user_id)")).Error; err != nil {
			return err
		}
		var user models.User
		err := tx.First(&user, models.SuperAdminID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("123456")) != nil {
			return nil
		}
		path := filepath.Join(filepath.Dir(cfg.SqlitePath), ".admin_recovery_code")
		if user.PasswordChangeRequired && user.BootstrapCodeHash != nil {
			if code, err := localsecret.Read(path); err == nil && localsecret.Matches(code, *user.BootstrapCodeHash) {
				return nil
			}
		}
		code, err := localsecret.Generate()
		if err != nil {
			return err
		}
		if err := localsecret.Write(path, code); err != nil {
			return err
		}
		if err := tx.Model(&user).Updates(map[string]any{
			"password_change_required": true, "bootstrap_code_hash": localsecret.Hash(code),
			"auth_version": gorm.Expr("auth_version + 1"),
		}).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		if err := tx.Model(&models.PersonalAccessToken{}).Where("user_id = ? AND revoked_at IS NULL", user.ID).Update("revoked_at", now).Error; err != nil {
			return err
		}
		log.Printf("旧默认密码管理员已受限，本地一次性恢复码保存到 %s", path)
		return nil
	})
}
