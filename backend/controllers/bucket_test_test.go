package controllers

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"oneimg/backend/database"
	"oneimg/backend/models"

	"github.com/gin-gonic/gin"
)

func TestBucketConnectionRejectsRemovedTelegramStorage(t *testing.T) {
	_, err := buildBucketConnectionCandidate(map[string]any{
		"type":         "telegram",
		"tg_bot_token": "token",
		"tg_receivers": "chat",
	})
	if err == nil {
		t.Fatal("buildBucketConnectionCandidate() should reject removed Telegram storage")
	}
}

func TestDeleteEmptyBucketPreservesUserPermissionCodes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupFeatureTestDB(t)
	db := database.GetDB().DB

	localBucket := models.Buckets{Id: 1, Name: "local", Type: "default", Config: map[string]any{}}
	remoteBucket := models.Buckets{Id: 2, Name: "remote", Type: "s3", Config: map[string]any{}}
	if err := db.Create(&localBucket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&remoteBucket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Settings{DefaultStorage: remoteBucket.Id}).Error; err != nil {
		t.Fatal(err)
	}
	user := models.User{
		ID:       2,
		Username: "manager",
		Password: "hash",
		Role:     models.RoleAdmin,
		Permission: models.Permission{
			Codes:   []string{"tag:update"},
			Buckets: []int{remoteBucket.Id},
		},
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodDelete, "/api/buckets/2", nil)
	context.Params = gin.Params{{Key: "id", Value: "2"}}
	DeleteBuckets(context)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	if err := db.First(&user, user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(user.Permission.Codes, []string{"tag:update"}) || len(user.Permission.Buckets) != 0 {
		t.Fatalf("permission = %+v", user.Permission)
	}
	var setting models.Settings
	if err := db.First(&setting).Error; err != nil {
		t.Fatal(err)
	}
	if setting.DefaultStorage != 1 {
		t.Fatalf("default storage = %d, want 1", setting.DefaultStorage)
	}
	if db.Migrator().HasTable("image_storages") {
		t.Fatal("bucket deletion must not require the legacy image_storages table")
	}
}
