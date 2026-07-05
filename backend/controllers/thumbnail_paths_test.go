package controllers

import "testing"

func TestLocalProxyPathMapsNewThumbnailPathToData(t *testing.T) {
	got := localProxyPath("/thumbnails/2026/07/photo.webp")
	want := "data/thumbnails/2026/07/photo.webp"
	if got != want && got != "./"+want {
		t.Fatalf("localProxyPath() = %q, want %q", got, want)
	}
}

func TestDefaultStorageThumbnailFilePathSupportsNewAndLegacyPaths(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "new thumbnail path",
			path: "/thumbnails/2026/07/photo.webp",
			want: "data/thumbnails/2026/07/photo.webp",
		},
		{
			name: "legacy thumbnail path",
			path: "/uploads/2026/07/thumbnails/photo.webp",
			want: "uploads/2026/07/thumbnails/photo.webp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := defaultStorageThumbnailFilePath(tt.path)
			if got != tt.want && got != "./"+tt.want {
				t.Fatalf("defaultStorageThumbnailFilePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsThumbnailPathSupportsNewAndLegacyPaths(t *testing.T) {
	if !isThumbnailPath("/thumbnails/2026/07/photo.webp") {
		t.Fatal("isThumbnailPath() should match new thumbnail path")
	}
	if !isThumbnailPath("/uploads/2026/07/thumbnails/photo.webp") {
		t.Fatal("isThumbnailPath() should match legacy thumbnail path")
	}
	if isThumbnailPath("/uploads/2026/07/photo.webp") {
		t.Fatal("isThumbnailPath() should not match main image path")
	}
}
