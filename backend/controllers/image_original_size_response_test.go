package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"oneimg/backend/database"
	"oneimg/backend/models"

	"github.com/gin-gonic/gin"
)

func TestImageListAndDetailReturnOriginalFileSize(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupUploadBatchTest(t)

	imageModel := models.Image{
		Url:              "/uploads/test/stored.webp",
		FileName:         "stored.webp",
		OriginalFileName: "photo.png",
		OriginalFileSize: 8192,
		FileSize:         4096,
		MimeType:         "image/webp",
		Storage:          "default",
		BucketId:         1,
		UserId:           1,
	}
	if err := database.GetDB().DB.Create(&imageModel).Error; err != nil {
		t.Fatal(err)
	}

	listRecorder := httptest.NewRecorder()
	listContext, _ := gin.CreateTestContext(listRecorder)
	listContext.Request = httptest.NewRequest(http.MethodGet, "/api/images?limit=20", nil)
	listContext.Set("user_id", 1)
	listContext.Set("user_role", models.RoleAdmin)
	GetImageList(listContext)

	var listResponse struct {
		Code int `json:"code"`
		Data struct {
			Images []struct {
				OriginalFileSize int64 `json:"original_file_size"`
			} `json:"images"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listResponse); err != nil {
		t.Fatal(err)
	}
	if listResponse.Code != http.StatusOK || len(listResponse.Data.Images) != 1 || listResponse.Data.Images[0].OriginalFileSize != 8192 {
		t.Fatalf("image list response = %+v", listResponse)
	}

	detailRecorder := httptest.NewRecorder()
	detailContext, _ := gin.CreateTestContext(detailRecorder)
	detailContext.Request = httptest.NewRequest(http.MethodGet, "/api/images/1", nil)
	detailContext.Params = gin.Params{{Key: "id", Value: "1"}}
	detailContext.Set("user_id", 1)
	detailContext.Set("user_role", models.RoleAdmin)
	GetImageDetail(detailContext)

	var detailResponse struct {
		Code int `json:"code"`
		Data struct {
			OriginalFileSize int64 `json:"original_file_size"`
		} `json:"data"`
	}
	if err := json.Unmarshal(detailRecorder.Body.Bytes(), &detailResponse); err != nil {
		t.Fatal(err)
	}
	if detailResponse.Code != http.StatusOK || detailResponse.Data.OriginalFileSize != 8192 {
		t.Fatalf("image detail response = %+v", detailResponse)
	}
}
