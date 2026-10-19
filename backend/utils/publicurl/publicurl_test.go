package publicurl

import (
	"net/url"
	"testing"

	"oneimg/backend/models"
)

func TestHistoricalPathDataIsNotURLSyntax(t *testing.T) {
	raw := `/uploads/photo"#?%2F.png`
	got := BuildForStorage("default", "", raw)
	u, err := url.Parse(got)
	if err != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != raw {
		t.Fatal(got, err)
	}
	if EncodeMediaPath("javascript:alert(1)") != "" {
		t.Fatal("active scheme accepted")
	}
}

func TestBuildForStorageUsesEachBucketDomain(t *testing.T) {
	tests := []struct {
		name    string
		storage string
		domain  string
		want    string
	}{
		{name: "s3 with domain", storage: "s3", domain: "img.example.com", want: "https://img.example.com/uploads/a.webp"},
		{name: "r2 with domain path", storage: "r2", domain: "https://cdn.example.com/media/", want: "https://cdn.example.com/media/uploads/a.webp"},
		{name: "s3 without domain", storage: "s3", domain: "", want: "/uploads/a.webp"},
		{name: "invalid domain falls back", storage: "s3", domain: "ftp://bad.example.com", want: "/uploads/a.webp"},
		{name: "ftp ignores domain", storage: "ftp", domain: "img.example.com", want: "/uploads/a.webp"},
		{name: "local ignores domain", storage: "default", domain: "img.example.com", want: "/uploads/a.webp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildForStorage(tt.storage, tt.domain, "/uploads/a.webp")
			if got != tt.want {
				t.Fatalf("BuildForStorage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBucketDomainOnlyForDirectStorage(t *testing.T) {
	r2 := models.Buckets{Type: "r2", Config: map[string]any{ConfigKey: "img.example.com"}}
	if got := BucketDomain(r2); got != "https://img.example.com" {
		t.Fatalf("BucketDomain(r2) = %q", got)
	}
	webdav := models.Buckets{Type: "webdav", Config: map[string]any{ConfigKey: "img.example.com"}}
	if got := BucketDomain(webdav); got != "" {
		t.Fatalf("BucketDomain(webdav) = %q, want empty", got)
	}
	nonString := models.Buckets{Type: "s3", Config: map[string]any{ConfigKey: 42}}
	if got := BucketDomain(nonString); got != "" {
		t.Fatalf("BucketDomain(non-string) = %q, want empty", got)
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
