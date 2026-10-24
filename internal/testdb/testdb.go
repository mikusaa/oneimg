// Package testdb owns disposable databases. An explicitly requested PostgreSQL
// database must be reachable; configuration errors never fall back to SQLite.
package testdb

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gorm.io/gorm"
	"oneimg/backend/config"
	"oneimg/backend/database"
)

func Open(sqlitePath string) (*database.Database, func(), error) {
	raw, enabled := os.LookupEnv("ONEIMG_TEST_POSTGRES_URL")
	if !enabled {
		db, err := database.Open(&config.Config{SqlitePath: sqlitePath})
		if err != nil {
			return nil, nil, err
		}
		return db, func() { _ = db.Close() }, nil
	}
	if raw == "" {
		return nil, nil, errors.New("ONEIMG_TEST_POSTGRES_URL is empty")
	}
	cfg := &config.Config{DBType: "postgres", DatabaseURL: raw}
	admin, err := database.OpenPostgres(cfg, "public")
	if err != nil {
		return nil, nil, err
	}
	defer admin.Close()
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, nil, err
	}
	name := "oneimg_test_" + hex.EncodeToString(nonce[:])
	if err := admin.DB.Exec(`CREATE SCHEMA "` + name + `"`).Error; err != nil {
		return nil, nil, errors.New("cannot create isolated test schema")
	}
	cleanupSchema := func() {
		connection, err := database.OpenPostgres(cfg, "public")
		if err != nil {
			return
		}
		defer connection.Close()
		_ = connection.DB.Exec(`DROP SCHEMA "` + name + `" CASCADE`).Error
	}
	db, err := database.OpenPostgres(cfg, name)
	if err != nil {
		cleanupSchema()
		return nil, nil, err
	}
	return db, func() { _ = db.Close(); cleanupSchema() }, nil
}

func New(t testing.TB) *database.Database {
	t.Helper()
	db, cleanup, err := Open(filepath.Join(t.TempDir(), "oneimg.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	if err := database.MigrateSchema(db.DB); err != nil {
		t.Fatal(err)
	}
	return db
}

func RejectWrites(t testing.TB, db *gorm.DB, table, event string) {
	t.Helper()
	if db.Dialector.Name() == "sqlite" {
		if err := db.Exec("CREATE TRIGGER injected_failure BEFORE " + event + " ON " + table + " BEGIN SELECT RAISE(ABORT, 'injected failure'); END").Error; err != nil {
			t.Fatal(err)
		}
		return
	}
	if err := db.Exec(`CREATE FUNCTION injected_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected failure'; END $$`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TRIGGER injected_failure BEFORE " + event + " ON " + table + " FOR EACH ROW EXECUTE FUNCTION injected_failure()").Error; err != nil {
		t.Fatal(err)
	}
}
