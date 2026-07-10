package localimport

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/chai2010/webp"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"oneimg/backend/models"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&models.Image{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	return db
}

func solidImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 24, 18))
	for y := 0; y < 18; y++ {
		for x := 0; x < 24; x++ {
			img.Set(x, y, color.RGBA{R: 100, G: 150, B: 200, A: 255})
		}
	}
	return img
}

func encodePNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, solidImage()); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	return buf.Bytes()
}

func encodeJPEG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, solidImage(), &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("jpeg.Encode() error = %v", err)
	}
	return buf.Bytes()
}

func encodeWebP(t *testing.T) []byte {
	t.Helper()
	data, err := webp.EncodeRGBA(solidImage(), 80)
	if err != nil {
		t.Fatalf("webp.EncodeRGBA() error = %v", err)
	}
	return data
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func quietOptions(root, dataRoot string) Options {
	return Options{
		Root:     root,
		DataRoot: dataRoot,
		BucketID: 1,
		UserID:   1,
		Username: "admin",
		Logger:   log.New(ioDiscard{}, "", 0),
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}

func TestImageURLFromPath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "uploads")
	path := filepath.Join(root, "2026", "07", "a.jpg")

	got, err := imageURLFromPath(root, path)
	if err != nil {
		t.Fatalf("imageURLFromPath() error = %v", err)
	}
	if got != "/uploads/2026/07/a.jpg" {
		t.Fatalf("imageURLFromPath() = %q, want /uploads/2026/07/a.jpg", got)
	}
}

func TestImportSkipsExistingImageURL(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "uploads")
	path := filepath.Join(root, "2026", "07", "a.jpg")
	writeFile(t, path, []byte{})

	db := testDB(t)
	if err := db.Create(&models.Image{
		Url:      "/uploads/2026/07/a.jpg",
		FileName: "a.jpg",
		FileSize: 1,
		BucketId: 1,
		UserId:   1,
		Storage:  "default",
		UUID:     "admin",
	}).Error; err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	summary, err := NewImporter(db, quietOptions(root, filepath.Join(tmp, "data"))).Run()
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if summary.SkippedExisting != 1 || summary.Imported != 0 {
		t.Fatalf("summary = %+v, want skipped existing only", summary)
	}

	var count int64
	db.Model(&models.Image{}).Count(&count)
	if count != 1 {
		t.Fatalf("image count = %d, want 1", count)
	}
}

func TestImportOrdinaryImagesGenerateWebPThumbnails(t *testing.T) {
	tests := []struct {
		name string
		file string
		data []byte
	}{
		{name: "png", file: "a.png", data: encodePNG(t)},
		{name: "jpeg", file: "b.jpg", data: encodeJPEG(t)},
		{name: "webp", file: "c.webp", data: encodeWebP(t)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			root := filepath.Join(tmp, "uploads")
			dataRoot := filepath.Join(tmp, "data")
			writeFile(t, filepath.Join(root, "2026", "07", tt.file), tt.data)

			db := testDB(t)
			summary, err := NewImporter(db, quietOptions(root, dataRoot)).Run()
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if summary.Imported != 1 || summary.Failed != 0 {
				t.Fatalf("summary = %+v, want one imported", summary)
			}

			var imageModel models.Image
			if err := db.First(&imageModel).Error; err != nil {
				t.Fatalf("First() error = %v", err)
			}
			if imageModel.Thumbnail == "" {
				t.Fatal("thumbnail should not be empty")
			}
			thumbPath := filepath.Join(dataRoot, filepath.FromSlash(imageModel.Thumbnail))
			thumbBytes, err := os.ReadFile(thumbPath)
			if err != nil {
				t.Fatalf("ReadFile(thumbnail) error = %v", err)
			}
			if _, err := webp.Decode(bytes.NewReader(thumbBytes)); err != nil {
				t.Fatalf("thumbnail should be webp: %v", err)
			}
		})
	}
}

func TestImportSVGAndUndecodableWebPHaveNoThumbnail(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "uploads")
	writeFile(t, filepath.Join(root, "2026", "07", "a.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"></svg>`))
	writeFile(t, filepath.Join(root, "2026", "07", "bad.webp"), []byte("not a decodable webp"))

	db := testDB(t)
	summary, err := NewImporter(db, quietOptions(root, filepath.Join(tmp, "data"))).Run()
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if summary.Imported != 2 || summary.Failed != 0 {
		t.Fatalf("summary = %+v, want two imported", summary)
	}

	var images []models.Image
	if err := db.Order("file_name").Find(&images).Error; err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	for _, imageModel := range images {
		if imageModel.Thumbnail != "" {
			t.Fatalf("%s thumbnail = %q, want empty", imageModel.FileName, imageModel.Thumbnail)
		}
		if imageModel.Width != 0 || imageModel.Height != 0 {
			t.Fatalf("%s size = %dx%d, want 0x0", imageModel.FileName, imageModel.Width, imageModel.Height)
		}
	}
}

func TestImportIgnoresThumbnailDirectories(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "uploads")
	writeFile(t, filepath.Join(root, "2026", "07", "thumbnails", "a.png"), encodePNG(t))

	db := testDB(t)
	summary, err := NewImporter(db, quietOptions(root, filepath.Join(tmp, "data"))).Run()
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if summary.Scanned != 0 || summary.Imported != 0 || summary.SkippedIgnored != 1 {
		t.Fatalf("summary = %+v, want ignored thumbnail directory", summary)
	}
}
