package services

import (
	"oneimg/backend/models"
	"testing"
	"time"
)

func TestVideoStatsAndFilters(t *testing.T) {
	db := newServiceTestDB(t)
	admin := createServiceTestUser(t, db, "media-admin", "correct-password", models.RoleAdmin)
	alice := createServiceTestUser(t, db, "media-alice", "correct-password", models.RoleUser)
	bob := createServiceTestUser(t, db, "media-bob", "correct-password", models.RoleUser)
	if err := db.Create(&models.Settings{}).Error; err != nil {
		t.Fatal(err)
	}
	location := time.FixedZone("Asia/Shanghai", 8*3600)
	now := time.Date(2026, 9, 1, 10, 0, 0, 0, location)
	day := localDay(now)
	records := []models.Image{
		{FileName: "legacy.png", FileSize: 11, UserId: alice.ID, CreatedAt: day.Add(-time.Second)},
		{FileName: "image.png", FileSize: 22, MimeType: "image/png", UserId: alice.ID, CreatedAt: day},
		{FileName: "video.mp4", FileSize: 33, MimeType: "video/mp4", UserId: alice.ID, CreatedAt: day},
		{FileName: "video.webm", FileSize: 44, MimeType: "video/webm", UserId: bob.ID, CreatedAt: now},
	}
	if err := db.Create(&records).Error; err != nil {
		t.Fatal(err)
	}
	service := NewStatsService(db)
	service.now = func() time.Time { return now }
	for _, tt := range []struct {
		user                                                           models.User
		images, videos, imageSize, videoSize, todayImages, todayVideos int64
	}{
		{admin, 2, 2, 33, 77, 1, 2}, {alice, 2, 1, 33, 33, 1, 1}, {bob, 0, 1, 0, 44, 0, 1},
	} {
		got, err := service.Dashboard(tt.user)
		if err != nil {
			t.Fatal(err)
		}
		if got.TotalImages != tt.images || got.TotalVideos != tt.videos || got.ImageSize != tt.imageSize || got.VideoSize != tt.videoSize || got.TotalSize != tt.imageSize+tt.videoSize || got.TodayImageUploads != tt.todayImages || got.TodayVideoUploads != tt.todayVideos || got.MonthImageUploads != tt.todayImages || got.MonthVideoUploads != tt.todayVideos || got.TodayUploads != tt.todayImages+tt.todayVideos || got.MonthUploads != got.TodayUploads {
			t.Fatalf("bad stats for %s: %+v", tt.user.Username, got)
		}
		for _, period := range []string{"day", "week", "month", "year"} {
			for _, point := range service.ImageTrend(tt.user, period) {
				if point.Count != point.ImageCount+point.VideoCount {
					t.Fatalf("bad trend %+v", point)
				}
			}
		}
		last := got.UploadTrend[len(got.UploadTrend)-1]
		if last.ImageCount != tt.todayImages || last.VideoCount != tt.todayVideos {
			t.Fatalf("bad last day %+v", last)
		}
		for _, kind := range []string{"image", "video"} {
			items, total, err := NewImageService(db).List(ImageListQuery{User: tt.user, Page: 1, PageSize: 1, Sort: "created_at", Order: "desc", MediaType: kind})
			if err != nil {
				t.Fatal(err)
			}
			want := tt.images
			if kind == "video" {
				want = tt.videos
			}
			if total != want || (want > 0 && len(items) != 1) {
				t.Fatalf("bad filter %s: %d, %d", kind, total, len(items))
			}
		}
	}
	if err := db.Delete(&records[2]).Error; err != nil {
		t.Fatal(err)
	}
	got, err := service.Dashboard(alice)
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalVideos != 0 || got.VideoSize != 0 || got.TotalImages != 2 {
		t.Fatalf("deletion not reflected %+v", got)
	}
	if err := db.Where("1=1").Delete(&models.Image{}).Error; err != nil {
		t.Fatal(err)
	}
	got, err = service.Dashboard(admin)
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalImages+got.TotalVideos+got.TotalSize+got.TodayUploads != 0 {
		t.Fatalf("empty stats %+v", got)
	}
}

func TestVideoSettingsDefaultsAndValidation(t *testing.T) {
	db := newServiceTestDB(t)
	settings := models.Settings{}
	if err := db.Create(&settings).Error; err != nil {
		t.Fatal(err)
	}
	if settings.VideoUploadEnabled || settings.MaxVideoFileSize != 100<<20 {
		t.Fatalf("bad defaults %+v", settings)
	}
	svc := NewSettingsService(db)
	for _, size := range []int{0, (1 << 20) - 1, (1 << 30) + 1} {
		if _, err := svc.Update(SettingsPatch{MaxVideoFileSize: &size}); err == nil {
			t.Fatal("accepted invalid size", size)
		}
	}
	enabled := true
	size := 1 << 20
	got, err := svc.Update(SettingsPatch{VideoUploadEnabled: &enabled, MaxVideoFileSize: &size})
	if err != nil || !got.VideoUploadEnabled || got.MaxVideoFileSize != size {
		t.Fatal(got, err)
	}
	enabled = false
	got, err = svc.Update(SettingsPatch{VideoUploadEnabled: &enabled})
	if err != nil || got.VideoUploadEnabled {
		t.Fatal(got, err)
	}
}
