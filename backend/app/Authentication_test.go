package app

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"oneimg/backend/config"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/localsecret"
)

func authenticationDB(t *testing.T) (*config.Config, *database.Database) {
	t.Helper()
	cfg := &config.Config{SqlitePath: filepath.Join(t.TempDir(), "data.db"), DefaultUser: "admin"}
	db, err := database.NewDB(sqlite.Open(cfg.SqlitePath))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.DB.AutoMigrate(&models.User{}, &models.PersonalAccessToken{}); err != nil {
		t.Fatal(err)
	}
	return cfg, db
}

func TestUniqueBootstrapAndFailClosed(t *testing.T) {
	secrets := map[string]bool{}
	for _, configured := range []string{"", "123456", "explicit-unique-password"} {
		cfg, db := authenticationDB(t)
		cfg.DefaultPass = configured
		if err := InitDefaultUser(cfg, db); err != nil {
			t.Fatal(err)
		}
		var user models.User
		if err := db.DB.First(&user).Error; err != nil {
			t.Fatal(err)
		}
		password := configured
		if configured == "" || configured == "123456" {
			var err error
			password, err = localsecret.Read(filepath.Join(filepath.Dir(cfg.SqlitePath), ".initial_admin_password"))
			if err != nil {
				t.Fatal(err)
			}
			if secrets[password] {
				t.Fatal("shared bootstrap password")
			}
			secrets[password] = true
		}
		if !user.PasswordChangeRequired || user.AuthVersion != 1 || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
			t.Fatal("invalid bootstrap")
		}
		if err := InitDefaultUser(cfg, db); err != nil {
			t.Fatal(err)
		}
		var current models.User
		db.DB.First(&current)
		if current.Password != user.Password {
			t.Fatal("existing password reset")
		}
	}
	cfg, db := authenticationDB(t)
	path := filepath.Join(filepath.Dir(cfg.SqlitePath), ".initial_admin_password")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := InitDefaultUser(cfg, db); err == nil {
		t.Fatal("unsafe fallback after file failure")
	}
	var count int64
	db.DB.Model(&models.User{}).Count(&count)
	if count != 0 {
		t.Fatal("partial bootstrap")
	}
}

func TestLegacyRecoveryMigrationIdempotent(t *testing.T) {
	cfg, db := authenticationDB(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.MinCost)
	user := models.User{ID: 1, Username: "renamed-admin", Password: string(hash)}
	if err := db.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	token := models.PersonalAccessToken{UserID: 1, Prefix: "legacy", Scopes: models.StringList{"images:read"}}
	if err := db.DB.Create(&token).Error; err != nil {
		t.Fatal(err)
	}
	if err := MigrateAuthenticationState(db.DB, cfg); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(filepath.Dir(cfg.SqlitePath), ".admin_recovery_code")
	code, err := localsecret.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	db.DB.First(&user, 1)
	db.DB.First(&token, token.ID)
	if !user.PasswordChangeRequired || user.BootstrapCodeHash == nil || !localsecret.Matches(code, *user.BootstrapCodeHash) || user.AuthVersion != 2 || token.RevokedAt == nil {
		t.Fatal("migration failed")
	}
	if err := MigrateAuthenticationState(db.DB, cfg); err != nil {
		t.Fatal(err)
	}
	again, _ := localsecret.Read(path)
	db.DB.First(&user, 1)
	if code != again || user.AuthVersion != 2 {
		t.Fatal("challenge changed on restart")
	}
	if err := os.WriteFile(path, []byte("damaged"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := MigrateAuthenticationState(db.DB, cfg); err != nil {
		t.Fatal(err)
	}
	again, err = localsecret.Read(path)
	if err != nil || code == again {
		t.Fatal("damaged challenge not replaced", err)
	}
}
