package images

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/textproto"
	"testing"

	"github.com/chai2010/webp"

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

func testLargePNGBytes(t *testing.T) ([]byte, image.Image) {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 512, 512))
	for y := 0; y < 512; y++ {
		for x := 0; x < 512; x++ {
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

func testGIFBytes(t *testing.T) []byte {
	t.Helper()

	img := image.NewPaletted(image.Rect(0, 0, 32, 32), []color.Color{
		color.RGBA{0, 0, 0, 255},
		color.RGBA{255, 255, 255, 255},
	})
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			if (x+y)%2 == 0 {
				img.SetColorIndex(x, y, 1)
			}
		}
	}

	var buf bytes.Buffer
	if err := gif.Encode(&buf, img, nil); err != nil {
		t.Fatalf("gif.Encode() error = %v", err)
	}
	return buf.Bytes()
}

func testWebPVP8XBytes(width, height int) []byte {
	payload := make([]byte, 10)
	w := width - 1
	h := height - 1
	payload[4] = byte(w)
	payload[5] = byte(w >> 8)
	payload[6] = byte(w >> 16)
	payload[7] = byte(h)
	payload[8] = byte(h >> 8)
	payload[9] = byte(h >> 16)

	var buf bytes.Buffer
	buf.WriteString("RIFF")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(4+8+len(payload)))
	buf.WriteString("WEBP")
	buf.WriteString("VP8X")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(payload)))
	buf.Write(payload)
	return buf.Bytes()
}

func testHeader(fileName, mimeType string, size int) *multipart.FileHeader {
	return &multipart.FileHeader{
		Filename: fileName,
		Size:     int64(size),
		Header: textproto.MIMEHeader{
			"Content-Type": []string{mimeType},
		},
	}
}

func TestProcessMainImageOriginalImageOverridesSaveWebP(t *testing.T) {
	svc := &ImageService{}
	fileBytes, img := testPNGBytes(t)

	gotBytes, gotFormat, gotMime, err := svc.processMainImage(fileBytes, img, "png", "image/png", int64(len(fileBytes)), models.Settings{
		OriginalImage:    true,
		SaveWebp:         true,
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
		SaveWebp:           true,
		MainImageQuality:   75,
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

func TestProcessMainImageSaveWebPUsesConfiguredQualityAndSavingsThreshold(t *testing.T) {
	svc := &ImageService{}
	fileBytes, img := testLargePNGBytes(t)

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

	if lowFormat != "webp" || lowMime != "image/webp" {
		t.Fatalf("processMainImage(low quality) format/mime = %q/%q, want webp/image/webp", lowFormat, lowMime)
	}
	if highFormat != "png" || highMime != "image/png" || !bytes.Equal(highQuality, fileBytes) {
		t.Fatalf("processMainImage(high quality) should retain PNG when savings are too small, got %q/%q", highFormat, highMime)
	}
	if bytes.Equal(lowQuality, fileBytes) {
		t.Fatal("processMainImage(low quality) should use the smaller WebP bytes")
	}
}

func TestShouldUseWebPRequiresRatioAndAbsoluteSavings(t *testing.T) {
	tests := []struct {
		name         string
		originalSize int
		encodedSize  int
		want         bool
	}{
		{name: "both thresholds", originalSize: 100_000, encodedSize: 60_000, want: true},
		{name: "ratio only", originalSize: 100_000, encodedSize: 75_000, want: false},
		{name: "bytes only", originalSize: 2_000_000, encodedSize: 1_960_000, want: false},
		{name: "same size", originalSize: 100_000, encodedSize: 100_000, want: false},
		{name: "larger", originalSize: 100_000, encodedSize: 110_000, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldUseWebP(make([]byte, tt.originalSize), make([]byte, tt.encodedSize)); got != tt.want {
				t.Fatalf("shouldUseWebP(%d, %d) = %v, want %v", tt.originalSize, tt.encodedSize, got, tt.want)
			}
		})
	}
}

func TestProcessMainImageDisabledWebPKeepsOriginal(t *testing.T) {
	svc := &ImageService{}
	fileBytes, img := testPNGBytes(t)

	gotBytes, gotFormat, gotMime, err := svc.processMainImage(
		fileBytes,
		img,
		"png",
		"image/png",
		int64(len(fileBytes)),
		models.Settings{SaveWebp: false, MainImageQuality: 85},
	)
	if err != nil {
		t.Fatalf("processMainImage() error = %v", err)
	}
	if !bytes.Equal(gotBytes, fileBytes) || gotFormat != "png" || gotMime != "image/png" {
		t.Fatalf("processMainImage() = %q/%q (%d bytes), want original png (%d bytes)", gotFormat, gotMime, len(gotBytes), len(fileBytes))
	}
}

func TestConvertToWebPHandlesJPEGAndTransparentPNG(t *testing.T) {
	svc := &ImageService{}
	opaque := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			opaque.SetRGBA(x, y, color.RGBA{R: uint8(x * 7), G: uint8(y * 7), B: 120, A: 255})
		}
	}
	var jpegBuffer bytes.Buffer
	if err := jpeg.Encode(&jpegBuffer, opaque, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	jpegImage, err := jpeg.Decode(bytes.NewReader(jpegBuffer.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if encoded, err := svc.convertToWebP(jpegImage, 85); err != nil || len(encoded) == 0 {
		t.Fatalf("convert jpeg to webp = %d bytes, %v", len(encoded), err)
	}

	transparent := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	transparent.SetNRGBA(0, 0, color.NRGBA{R: 255, G: 40, B: 20, A: 80})
	encoded, err := svc.convertToWebP(transparent, 85)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := webp.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, alpha := decoded.At(0, 0).RGBA()
	if alpha == 0 || alpha == 0xffff {
		t.Fatalf("transparent alpha was not preserved: %d", alpha)
	}
}

func TestShouldSkipCompressionSupportsMimeExtensionAndCase(t *testing.T) {
	svc := &ImageService{}

	if !svc.shouldSkipCompression("gif", "image/gif", "") {
		t.Fatal("shouldSkipCompression() should keep default gif skip rule when setting is empty")
	}
	if !svc.shouldSkipCompression("webp", "image/webp", "") {
		t.Fatal("shouldSkipCompression() should keep default webp skip rule when setting is empty")
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

func TestShouldKeepOriginalOnDecodeErrorOnlyForSkippedWebP(t *testing.T) {
	svc := &ImageService{}

	if !svc.shouldKeepOriginalOnDecodeError("webp", "image/webp", "image/webp") {
		t.Fatal("shouldKeepOriginalOnDecodeError() should keep skipped webp")
	}
	if svc.shouldKeepOriginalOnDecodeError("png", "image/png", "image/png") {
		t.Fatal("shouldKeepOriginalOnDecodeError() should not keep failed png")
	}
}

func TestProcessImageSaveOriginalNameUsesFinalExtension(t *testing.T) {
	svc := &ImageService{}
	fileBytes, _ := testLargePNGBytes(t)
	header := testHeader("sample.png", "image/png", len(fileBytes))

	processed, err := svc.ProcessImage(readSeekCloser{bytes.NewReader(fileBytes)}, header, models.Settings{
		SaveOriginalName: true,
		SaveWebp:         true,
		MainImageQuality: 10,
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
	if processed.ContentHash == "" || processed.ContentHash != HashBytes(processed.CompressedBytes) {
		t.Fatal("ProcessImage() should set content hash from final main image bytes")
	}
}

func TestProcessImageKeepsOriginalMetadataWhenWebPSavingsAreTooSmall(t *testing.T) {
	svc := &ImageService{}
	fileBytes, _ := testPNGBytes(t)
	header := testHeader("sample.png", "image/png", len(fileBytes))

	processed, err := svc.ProcessImage(readSeekCloser{bytes.NewReader(fileBytes)}, header, models.Settings{
		SaveOriginalName: true,
		SaveWebp:         true,
		MainImageQuality: 85,
	}, 1)
	if err != nil {
		t.Fatalf("ProcessImage() error = %v", err)
	}
	if !bytes.Equal(processed.CompressedBytes, fileBytes) {
		t.Fatal("ProcessImage() should keep the original bytes when savings are below the absolute threshold")
	}
	if processed.UniqueFileName != "sample.png" || processed.MimeType != "image/png" || processed.OutputExt != ".png" {
		t.Fatalf("ProcessImage() = %q %q %q, want sample.png image/png .png", processed.UniqueFileName, processed.MimeType, processed.OutputExt)
	}
	if processed.ContentHash != HashBytes(fileBytes) {
		t.Fatal("ProcessImage() content hash should use the retained original bytes")
	}
}

func TestProcessImagePNGGeneratesWebPThumbnail(t *testing.T) {
	svc := &ImageService{}
	fileBytes, _ := testPNGBytes(t)
	header := testHeader("sample.png", "image/png", len(fileBytes))

	processed, err := svc.ProcessImage(readSeekCloser{bytes.NewReader(fileBytes)}, header, models.Settings{
		SaveWebp:         true,
		MainImageQuality: 85,
	}, 1)
	if err != nil {
		t.Fatalf("ProcessImage() error = %v", err)
	}
	if len(processed.ThumbnailBytes) == 0 {
		t.Fatal("ProcessImage() should generate thumbnail bytes")
	}
	if _, err := webp.Decode(bytes.NewReader(processed.ThumbnailBytes)); err != nil {
		t.Fatalf("thumbnail should be webp: %v", err)
	}
}

func TestProcessImageGIFGeneratesWebPThumbnail(t *testing.T) {
	svc := &ImageService{}
	fileBytes := testGIFBytes(t)
	header := testHeader("sample.gif", "image/gif", len(fileBytes))

	processed, err := svc.ProcessImage(readSeekCloser{bytes.NewReader(fileBytes)}, header, models.Settings{}, 1)
	if err != nil {
		t.Fatalf("ProcessImage() error = %v", err)
	}
	if len(processed.ThumbnailBytes) == 0 {
		t.Fatal("ProcessImage() should generate thumbnail bytes for decodable gif")
	}
	if _, err := webp.Decode(bytes.NewReader(processed.ThumbnailBytes)); err != nil {
		t.Fatalf("gif thumbnail should be webp: %v", err)
	}
}

func TestProcessImageSVGHasNoThumbnail(t *testing.T) {
	svc := &ImageService{}
	fileBytes := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"></svg>`)
	header := testHeader("sample.svg", "image/svg+xml", len(fileBytes))

	processed, err := svc.ProcessImage(readSeekCloser{bytes.NewReader(fileBytes)}, header, models.Settings{}, 1)
	if err != nil {
		t.Fatalf("ProcessImage() error = %v", err)
	}
	if len(processed.ThumbnailBytes) != 0 {
		t.Fatal("ProcessImage() should not generate svg thumbnail")
	}
}

func TestProcessImageSkippedWebPBypassesDecode(t *testing.T) {
	svc := &ImageService{}
	fileBytes := testWebPVP8XBytes(640, 360)
	header := testHeader("animated.webp", "image/webp", len(fileBytes))

	processed, err := svc.ProcessImage(readSeekCloser{bytes.NewReader(fileBytes)}, header, models.Settings{
		SkipCompressFormat: "image/webp",
	}, 1)
	if err != nil {
		t.Fatalf("ProcessImage() error = %v", err)
	}
	if !bytes.Equal(processed.CompressedBytes, fileBytes) {
		t.Fatal("ProcessImage() should keep skipped webp bytes")
	}
	if processed.MimeType != "image/webp" || processed.OutputExt != ".webp" || processed.Width != 640 || processed.Height != 360 {
		t.Fatalf("ProcessImage() = mime %q ext %q size %dx%d, want image/webp .webp 640x360", processed.MimeType, processed.OutputExt, processed.Width, processed.Height)
	}
	if len(processed.ThumbnailBytes) != 0 {
		t.Fatal("ProcessImage() should not generate thumbnail for skipped undecodable webp")
	}
}

func TestProcessImageSkippedInvalidWebPStillUploadsWithoutSize(t *testing.T) {
	svc := &ImageService{}
	fileBytes := []byte("not a decodable animated webp")
	header := testHeader("animated.webp", "image/webp", len(fileBytes))

	processed, err := svc.ProcessImage(readSeekCloser{bytes.NewReader(fileBytes)}, header, models.Settings{
		SkipCompressFormat: "image/webp",
	}, 1)
	if err != nil {
		t.Fatalf("ProcessImage() error = %v", err)
	}
	if processed.Width != 0 || processed.Height != 0 {
		t.Fatalf("ProcessImage() size = %dx%d, want 0x0", processed.Width, processed.Height)
	}
}
