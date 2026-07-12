package localimport

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"oneimg/backend/models"
	"oneimg/backend/utils/images"
	"oneimg/backend/utils/md5"
)

type Options struct {
	Root               string
	DataRoot           string
	BucketID           int
	UserID             int
	Username           string
	DryRun             bool
	DateSource         string
	UpdateExistingDate bool
	Logger             *log.Logger
}

type Summary struct {
	Scanned         int
	Imported        int
	WouldImport     int
	Updated         int
	WouldUpdate     int
	SkippedExisting int
	SkippedIgnored  int
	Failed          int
}

type Importer struct {
	db      *gorm.DB
	options Options
}

func DefaultOptions() Options {
	return Options{
		Root:       "./uploads",
		DataRoot:   "./data",
		BucketID:   1,
		UserID:     1,
		Username:   "admin",
		DateSource: "mtime",
		Logger:     log.Default(),
	}
}

func RunCLI(args []string, db *gorm.DB) int {
	options := DefaultOptions()
	flags := flag.NewFlagSet("import-local", flag.ContinueOnError)
	flags.StringVar(&options.Root, "root", options.Root, "本地 uploads 根目录")
	flags.IntVar(&options.BucketID, "bucket-id", options.BucketID, "导入到的存储桶 ID")
	flags.IntVar(&options.UserID, "user-id", options.UserID, "图片归属用户 ID")
	flags.StringVar(&options.Username, "username", options.Username, "用于生成权限 MD5/UUID 的用户名")
	flags.StringVar(&options.DateSource, "date-source", options.DateSource, "导入日期来源：mtime、path、now")
	flags.BoolVar(&options.UpdateExistingDate, "update-existing-date", false, "更新已入库图片的 created_at")
	flags.BoolVar(&options.DryRun, "dry-run", false, "只扫描并打印统计，不写入数据库或缩略图")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	importer := NewImporter(db, options)
	summary, err := importer.Run()
	if err != nil {
		options.Logger.Printf("导入失败: %v", err)
		return 1
	}

	if options.DryRun {
		options.Logger.Printf("扫描完成 dry-run=true scanned=%d would_import=%d would_update=%d skipped_existing=%d skipped_ignored=%d failed=%d",
			summary.Scanned, summary.WouldImport, summary.WouldUpdate, summary.SkippedExisting, summary.SkippedIgnored, summary.Failed)
	} else {
		options.Logger.Printf("导入完成 scanned=%d imported=%d updated=%d skipped_existing=%d skipped_ignored=%d failed=%d",
			summary.Scanned, summary.Imported, summary.Updated, summary.SkippedExisting, summary.SkippedIgnored, summary.Failed)
	}

	if summary.Failed > 0 {
		return 1
	}
	return 0
}

func NewImporter(db *gorm.DB, options Options) *Importer {
	defaults := DefaultOptions()
	if options.Root == "" {
		options.Root = defaults.Root
	}
	if options.DataRoot == "" {
		options.DataRoot = defaults.DataRoot
	}
	if options.BucketID == 0 {
		options.BucketID = defaults.BucketID
	}
	if options.UserID == 0 {
		options.UserID = defaults.UserID
	}
	if options.Username == "" {
		options.Username = defaults.Username
	}
	if options.DateSource == "" {
		options.DateSource = defaults.DateSource
	}
	if options.Logger == nil {
		options.Logger = defaults.Logger
	}
	if images.ImageSvc == nil {
		images.InitImageService()
	}

	return &Importer{db: db, options: options}
}

func (i *Importer) Run() (Summary, error) {
	var summary Summary
	if i.db == nil {
		return summary, fmt.Errorf("database is nil")
	}

	root, err := filepath.Abs(i.options.Root)
	if err != nil {
		return summary, fmt.Errorf("resolve root: %w", err)
	}
	if info, err := os.Stat(root); err != nil {
		return summary, fmt.Errorf("stat root: %w", err)
	} else if !info.IsDir() {
		return summary, fmt.Errorf("root is not a directory: %s", root)
	}

	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			summary.Failed++
			i.options.Logger.Printf("跳过无法访问的路径 %s: %v", path, walkErr)
			if path == root {
				return walkErr
			}
			return nil
		}

		if entry.IsDir() {
			if strings.EqualFold(entry.Name(), "thumbnails") {
				summary.SkippedIgnored++
				return filepath.SkipDir
			}
			return nil
		}

		if !isSupportedImageFile(path) {
			summary.SkippedIgnored++
			return nil
		}

		summary.Scanned++
		if err := i.importFile(root, path, &summary); err != nil {
			summary.Failed++
			i.options.Logger.Printf("导入失败 %s: %v", path, err)
		}
		return nil
	})
	if err != nil {
		return summary, err
	}

	return summary, nil
}

func (i *Importer) importFile(root, path string, summary *Summary) error {
	imageURL, err := imageURLFromPath(root, path)
	if err != nil {
		return err
	}
	if isThumbnailURL(imageURL) {
		summary.SkippedIgnored++
		return nil
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}
	createdAt, err := i.createdAtForImage(imageURL, fileInfo)
	if err != nil {
		return err
	}

	var existing models.Image
	result := i.db.Where("url = ?", imageURL).Limit(1).Find(&existing)
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		return fmt.Errorf("check existing image: %w", result.Error)
	}
	if result.RowsAffected > 0 {
		updated, err := i.updateExistingImage(&existing, path, createdAt)
		if err != nil {
			return err
		}
		if updated {
			if i.options.DryRun {
				summary.WouldUpdate++
			} else {
				summary.Updated++
			}
			return nil
		}
		summary.SkippedExisting++
		return nil
	}

	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	mimeType := detectImageMimeType(path, fileBytes)
	info, err := images.ImageSvc.InspectOriginalImage(fileBytes, filepath.Base(path), mimeType)
	if err != nil {
		return err
	}
	mimeType = info.MimeType
	if mimeType == "" {
		mimeType = detectImageMimeType(path, fileBytes)
	}

	thumbnailURL := ""
	if len(info.ThumbnailBytes) > 0 {
		thumbnailURL = thumbnailURLFromImageURL(imageURL)
		if !i.options.DryRun {
			if err := i.writeThumbnail(thumbnailURL, info.ThumbnailBytes); err != nil {
				return fmt.Errorf("write thumbnail: %w", err)
			}
		}
	}

	if i.options.DryRun {
		summary.WouldImport++
		return nil
	}

	imageModel := models.Image{
		Url:         imageURL,
		Thumbnail:   thumbnailURL,
		FileName:    filepath.Base(path),
		FileSize:    int64(len(fileBytes)),
		MimeType:    mimeType,
		Width:       info.Width,
		Height:      info.Height,
		Storage:     "default",
		BucketId:    i.options.BucketID,
		UserId:      i.options.UserID,
		MD5:         md5.Md5(i.options.Username + filepath.Base(path)),
		ContentHash: images.HashBytes(fileBytes),
		UUID:        i.options.Username,
		CreatedAt:   createdAt,
	}
	if err := i.db.Create(&imageModel).Error; err != nil {
		return fmt.Errorf("create image record: %w", err)
	}

	summary.Imported++
	return nil
}

func (i *Importer) updateExistingImage(existing *models.Image, path string, createdAt time.Time) (bool, error) {
	updates := map[string]interface{}{}
	if i.options.UpdateExistingDate {
		updates["created_at"] = createdAt
	}
	if strings.TrimSpace(existing.ContentHash) == "" {
		fileBytes, err := os.ReadFile(path)
		if err != nil {
			return false, fmt.Errorf("read file for content hash: %w", err)
		}
		updates["content_hash"] = images.HashBytes(fileBytes)
	}
	if len(updates) == 0 {
		return false, nil
	}
	if i.options.DryRun {
		return true, nil
	}
	if err := i.db.Model(existing).Updates(updates).Error; err != nil {
		return false, fmt.Errorf("update existing image: %w", err)
	}
	return true, nil
}

func (i *Importer) writeThumbnail(thumbnailURL string, data []byte) error {
	thumbPath := filepath.Join(i.options.DataRoot, strings.TrimPrefix(thumbnailURL, "/"))
	if err := os.MkdirAll(filepath.Dir(thumbPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(thumbPath, data, 0644)
}

func (i *Importer) createdAtForImage(imageURL string, fileInfo os.FileInfo) (time.Time, error) {
	switch strings.ToLower(strings.TrimSpace(i.options.DateSource)) {
	case "", "path":
		if createdAt, ok := createdAtFromImageURL(imageURL); ok {
			return createdAt, nil
		}
		return fallbackModTime(fileInfo), nil
	case "mtime":
		return fallbackModTime(fileInfo), nil
	case "now":
		return time.Now(), nil
	default:
		return time.Time{}, fmt.Errorf("unsupported date source: %s", i.options.DateSource)
	}
}

func imageURLFromPath(root, path string) (string, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", fmt.Errorf("resolve relative path: %w", err)
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || strings.HasPrefix(rel, "../") || rel == ".." {
		return "", fmt.Errorf("path is outside root")
	}
	return "/uploads/" + strings.TrimPrefix(rel, "/"), nil
}

func createdAtFromImageURL(imageURL string) (time.Time, bool) {
	rel := strings.TrimPrefix(strings.TrimPrefix(filepath.ToSlash(imageURL), "/"), "uploads/")
	parts := strings.Split(rel, "/")
	if len(parts) < 2 {
		return time.Time{}, false
	}

	year, err := strconv.Atoi(parts[0])
	if err != nil || year < 1970 || year > 9999 {
		return time.Time{}, false
	}
	month, err := strconv.Atoi(parts[1])
	if err != nil || month < 1 || month > 12 {
		return time.Time{}, false
	}

	day := 1
	if len(parts) >= 3 {
		parsedDay, err := strconv.Atoi(parts[2])
		if err == nil {
			day = parsedDay
		}
	}
	if day < 1 || day > daysInMonth(year, time.Month(month)) {
		return time.Time{}, false
	}

	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local), true
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.Local).Day()
}

func fallbackModTime(fileInfo os.FileInfo) time.Time {
	modTime := fileInfo.ModTime()
	if modTime.IsZero() {
		return time.Now()
	}
	return modTime
}

func thumbnailURLFromImageURL(imageURL string) string {
	rel := strings.TrimPrefix(strings.TrimPrefix(imageURL, "/"), "uploads/")
	dir := filepath.ToSlash(filepath.Dir(rel))
	base := filepath.Base(rel)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext) + ".webp"
	if dir == "." || dir == "" {
		return "/thumbnails/" + name
	}
	return "/thumbnails/" + strings.Trim(dir, "/") + "/" + name
}

func detectImageMimeType(path string, fileBytes []byte) string {
	if mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path))); mimeType != "" {
		return strings.Split(mimeType, ";")[0]
	}
	if len(fileBytes) == 0 {
		return ""
	}
	return http.DetectContentType(fileBytes)
}

func isSupportedImageFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg":
		return true
	default:
		return false
	}
}

func isThumbnailURL(url string) bool {
	clean := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(url)), "/")
	return strings.HasPrefix(clean, "thumbnails/") || strings.Contains(clean, "/thumbnails/")
}
