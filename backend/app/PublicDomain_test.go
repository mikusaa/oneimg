package app

import (
	"testing"

	"oneimg/backend/models"
	"oneimg/backend/utils/publicurl"

	"gorm.io/gorm"
	"oneimg/backend/database"
	"oneimg/internal/testdb"
)

func openPublicDomainDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := testdb.New(t).DB
	local := models.Buckets{Id: 1, Name: "local", Type: "default", Config: map[string]any{}}
	if err := db.Create(&local).Error; err != nil {
		t.Fatalf("create local bucket: %v", err)
	}
	if err := database.SyncSequences(db, nil); err != nil {
		t.Fatal(err)
	}
	return db
}

func createBucket(t *testing.T, db *gorm.DB, name, kind string, config map[string]any) models.Buckets {
	t.Helper()
	bucket := models.Buckets{Name: name, Type: kind, Config: config}
	if err := db.Create(&bucket).Error; err != nil {
		t.Fatalf("create bucket %s: %v", name, err)
	}
	return bucket
}

func reloadBucket(t *testing.T, db *gorm.DB, id int) models.Buckets {
	t.Helper()
	var bucket models.Buckets
	if err := db.First(&bucket, id).Error; err != nil {
		t.Fatalf("load bucket %d: %v", id, err)
	}
	return bucket
}

func legacyDomain(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var setting models.Settings
	if err := db.First(&setting).Error; err != nil {
		t.Fatalf("load settings: %v", err)
	}
	return setting.PublicImageDomain
}

func TestMigratePublicImageDomainMovesToDefaultDirectStorage(t *testing.T) {
	db := openPublicDomainDB(t)
	r2 := createBucket(t, db, "r2", "r2", map[string]any{"r2_bucket": "media", "r2_secret_key": "enc:v1:opaque"})
	other := createBucket(t, db, "s3", "s3", map[string]any{"s3_bucket": "other"})
	if err := db.Create(&models.Settings{DefaultStorage: r2.Id, PublicImageDomain: "img.example.com/"}).Error; err != nil {
		t.Fatalf("create settings: %v", err)
	}

	if err := MigratePublicImageDomain(db); err != nil {
		t.Fatalf("MigratePublicImageDomain() error = %v", err)
	}
	migrated := reloadBucket(t, db, r2.Id)
	if got := migrated.Config[publicurl.ConfigKey]; got != "https://img.example.com" {
		t.Fatalf("migrated domain = %v", got)
	}
	if migrated.Config["r2_secret_key"] != "enc:v1:opaque" || migrated.Config["r2_bucket"] != "media" {
		t.Fatalf("existing config changed: %#v", migrated.Config)
	}
	if _, ok := reloadBucket(t, db, other.Id).Config[publicurl.ConfigKey]; ok {
		t.Fatal("non-default storage should not receive the domain")
	}
	if got := legacyDomain(t, db); got != "" {
		t.Fatalf("legacy domain = %q, want cleared", got)
	}

	// 清空旧字段后再次启动不会覆盖用户之后在存储里的修改。
	if err := db.Model(&migrated).Select("Config").Updates(models.Buckets{Config: map[string]any{"r2_bucket": "media"}}).Error; err != nil {
		t.Fatalf("clear bucket domain: %v", err)
	}
	if err := MigratePublicImageDomain(db); err != nil {
		t.Fatalf("second MigratePublicImageDomain() error = %v", err)
	}
	if _, ok := reloadBucket(t, db, r2.Id).Config[publicurl.ConfigKey]; ok {
		t.Fatal("second run should not restore the domain")
	}
}

func TestMigratePublicImageDomainClearsIneffectiveValue(t *testing.T) {
	db := openPublicDomainDB(t)
	webdav := createBucket(t, db, "dav", "webdav", map[string]any{"webdav_url": "https://dav.example.com"})
	if err := db.Create(&models.Settings{DefaultStorage: webdav.Id, PublicImageDomain: "https://img.example.com"}).Error; err != nil {
		t.Fatalf("create settings: %v", err)
	}

	if err := MigratePublicImageDomain(db); err != nil {
		t.Fatalf("MigratePublicImageDomain() error = %v", err)
	}
	if _, ok := reloadBucket(t, db, webdav.Id).Config[publicurl.ConfigKey]; ok {
		t.Fatal("unsupported storage should not receive the domain")
	}
	if got := legacyDomain(t, db); got != "" {
		t.Fatalf("legacy domain = %q, want cleared", got)
	}
}

func TestMigratePublicImageDomainKeepsExistingBucketDomain(t *testing.T) {
	db := openPublicDomainDB(t)
	s3 := createBucket(t, db, "s3", "s3", map[string]any{publicurl.ConfigKey: "https://cdn.example.com"})
	if err := db.Create(&models.Settings{DefaultStorage: s3.Id, PublicImageDomain: "https://img.example.com"}).Error; err != nil {
		t.Fatalf("create settings: %v", err)
	}

	if err := MigratePublicImageDomain(db); err != nil {
		t.Fatalf("MigratePublicImageDomain() error = %v", err)
	}
	if got := reloadBucket(t, db, s3.Id).Config[publicurl.ConfigKey]; got != "https://cdn.example.com" {
		t.Fatalf("bucket domain = %v, want existing value", got)
	}
	if got := legacyDomain(t, db); got != "" {
		t.Fatalf("legacy domain = %q, want cleared", got)
	}
}

func TestMigratePublicImageDomainWithoutValueIsNoop(t *testing.T) {
	db := openPublicDomainDB(t)
	r2 := createBucket(t, db, "r2", "r2", map[string]any{"r2_bucket": "media"})
	if err := db.Create(&models.Settings{DefaultStorage: r2.Id}).Error; err != nil {
		t.Fatalf("create settings: %v", err)
	}
	if err := MigratePublicImageDomain(db); err != nil {
		t.Fatalf("MigratePublicImageDomain() error = %v", err)
	}
	if _, ok := reloadBucket(t, db, r2.Id).Config[publicurl.ConfigKey]; ok {
		t.Fatal("no domain should be written")
	}
}
