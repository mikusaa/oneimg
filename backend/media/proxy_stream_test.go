package media

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"oneimg/backend/models"
)

func TestMediaRanges(t *testing.T) {
	gin.SetMode(gin.TestMode)
	data := []byte("0123456789abcdefghij")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"test"`)
		http.ServeContent(w, r, "sample.mp4", time.Time{}, bytes.NewReader(data))
	}))
	defer upstream.Close()
	client := awss3.NewFromConfig(aws.Config{Region: "us-east-1", Credentials: credentials.NewStaticCredentialsProvider("test", "test", "")}, func(o *awss3.Options) { o.BaseEndpoint = aws.String(upstream.URL); o.UsePathStyle = true })
	t.Chdir(t.TempDir())
	if err := os.WriteFile("sample.mp4", data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, storage := range []string{"default", "s3", "r2", "webdav"} {
		t.Run(storage, func(t *testing.T) {
			for _, tt := range []struct {
				method, byteRange  string
				status             int
				body, contentRange string
			}{
				{"GET", "", 200, string(data), ""}, {"GET", "bytes=3-7", 206, "34567", "bytes 3-7/20"},
				{"GET", "bytes=16-", 206, "ghij", "bytes 16-19/20"}, {"GET", "bytes=-4", 206, "ghij", "bytes 16-19/20"},
				{"GET", "bytes=20-", 416, "", "bytes */20"}, {"GET", "bytes=8-3", 416, "", "bytes */20"},
				{"GET", "bytes=0-1,5-6", 200, string(data), ""}, {"HEAD", "bytes=2-5", 200, "", ""},
			} {
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				c.Request = httptest.NewRequest(tt.method, "/sample.mp4", nil)
				c.Request.Header.Set("Range", tt.byteRange)
				// Local ServeContent supports multipart ranges by default; the public proxy normalizes these below.
				switch storage {
				case "default":
					proxyLocalFile(c, "sample.mp4", "video/mp4")
				case "s3":
					proxyS3File(c, "sample.mp4", "video/mp4", 999, models.Buckets{Type: "s3", Config: map[string]any{"s3_bucket": "test"}}, client)
				case "r2":
					proxyR2File(c, "sample.mp4", "video/mp4", 999, models.Buckets{Type: "r2", Config: map[string]any{"r2_bucket": "test"}}, client)
				case "webdav":
					proxyWebDAVFile(c, "sample.mp4", "video/mp4", 999, models.Buckets{Type: "webdav", Config: map[string]any{"webdav_url": upstream.URL}})
				}
				c.Writer.WriteHeaderNow()
				expectedRange := tt.contentRange
				if storage == "webdav" && tt.byteRange == "bytes=8-3" {
					expectedRange = ""
				} // Preserve upstream invalid-range response.
				if recorder.Code != tt.status || recorder.Header().Get("Content-Range") != expectedRange {
					t.Fatalf("%s %s: status %d headers %v body %s", tt.method, tt.byteRange, recorder.Code, recorder.Header(), recorder.Body.String())
				}
				if tt.status != 416 && recorder.Body.String() != tt.body {
					t.Fatalf("bad body %s %s: %q", tt.method, tt.byteRange, recorder.Body.String())
				}
				if tt.method == "HEAD" && recorder.Header().Get("Content-Length") != "20" {
					t.Fatal("HEAD length", recorder.Header())
				}
			}
		})
	}
}

func TestWebDAVRangeFallbackAndCancellation(t *testing.T) {
	canceled := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/blocked" {
			w.WriteHeader(200)
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			close(canceled)
			return
		}
		w.Header().Set("Content-Length", "10")
		io.WriteString(w, "0123456789")
	}))
	defer upstream.Close()
	bucket := models.Buckets{Type: "webdav", Config: map[string]any{"webdav_url": upstream.URL}}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/sample", nil)
	c.Request.Header.Set("Range", "bytes=2-4")
	proxyWebDAVFile(c, "sample", "video/mp4", 1000, bucket)
	if recorder.Code != 200 || recorder.Body.String() != "0123456789" || recorder.Header().Get("Content-Range") != "" {
		t.Fatal("falsified range response")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/blocked", nil).WithContext(ctx)
	done := make(chan struct{})
	go func() { proxyWebDAVFile(c, "blocked", "video/mp4", 0, bucket); close(done) }()
	time.AfterFunc(30*time.Millisecond, cancel)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("proxy did not cancel")
	}
	select {
	case <-canceled:
	case <-time.After(2 * time.Second):
		t.Fatal("upstream did not cancel")
	}
}
