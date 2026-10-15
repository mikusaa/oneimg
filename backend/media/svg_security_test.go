package media

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"oneimg/backend/models"
)

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
			{"GET", "", "", 200}, {"HEAD", "", "", 200}, {"GET", "bytes=0-9", "", 206}, {"GET", "", "Wed, 21 Oct 2037 07:28:00 GMT", 304},
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
