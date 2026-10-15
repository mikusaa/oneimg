package services

import (
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"oneimg/backend/config"
	"oneimg/backend/interfaces"
	"oneimg/backend/models"
	"oneimg/backend/utils/secureconfig"
)

type topologyUploader struct {
	accountingUploader
	started chan string
	resume  <-chan struct{}
}

func (u topologyUploader) Upload(c *gin.Context, settings *models.Settings, bucket *models.Buckets, file *multipart.FileHeader) (*interfaces.ImageUploadResult, error) {
	u.started <- secureconfig.GetString(bucket.Config, "webdav_url")
	<-u.resume
	return u.accountingUploader.Upload(c, settings, bucket, file)
}

func TestEndpointUpdateWaitsForUploadRecordCommit(t *testing.T) {
	db := newServiceTestDB(t)
	config.App = &config.Config{ConfigSecret: "test-only-secret"}
	if err := db.Create(&models.Settings{DefaultStorage: 2}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Buckets{Id: 2, Name: "remote", Type: "webdav", Capacity: 100, Config: map[string]any{
		"webdav_url": "http://old.example.test", "webdav_user": "test", "webdav_pass": "test-only",
	}}).Error; err != nil {
		t.Fatal(err)
	}
	started, resume := make(chan string, 1), make(chan struct{})
	service := NewUploadService(db)
	service.storageUploader = func(*models.Buckets) (interfaces.StorageUploader, error) {
		return topologyUploader{accountingUploader{storage: "webdav", fileSize: 10}, started, resume}, nil
	}
	uploaded := make(chan error, 1)
	go func() {
		results, err := service.UploadBatch(&gin.Context{}, models.User{ID: 1, Role: models.RoleAdmin}, []*multipart.FileHeader{{Filename: "normal.webp"}}, 2, nil)
		if err == nil && len(results) == 1 {
			err = results[0].Error
		}
		uploaded <- err
	}()
	if endpoint := <-started; endpoint != "http://old.example.test" {
		t.Fatal("upload used an unexpected endpoint", endpoint)
	}
	updated, updating := make(chan error, 1), make(chan struct{})
	go func() {
		close(updating)
		_, err := NewStorageService(db).Update(2, StorageInput{Name: "changed", Type: "webdav", Config: map[string]any{
			"webdav_url": "http://new.example.test", "webdav_pass": "new-test-only",
		}})
		updated <- err
	}()
	<-updating
	select {
	case err := <-updated:
		close(resume)
		t.Fatal("endpoint changed before upload completion", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(resume)
	if err := <-uploaded; err != nil {
		t.Fatal(err)
	}
	if err := <-updated; err != nil {
		t.Fatal(err)
	}
	var bucket models.Buckets
	if err := db.First(&bucket, 2).Error; err != nil {
		t.Fatal(err)
	}
	if bucket.Name != "changed" || bucket.Capacity != 0 || bucket.Usage != 10 || secureconfig.GetString(bucket.Config, "webdav_url") != "http://new.example.test" {
		t.Fatal("successful update did not preserve usage or serialize config", bucket.Name, bucket.Capacity, bucket.Usage)
	}
	rawSecret, _ := bucket.Config["webdav_pass"].(string)
	if !secureconfig.IsEncryptedValue(rawSecret) || secureconfig.GetString(bucket.Config, "webdav_pass") != "new-test-only" {
		t.Fatal("update did not encrypt and preserve the replacement credential")
	}
	var count int64
	if err := db.Model(&models.Image{}).Where("bucket_id = ?", 2).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("upload record was not committed", count, err)
	}
}

func TestEndpointUpdateCannotChangeCrossBucketReferencesDuringDeletion(t *testing.T) {
	db := newServiceTestDB(t)
	config.App = &config.Config{ConfigSecret: "test-only-secret"}
	deleting, resume := make(chan struct{}), make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Error("unexpected upstream method", r.Method)
		}
		close(deleting)
		<-resume
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	buckets := []models.Buckets{
		{Id: 2, Name: "a", Type: "webdav", Config: map[string]any{"webdav_url": upstream.URL, "webdav_user": "test", "webdav_pass": "test-only"}},
		{Id: 3, Name: "b", Type: "webdav", Config: map[string]any{"webdav_url": "http://unused.example.test", "webdav_user": "test", "webdav_pass": "test-only"}},
	}
	if err := db.Create(&buckets).Error; err != nil {
		t.Fatal(err)
	}
	records := []models.Image{{Url: "/same.png", Storage: "webdav", BucketId: 2}, {Url: "/same.png", Storage: "webdav", BucketId: 3}}
	if err := db.Create(&records).Error; err != nil {
		t.Fatal(err)
	}
	deleted := make(chan error, 1)
	go func() { deleted <- NewImageService(db).Delete(context.Background(), records[0].Id) }()
	<-deleting
	updated, started := make(chan error, 1), make(chan struct{})
	go func() {
		close(started)
		_, err := NewStorageService(db).Update(3, StorageInput{Name: "b", Type: "webdav", Config: map[string]any{"webdav_url": upstream.URL, "webdav_pass": "new-test-only"}})
		updated <- err
	}()
	<-started
	select {
	case err := <-updated:
		close(resume)
		t.Fatal("configuration changed during a physical deletion", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(resume)
	if err := <-deleted; err != nil {
		t.Fatal(err)
	}
	if err := <-updated; err != nil {
		t.Fatal(err)
	}
}

func TestPhysicalEndpointDefaultPortIPv6Alias(t *testing.T) {
	a, err := normalizedPhysicalEndpoint("https://[2001:db8::1]")
	if err != nil {
		t.Fatal(err)
	}
	b, err := normalizedPhysicalEndpoint("https://[2001:db8::1]:443")
	if err != nil || a != b {
		t.Fatal("default-port IPv6 alias escaped reference checking", a, b, err)
	}
}

func TestHistoricalCrossReferencesAndConcurrentDeletion(t *testing.T) {
	t.Chdir(t.TempDir())
	db := newServiceTestDB(t)
	if err := db.Create(&models.Buckets{Id: 1, Name: "local", Type: "default", Usage: 20}).Error; err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"uploads/shared.png", "uploads/b.png", "data/thumbnails/only-a.webp"} {
		writeAccountingFile(t, file, []byte("original"))
	}
	records := []models.Image{
		{Url: "/uploads/shared.png", Thumbnail: "/thumbnails/only-a.webp", Storage: "default", BucketId: 1, FileSize: 10},
		{Url: "uploads/b.png", Thumbnail: "uploads/shared.png", Storage: "default", BucketId: 1, FileSize: 10},
	}
	if err := db.Create(&records).Error; err != nil {
		t.Fatal(err)
	}
	if count, err := CountSharedPhysicalObjects(db); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	service := NewImageService(db)
	if err := service.Delete(context.Background(), records[0].Id); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("uploads/shared.png"); err != nil {
		t.Fatal("shared main/thumbnail removed")
	}
	if _, err := os.Stat("data/thumbnails/only-a.webp"); !os.IsNotExist(err) {
		t.Fatal("unreferenced thumbnail retained")
	}
	if err := service.Delete(context.Background(), records[1].Id); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("uploads/shared.png"); !os.IsNotExist(err) {
		t.Fatal("last reference not cleaned")
	}
	for i := range records {
		records[i].Id = 0
		records[i].Url = "/uploads/shared.png"
		records[i].Thumbnail = ""
	}
	writeAccountingFile(t, "uploads/shared.png", []byte("new"))
	if err := db.Create(&records).Error; err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, record := range records {
		wg.Add(1)
		go func(id int) { defer wg.Done(); errs <- service.Delete(context.Background(), id) }(record.Id)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	objectLocks.Lock()
	defer objectLocks.Unlock()
	if len(objectLocks.entries) != 0 {
		t.Fatal("object lock entries leaked")
	}
}

func TestPhysicalIdentityAcrossLogicalBuckets(t *testing.T) {
	db := newServiceTestDB(t)
	cfg := map[string]any{"s3_endpoint": "https://storage.example.test", "s3_bucket": "media"}
	a := models.Buckets{Id: 2, Name: "a", Type: "s3", Config: cfg}
	b := models.Buckets{Id: 3, Name: "b", Type: "s3", Config: cfg}
	if err := db.Create(&[]models.Buckets{a, b}).Error; err != nil {
		t.Fatal(err)
	}
	records := []models.Image{{Url: "/same.png", Storage: "s3", BucketId: 2}, {Thumbnail: "same.png", Storage: "s3", BucketId: 3}}
	if err := db.Create(&records).Error; err != nil {
		t.Fatal(err)
	}
	target, err := unreferencedImageObjects(db, records[0], a, []int{records[0].Id})
	if err != nil || target.Url != "" {
		t.Fatal("logical bucket alias defeated reference protection", err)
	}
}

func TestCustomLocalPathDeletionUsesActualObject(t *testing.T) {
	t.Chdir(t.TempDir())
	db := newServiceTestDB(t)
	if err := db.Create(&models.Buckets{Id: 1, Type: "default", Usage: 10}).Error; err != nil {
		t.Fatal(err)
	}
	writeAccountingFile(t, "custom/photo.png", []byte("actual object"))
	writeAccountingFile(t, "uploads/custom/photo.png", []byte("unrelated object"))
	record := models.Image{Url: "/custom/photo.png", Storage: "default", BucketId: 1, FileSize: 10}
	if err := db.Create(&record).Error; err != nil {
		t.Fatal(err)
	}
	if err := NewImageService(db).Delete(context.Background(), record.Id); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("custom/photo.png"); !os.IsNotExist(err) {
		t.Fatal("actual object was not deleted", err)
	}
	data, err := os.ReadFile("uploads/custom/photo.png")
	if err != nil || string(data) != "unrelated object" {
		t.Fatal("a different object was deleted", err)
	}
}
