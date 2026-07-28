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
