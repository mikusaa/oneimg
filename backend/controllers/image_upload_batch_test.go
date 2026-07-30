package controllers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/images"

	"github.com/gin-gonic/gin"
)

type uploadBatchResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"message"`
	Data struct {
		Files       []uploadBatchFile `json:"files"`
		Count       int               `json:"count"`
		FailedCount int               `json:"failed_count"`
	} `json:"data"`
}

type uploadBatchFile struct {
	Success          bool   `json:"success"`
	Message          string `json:"message"`
	OriginalFileName string `json:"original_filename"`
	OriginalFileSize int64  `json:"original_file_size"`
}

func setupUploadBatchTest(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	setupFeatureTestDB(t)
	images.InitImageService()

	db := database.GetDB().DB
	setting := models.Settings{
		OriginalImage:  true,
		SaveWebp:       false,
		Thumbnail:      false,
		DefaultStorage: 1,
		MaxFileSize:    1024 * 1024,
		AllowedTypes:   "image/png",
		DefaultPath:    "uploads/test",
		FileName:       "{random}",
	}
	if err := db.Create(&setting).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Buckets{
		Id: 1, Name: "本地默认存储", Type: "default", Config: map[string]any{},
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func performImageUpload(t *testing.T, files []struct {
	name        string
	contentType string
	data        []byte
}) uploadBatchResponse {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, file := range files {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", `form-data; name="images[]"; filename="`+file.name+`"`)
		header.Set("Content-Type", file.contentType)
		part, err := writer.CreatePart(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(file.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.WriteField("bucket_id", "1"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/upload/images", &body)
	context.Request.Header.Set("Content-Type", writer.FormDataContentType())
	context.Set("user_id", 1)
	context.Set("user_role", models.RoleAdmin)
	UploadImages(context)

	var response uploadBatchResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body.String(), err)
	}
	return response
}

func TestUploadImagesReturnsOrderedPerFileResults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupUploadBatchTest(t)

	validPNG, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	response := performImageUpload(t, []struct {
		name        string
		contentType string
		data        []byte
	}{
		{name: "valid.png", contentType: "image/png", data: validPNG},
		{name: "broken.png", contentType: "image/png", data: []byte("not a png")},
	})

	if response.Code != 200 || response.Data.Count != 1 || response.Data.FailedCount != 1 {
		t.Fatalf("response = %+v", response)
	}
	if len(response.Data.Files) != 2 {
		t.Fatalf("file result count = %d", len(response.Data.Files))
	}
	if !response.Data.Files[0].Success || response.Data.Files[0].OriginalFileName != "valid.png" {
		t.Fatalf("first result = %+v", response.Data.Files[0])
	}
	if response.Data.Files[0].OriginalFileSize != int64(len(validPNG)) {
		t.Fatalf("original file size = %d, want %d", response.Data.Files[0].OriginalFileSize, len(validPNG))
	}
	var storedImage models.Image
	if err := database.GetDB().DB.First(&storedImage).Error; err != nil {
		t.Fatal(err)
	}
	if storedImage.OriginalFileSize != int64(len(validPNG)) {
		t.Fatalf("stored original file size = %d, want %d", storedImage.OriginalFileSize, len(validPNG))
	}
	if response.Data.Files[1].Success || response.Data.Files[1].OriginalFileName != "broken.png" || response.Data.Files[1].Message == "" {
		t.Fatalf("second result = %+v", response.Data.Files[1])
	}
}

func TestUploadImagesReturnsResultsWhenAllFilesFail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupUploadBatchTest(t)

	response := performImageUpload(t, []struct {
		name        string
		contentType string
		data        []byte
	}{
		{name: "broken.png", contentType: "image/png", data: []byte("not a png")},
	})

	if response.Code != 500 || response.Data.Count != 0 || response.Data.FailedCount != 1 {
		t.Fatalf("response = %+v", response)
	}
	if len(response.Data.Files) != 1 || response.Data.Files[0].Success || response.Data.Files[0].OriginalFileName != "broken.png" {
		t.Fatalf("file results = %+v", response.Data.Files)
	}
}

func TestGetUploadConfigIncludesQueueValidationSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupUploadBatchTest(t)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/uploadConfig", nil)
	context.Set("user_id", 1)
	context.Set("user_role", models.RoleAdmin)
	GetUploadConfig(context)

	var response struct {
		Code int `json:"code"`
		Data struct {
			MaxFileSize  int    `json:"max_file_size"`
			AllowedTypes string `json:"allowed_types"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || response.Data.MaxFileSize != 1024*1024 || response.Data.AllowedTypes != "image/png" {
		t.Fatalf("response = %+v", response)
	}
}
