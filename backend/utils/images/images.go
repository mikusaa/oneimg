package images

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"math/rand"
	"mime/multipart"
	"oneimg/backend/models"
	"oneimg/backend/utils/watermark"
	"path/filepath"
	"strings"
	"time"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
	"github.com/google/uuid"
	"golang.org/x/exp/slices"
)

// 常量定义
const (
	DefaultCompressQuality = 85
	ThumbnailMaxWidth      = 300
	ThumbnailMaxHeight     = 300
	ThumbnailQuality       = 80
	CompressSizeThreshold  = 1024 * 1024 // 1MB
)

// 特殊格式常量
var (
	ErrUnsupportedFormat  = errors.New("unsupported image format")
	ErrFileTooLarge       = errors.New("file size exceeds limit")
	ErrMissingContentType = errors.New("missing content type")
	ErrSVGThumbnail       = errors.New("svg thumbnail generation not supported")
)

type ImageService struct{}

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
}

func (s *ImageService) buildSkippedImage(fileBytes []byte, originalFileName, mimeType string, setting models.Settings, userRole int) *ProcessedImage {
	format := formatFromFilename(originalFileName)
	if format == "" {
		format = strings.TrimPrefix(strings.TrimPrefix(mimeType, "image/"), ".")
	}
	finalMimeType := normalizeMimeType(format, mimeType)
	finalExt := extensionForImage(format, finalMimeType)
	fileName := s.buildOutputFileName(originalFileName, finalExt, setting, userRole)

	return &ProcessedImage{
		OriginalBytes:   fileBytes,
		CompressedBytes: fileBytes,
		ThumbnailBytes:  []byte{},
		Width:           0,
		Height:          0,
		Format:          format,
		MimeType:        finalMimeType,
		OutputExt:       finalExt,
		UniqueFileName:  fileName,
	}
}

// ProcessImage 处理图片（压缩、获取尺寸等）
func (s *ImageService) ProcessImage(
	file multipart.File,
	header *multipart.FileHeader,
	setting models.Settings,
	userRole int,
) (*ProcessedImage, error) {
	// 1. 读取文件内容（一次性读取，避免多次IO）
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read file failed: %w", err)
	}

	mimeType := normalizeMimeType(formatFromFilename(header.Filename), header.Header.Get("Content-Type"))
	originalFileName := header.Filename

	// 2. 解码图片（获取原图信息）
	img, format, err := s.decodeImage(bytes.NewReader(fileBytes), mimeType) // 新增MIME参数
	if err != nil {
		if s.shouldKeepOriginalOnDecodeError(formatFromFilename(originalFileName), mimeType, setting.SkipCompressFormat) {
			return s.buildSkippedImage(fileBytes, originalFileName, mimeType, setting, userRole), nil
		}
		return nil, fmt.Errorf("decode image failed: %w", err)
	}

	// 3. 获取图片基本信息
	var width, height int
	bounds := img.Bounds()
	if format != "svg" {
		width, height = bounds.Dx(), bounds.Dy()
	}
	// 4. 处理主图片（压缩/格式转换）
	processedBytes, finalFormat, finalMimeType, err := s.processMainImage(
		fileBytes, img, format, mimeType, header.Size, setting,
	)
	if err != nil {
		return nil, fmt.Errorf("process main image failed: %w", err)
	}

	// 5. 处理文件扩展名
	finalExt := extensionForImage(finalFormat, finalMimeType)

	// 6. 生成缩略图。缩略图仅作后台快速预览，统一生成小 WebP；不可生成时留空。
	thumbnailBytes, err := s.generateThumbnail(img, finalFormat, finalMimeType)
	if err != nil {
		if !errors.Is(err, ErrSVGThumbnail) {
			log.Printf("generate thumbnail failed: %v", err)
		}
		thumbnailBytes = []byte{}
	}

	// 7. 处理文件名
	fileName := s.buildOutputFileName(originalFileName, finalExt, setting, userRole)

	// 8. 组装返回结果
	return &ProcessedImage{
		OriginalBytes:   fileBytes,
		CompressedBytes: processedBytes,
		ThumbnailBytes:  thumbnailBytes,
		Width:           width,
		Height:          height,
		Format:          finalFormat,
		MimeType:        finalMimeType,
		OutputExt:       finalExt,
		UniqueFileName:  fileName,
	}, nil
}

// processMainImage 处理主图片（拆分逻辑，提高可读性）
func (s *ImageService) processMainImage(
	fileBytes []byte,
	img image.Image,
	format, mimeType string,
	fileSize int64,
	setting models.Settings,
) ([]byte, string, string, error) {
	finalMimeType := normalizeMimeType(format, mimeType)

	// 保存原图优先级最高：不转格式、不压缩、不加水印。
	if setting.OriginalImage {
		return fileBytes, format, finalMimeType, nil
	}

	// 命中跳过格式时，主图保持原文件；缩略图仍按后续流程单独生成。
	if s.shouldSkipCompression(format, finalMimeType, setting.SkipCompressFormat) {
		return fileBytes, format, finalMimeType, nil
	}

	// 添加水印
	if setting.WatermarkEnable {
		watermarkCfg := watermark.WatermarkSetting(setting)
		fileReader := bytes.NewReader(fileBytes)
		processedReader, err := watermark.ProcessImageWithWatermark(fileReader, mimeType, watermarkCfg)
		if err != nil {
			return nil, "", "", fmt.Errorf("添加水印失败：%w", err)
		}
		fileBytes, err = io.ReadAll(processedReader)
		if err != nil {
			return nil, "", "", fmt.Errorf("读取水印后图片数据失败：%w", err)
		}
		img, _, err = image.Decode(bytes.NewReader(fileBytes))
		if err != nil {
			return nil, "", "", fmt.Errorf("解码水印后图片失败：%w", err)
		}
	}

	quality := normalizeMainImageQuality(setting.MainImageQuality)

	// WebP格式处理
	if strings.ToLower(format) == "webp" {
		if fileSize <= CompressSizeThreshold {
			return fileBytes, "webp", "image/webp", nil
		}
		compressed, err := s.compressWebP(img, quality)
		if err != nil {
			return nil, "", "", fmt.Errorf("compress webp: %w", err)
		}
		return compressed, "webp", "image/webp", nil
	}

	// 需要转换为WebP
	if setting.SaveWebp {
		webpData, err := s.convertToWebP(img, quality)
		if err != nil {
			return nil, "", "", fmt.Errorf("convert to webp: %w", err)
		}
		log.Println("转换webp")
		return webpData, "webp", "image/webp", nil
	}

	// 默认进行压缩
	compressed, err := s.compressWebP(img, quality)
	if err != nil {
		return nil, "", "", fmt.Errorf("compress webp: %w", err)
	}
	return compressed, "webp", "image/webp", nil
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
		rules = parseFormatList("image/gif,image/svg+xml,image/webp")
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

func (s *ImageService) shouldKeepOriginalOnDecodeError(format, mimeType, skipFormats string) bool {
	if !s.shouldSkipCompression(format, mimeType, skipFormats) {
		return false
	}
	format = normalizeFormatToken(format)
	mimeType = normalizeFormatToken(mimeType)
	return format == "webp" || mimeType == "image/webp"
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

func (s *ImageService) buildOutputFileName(originalFileName, finalExt string, setting models.Settings, userRole int) string {
	if setting.SaveOriginalName {
		ext := filepath.Ext(originalFileName)
		return strings.TrimSuffix(originalFileName, ext) + finalExt
	}

	pattern := setting.FileName
	if pattern == "" {
		pattern = "{random}"
	}
	return s.ReplaceMagicVariables(pattern, originalFileName, userRole) + finalExt
}

func normalizeMainImageQuality(quality int) int {
	if quality < 0 || quality > 100 {
		return DefaultCompressQuality
	}
	return quality
}

// decodeImage 解码图片，支持webp/gif/png/jpeg/SVG等格式
// 优化点：增加SVG处理，避免解码失败
func (s *ImageService) decodeImage(reader io.Reader, mimeType string) (image.Image, string, error) {
	// 读取数据到缓冲区（复用）
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", fmt.Errorf("read image data: %w", err)
	}
	buf := bytes.NewReader(data)

	// 优先处理SVG（MIME类型判断）
	prefixLen := len(data)
	if prefixLen > 100 {
		prefixLen = 100
	}
	if mimeType == "image/svg+xml" || strings.HasPrefix(strings.ToLower(string(data[:prefixLen])), "<svg") {
		// SVG返回空的image.Image（不解析矢量图），格式标记为svg
		return image.NewRGBA(image.Rect(0, 0, 0, 0)), "svg", nil
	}

	// 按优先级解码（常用格式优先）
	decodeFuncs := []struct {
		decode func(*bytes.Reader) (image.Image, error)
		format string
	}{
		{func(r *bytes.Reader) (image.Image, error) { return webp.Decode(r) }, "webp"},
		{func(r *bytes.Reader) (image.Image, error) { return gif.Decode(r) }, "gif"},
		{func(r *bytes.Reader) (image.Image, error) { return png.Decode(r) }, "png"},
		{func(r *bytes.Reader) (image.Image, error) { return jpeg.Decode(r) }, "jpeg"},
	}

	for _, df := range decodeFuncs {
		buf.Seek(0, io.SeekStart) // 重置读取指针
		img, err := df.decode(buf)
		if err == nil {
			return img, df.format, nil
		}
	}

	// 最后尝试标准库的自动检测
	buf.Seek(0, io.SeekStart)
	img, format, err := image.Decode(buf)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrUnsupportedFormat, err)
	}

	return img, format, nil
}

// convertToWebP 将图片转换为webp格式
func (s *ImageService) convertToWebP(img image.Image, quality int) ([]byte, error) {
	if quality < 0 || quality > 100 {
		return nil, fmt.Errorf("invalid quality: %d (must be 0-100)", quality)
	}

	data, err := webp.EncodeRGBA(img, float32(quality))
	if err != nil {
		return nil, fmt.Errorf("encode webp: %w", err)
	}

	return data, nil
}

// compressWebP 压缩webp图片
func (s *ImageService) compressWebP(img image.Image, quality int) ([]byte, error) {
	return s.convertToWebP(img, quality)
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
		return fmt.Errorf("unsupported content type: %s (allowed: %s)",
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
	thumbnail := imaging.Fit(img, maxWidth, maxHeight, imaging.Lanczos)

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

// generateUniqueFileName 生成唯一文件名
func generateUniqueFileName() string {
	timestamp := time.Now().UnixNano()
	hash := fmt.Sprintf("%x", timestamp)
	rand.New(rand.NewSource(time.Now().UnixNano()))
	randomNum := rand.Intn(900) + 100
	return fmt.Sprintf("%s%d", hash, randomNum)
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
	roleStr := "guest"
	if role == 1 {
		roleStr = "admin"
	}
	pattern = strings.ReplaceAll(pattern, "{role}", roleStr)

	// 随机变量
	if strings.Contains(pattern, "{random}") {
		pattern = strings.ReplaceAll(pattern, "{random}", generateUniqueFileName())
	}

	// UUID变量
	if strings.Contains(pattern, "{uuid}") {
		pattern = strings.ReplaceAll(pattern, "{uuid}", uuid.New().String())
	}

	// 原始文件名（不含扩展名）
	ext := filepath.Ext(originalName)
	nameWithoutExt := strings.TrimSuffix(originalName, ext)
	pattern = strings.ReplaceAll(pattern, "{filename}", nameWithoutExt)

	return pattern
}
