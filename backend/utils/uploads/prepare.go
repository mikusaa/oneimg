package uploads

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"oneimg/backend/models"
	"oneimg/backend/utils/images"
	"oneimg/backend/utils/videos"
)

type preparedUpload struct {
	*images.ProcessedImage
	Body         io.ReadSeeker
	FileSize     int64
	OriginalSize int64
	close        func() error
}

func (p *preparedUpload) Close() { _ = p.close() }

type contextFile struct {
	io.ReadSeeker
	ctx context.Context
}

func (f contextFile) Read(b []byte) (int, error) {
	if err := f.ctx.Err(); err != nil {
		return 0, err
	}
	return f.ReadSeeker.Read(b)
}

func prepareUpload(ctx context.Context, header *multipart.FileHeader, setting *models.Settings, role int) (*preparedUpload, error) {
	file, err := header.Open()
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			_ = file.Close()
		}
	}()
	body := contextFile{ReadSeeker: file, ctx: ctx}
	head, err := io.ReadAll(io.LimitReader(body, videos.HeaderLimit))
	if err != nil {
		return nil, err
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	mime := strings.ToLower(strings.TrimSpace(strings.SplitN(header.Header.Get("Content-Type"), ";", 2)[0]))
	ext := strings.ToLower(filepath.Ext(header.Filename))
	detected := videos.Detect(head)
	isVideo := detected != "" || ext == ".mp4" || ext == ".webm" || strings.HasPrefix(mime, "video/")
	if isVideo {
		if !setting.VideoUploadEnabled {
			return nil, fmt.Errorf("视频上传未开启")
		}
		expected := map[string]string{".mp4": "video/mp4", ".webm": "video/webm"}[ext]
		if expected == "" || detected != expected || (mime != "" && mime != "application/octet-stream" && mime != "binary/octet-stream" && mime != expected) {
			return nil, fmt.Errorf("视频格式无效或文件内容与扩展名不匹配，仅支持 MP4/WebM")
		}
		limit := int64(setting.MaxVideoFileSize)
		if limit <= 0 {
			limit = 100 << 20
		}
		if header.Size <= 0 || header.Size > limit {
			return nil, fmt.Errorf("视频超过单文件大小限制")
		}
		hash := sha256.New()
		size, err := io.Copy(hash, io.LimitReader(body, limit+1))
		if err != nil {
			return nil, err
		}
		if size > limit || size != header.Size {
			return nil, fmt.Errorf("视频大小无效或超过限制")
		}
		if _, err = file.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		processed := &images.ProcessedImage{MimeType: expected, OutputExt: ext, Format: strings.TrimPrefix(ext, "."), UniqueFileName: images.ImageSvc.BuildOutputFileName(header.Filename, ext, *setting, role), ContentHash: hex.EncodeToString(hash.Sum(nil))}
		success = true
		return &preparedUpload{ProcessedImage: processed, Body: body, FileSize: size, OriginalSize: size, close: file.Close}, nil
	}
	if err := images.ValidateImageFile(header, setting); err != nil {
		return nil, err
	}
	ctx, release, err := images.AcquireProcessing(ctx)
	if err != nil {
		return nil, err
	}
	processed, err := images.ImageSvc.ProcessImageContext(ctx, body, header, *setting, role)
	if err != nil {
		release()
		return nil, err
	}
	success = true
	return &preparedUpload{ProcessedImage: processed, Body: contextFile{ReadSeeker: bytes.NewReader(processed.CompressedBytes), ctx: ctx}, FileSize: int64(len(processed.CompressedBytes)), OriginalSize: int64(len(processed.OriginalBytes)), close: func() error {
		defer release()
		return file.Close()
	}}, nil
}

func saveStream(path string, body io.Reader) (err error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(path)
		}
	}()
	_, err = io.Copy(file, body)
	return err
}
