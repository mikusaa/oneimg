package storageobject

import (
	"path/filepath"
	"strings"
)

// LocalPath keeps writes, reads, rollback and reference checks on the same physical object.
func LocalPath(key string) string {
	clean := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(key)), "/")
	if strings.HasPrefix(clean, "thumbnails/") {
		return filepath.Join("data", filepath.FromSlash(clean))
	}
	return filepath.FromSlash(clean)
}
