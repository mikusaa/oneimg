package uploads

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"mime/multipart"
	"net/textproto"
	"os"
	"regexp"
	"testing"

	"oneimg/backend/models"
	"oneimg/backend/utils/images"
)

func uploadHeader(t *testing.T, name, mime string, data []byte) *multipart.FileHeader {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	head := make(textproto.MIMEHeader)
	head.Set("Content-Disposition", `form-data; name="images"; filename="`+name+`"`)
	if mime != "" {
		head.Set("Content-Type", mime)
	}
	part, err := writer.CreatePart(head)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write(data); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	reader := multipart.NewReader(&buf, writer.Boundary())
	form, err := reader.ReadForm(1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { form.RemoveAll() })
	return form.File["images"][0]
}

func TestPrepareVideoStreamsAndValidates(t *testing.T) {
	images.InitImageService()
	for _, format := range []string{"mp4", "webm"} {
		data, err := os.ReadFile("../videos/testdata/sample." + format)
		if err != nil {
			t.Fatal(err)
		}
		for _, mime := range []string{"", "application/octet-stream", "video/" + format} {
			header := uploadHeader(t, "sample."+format, mime, data)
			settings := models.Settings{VideoUploadEnabled: true, MaxVideoFileSize: len(data), SaveOriginalName: true}
			prepared, err := prepareUpload(context.Background(), header, &settings, 1)
			if err != nil {
				t.Fatal(err)
			}
			if len(prepared.OriginalBytes) != 0 || len(prepared.CompressedBytes) != 0 || len(prepared.ThumbnailBytes) != 0 || prepared.Width != 0 || prepared.Height != 0 {
				t.Fatal("video entered image pipeline")
			}
			output, err := io.ReadAll(prepared.Body)
			if err != nil || !bytes.Equal(output, data) {
				t.Fatal("stream changed", err)
			}
			digest := sha256.Sum256(data)
			if prepared.ContentHash != hex.EncodeToString(digest[:]) || prepared.FileSize != int64(len(data)) || prepared.MimeType != "video/"+format || !regexp.MustCompile(`^sample-[a-f0-9]{32}\.`+format+`$`).MatchString(prepared.UniqueFileName) {
				t.Fatalf("invalid metadata: %+v", prepared)
			}
			prepared.Close()
			settings.MaxVideoFileSize = len(data) - 1
			if _, err = prepareUpload(context.Background(), header, &settings, 1); err == nil {
				t.Fatal("accepted oversized video")
			}
			settings.VideoUploadEnabled = false
			if _, err = prepareUpload(context.Background(), header, &settings, 1); err == nil {
				t.Fatal("accepted disabled video")
			}
		}
		for _, test := range []struct {
			name, mime string
			data       []byte
		}{
			{"sample.png", "image/png", data}, {"sample." + format, "image/png", data}, {"fake." + format, "video/" + format, []byte("not a video")},
		} {
			header := uploadHeader(t, test.name, test.mime, test.data)
			if _, err := prepareUpload(context.Background(), header, &models.Settings{VideoUploadEnabled: true, MaxVideoFileSize: 100 << 20}, 1); err == nil {
				t.Fatal("accepted disguised file", test.name)
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := prepareUpload(ctx, uploadHeader(t, "sample."+format, "", data), &models.Settings{VideoUploadEnabled: true, MaxVideoFileSize: 100 << 20}, 1); err == nil {
			t.Fatal("ignored cancellation")
		}
	}
}
