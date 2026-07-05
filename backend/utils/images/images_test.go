package images

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/textproto"
	"testing"

	"oneimg/backend/models"
)

type readSeekCloser struct {
	*bytes.Reader
}

func (r readSeekCloser) Close() error {
	return nil
}

var _ multipart.File = readSeekCloser{}

func testPNGBytes(t *testing.T) ([]byte, image.Image) {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8((x*y + y) % 255),
				G: uint8((x*3 + y*5) % 255),
				B: uint8((x*7 + y*11) % 255),
				A: 255,
			})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	return buf.Bytes(), img
}

func TestProcessMainImageOriginalImageOverridesSaveWebP(t *testing.T) {
	svc := &ImageService{}
	fileBytes, img := testPNGBytes(t)

	gotBytes, gotFormat, gotMime, err := svc.processMainImage(fileBytes, img, "png", "image/png", int64(len(fileBytes)), models.Settings{
		OriginalImage:  true,
		SaveWebp:       true,
		MainImageQuality: 60,
	})
	if err != nil {
		t.Fatalf("processMainImage() error = %v", err)
	}
	if !bytes.Equal(gotBytes, fileBytes) {
		t.Fatal("processMainImage() should keep original bytes when original_image is enabled")
	}
	if gotFormat != "png" || gotMime != "image/png" {
		t.Fatalf("processMainImage() format/mime = %q/%q, want png/image/png", gotFormat, gotMime)
	}
}

func TestProcessMainImageSkipCompressFormatKeepsOriginal(t *testing.T) {
	svc := &ImageService{}
	fileBytes, img := testPNGBytes(t)

	gotBytes, gotFormat, gotMime, err := svc.processMainImage(fileBytes, img, "png", "image/png", int64(len(fileBytes)), models.Settings{
		SaveWebp:            true,
		MainImageQuality:    75,
		SkipCompressFormat: " PNG , image/svg+xml ",
	})
	if err != nil {
		t.Fatalf("processMainImage() error = %v", err)
	}
	if !bytes.Equal(gotBytes, fileBytes) {
		t.Fatal("processMainImage() should keep original bytes for skipped format")
	}
	if gotFormat != "png" || gotMime != "image/png" {
		t.Fatalf("processMainImage() format/mime = %q/%q, want png/image/png", gotFormat, gotMime)
	}
}

func TestProcessMainImageSaveWebPUsesConfiguredQuality(t *testing.T) {
	svc := &ImageService{}
	fileBytes, img := testPNGBytes(t)

	lowQuality, lowFormat, lowMime, err := svc.processMainImage(fileBytes, img, "png", "image/png", int64(len(fileBytes)), models.Settings{
		SaveWebp:         true,
		MainImageQuality: 10,
	})
	if err != nil {
		t.Fatalf("processMainImage(low quality) error = %v", err)
	}
	highQuality, highFormat, highMime, err := svc.processMainImage(fileBytes, img, "png", "image/png", int64(len(fileBytes)), models.Settings{
		SaveWebp:         true,
		MainImageQuality: 90,
	})
	if err != nil {
		t.Fatalf("processMainImage(high quality) error = %v", err)
	}

	if lowFormat != "webp" || lowMime != "image/webp" || highFormat != "webp" || highMime != "image/webp" {
		t.Fatalf("processMainImage() should convert to webp, got %q/%q and %q/%q", lowFormat, lowMime, highFormat, highMime)
	}
	if bytes.Equal(lowQuality, highQuality) {
		t.Fatal("processMainImage() should produce different bytes for different main_image_quality values")
	}
}

func TestShouldSkipCompressionSupportsMimeExtensionAndCase(t *testing.T) {
	svc := &ImageService{}

	if !svc.shouldSkipCompression("gif", "image/gif", "") {
		t.Fatal("shouldSkipCompression() should keep default gif skip rule when setting is empty")
	}
	if !svc.shouldSkipCompression("png", "image/png", " GIF , .PNG , image/svg+xml ") {
		t.Fatal("shouldSkipCompression() should match extension with case and spaces")
	}
	if !svc.shouldSkipCompression("svg", "image/svg+xml", "gif,svg") {
		t.Fatal("shouldSkipCompression() should match svg extension")
	}
	if svc.shouldSkipCompression("jpeg", "image/jpeg", "gif,svg") {
		t.Fatal("shouldSkipCompression() should not match unspecified jpeg")
	}
}

func TestProcessImageSaveOriginalNameUsesFinalExtension(t *testing.T) {
	svc := &ImageService{}
	fileBytes, _ := testPNGBytes(t)
	header := &multipart.FileHeader{
		Filename: "sample.png",
		Size:     int64(len(fileBytes)),
		Header: textproto.MIMEHeader{
			"Content-Type": []string{"image/png"},
		},
	}

	processed, err := svc.ProcessImage(readSeekCloser{bytes.NewReader(fileBytes)}, header, models.Settings{
		SaveOriginalName: true,
		SaveWebp:         true,
		MainImageQuality: 85,
	}, 1)
	if err != nil {
		t.Fatalf("ProcessImage() error = %v", err)
	}
	if processed.UniqueFileName != "sample.webp" {
		t.Fatalf("ProcessImage() filename = %q, want sample.webp", processed.UniqueFileName)
	}
	if processed.MimeType != "image/webp" || processed.OutputExt != ".webp" {
		t.Fatalf("ProcessImage() mime/ext = %q/%q, want image/webp/.webp", processed.MimeType, processed.OutputExt)
	}
}
