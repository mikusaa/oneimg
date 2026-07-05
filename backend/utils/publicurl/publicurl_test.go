package publicurl

import (
	"testing"

	"oneimg/backend/models"
)

func TestBuildForStorageOnlyRewritesDefaultSupportedBucket(t *testing.T) {
	setting := models.Settings{
		DefaultStorage:    2,
		PublicImageDomain: "img.example.com",
	}

	tests := []struct {
		name    string
		storage string
		bucket  int
		want    string
	}{
		{
			name:    "default supported bucket",
			storage: "s3",
			bucket:  2,
			want:    "https://img.example.com/uploads/a.webp",
		},
		{
			name:    "same storage different bucket",
			storage: "s3",
			bucket:  8,
			want:    "/uploads/a.webp",
		},
		{
			name:    "unsupported storage default bucket",
			storage: "default",
			bucket:  2,
			want:    "/uploads/a.webp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildForStorage(setting, tt.storage, tt.bucket, "/uploads/a.webp")
			if got != tt.want {
				t.Fatalf("BuildForStorage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildCDNForStorageOnlyRewritesDefaultStorage(t *testing.T) {
	setting := models.Settings{
		CDNDomain: "http://localhost:8081",
	}

	tests := []struct {
		name    string
		storage string
		path    string
		want    string
	}{
		{
			name:    "default storage strips uploads prefix",
			storage: "default",
			path:    "/uploads/2026/07/a.webp",
			want:    "http://localhost:8081/2026/07/a.webp",
		},
		{
			name:    "default storage strips uploads prefix without leading slash",
			storage: "default",
			path:    "uploads/2026/07/a.webp",
			want:    "http://localhost:8081/2026/07/a.webp",
		},
		{
			name:    "non default storage is unchanged",
			storage: "s3",
			path:    "/uploads/2026/07/a.webp",
			want:    "/uploads/2026/07/a.webp",
		},
		{
			name:    "absolute url is unchanged",
			storage: "default",
			path:    "https://example.com/uploads/2026/07/a.webp",
			want:    "https://example.com/uploads/2026/07/a.webp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildCDNForStorage(setting, tt.storage, tt.path)
			if got != tt.want {
				t.Fatalf("BuildCDNForStorage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildCDNForStorageWithoutDomainIsUnchanged(t *testing.T) {
	got := BuildCDNForStorage(models.Settings{}, "default", "/uploads/2026/07/a.webp")
	want := "/uploads/2026/07/a.webp"
	if got != want {
		t.Fatalf("BuildCDNForStorage() = %q, want %q", got, want)
	}
}
