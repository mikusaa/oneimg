package images

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"mime/multipart"
	"oneimg/backend/models"
	"path/filepath"
	"strings"
	"time"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
	"golang.org/x/exp/slices"
)

// 常量定义
const (
	DefaultCompressQuality = 85
	ThumbnailMaxWidth      = 300
	ThumbnailMaxHeight     = 300
	ThumbnailQuality       = 80
	MinWebPSavingsBytes    = 32 * 1024
	MinWebPSavingsRatio    = 0.05
	SlowProcessingDuration = 2 * time.Second
)

// 特殊格式常量
var (
	ErrUnsupportedFormat  = errors.New("unsupported image format")
	ErrFileTooLarge       = errors.New("file size exceeds limit")
	ErrMissingContentType = errors.New("missing content type")
	ErrSVGThumbnail       = errors.New("svg thumbnail generation not supported")
)

type ImageService struct {
	decodeRaster func([]byte, string) (image.Image, error)
}

var ImageSvc *ImageService

// InitImageService 初始化图片服务（线程安全）
func InitImageService() {
	if ImageSvc == nil {
		ImageSvc = &ImageService{}
	}
}

// ProcessedImage 处理后的图片数据
type ProcessedImage struct {
	OriginalBytes   []byte // 原始文件字节
	CompressedBytes []byte // 处理后的字节
	ThumbnailBytes  []byte // 缩略图字节
	Width           int    // 图片宽度
	Height          int    // 图片高度
	Format          string // 最终格式
	MimeType        string // 最终MIME类型
	OutputExt       string // 输出文件扩展名
	UniqueFileName  string // 唯一文件名
	ContentHash     string // 最终主图字节哈希
}

type OriginalImageInfo struct {
	Width          int
	Height         int
	Format         string
	MimeType       string
	ThumbnailBytes []byte
}

// InspectOriginalImage 只读取原图元数据并生成内部缩略图，不改变主图字节。
func (s *ImageService) InspectOriginalImage(data []byte, name, mime string) (*OriginalImageInfo, error) {
	return s.InspectOriginalImageContext(context.Background(), data, name, mime)
}

func (s *ImageService) InspectOriginalImageContext(ctx context.Context, data []byte, name, mime string) (*OriginalImageInfo, error) {
	_, release, err := AcquireProcessing(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	cfg, format, animated, err := inspectConfig(data)
	if err != nil {
		return nil, err
	}
	info := &OriginalImageInfo{Width: cfg.Width, Height: cfg.Height, Format: format, MimeType: normalizeMimeType(format, "")}
	if animated {
		return info, nil
	}
	img, _, err := s.decodeBytes(data)
	if err != nil {
		return nil, err
	}
	info.Width, info.Height = img.Bounds().Dx(), img.Bounds().Dy()
	info.ThumbnailBytes, err = s.generateThumbnail(img, format, info.MimeType)
	return info, err
}

func (s *ImageService) ProcessImage(file io.Reader, header *multipart.FileHeader, setting models.Settings, role int) (*ProcessedImage, error) {
	return s.ProcessImageContext(context.Background(), file, header, setting, role)
}

func (s *ImageService) ProcessImageContext(ctx context.Context, file io.Reader, header *multipart.FileHeader, setting models.Settings, role int) (*ProcessedImage, error) {
	ctx, release, err := AcquireProcessing(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	data, err := ReadBounded(ctx, file, int64(setting.MaxFileSize))
	if err != nil {
		return nil, err
	}
	cfg, format, animated, err := inspectConfig(data)
	if err != nil {
		return nil, err
	}
	mime := normalizeMimeType(format, "")
	// The confirmed format, not the declared MIME or filename, controls admission.
	if setting.AllowedTypes != "" && !slices.Contains(strings.Split(setting.AllowedTypes, ","), mime) {
		return nil, ErrUnsupportedFormat
	}
	processed := &ProcessedImage{OriginalBytes: data, CompressedBytes: data, Width: cfg.Width, Height: cfg.Height, Format: format, MimeType: mime}
	if !animated {
		img, _, err := s.decodeBytes(data)
		if err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		processed.Width, processed.Height = img.Bounds().Dx(), img.Bounds().Dy()
		processed.CompressedBytes, processed.Format, processed.MimeType, err = s.processMainImage(data, img, format, mime, int64(len(data)), setting)
		if err != nil {
			return nil, err
		}
		processed.ThumbnailBytes, err = s.generateThumbnail(img, format, mime)
		if err != nil {
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	processed.OutputExt = extensionForImage(processed.Format, processed.MimeType)
	processed.UniqueFileName = s.buildOutputFileName(header.Filename, processed.OutputExt, setting, role)
	processed.ContentHash = HashBytes(processed.CompressedBytes)
	return processed, nil
}

func HashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// processMainImage 处理主图片（拆分逻辑，提高可读性）
func (s *ImageService) processMainImage(
	fileBytes []byte,
	img image.Image,
	format, mimeType string,
	_ int64,
	setting models.Settings,
) ([]byte, string, string, error) {
	finalMimeType := normalizeMimeType(format, mimeType)

	// 保存原图优先级最高：不转格式、不压缩。
	if setting.OriginalImage {
		return fileBytes, format, finalMimeType, nil
	}

	// 命中跳过格式时，主图保持原文件；缩略图仍按后续流程单独生成。
	if s.shouldSkipCompression(format, finalMimeType, setting.SkipCompressFormat) {
		return fileBytes, format, finalMimeType, nil
	}

	// 未开启 WebP 时保持原始主图；缩略图仍由独立流程生成。
	if !setting.SaveWebp {
		return fileBytes, format, finalMimeType, nil
	}

	webpData, err := s.convertToWebP(img, normalizeMainImageQuality(setting.MainImageQuality))
	if err != nil {
		return nil, "", "", fmt.Errorf("convert to webp: %w", err)
	}
	if !shouldUseWebP(fileBytes, webpData) {
		return fileBytes, format, finalMimeType, nil
	}
	return webpData, "webp", "image/webp", nil
}

func shouldUseWebP(original, encoded []byte) bool {
	if len(original) == 0 || len(encoded) >= len(original) {
		return false
	}
	savedBytes := len(original) - len(encoded)
	savedRatio := float64(savedBytes) / float64(len(original))
	return savedBytes >= MinWebPSavingsBytes && savedRatio >= MinWebPSavingsRatio
}

func logSlowImageProcessing(
	fileName string,
	width, height int,
	decodeDuration, mainImageDuration, thumbnailDuration, totalDuration time.Duration,
	originalSize, finalSize int,
	finalFormat string,
) {
	retainedPercent := 100.0
	if originalSize > 0 {
		retainedPercent = float64(finalSize) / float64(originalSize) * 100
	}
	log.Printf(
		"图片处理较慢: 文件=%q 尺寸=%dx%d 解码=%s 主图=%s 缩略图=%s 总计=%s 原图=%dB 保存后=%dB 占比=%.1f%% 格式=%s",
		fileName,
		width,
		height,
		decodeDuration.Round(time.Millisecond),
		mainImageDuration.Round(time.Millisecond),
		thumbnailDuration.Round(time.Millisecond),
		totalDuration.Round(time.Millisecond),
		originalSize,
		finalSize,
		retainedPercent,
		finalFormat,
	)
}

// generateThumbnail 生成 WebP 缩略图。SVG 和空图不生成缩略图，由前端回退主图预览。
func (s *ImageService) generateThumbnail(
	img image.Image,
	format, mimeType string,
) ([]byte, error) {
	if format == "svg" || mimeType == "image/svg+xml" {
		return []byte{}, ErrSVGThumbnail
	}

	return s.generateWebPThumbnail(img, ThumbnailMaxWidth, ThumbnailMaxHeight, ThumbnailQuality)
}

func (s *ImageService) shouldSkipCompression(format, mimeType, skipFormats string) bool {
	rules := parseFormatList(skipFormats)
	if len(rules) == 0 {
		rules = parseFormatList("image/gif,image/webp")
	}

	format = normalizeFormatToken(format)
	mimeType = normalizeFormatToken(mimeType)
	ext := strings.TrimPrefix(format, ".")

	for _, rule := range rules {
		if rule == "" {
			continue
		}
		if rule == mimeType || rule == format || rule == ext || rule == "."+ext {
			return true
		}
		if strings.HasPrefix(rule, "image/") && strings.TrimPrefix(rule, "image/") == ext {
			return true
		}
	}
	return false
}

func parseFormatList(value string) []string {
	parts := strings.Split(value, ",")
	formats := make([]string, 0, len(parts))
	for _, part := range parts {
		format := normalizeFormatToken(part)
		if format != "" {
			formats = append(formats, format)
		}
	}
	return formats
}

func normalizeFormatToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "jpg" {
		return "jpeg"
	}
	if value == ".jpg" {
		return ".jpeg"
	}
	return value
}

func normalizeMimeType(format, mimeType string) string {
	mimeType = strings.TrimSpace(mimeType)
	if mimeType != "" {
		return mimeType
	}
	switch strings.ToLower(strings.TrimPrefix(format, ".")) {
	case "jpeg", "jpg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	case "svg":
		return "image/svg+xml"
	default:
		return mimeType
	}
}

func formatFromFilename(fileName string) string {
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(fileName)), ".")
}

func extensionForImage(format, mimeType string) string {
	outputExt := map[string]string{
		"image/jpeg":    ".jpg",
		"image/png":     ".png",
		"image/gif":     ".gif",
		"image/webp":    ".webp",
		"image/svg+xml": ".svg",
		"image/bmp":     ".bmp",
		"image/tiff":    ".tiff",
		"image/heic":    ".heic",
		"image/heif":    ".heif",
	}
	if ext := outputExt[mimeType]; ext != "" {
		return ext
	}
	format = strings.TrimPrefix(format, ".")
	if format == "" {
		return ""
	}
	return "." + format
}

func (s *ImageService) buildOutputFileName(original, ext string, setting models.Settings, role int) string {
	id, err := NewObjectID()
	if err != nil {
		panic(err)
	} // Never fall back to a predictable object identity.
	name := ""
	if setting.SaveOriginalName {
		name = strings.TrimSuffix(original, filepath.Ext(original))
	} else {
		pattern := setting.FileName
		if pattern == "" || pattern == "{random}" || pattern == "{uuid}" {
			return id + ext
		}
		name = s.ReplaceMagicVariables(pattern, original, role)
	}
	return SafeName(name) + "-" + id + ext
}

func normalizeMainImageQuality(quality int) int {
	if quality < 0 || quality > 100 {
		return DefaultCompressQuality
	}
	return quality
}

func decodeAutoOrientedJPEG(reader *bytes.Reader) (image.Image, error) {
	// imaging.Decode auto-detects formats, so confirm JPEG first to preserve format reporting.
	if _, err := jpeg.DecodeConfig(reader); err != nil {
		return nil, err
	}
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("reset jpeg reader: %w", err)
	}
	return imaging.Decode(reader, imaging.AutoOrientation(true))
}

// decodeImage inspects dimensions before choosing exactly one full decoder.
func (s *ImageService) decodeImage(reader io.Reader, _ string) (image.Image, string, error) {
	var data []byte
	if r, ok := reader.(*bytes.Reader); ok {
		// Callers already hold a bounded buffer. ReadAll on bytes.Reader avoids decoder retries.
		data = make([]byte, r.Len())
		_, _ = io.ReadFull(r, data)
	} else {
		var err error
		data, err = ReadBounded(context.Background(), reader, 10<<20)
		if err != nil {
			return nil, "", err
		}
	}
	return s.decodeBytes(data)
}

func (s *ImageService) decodeBytes(data []byte) (image.Image, string, error) {
	_, format, animated, err := inspectConfig(data)
	if err != nil {
		return nil, "", err
	}
	if animated {
		return nil, "", ErrUnsupportedFormat
	}
	if s.decodeRaster != nil {
		img, err := s.decodeRaster(data, format)
		return img, format, err
	}
	var img image.Image
	switch format {
	case "jpeg":
		img, err = decodeAutoOrientedJPEG(bytes.NewReader(data))
	case "png":
		img, err = png.Decode(bytes.NewReader(data))
	case "gif":
		img, err = gif.Decode(bytes.NewReader(data))
	case "webp":
		img, err = webp.Decode(bytes.NewReader(data))
	default:
		return nil, "", ErrUnsupportedFormat
	}
	if err != nil {
		return nil, "", fmt.Errorf("%w: invalid raster encoding", ErrUnsupportedFormat)
	}
	return img, format, nil
}

func validWebPBitstream(chunk riffChunk) bool {
	data := make([]byte, 20+len(chunk.data)+len(chunk.data)%2)
	copy(data[:4], "RIFF")
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(data)-8))
	copy(data[8:12], "WEBP")
	copy(data[12:16], chunk.kind)
	binary.LittleEndian.PutUint32(data[16:20], uint32(len(chunk.data)))
	copy(data[20:], chunk.data)
	_, _, _, err := webp.GetInfo(data)
	return err == nil
}

// convertToWebP 将图片转换为webp格式
func (s *ImageService) convertToWebP(img image.Image, quality int) ([]byte, error) {
	if quality < 0 || quality > 100 {
		return nil, fmt.Errorf("invalid quality: %d (must be 0-100)", quality)
	}

	var output bytes.Buffer
	if err := webp.Encode(&output, img, &webp.Options{Quality: float32(quality)}); err != nil {
		return nil, fmt.Errorf("encode webp: %w", err)
	}

	return output.Bytes(), nil
}

// ValidateImage 验证图片格式和大小
func (s *ImageService) ValidateImage(
	header *multipart.FileHeader,
	allowedTypes []string,
	maxSize int64,
) error {
	// 检查文件大小
	if header.Size > maxSize {
		return fmt.Errorf("%w: max size %d bytes, got %d bytes",
			ErrFileTooLarge, maxSize, header.Size)
	}

	// 检查Content-Type
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		return ErrMissingContentType
	}

	// 检查是否允许的类型
	if !slices.Contains(allowedTypes, mimeType) {
		return fmt.Errorf("%w: content type %s (allowed: %s)", ErrUnsupportedFormat,
			mimeType, strings.Join(allowedTypes, ", "))
	}

	return nil
}

// generateWebPThumbnail 生成webp格式缩略图
func (s *ImageService) generateWebPThumbnail(
	img image.Image,
	maxWidth, maxHeight, quality int,
) ([]byte, error) {
	// 空图片（如SVG）直接返回空字节
	if img.Bounds().Dx() == 0 && img.Bounds().Dy() == 0 {
		return []byte{}, ErrSVGThumbnail
	}

	// 调整图片大小
	thumbnail := imaging.Fit(img, maxWidth, maxHeight, imaging.Linear)

	// 转换为WebP
	return s.convertToWebP(thumbnail, quality)
}

// ValidateImageFile 验证图片文件
func ValidateImageFile(header *multipart.FileHeader, setting *models.Settings) error {
	allowedTypes := strings.Split(setting.AllowedTypes, ",")
	return ImageSvc.ValidateImage(header, allowedTypes, int64(setting.MaxFileSize))
}

// ReadFileContent 读取文件内容
func ReadFileContent(header *multipart.FileHeader) ([]byte, error) {
	file, err := header.Open()
	if err != nil {
		return nil, fmt.Errorf("打开文件失败：%v", err)
	}
	defer file.Close()

	return io.ReadAll(file)
}

// GetFileMimeType 获取文件MIME类型
func GetFileMimeType(header *multipart.FileHeader) string {
	return header.Header.Get("Content-Type")
}

// ReplaceMagicVariables 替换魔法变量
func (s *ImageService) ReplaceMagicVariables(pattern string, originalName string, role int) string {
	now := time.Now()

	// 基础时间变量
	pattern = strings.ReplaceAll(pattern, "{year}", now.Format("2006"))
	pattern = strings.ReplaceAll(pattern, "{month}", now.Format("01"))
	pattern = strings.ReplaceAll(pattern, "{moon}", now.Format("01"))
	pattern = strings.ReplaceAll(pattern, "{day}", now.Format("02"))
	pattern = strings.ReplaceAll(pattern, "{hour}", now.Format("15"))
	pattern = strings.ReplaceAll(pattern, "{minute}", now.Format("04"))
	pattern = strings.ReplaceAll(pattern, "{second}", now.Format("05"))

	// 角色变量
	roleStr := "user"
	if role == 1 {
		roleStr = "admin"
	}
	pattern = strings.ReplaceAll(pattern, "{role}", roleStr)

	// 随机变量
	if strings.Contains(pattern, "{random}") {
		pattern = strings.ReplaceAll(pattern, "{random}", mustObjectID())
	}

	// UUID变量
	if strings.Contains(pattern, "{uuid}") {
		pattern = strings.ReplaceAll(pattern, "{uuid}", mustObjectID())
	}

	// 原始文件名（不含扩展名）
	ext := filepath.Ext(originalName)
	nameWithoutExt := SafeName(strings.TrimSuffix(originalName, ext))
	pattern = strings.ReplaceAll(pattern, "{filename}", nameWithoutExt)

	return pattern
}

// BuildOutputFileName applies the existing naming policy to a validated media extension.
func (s *ImageService) BuildOutputFileName(name, ext string, setting models.Settings, role int) string {
	return s.buildOutputFileName(name, ext, setting, role)
}

func mustObjectID() string {
	id, err := NewObjectID()
	if err != nil {
		panic(err)
	}
	return id
}
