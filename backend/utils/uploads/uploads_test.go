package uploads

import "testing"

func TestBuildThumbnailPathUsesIndependentPrefix(t *testing.T) {
	tests := []struct {
		name     string
		subDir   string
		fileName string
		want     string
	}{
		{
			name:     "uploads path strips uploads prefix",
			subDir:   "uploads/2026/07",
			fileName: "photo.png",
			want:     "thumbnails/2026/07/photo.webp",
		},
		{
			name:     "leading slash path",
			subDir:   "/uploads/2026/07",
			fileName: "photo.webp",
			want:     "thumbnails/2026/07/photo.webp",
		},
		{
			name:     "custom path stays under thumbnails",
			subDir:   "custom/dir",
			fileName: "photo",
			want:     "thumbnails/custom/dir/photo.webp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildThumbnailPath(tt.subDir, tt.fileName); got != tt.want {
				t.Fatalf("buildThumbnailPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLocalThumbnailFilePathUsesDataDirectory(t *testing.T) {
	got := localThumbnailFilePath("/thumbnails/2026/07/photo.webp")
	want := "data/thumbnails/2026/07/photo.webp"
	if got != want && got != "./"+want {
		t.Fatalf("localThumbnailFilePath() = %q, want %q", got, want)
	}
}
