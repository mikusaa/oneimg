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
	"strings"

	"gorm.io/gorm"

	"oneimg/backend/models"
	"oneimg/backend/utils/images"
	"oneimg/backend/utils/md5"
)

type Options struct {
	Root     string
	DataRoot string
	BucketID int
	UserID   int
	Username string
	DryRun   bool
	Logger   *log.Logger
}

type Summary struct {
	Scanned         int
	Imported        int
	WouldImport     int
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
		Root:     "./uploads",
		DataRoot: "./data",
		BucketID: 1,
		UserID:   1,
		Username: "admin",
		Logger:   log.Default(),
	}
}

func RunCLI(args []string, db *gorm.DB) int {
	options := DefaultOptions()
	flags := flag.NewFlagSet("import-local", flag.ContinueOnError)
	flags.StringVar(&options.Root, "root", options.Root, "本地 uploads 根目录")
	flags.IntVar(&options.BucketID, "bucket-id", options.BucketID, "导入到的存储桶 ID")
	flags.IntVar(&options.UserID, "user-id", options.UserID, "图片归属用户 ID")
	flags.StringVar(&options.Username, "username", options.Username, "用于生成权限 MD5/UUID 的用户名")
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
		options.Logger.Printf("扫描完成 dry-run=true scanned=%d would_import=%d skipped_existing=%d skipped_ignored=%d failed=%d",
			summary.Scanned, summary.WouldImport, summary.SkippedExisting, summary.SkippedIgnored, summary.Failed)
	} else {
		options.Logger.Printf("导入完成 scanned=%d imported=%d skipped_existing=%d skipped_ignored=%d failed=%d",
			summary.Scanned, summary.Imported, summary.SkippedExisting, summary.SkippedIgnored, summary.Failed)
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

	var count int64
	if err := i.db.Model(&models.Image{}).Where("url = ?", imageURL).Count(&count).Error; err != nil {
		return fmt.Errorf("check existing image: %w", err)
	}
	if count > 0 {
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
		Url:       imageURL,
		Thumbnail: thumbnailURL,
		FileName:  filepath.Base(path),
		FileSize:  int64(len(fileBytes)),
		MimeType:  mimeType,
		Width:     info.Width,
		Height:    info.Height,
		Storage:   "default",
		BucketId:  i.options.BucketID,
		UserId:    i.options.UserID,
		MD5:       md5.Md5(i.options.Username + filepath.Base(path)),
		UUID:      i.options.Username,
	}
	if err := i.db.Create(&imageModel).Error; err != nil {
		return fmt.Errorf("create image record: %w", err)
	}

	summary.Imported++
	return nil
}

func (i *Importer) writeThumbnail(thumbnailURL string, data []byte) error {
	thumbPath := filepath.Join(i.options.DataRoot, strings.TrimPrefix(thumbnailURL, "/"))
	if err := os.MkdirAll(filepath.Dir(thumbPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(thumbPath, data, 0644)
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
