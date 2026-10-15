package media

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"oneimg/backend/models"
)

func TestRemoteSVGMIMEProtectsNonSVGExtension(t *testing.T) {
	data := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>window.pwned=1</script></svg>`)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml ; charset=utf-8")
		http.ServeContent(w, r, "legacy.png", time.Time{}, bytes.NewReader(data))
	}))
	defer upstream.Close()
	client := awss3.NewFromConfig(aws.Config{Region: "us-east-1", Credentials: credentials.NewStaticCredentialsProvider("test", "test-only", "")}, func(o *awss3.Options) {
		o.BaseEndpoint, o.UsePathStyle = aws.String(upstream.URL), true
	})
	for _, storage := range []string{"s3", "r2", "webdav"} {
		cases := []struct {
			method, byteRange string
			status            int
		}{{"GET", "", 200}, {"HEAD", "", 200}, {"GET", "bytes=0-9", 206}}
		// Object HEAD preserves MIME before range validation; WebDAV errors may omit it.
		if storage != "webdav" {
			cases = append(cases, struct {
				method, byteRange string
				status            int
			}{"GET", "bytes=9999-", 416})
		}
		for _, test := range cases {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(test.method, "/legacy.png", nil)
			c.Request.Header.Set("Range", test.byteRange)
			switch storage {
			case "s3":
				proxyS3File(c, "legacy.png", "image/png", 0, models.Buckets{Type: "s3", Config: map[string]any{"s3_bucket": "test"}}, client)
			case "r2":
				proxyR2File(c, "legacy.png", "image/png", 0, models.Buckets{Type: "r2", Config: map[string]any{"r2_bucket": "test"}}, client)
			case "webdav":
				proxyWebDAVFile(c, "legacy.png", "image/png", 0, models.Buckets{Type: "webdav", Config: map[string]any{"webdav_url": upstream.URL}})
			}
			c.Writer.WriteHeaderNow()
			if w.Code != test.status || w.Header().Get("Content-Type") != "image/svg+xml" || w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("Content-Security-Policy") == "" {
				t.Fatal(storage, test, w.Code, w.Header())
			}
		}
	}
}

func TestLegacySVGPolicyOnGETHEADRangeAndValidation(t *testing.T) {
	t.Chdir(t.TempDir())
	data := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect width="10" height="10"/><script>window.pwned=1</script></svg>`)
	if err := os.WriteFile("legacy.svg", data, 0600); err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"svg"`)
		http.ServeContent(w, r, "legacy.svg", time.Unix(1700000000, 0), bytes.NewReader(data))
	}))
	defer upstream.Close()
	for _, storage := range []string{"default", "webdav"} {
		for _, test := range []struct {
			method, rangeValue, condition string
			status                        int
		}{
			{"GET", "", "", 200}, {"HEAD", "", "", 200}, {"GET", "bytes=0-9", "", 206}, {"GET", "bytes=9999-", "", 416}, {"GET", "", "Wed, 21 Oct 2037 07:28:00 GMT", 304},
		} {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(test.method, "/legacy.svg", nil)
			c.Request.Header.Set("Range", test.rangeValue)
			c.Request.Header.Set("If-Modified-Since", test.condition)
			if storage == "default" {
				proxyLocalFile(c, "legacy.svg", "image/webp")
			} else {
				if test.condition != "" {
					c.Request.Header.Set("If-None-Match", `"svg"`)
				}
				proxyWebDAVFile(c, "legacy.svg", "image/webp", int64(len(data)), models.Buckets{Type: "webdav", Config: map[string]any{"webdav_url": upstream.URL}})
			}
			c.Writer.WriteHeaderNow()
			// WebDAV forwards cache validators below, preserving the upstream status.
			if w.Code != test.status {
				t.Fatal(storage, test, w.Code)
			}
			h := w.Header()
			if h.Get("Content-Type") != "image/svg+xml" || h.Get("X-Content-Type-Options") != "nosniff" || !strings.HasPrefix(h.Get("Content-Security-Policy"), "sandbox; default-src 'none'") || h.Get("Cache-Control") != "no-cache, max-age=0, must-revalidate" {
				t.Fatal(h)
			}
		}
	}
}
