package publicurl

import (
	"fmt"
	"net/url"
	"strings"

	"oneimg/backend/models"
)

func NormalizeDomain(value string) (string, error) {
	domain := strings.TrimSpace(value)
	if domain == "" {
		return "", nil
	}

	if !strings.Contains(domain, "://") {
		domain = "https://" + domain
	}

	parsed, err := url.Parse(domain)
	if err != nil {
		return "", fmt.Errorf("访问域名格式错误")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("访问域名仅支持 http 或 https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("访问域名缺少主机名")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("访问域名不能包含查询参数或锚点")
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return strings.TrimRight(parsed.String(), "/"), nil
}

// ConfigKey 是 S3/R2 存储配置里的直链域名字段。域名属于存储本身，与系统默认存储无关。
const ConfigKey = "public_domain"

// BucketDomain 返回存储配置的直链域名；不支持直链的类型或格式错误时返回空字符串。
func BucketDomain(bucket models.Buckets) string {
	if !SupportsStorage(bucket.Type) {
		return ""
	}
	value, _ := bucket.Config[ConfigKey].(string)
	domain, err := NormalizeDomain(value)
	if err != nil {
		return ""
	}
	return domain
}

// BuildForStorage 为 S3/R2 文件拼接所在存储的直链域名；域名为空或其他存储类型时返回经 OneImg 的路径。
func BuildForStorage(storage string, domain string, imagePath string) string {
	path := EncodeMediaPath(imagePath)
	if !SupportsStorage(storage) || path == "" || isAbsoluteURL(path) {
		return path
	}
	domain, err := NormalizeDomain(domain)
	if err != nil || domain == "" {
		return path
	}
	return domain + path
}

// Database paths are physical names, not URL syntax. Encode once at the DTO boundary.
func EncodeMediaPath(value string) string {
	if value == "" {
		return ""
	}
	if isAbsoluteURL(value) {
		u, err := url.Parse(value)
		if err != nil || u.Host == "" || u.User != nil {
			return ""
		}
		return u.String()
	}
	if parsed, err := url.Parse(value); err == nil && parsed.Scheme != "" {
		return ""
	}
	path := "/" + strings.TrimLeft(value, "/")
	return (&url.URL{Path: path}).String()
}

func BuildCDNForStorage(setting models.Settings, storage string, imagePath string) string {
	path := strings.TrimSpace(imagePath)
	if storage != "default" || path == "" || isAbsoluteURL(path) {
		return imagePath
	}

	domain, err := NormalizeDomain(setting.CDNDomain)
	if err != nil || domain == "" {
		return imagePath
	}

	cdnPath := strings.TrimPrefix(path, "/")
	if !strings.HasPrefix(cdnPath, "uploads/") {
		return imagePath
	}
	cdnPath = strings.TrimPrefix(cdnPath, "uploads/")
	if cdnPath == "" {
		return imagePath
	}

	return strings.TrimRight(domain, "/") + "/" + cdnPath
}

func SupportsStorage(storage string) bool {
	return storage == "r2" || storage == "s3"
}

func isAbsoluteURL(value string) bool {
	value = strings.ToLower(value)
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}
