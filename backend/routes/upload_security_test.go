package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"oneimg/backend/config"
	"oneimg/backend/models"
	"oneimg/backend/services"
	"oneimg/backend/utils/images"
)

func TestPicGoMultipartUploadKeepsResponseDeduplicationAndPublicReads(t *testing.T) {
	t.Chdir(t.TempDir())
	images.InitImageService()
	router, system := setupTestRouter(t, &config.Config{SqlitePath: filepath.Join(t.TempDir(), "db"), SessionSecret: "test", ConfigSecret: "test"})
	user := models.User{Username: "picgo-test", Role: models.RoleAdmin}
	if err := system.Database.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := system.Database.DB.Create(&models.Buckets{Id: 1, Type: "default"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := system.Database.DB.Model(&models.Settings{}).Where("id = ?", 1).UpdateColumn("original_image", true).Error; err != nil {
		t.Fatal(err)
	}
	tag := models.Tags{Name: "picgo"}
	if err := system.Database.DB.Create(&tag).Error; err != nil {
		t.Fatal(err)
	}
	token, err := system.Services.Tokens.Create(user.ID, services.CreateTokenInput{AuthVersion: user.AuthVersion, Name: "picgo", Scopes: []string{"images:write"}})
	if err != nil {
		t.Fatal(err)
	}
	var encoded, body bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 24, 16))); err != nil {
		t.Fatal(err)
	}
	writer := multipart.NewWriter(&body)
	part, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": {`form-data; name="images"; filename="normal.png"`},
		"Content-Type":        {"image/png"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(encoded.Bytes()); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{"bucket_id": "1", "tag_ids": strconv.Itoa(tag.Id)} {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	var imageID int
	var publicURL string
	for attempt := 0; attempt < 2; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/images", bytes.NewReader(body.Bytes()))
		request.Header.Set("Content-Type", writer.FormDataContentType())
		request.Header.Set("Authorization", "Bearer "+token.Plain)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		var envelope struct {
			Data struct {
				Files []struct {
					Success   bool `json:"success"`
					Duplicate bool `json:"duplicate"`
					Image     struct {
						ID     int    `json:"id"`
						URL    string `json:"url"`
						Width  int    `json:"width"`
						Height int    `json:"height"`
					} `json:"image"`
				} `json:"files"`
			} `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusOK || len(envelope.Data.Files) != 1 {
			t.Fatal(response.Code, response.Body.String())
		}
		file := envelope.Data.Files[0]
		if !file.Success || file.Duplicate != (attempt == 1) || file.Image.Width != 24 || file.Image.Height != 16 {
			t.Fatal(response.Body.String())
		}
		if attempt == 0 {
			imageID, publicURL = file.Image.ID, file.Image.URL
		} else if file.Image.ID != imageID || file.Image.URL != publicURL {
			t.Fatal("duplicate upload changed the object identity")
		}
	}
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(method, publicURL, nil))
		if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/png" {
			t.Fatal(method, response.Code, response.Header())
		}
		if method == http.MethodGet && !bytes.Equal(response.Body.Bytes(), encoded.Bytes()) {
			t.Fatal("original-image upload did not preserve the bytes")
		}
		if method == http.MethodHead && response.Body.Len() != 0 {
			t.Fatal("HEAD returned a response body")
		}
	}
}

type spoolObserver struct {
	io.Reader
	temp    string
	spooled bool
}

func (r *spoolObserver) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	matches, _ := filepath.Glob(filepath.Join(r.temp, "multipart-*"))
	r.spooled = r.spooled || len(matches) > 0
	return n, err
}

func TestBusyMultipartSpoolsFilesInsteadOfRetainingImageBuffers(t *testing.T) {
	temp := t.TempDir()
	t.Setenv("TMPDIR", temp)
	router, system := setupTestRouter(t, &config.Config{SqlitePath: filepath.Join(t.TempDir(), "db"), SessionSecret: "test", ConfigSecret: "test"})
	hash, _ := bcrypt.GenerateFromPassword([]byte("test-only-password"), bcrypt.MinCost)
	user := models.User{Username: "memory-test", Password: string(hash), Role: models.RoleAdmin}
	if err := system.Database.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := system.Database.DB.Create(&models.Buckets{Id: 1, Type: "default"}).Error; err != nil {
		t.Fatal(err)
	}
	token, err := system.Services.Tokens.Create(user.ID, services.CreateTokenInput{AuthVersion: user.AuthVersion, Name: "test", Scopes: []string{"images:write"}})
	if err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": {`form-data; name="images"; filename="large.png"`},
		"Content-Type":        {"image/png"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(make([]byte, 2<<20)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	_, release, err := images.AcquireProcessing(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	observer := &spoolObserver{Reader: bytes.NewReader(body.Bytes()), temp: temp}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/images", observer)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token.Plain)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "image_processing_busy") {
		t.Fatal(response.Code, response.Body.String())
	}
	if !observer.spooled {
		t.Fatal("encoded image was fully retained before acquiring a processing slot")
	}
	matches, _ := filepath.Glob(filepath.Join(temp, "multipart-*"))
	if len(matches) != 0 {
		t.Fatal("multipart temporary files were not cleaned")
	}
}
