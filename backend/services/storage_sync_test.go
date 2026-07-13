package services

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalLocalPathMapsSplitThumbnails(t *testing.T) {
	path, err := canonicalLocalPath("/thumbnails/2026/07/a.webp")
	if err != nil {
		t.Fatalf("canonicalLocalPath() error = %v", err)
	}

	expectedSuffix := filepath.Join("data", "thumbnails", "2026", "07", "a.webp")
	if !strings.HasSuffix(path, expectedSuffix) {
		t.Fatalf("canonicalLocalPath() = %q, want suffix %q", path, expectedSuffix)
	}
}

func TestCanonicalLocalPathRejectsEscapes(t *testing.T) {
	if _, err := canonicalLocalPath("/uploads/../../data.db"); err == nil {
		t.Fatal("canonicalLocalPath() should reject escaping paths")
	}
}
