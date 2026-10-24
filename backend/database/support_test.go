package database_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"log"
	"math"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"oneimg/backend/config"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/internal/testdb"

	"gorm.io/gorm"
)

func TestPostgreSQLURLValidationAndRedaction(t *testing.T) {
	t.Setenv("PGOPTIONS", "-c search_path=untrusted -c timezone=Asia/Shanghai")
	for _, raw := range []string{"", "host=localhost password=secret", "mysql://user:secret@localhost/db", "postgres://user:secret@localhost", "postgres://user:secret@localhost/db?sslmode=require", "postgres://localhost/db?sslmode=disable&sslmode=verify-full", "postgres://localhost/db?connect_timeout=0", "postgres://localhost/db?connect_timeout=x", "postgres://localhost/db?options=-c+search_path%3Devil", "postgres://localhost/db?host=other", "postgres://localhost/db?connect_timeout=1&connect_timeout=2"} {
		if _, err := database.PostgreSQLConfig(raw); err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatal("unsafe connection config accepted or exposed", err)
		}
	}
	cfg, err := database.PostgreSQLConfig("postgresql://user:secret@db.example.invalid/oneimg")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConnectTimeout != 5*time.Second || cfg.TLSConfig == nil || cfg.TLSConfig.InsecureSkipVerify || cfg.RuntimeParams["timezone"] != "UTC" {
		t.Fatal("unsafe connection defaults")
	}
	if _, exists := cfg.RuntimeParams["options"]; exists {
		t.Fatal("environment options bypassed managed session settings")
	}
	for _, raw := range []string{"postgres://localhost/db?TimeZone=Asia/Shanghai", "postgres://localhost/db?search_path=untrusted", "postgres://localhost/db?OPTIONS=-c+timezone%3DAsia/Shanghai"} {
		if _, err := database.PostgreSQLConfig(raw); err == nil {
			t.Fatal("managed session override accepted")
		}
	}
	plain, err := database.PostgreSQLConfig("postgres://user:secret@localhost/oneimg?sslmode=disable&connect_timeout=1")
	if err != nil || plain.TLSConfig != nil || plain.ConnectTimeout != time.Second {
		t.Fatal("explicit isolated connection failed", err)
	}
}

func TestPostgreSQLTLSConfigValidatesCAAndHostname(t *testing.T) {
	now := time.Now()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test-only CA"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{SerialNumber: big.NewInt(2), DNSNames: []string{"db.example.invalid"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, KeyUsage: x509.KeyUsageDigitalSignature}, ca, &serverKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caPath := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0600); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		name, host, path string
		valid            bool
	}{
		{"valid", "db.example.invalid", caPath, true},
		{"wrong hostname", "other.example.invalid", caPath, false},
		{"wrong CA", "db.example.invalid", "", false},
	} {
		t.Run(item.name, func(t *testing.T) {
			raw := "postgres://user:fake@" + item.host + "/db?sslmode=verify-full"
			if item.path != "" {
				raw += "&sslrootcert=" + item.path
			}
			cfg, err := database.PostgreSQLConfig(raw)
			if err != nil {
				t.Fatal(err)
			}
			client, server := net.Pipe()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			go func() {
				connection := tls.Server(server, &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{certDER}, PrivateKey: serverKey}}, MinVersion: tls.VersionTLS12})
				defer connection.Close()
				_ = connection.HandshakeContext(ctx)
			}()
			connection := tls.Client(client, cfg.TLSConfig)
			defer connection.Close()
			err = connection.HandshakeContext(ctx)
			if (err == nil) != item.valid {
				t.Fatal("TLS verification mismatch", err)
			}
		})
	}
}

func TestPostgreSQLInitialPingTimeoutIsSanitized(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		connection, err := listener.Accept()
		if err == nil {
			defer connection.Close()
			<-done
		}
	}()
	defer close(done)
	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(previous)
	start := time.Now()
	db, err := database.OpenPostgres(&config.Config{DatabaseURL: "postgres://user:must-never-appear@" + listener.Addr().String() + "/db?sslmode=disable&connect_timeout=1"}, "public")
	if db != nil || err == nil || time.Since(start) > 3*time.Second {
		t.Fatal("initial ping did not time out safely", err)
	}
	if strings.Contains(err.Error()+logs.String(), "must-never-appear") {
		t.Fatal("connection secret leaked")
	}
}

func TestSchemaRestartBackfillAndPasskeyConstraint(t *testing.T) {
	db := testdb.New(t).DB
	user := models.User{Username: "Straße", Password: "hash"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	image := models.Image{FileName: "E\u0301 Σ", OriginalFileName: "original", ContentHash: "ABC", Url: "/uploads/X"}
	if err := db.Create(&image).Error; err != nil {
		t.Fatal(err)
	}
	created := image.CreatedAt
	if err := db.Exec("UPDATE images SET file_name_search='old'").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("id = ?", database.SearchVersion).Delete(&database.SchemaMigration{}).Error; err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := database.MigrateSchema(db); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.First(&image, image.Id).Error; err != nil || image.FileNameSearch != "é σ" || !image.CreatedAt.Equal(created) {
		t.Fatal("backfill changed original timestamp or failed", err)
	}
	for index, name := range []string{"Key", "key", "É", "é"} {
		err := db.Create(&models.PasskeyCredential{UserID: user.ID, Name: name, CredentialID: name, CredentialData: "encrypted"}).Error
		if index == 1 {
			if !errors.Is(err, gorm.ErrDuplicatedKey) {
				t.Fatal("ASCII duplicate not translated", err)
			}
		} else if err != nil {
			t.Fatal("non-ASCII identity changed", err)
		}
	}
}

func TestSchemaRepairsSearchKeysWrittenAfterRollback(t *testing.T) {
	db := testdb.New(t).DB
	user := models.User{Username: "张三", Password: "hash"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	current := models.Image{FileName: "Straße.png", OriginalFileName: "Straße.png", Url: "/uploads/a.png", ContentHash: "ABC"}
	if err := db.Create(&current).Error; err != nil {
		t.Fatal(err)
	}
	// An older build ignores the derived columns: it renames without updating
	// the key and inserts rows without one, while the search marker remains.
	if err := db.Exec("UPDATE users SET username = ? WHERE id = ?", "Renamed", user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO images (url, file_name, original_filename, file_size, bucket_id, user_id, content_hash) VALUES (?, ?, ?, 1, 1, 1, ?)", "/uploads/Rollback.png", "Rollback.png", "", "DEF").Error; err != nil {
		t.Fatal(err)
	}
	if err := database.MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&user, user.ID).Error; err != nil || user.UsernameSearch != "renamed" {
		t.Fatal("stale username key was not repaired", user.UsernameSearch, err)
	}
	var legacy models.Image
	if err := db.Where("file_name = ?", "Rollback.png").First(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	if legacy.FileNameSearch != "rollback.png" || legacy.URLSearch != "/uploads/rollback.png" || legacy.ContentHashSearch != "def" || legacy.OriginalFileNameSearch != "" {
		t.Fatal("missing image keys were not repaired", legacy)
	}
	if err := db.First(&current, current.Id).Error; err != nil || current.FileNameSearch != "strasse.png" {
		t.Fatal("current image key changed", current.FileNameSearch, err)
	}
}

func TestSchemaBackfillFailureRollsBack(t *testing.T) {
	db := testdb.New(t).DB
	if err := db.Create(&models.User{Username: "before", Password: "hash"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("id = ?", database.SearchVersion).Delete(&database.SchemaMigration{}).Error; err != nil {
		t.Fatal(err)
	}
	// Only stale keys are rewritten, so the injected failure needs one.
	if err := db.Exec("UPDATE users SET username_search = NULL").Error; err != nil {
		t.Fatal(err)
	}
	testdb.RejectWrites(t, db, "users", "UPDATE")
	if err := database.MigrateSchema(db); err == nil {
		t.Fatal("failed backfill accepted")
	}
	var count int64
	if err := db.Model(&database.SchemaMigration{}).Where("id = ?", database.SearchVersion).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("failed backfill marked completed", err)
	}
	if err := db.Exec("DROP TRIGGER injected_failure" + map[bool]string{true: " ON users", false: ""}[db.Dialector.Name() == "postgres"]).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.MigrateSchema(db); err != nil {
		t.Fatal("backfill restart failed", err)
	}
}

func TestPostgreSQLRejectsUnknownSchemaAndPreserves64BitSequences(t *testing.T) {
	if _, ok := os.LookupEnv("ONEIMG_TEST_POSTGRES_URL"); !ok {
		t.Skip("PostgreSQL integration explicitly disabled")
	}
	value, cleanup, err := testdb.Open(filepath.Join(t.TempDir(), "unused"))
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	db := value.DB
	if err := db.Exec("CREATE TABLE previous_version(id bigint)").Error; err != nil {
		t.Fatal(err)
	}
	if err := database.MigrateSchema(db); err == nil {
		t.Fatal("unknown PostgreSQL schema accepted")
	}
	if err := db.Exec("DROP TABLE previous_version").Error; err != nil {
		t.Fatal(err)
	}
	for _, object := range []struct{ create, drop string }{
		{`CREATE FUNCTION previous_function() RETURNS integer LANGUAGE sql AS 'SELECT 1'`, "DROP FUNCTION previous_function()"},
		{`CREATE TYPE previous_enum AS ENUM ('one')`, "DROP TYPE previous_enum"},
	} {
		if err := db.Exec(object.create).Error; err != nil {
			t.Fatal(err)
		}
		if err := database.MigrateSchema(db); err == nil {
			t.Fatal("unknown schema object accepted")
		}
		if err := db.Exec(object.drop).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := database.MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	if err := database.CheckSequences(db, nil); err != nil {
		t.Fatal("empty sequence incorrect", err)
	}
	high := int64(1 << 33)
	user := models.User{ID: int(high), Username: "high", Password: "hash"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Image{Id: int(high), FileName: "high", BucketId: int(high), UserId: int(high)}).Error; err != nil {
		t.Fatal("association ID truncated", err)
	}
	if err := database.SyncSequences(db, map[string]int64{"users": high + 10, "tags": 50}); err != nil {
		t.Fatal(err)
	}
	if err := database.CheckSequences(db, map[string]int64{"users": high + 10, "tags": 50}); err != nil {
		t.Fatal(err)
	}
	next := models.User{Username: "next", Password: "hash"}
	if err := db.Create(&next).Error; err != nil || int64(next.ID) != high+11 {
		t.Fatal("high-water lost", next.ID, err)
	}
	for range 2 {
		if err := database.SyncSequences(db, nil); err != nil {
			t.Fatal(err)
		}
	}
	next = models.User{Username: "later", Password: "hash"}
	if err := db.Create(&next).Error; err != nil || int64(next.ID) != high+12 {
		t.Fatal("sequence rewound", next.ID, err)
	}
}

func TestUsageInt64Bounds(t *testing.T) {
	db := testdb.New(t).DB
	bucket := models.Buckets{Name: "near limit", Type: "default", Usage: math.MaxInt64 - 2}
	if err := db.Create(&bucket).Error; err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		amount  int64
		allowed bool
	}{{3, false}, {2, true}, {1, false}, {0, true}, {-1, false}} {
		allowed, err := database.ReserveUsage(db, bucket.Id, item.amount)
		if item.amount < 0 {
			if err == nil {
				t.Fatal("negative amount accepted")
			}
			continue
		}
		if err != nil || allowed != item.allowed {
			t.Fatal("quota bound mismatch", item, allowed, err)
		}
	}
	for _, pair := range [][2]int64{{math.MaxInt64, 1}, {-1, 0}, {0, -1}} {
		if _, err := database.StorageBytes(pair[0], pair[1]); err == nil {
			t.Fatal("invalid size accepted")
		}
	}
}
