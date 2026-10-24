package app

import (
	"os"
	"path/filepath"
	"testing"

	"oneimg/backend/config"
	"oneimg/backend/models"
	"oneimg/backend/services"
	"oneimg/internal/testdb"
)

func TestDatabaseInitializationAndStateDirectory(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	db, cleanup, err := testdb.Open(filepath.Join(root, "database", "oneimg.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	cfg := &config.Config{DBType: db.DB.Dialector.Name(), SqlitePath: filepath.Join(root, "database", "oneimg.db"), StateDir: filepath.Join(root, "state"), DefaultUser: "admin", AppURL: "http://localhost:8080", ConfigSecret: "initialization-test-secret", ImageMaxPixels: 40000000, ImageMaxDimension: 16384}
	for range 2 {
		system, err := InitializeDatabase(cfg, db)
		if err != nil {
			t.Fatal(err)
		}
		bucket, err := system.Services.Storage.Create(services.StorageInput{Name: "temporary remote", Type: "webdav", Config: map[string]any{"webdav_url": "https://example.invalid", "webdav_user": "test", "webdav_pass": "test"}})
		if err != nil {
			t.Fatal("fixed ID sequence not synchronized", err)
		}
		if bucket.Id <= 1 {
			t.Fatal("automatic ID collided with built-in bucket")
		}
		if err := db.DB.Delete(&bucket).Error; err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(cfg.StateDir, ".initial_admin_password")); err != nil {
		t.Fatal("bootstrap state not persisted", err)
	}
	if _, err := os.Stat(filepath.Join(root, "database", ".initial_admin_password")); !os.IsNotExist(err) {
		t.Fatal("state incorrectly followed SQLite path", err)
	}
	var users int64
	if err := db.DB.Model(&models.User{}).Count(&users).Error; err != nil || users != 1 {
		t.Fatal("repeated initialization duplicated user", err)
	}
}
