package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"
)

func TestCreateDefaultEnvUsesDataDirectory(t *testing.T) {
	originalWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tempDirectory := t.TempDir()
	t.Cleanup(func() {
		_ = os.Chdir(originalWorkingDirectory)
	})
	if err := os.Chdir(tempDirectory); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ONEIMG_ENV_FILE", "")

	CreateDefaultEnv()
	envPath := filepath.Join("data", ".env")
	values, err := godotenv.Read(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if values["SESSION_SECRET"] == "" || values["CONFIG_SECRET"] == "" {
		t.Fatal("generated environment file is missing persistent secrets")
	}
	if values["SQLITE_PATH"] != "./data/data.db" {
		t.Fatalf("SQLITE_PATH = %q, want ./data/data.db", values["SQLITE_PATH"])
	}
	for _, key := range []string{
		"DB_TYPE",
		"DB_HOST",
		"DB_PORT",
		"DB_USER",
		"DB_PASSWORD",
		"DB_NAME",
		"DB_CA_CERT_PATH",
		"DB_SKIP_CERT_VERIFY",
	} {
		if _, exists := values[key]; exists {
			t.Fatalf("generated environment file still contains removed setting %s", key)
		}
	}
	if _, err := os.Stat(".env"); !os.IsNotExist(err) {
		t.Fatalf("unexpected root environment file: %v", err)
	}
}

func TestExistingRootEnvTakesPrecedence(t *testing.T) {
	originalWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tempDirectory := t.TempDir()
	t.Cleanup(func() {
		_ = os.Chdir(originalWorkingDirectory)
	})
	if err := os.Chdir(tempDirectory); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ONEIMG_ENV_FILE", "")
	if err := os.WriteFile(".env", []byte("CONFIG_SECRET=existing\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if got := envFilePath(); got != ".env" {
		t.Fatalf("envFilePath() = %q, want .env", got)
	}
}
