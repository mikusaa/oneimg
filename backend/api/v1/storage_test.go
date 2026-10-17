package v1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"oneimg/backend/models"
	"oneimg/backend/services"

	"github.com/gin-gonic/gin"
)

func TestStorageDTOIncludesExactnessAndOptionalFilesystem(t *testing.T) {
	local := toStorageDTO(services.StorageBucketSummary{
		Bucket:     models.Buckets{Id: 1, Name: "local", Type: "default", Capacity: 0},
		UsageBytes: 12, UsageExact: true,
		Filesystem: &services.FilesystemMetrics{TotalBytes: 100, UsedBytes: 40, AvailableBytes: 55},
	})
	encoded, err := json.Marshal(local)
	if err != nil {
		t.Fatal(err)
	}
	value := string(encoded)
	for _, expected := range []string{`"usage_bytes":12`, `"usage_exact":true`, `"filesystem"`, `"available_bytes":55`} {
		if !strings.Contains(value, expected) {
			t.Fatalf("response %s does not contain %s", value, expected)
		}
	}

	remote := toStorageDTO(services.StorageBucketSummary{
		Bucket:     models.Buckets{Id: 2, Name: "remote", Type: "s3", Capacity: 100},
		UsageBytes: 80, UsageExact: false,
	})
	encoded, err = json.Marshal(remote)
	if err != nil {
		t.Fatal(err)
	}
	value = string(encoded)
	if !strings.Contains(value, `"usage_exact":false`) || strings.Contains(value, `"filesystem"`) {
		t.Fatalf("unexpected remote response: %s", value)
	}
}

func TestHandleStorageErrorMapsDefaultStorageDeletion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{services.ErrDefaultStorage, http.StatusForbidden, "default_storage_protected"},
		{fmt.Errorf("wrapped: %w", services.ErrStorageInUseAsDefault), http.StatusConflict, "storage_is_upload_default"},
	} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/storage-buckets/2", nil)
		if !handleStorageError(c, tc.err) {
			t.Fatalf("%v was not handled", tc.err)
		}
		var body struct {
			Code   string `json:"code"`
			Detail string `json:"detail"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if recorder.Code != tc.status || body.Code != tc.code || body.Detail == "" {
			t.Fatalf("%v => %d %+v", tc.err, recorder.Code, body)
		}
	}
}

func TestDeleteStorageRejectsInvalidPurgeObjects(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, raw := range []string{"1", "yes", "TRUE", ""} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/storage-buckets/2?purge_objects="+raw, nil)
		c.Params = gin.Params{{Key: "id", Value: "2"}}
		// 参数校验先于服务调用，空 Server 不会触达存储。
		(&Server{}).deleteStorage(c)
		if recorder.Code != http.StatusUnprocessableEntity || !strings.Contains(recorder.Body.String(), "invalid_query_parameter") {
			t.Fatalf("purge_objects=%q => %d %s", raw, recorder.Code, recorder.Body.String())
		}
	}
}
