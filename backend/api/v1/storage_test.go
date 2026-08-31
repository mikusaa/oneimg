package v1

import (
	"encoding/json"
	"strings"
	"testing"

	"oneimg/backend/models"
	"oneimg/backend/services"
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
