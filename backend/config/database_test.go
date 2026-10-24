package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func cleanDatabaseEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{"DB_TYPE", "DATABASE_URL", "STATE_DIR", "DB_MAX_OPEN_CONNS", "DB_MAX_IDLE_CONNS", "DB_CONN_MAX_LIFETIME", "DB_CONN_MAX_IDLE_TIME"} {
		t.Setenv(key, "")
	}
}

func TestDatabaseDefaultsAndValidation(t *testing.T) {
	cleanDatabaseEnvironment(t)
	cfg := &Config{SqlitePath: "/test/custom/oneimg.db"}
	if err := cfg.loadDatabaseEnvironment(); err != nil {
		t.Fatal(err)
	}
	if cfg.DatabaseType() != "sqlite" || cfg.StateDirectory() != "/test/custom" || cfg.DBMaxOpenConns != 10 || cfg.DBMaxIdleConns != 5 || cfg.DBConnMaxLifetime != 30*time.Minute || cfg.DBConnMaxIdleTime != 5*time.Minute {
		t.Fatal("unexpected database defaults")
	}
	for _, item := range []struct{ key, value string }{
		{"DB_TYPE", "mysql"}, {"DB_MAX_OPEN_CONNS", "0"}, {"DB_MAX_OPEN_CONNS", "-1"},
		{"DB_MAX_IDLE_CONNS", "-1"}, {"DB_MAX_IDLE_CONNS", "11"},
		{"DB_CONN_MAX_LIFETIME", "bad"}, {"DB_CONN_MAX_LIFETIME", "0s"},
		{"DB_CONN_MAX_IDLE_TIME", "-1s"},
	} {
		t.Run(item.key+"="+item.value, func(t *testing.T) {
			t.Setenv(item.key, item.value)
			if err := new(Config).loadDatabaseEnvironment(); err == nil {
				t.Fatal("invalid database environment accepted")
			}
		})
	}
	t.Setenv("DB_TYPE", "postgres")
	if err := cfg.loadDatabaseEnvironment(); err == nil {
		t.Fatal("PostgreSQL URL was optional")
	}
	t.Setenv("DATABASE_URL", "postgres://localhost/oneimg")
	if err := cfg.loadDatabaseEnvironment(); err != nil || cfg.StateDirectory() != "./data" {
		t.Fatal("PostgreSQL state default", err)
	}
	t.Setenv("STATE_DIR", "/test/original-state")
	if err := cfg.loadDatabaseEnvironment(); err != nil || cfg.StateDirectory() != "/test/original-state" {
		t.Fatal("explicit state directory ignored", err)
	}
}

func TestMigrationConfigurationHasNoFilesystemSideEffects(t *testing.T) {
	cleanDatabaseEnvironment(t)
	root := t.TempDir()
	t.Chdir(root)
	path := filepath.Join(root, "read-only.env")
	t.Setenv("ONEIMG_ENV_FILE", path)
	t.Setenv("DB_TYPE", "sqlite")
	t.Setenv("DATABASE_URL", "postgres://localhost/oneimg?sslmode=disable")
	t.Setenv("CONFIG_SECRET", "existing-test-secret")
	cfg, err := LoadMigration()
	if err != nil || cfg.ConfigSecret != "existing-test-secret" {
		t.Fatal("migration config failed", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("migration configuration created files", err)
	}
	// Existing files only supplement the process environment.
	if err := os.WriteFile(path, []byte("DB_TYPE=sqlite\nCONFIG_SECRET=other-test-secret\n"), 0400); err != nil {
		t.Fatal(err)
	}
	cfg, err = LoadMigration()
	if err != nil || cfg.DatabaseType() != "postgres" || cfg.ConfigSecret != "existing-test-secret" {
		t.Fatal("environment precedence changed", err)
	}
}
