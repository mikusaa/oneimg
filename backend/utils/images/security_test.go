package images

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/png"
	"math"
	"regexp"
	"strings"
	"testing"

	"oneimg/backend/models"
)

func TestBudgetPrecedesFullDecodeAndReleasesSlot(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	calls := 0
	svc := &ImageService{decodeRaster: func([]byte, string) (image.Image, error) { calls++; return nil, errors.New("decoder called") }}
	for _, dims := range [][2]uint32{{16385, 1}, {10000, 10000}, {0, 10}, {math.MaxUint32, math.MaxUint32}} {
		header := append([]byte(nil), data[:33]...)
		binary.BigEndian.PutUint32(header[16:20], dims[0])
		binary.BigEndian.PutUint32(header[20:24], dims[1])
		binary.BigEndian.PutUint32(header[29:33], crc32.ChecksumIEEE(header[12:29]))
		_, err := svc.ProcessImage(bytes.NewReader(header), testHeader("a.png", "image/png", len(header)), models.Settings{OriginalImage: true}, 1)
		if err == nil {
			t.Fatal("oversized header accepted")
		}
		if dims[0] != 0 && !errors.Is(err, ErrDimensionsExceeded) && dims[0] != math.MaxUint32 {
			t.Fatal(err)
		}
		if calls != 0 {
			t.Fatal("budget checked after full decode")
		}
	}
	ctx, release, err := AcquireProcessing(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ProcessImage(bytes.NewReader(data), testHeader("a.png", "image/png", len(data)), models.Settings{}, 1); !errors.Is(err, ErrProcessingBusy) {
		t.Fatal(err)
	}
	if _, err := (&ImageService{}).ProcessImageContext(ctx, bytes.NewReader(data), testHeader("a.png", "image/png", len(data)), models.Settings{}, 1); err != nil {
		t.Fatal(err)
	}
	release()
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (&ImageService{}).ProcessImageContext(cancelCtx, bytes.NewReader(data), testHeader("a.png", "image/png", len(data)), models.Settings{}, 1); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := (&ImageService{}).ProcessImage(bytes.NewReader(data), testHeader("a.png", "image/png", len(data)), models.Settings{MaxFileSize: 1}, 1); !errors.Is(err, ErrFileTooLarge) {
		t.Fatal(err)
	}
	if _, release, err := AcquireProcessing(context.Background()); err != nil {
		t.Fatal("slot leaked")
	} else {
		release()
	}
}

func TestNamesAreUniqueSafeAndBounded(t *testing.T) {
	svc := &ImageService{}
	original := "../目录/恶意\u202e\"?#\\name" + strings.Repeat("照", 100) + ".png"
	for _, setting := range []models.Settings{{SaveOriginalName: true}, {FileName: "fixed"}, {FileName: "{filename}"}, {FileName: "{random}"}} {
		a, b := svc.BuildOutputFileName(original, ".png", setting, 1), svc.BuildOutputFileName(original, ".png", setting, 1)
		if a == b || !regexp.MustCompile(`[a-f0-9]{32}\.png$`).MatchString(a) {
			t.Fatal("non-unique object identity")
		}
		if len(a) > 117 || strings.ContainsAny(a, `/\:"'<>|?*`) || strings.ContainsRune(a, '\u202e') {
			t.Fatalf("unsafe name %q", a)
		}
	}
	fragment := svc.ReplaceMagicVariables("{filename}", original, 1)
	if strings.ContainsAny(fragment, `/\:"'<>|?*`) || len(fragment) > 80 {
		t.Fatal(fragment)
	}
}
