package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

func (c *Config) DatabaseType() string {
	if c.DBType == "" {
		return "sqlite"
	}
	return c.DBType
}

func (c *Config) StateDirectory() string {
	if c.StateDir != "" {
		return c.StateDir
	}
	if c.DatabaseType() == "sqlite" && c.SqlitePath != "" {
		return filepath.Dir(c.SqlitePath)
	}
	return "./data"
}

func (c *Config) loadDatabaseEnvironment() error {
	c.DBType = strings.TrimSpace(getEnv("DB_TYPE", "sqlite"))
	if c.DBType != "sqlite" && c.DBType != "postgres" {
		return errors.New("DB_TYPE must be sqlite or postgres")
	}
	c.DatabaseURL = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	c.StateDir = strings.TrimSpace(os.Getenv("STATE_DIR"))
	if c.DBType == "postgres" && c.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required for postgres")
	}
	var err error
	for _, item := range []struct {
		key      string
		fallback int
		out      *int
	}{
		{"DB_MAX_OPEN_CONNS", 10, &c.DBMaxOpenConns},
		{"DB_MAX_IDLE_CONNS", 5, &c.DBMaxIdleConns},
	} {
		*item.out, err = strconv.Atoi(getEnv(item.key, strconv.Itoa(item.fallback)))
		if err != nil || *item.out < 0 || (item.key == "DB_MAX_OPEN_CONNS" && *item.out == 0) {
			return fmt.Errorf("%s is invalid", item.key)
		}
	}
	if c.DBMaxIdleConns > c.DBMaxOpenConns {
		return errors.New("DB_MAX_IDLE_CONNS exceeds DB_MAX_OPEN_CONNS")
	}
	for _, item := range []struct {
		key, fallback string
		out           *time.Duration
	}{
		{"DB_CONN_MAX_LIFETIME", "30m", &c.DBConnMaxLifetime},
		{"DB_CONN_MAX_IDLE_TIME", "5m", &c.DBConnMaxIdleTime},
	} {
		*item.out, err = time.ParseDuration(getEnv(item.key, item.fallback))
		if err != nil || *item.out <= 0 {
			return fmt.Errorf("%s must be a positive duration", item.key)
		}
	}
	return nil
}

// LoadMigration reads existing configuration without creating files or secrets.
func LoadMigration() (*Config, error) {
	path := envFilePath()
	if _, err := os.Stat(path); err == nil {
		if err := godotenv.Load(path); err != nil {
			return nil, errors.New("cannot load existing environment file")
		}
	} else if !os.IsNotExist(err) {
		return nil, errors.New("cannot inspect environment file")
	}
	c := &Config{SqlitePath: getEnv("SQLITE_PATH", "./data/data.db"), ConfigSecret: strings.TrimSpace(os.Getenv("CONFIG_SECRET"))}
	if err := c.loadDatabaseEnvironment(); err != nil {
		return nil, err
	}
	// The command always imports SQLite into PostgreSQL. An existing SQLite
	// deployment's DB_TYPE must not need changing merely to run preflight.
	c.DBType = "postgres"
	if c.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required for db-migrate")
	}
	return c, nil
}
