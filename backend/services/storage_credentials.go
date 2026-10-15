package services

import (
	"net/url"
	"strings"

	"oneimg/backend/models"
	"oneimg/backend/utils/secureconfig"
)

// Both updates and connection tests use the same validation-to-secret-use boundary.
func mergeStorageConfig(existing models.Buckets, supplied map[string]any) (map[string]any, error) {
	old, err := secureconfig.DecryptBucketConfigValues(existing.Config)
	if err != nil {
		return nil, err
	}
	merged := make(map[string]any, len(old)+len(supplied))
	for key, value := range old {
		merged[key] = value
	}
	for key, value := range supplied {
		if secureconfig.IsBucketSensitiveKey(key) && (value == nil || strings.TrimSpace(secureconfig.GetString(supplied, key)) == "") {
			continue
		}
		merged[key] = value
	}
	keys := map[string][]string{
		"webdav": {"webdav_url", "webdav_user"},
		"ftp":    {"ftp_host", "ftp_port", "ftp_user"},
		"s3":     {"s3_endpoint", "s3_bucket", "s3_access_key"},
		"r2":     {"r2_endpoint", "r2_bucket", "r2_access_key"},
	}
	changed := false
	for _, key := range keys[existing.Type] {
		left, right := secureconfig.GetString(old, key), secureconfig.GetString(merged, key)
		if key == "webdav_url" || strings.HasSuffix(key, "_endpoint") {
			left, right = normalizedEndpoint(left), normalizedEndpoint(right)
		}
		if left != right {
			changed = true
		}
	}
	secretKey := map[string]string{"webdav": "webdav_pass", "ftp": "ftp_pass", "s3": "s3_secret_key", "r2": "r2_secret_key"}[existing.Type]
	if changed && strings.TrimSpace(secureconfig.GetString(supplied, secretKey)) == "" {
		return nil, ErrNewEndpointCredentials
	}
	return merged, nil
}

func normalizedEndpoint(value string) string {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return value
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.Path, "/")
	return u.String()
}
