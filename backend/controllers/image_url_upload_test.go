package controllers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"oneimg/backend/database"
	"oneimg/backend/models"

	"github.com/gin-gonic/gin"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestURLUploadRequestAcceptsNumericAndStringIDs(t *testing.T) {
	tests := []struct {
		name       string
		payload    string
		wantTag    int
		wantBucket int
	}{
		{name: "numeric", payload: `{"url":"https://example.com/a.png","tag_id":2,"bucket_id":3}`, wantTag: 2, wantBucket: 3},
		{name: "string", payload: `{"url":"https://example.com/a.png","tag_id":"2","bucket_id":"3"}`, wantTag: 2, wantBucket: 3},
		{name: "empty", payload: `{"url":"https://example.com/a.png","tag_id":"","bucket_id":null}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var request urlUploadRequest
			if err := json.Unmarshal([]byte(tt.payload), &request); err != nil {
				t.Fatalf("unmarshal request: %v", err)
			}
			if int(request.TagID) != tt.wantTag || int(request.BucketID) != tt.wantBucket {
				t.Fatalf("ids = %d/%d, want %d/%d", request.TagID, request.BucketID, tt.wantTag, tt.wantBucket)
			}
		})
	}
}

func TestNewRemoteImageRequestSetsPixivReferer(t *testing.T) {
	request, err := newRemoteImageRequest(context.Background(), "https://i.pximg.net/image.png")
	if err != nil {
		t.Fatal(err)
	}
	if got := request.Header.Get("Referer"); got != "https://www.pixiv.net/" {
		t.Fatalf("Referer = %q", got)
	}

	request, err = newRemoteImageRequest(context.Background(), "https://example.com/image.png")
	if err != nil {
		t.Fatal(err)
	}
	if got := request.Header.Get("Referer"); got != "" {
		t.Fatalf("unexpected Referer = %q", got)
	}
}

func TestURLUploadStoresOriginalFileSize(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupUploadBatchTest(t)

	pngBytes, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	originalClient := remoteImageHTTPClient
	remoteImageHTTPClient = &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/png"}},
			Body:       io.NopCloser(bytes.NewReader(pngBytes)),
		}, nil
	})}
	t.Cleanup(func() { remoteImageHTTPClient = originalClient })

	payload, err := json.Marshal(map[string]any{"url": "https://example.test/remote.png", "bucket_id": 1})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/upload/url", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("user_id", 1)
	ctx.Set("user_role", models.RoleAdmin)
	UploadImagesByURL(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("URL upload status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Code int `json:"code"`
		Data struct {
			File struct {
				OriginalFileSize int64 `json:"original_file_size"`
			} `json:"file"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || response.Data.File.OriginalFileSize != int64(len(pngBytes)) {
		t.Fatalf("URL upload response = %+v, want original size %d", response, len(pngBytes))
	}

	var stored models.Image
	if err := database.GetDB().DB.First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.OriginalFileSize != int64(len(pngBytes)) {
		t.Fatalf("stored original_file_size = %d, want %d", stored.OriginalFileSize, len(pngBytes))
	}
}
