package services

import (
	"context"
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"oneimg/backend/database"
	"oneimg/backend/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := database.NewDB(sqlite.Open(filepath.Join(t.TempDir(), "oneimg.db")))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.DB.AutoMigrate(
		&models.Tags{},
		&models.User{},
		&models.PasskeyCredential{},
		&models.PersonalAccessToken{},
		&models.Image{},
		&models.Settings{},
		&models.ImageToTags{},
		&models.Buckets{},
	); err != nil {
		t.Fatal(err)
	}
	return db.DB
}

func createServiceTestUser(t *testing.T, db *gorm.DB, username, password string, role int) models.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	user := models.User{Username: username, Password: string(hash), Role: role, Permission: models.Permission{Codes: []string{}, Buckets: []int{}}}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	return user
}

func TestPersonalAccessTokenLifecycle(t *testing.T) {
	db := newServiceTestDB(t)
	user := createServiceTestUser(t, db, "token-user", "correct-password", models.RoleAdmin)
	service := NewTokenService(db, "test-token-hmac-secret")
	now := time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	created, err := service.Create(user.ID, CreateTokenInput{
		Name: "automation", Scopes: []string{"images:read", "images:read", "tags:read"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^oneimg_pat_[A-Za-z0-9_-]{12}_[A-Za-z0-9_-]{43}$`).MatchString(created.Plain) {
		t.Fatalf("unexpected token format: %q", created.Plain)
	}
	if created.Token.SecretHash == created.Plain || len(created.Token.SecretHash) != sha256HexLength {
		t.Fatalf("token hash was not stored as SHA-256 hex: %q", created.Token.SecretHash)
	}
	if created.Token.ExpiresAt == nil || !created.Token.ExpiresAt.Equal(now.AddDate(0, 0, 90)) {
		t.Fatalf("default expiry = %v", created.Token.ExpiresAt)
	}
	if len(created.Token.Scopes) != 2 {
		t.Fatalf("normalized scopes = %#v", created.Token.Scopes)
	}

	var stored models.PersonalAccessToken
	if err := db.First(&stored, created.Token.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.SecretHash != created.Token.SecretHash || stored.SecretHash == created.Plain {
		t.Fatal("database must contain only the HMAC digest")
	}
	authenticated, authenticatedUser, err := service.Authenticate(created.Plain)
	if err != nil {
		t.Fatal(err)
	}
	if authenticatedUser.ID != user.ID || authenticated.LastUsedAt == nil || !authenticated.LastUsedAt.Equal(now) {
		t.Fatalf("authenticated token = %#v, user = %#v", authenticated, authenticatedUser)
	}

	never := 0
	permanent, err := service.Create(user.ID, CreateTokenInput{Name: "permanent", Scopes: []string{"stats:read"}, ExpirationDays: &never})
	if err != nil {
		t.Fatal(err)
	}
	if permanent.Token.ExpiresAt != nil {
		t.Fatalf("permanent token expiry = %v", permanent.Token.ExpiresAt)
	}

	if err := service.Revoke(user.ID, created.Token.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.Authenticate(created.Plain); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("revoked token error = %v", err)
	}
	if err := db.Delete(&user).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.Authenticate(permanent.Plain); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("deleted user token error = %v", err)
	}
}

const sha256HexLength = 64

func TestParseTokenPrefixAllowsBase64URLUnderscore(t *testing.T) {
	prefix := "abc_defghijk"
	plain := personalTokenPrefix + prefix + "_" + strings.Repeat("A", personalTokenSecretLength)

	parsed, ok := parseTokenPrefix(plain)
	if !ok || parsed != prefix {
		t.Fatalf("parseTokenPrefix(%q) = %q, %t", plain, parsed, ok)
	}

	for _, malformed := range []string{
		personalTokenPrefix + prefix + strings.Repeat("A", personalTokenSecretLength),
		personalTokenPrefix + prefix + "_" + strings.Repeat("A", personalTokenSecretLength-1),
		personalTokenPrefix + prefix + "_" + strings.Repeat("A", personalTokenSecretLength+1),
	} {
		if parsed, ok := parseTokenPrefix(malformed); ok {
			t.Fatalf("malformed token parsed as %q", parsed)
		}
	}
}

func TestPersonalAccessTokenValidation(t *testing.T) {
	db := newServiceTestDB(t)
	user := createServiceTestUser(t, db, "scope-user", "correct-password", models.RoleAdmin)
	service := NewTokenService(db, "test-token-hmac-secret")
	for _, test := range []struct {
		name  string
		input CreateTokenInput
		err   error
	}{
		{"missing name", CreateTokenInput{Scopes: []string{"images:read"}}, ErrInvalidTokenName},
		{"missing scope", CreateTokenInput{Name: "test"}, ErrTokenScopesRequired},
		{"unknown scope", CreateTokenInput{Name: "test", Scopes: []string{"admin:all"}}, ErrInvalidTokenScope},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.Create(user.ID, test.input)
			if !errors.Is(err, test.err) {
				t.Fatalf("error = %v, want %v", err, test.err)
			}
		})
	}
}

func TestImageTagTransactionIsAtomicAndIdempotent(t *testing.T) {
	db := newServiceTestDB(t)
	image := models.Image{FileName: "image.png", BucketId: 1, UserId: 1}
	tag := models.Tags{Name: "valid"}
	if err := db.Create(&image).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&tag).Error; err != nil {
		t.Fatal(err)
	}
	service := NewTagService(db)

	if err := service.UpdateImageTags([]int{image.Id}, []int{tag.Id, tag.Id + 1000}, nil); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("invalid batch error = %v", err)
	}
	var count int64
	if err := db.Model(&models.ImageToTags{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("invalid batch wrote %d links", count)
	}

	for range 2 {
		if err := service.UpdateImageTags([]int{image.Id}, []int{tag.Id}, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Model(&models.ImageToTags{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("idempotent update wrote %d links", count)
	}
}

func TestSettingsUpdateRollsBackAtomically(t *testing.T) {
	db := newServiceTestDB(t)
	setting := models.Settings{SEOTitle: "before", DefaultStorage: 1}
	if err := db.Create(&setting).Error; err != nil {
		t.Fatal(err)
	}
	title := "after"
	missingBucket := 999
	_, err := NewSettingsService(db).Update(SettingsPatch{SEOTitle: &title, DefaultStorage: &missingBucket})
	if !errors.Is(err, ErrBucketNotFound) {
		t.Fatalf("error = %v", err)
	}
	var stored models.Settings
	if err := db.First(&stored, setting.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.SEOTitle != "before" || stored.DefaultStorage != 1 {
		t.Fatalf("settings changed after rollback: %#v", stored)
	}
}

func TestStorageDeleteFailureKeepsDatabaseRecords(t *testing.T) {
	db := newServiceTestDB(t)
	bucket := models.Buckets{Id: 2, Name: "broken", Type: "unsupported", Capacity: 1024}
	if err := db.Create(&bucket).Error; err != nil {
		t.Fatal(err)
	}
	image := models.Image{FileName: "image.png", Url: "remote/image.png", Storage: bucket.Type, BucketId: bucket.Id, UserId: 1}
	if err := db.Create(&image).Error; err != nil {
		t.Fatal(err)
	}
	err := NewStorageService(db).Delete(context.Background(), bucket.Id)
	if !errors.Is(err, ErrStorageDelete) {
		t.Fatalf("error = %v", err)
	}
	for model, id := range map[any]int{&models.Buckets{}: bucket.Id, &models.Image{}: image.Id} {
		var count int64
		if err := db.Model(model).Where("id = ?", id).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("%T record was deleted", model)
		}
	}
}

func TestDashboardStatsQueriesAreIndependentAndUserScoped(t *testing.T) {
	db := newServiceTestDB(t)
	admin := createServiceTestUser(t, db, "stats-admin", "correct-password", models.RoleAdmin)
	alice := createServiceTestUser(t, db, "stats-alice", "correct-password", models.RoleUser)
	bob := createServiceTestUser(t, db, "stats-bob", "correct-password", models.RoleUser)
	if err := db.Create(&models.Settings{}).Error; err != nil {
		t.Fatal(err)
	}

	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Date(2026, time.August, 30, 10, 0, 0, 0, location)
	images := []models.Image{
		{FileName: "before-month.jpg", FileSize: 10, MimeType: "image/jpeg", UserId: alice.ID, CreatedAt: time.Date(2026, time.July, 31, 23, 59, 59, 0, location)},
		{FileName: "month-start.png", FileSize: 20, MimeType: "image/png", UserId: alice.ID, CreatedAt: time.Date(2026, time.August, 1, 0, 0, 0, 0, location)},
		{FileName: "before-today.png", FileSize: 30, MimeType: "image/png", UserId: alice.ID, CreatedAt: time.Date(2026, time.August, 29, 23, 59, 59, 0, location)},
		{FileName: "today-start.webp", FileSize: 40, MimeType: "image/webp", UserId: alice.ID, CreatedAt: time.Date(2026, time.August, 30, 0, 0, 0, 0, location)},
		{FileName: "today-bob.jpg", FileSize: 50, MimeType: "image/jpeg", UserId: bob.ID, CreatedAt: time.Date(2026, time.August, 30, 9, 0, 0, 0, location)},
		{FileName: "month-bob.png", FileSize: 60, MimeType: "image/png", UserId: bob.ID, CreatedAt: time.Date(2026, time.August, 15, 12, 0, 0, 0, location)},
	}
	if err := db.Create(&images).Error; err != nil {
		t.Fatal(err)
	}

	service := NewStatsService(db)
	service.now = func() time.Time { return now }
	for _, test := range []struct {
		name                      string
		user                      models.User
		total, size, today, month int64
	}{
		{name: "administrator sees all images", user: admin, total: 6, size: 210, today: 2, month: 5},
		{name: "ordinary user sees own images", user: alice, total: 4, size: 100, today: 1, month: 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			stats, err := service.Dashboard(test.user)
			if err != nil {
				t.Fatal(err)
			}
			if stats.TotalImages != test.total || stats.TotalSize != test.size || stats.TodayUploads != test.today || stats.MonthUploads != test.month {
				t.Fatalf("dashboard totals = (%d, %d, %d, %d), want (%d, %d, %d, %d)", stats.TotalImages, stats.TotalSize, stats.TodayUploads, stats.MonthUploads, test.total, test.size, test.today, test.month)
			}
			if int64(len(stats.RecentImages)) != test.total {
				t.Fatalf("recent image count = %d, want %d", len(stats.RecentImages), test.total)
			}
		})
	}

	trend := service.ImageTrend(admin, "day")
	if len(trend) != 30 || trend[len(trend)-2].Count != 1 || trend[len(trend)-1].Count != 2 {
		t.Fatalf("unexpected daily trend tail: %#v", trend[len(trend)-2:])
	}
}

func TestRemoteImportRejectsNonPublicAddresses(t *testing.T) {
	for _, rawURL := range []string{
		"http://127.0.0.1/image.png",
		"http://10.0.0.1/image.png",
		"http://169.254.169.254/latest/meta-data",
		"http://[::1]/image.png",
		"http://[fe80::1]/image.png",
		"http://224.0.0.1/image.png",
	} {
		t.Run(rawURL, func(t *testing.T) {
			if _, err := validateRemoteURL(rawURL); !errors.Is(err, ErrImportSSRF) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}
