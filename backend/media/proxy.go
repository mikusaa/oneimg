package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/buckets"
	"oneimg/backend/utils/ftp"
	"oneimg/backend/utils/s3"
	"oneimg/backend/utils/settings"
	"oneimg/backend/utils/webdav"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type mediaProxyError struct {
	Status int    `json:"status"`
	Detail string `json:"detail"`
}

func mediaError(status int, detail string) mediaProxyError {
	return mediaProxyError{Status: status, Detail: detail}
}

func ImageProxy(c *gin.Context) bool {
	// 获取并清理路径
	cleanPath := c.Request.URL.Path
	if cleanPath == "" || cleanPath == "/" {
		// 根路径不应由图片代理处理，由 NoRoute 后续逻辑处理
		return false
	}

	// 获取数据库实例
	db := database.GetDB()
	if db == nil || db.DB == nil {
		return false
	}

	// 查询图片信息
	var imageModel models.Image
	sqlResult := db.DB.Where("Url = ? OR Thumbnail = ?", cleanPath, cleanPath).Limit(1).Find(&imageModel)
	if sqlResult.Error != nil || sqlResult.RowsAffected == 0 {
		// 图片不存在，直接返回，交给 NoRoute 后续逻辑处理（如渲染 SPA）
		return false
	}

	// 获取配置信息
	setting, setErr := settings.GetSettings()
	if setErr != nil {
		c.JSON(http.StatusInternalServerError, mediaError(500, fmt.Sprintf("获取系统配置失败: %v", setErr)))
		return true
	}

	// 检查是否开启来源白名单
	if setting.RefererWhiteEnable && setting.RefererWhiteList != "" {
		// 校验Referer白名单
		if !checkReferer(c.Request.Referer(), setting.RefererWhiteList, GetSelfDomain(c)) {
			c.JSON(http.StatusForbidden, mediaError(403, "来源非法"))
			return true
		}
	}

	// 校验图片元信息
	if imageModel.Width == 0 && imageModel.Height == 0 {
		log.Printf("图片[%s]元信息不完整（宽高为0），继续代理访问", cleanPath)
	}

	// 获取存储配置
	var bucket models.Buckets
	if err := db.DB.Where("id = ?", imageModel.BucketId).First(&bucket).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusForbidden, mediaError(400, "存储配置不存在"))
			return true
		}
		c.JSON(http.StatusForbidden, mediaError(400, "存储配置查询失败"))
		return true
	}

	var imageUrl string
	mimeType := imageModel.MimeType
	isThumbnail := imageModel.Thumbnail == cleanPath
	// 判断当前访问的是缩略图还是原图
	if isThumbnail {
		imageUrl = imageModel.Thumbnail
		mimeType = "image/webp"
	} else {
		imageUrl = imageModel.Url
	}

	switch imageModel.Storage {
	case "default":
		proxyLocalFile(c, imageUrl, mimeType)

	case "webdav":
		proxyWebDAVFile(c, imageUrl, mimeType, imageModel.FileSize, bucket)
	case "r2":
		// 初始化S3客户端
		s3Client, err := s3.NewS3Client(setting, bucket)
		if err != nil {
			c.JSON(http.StatusInternalServerError, mediaError(500, fmt.Sprintf("R2客户端初始化失败: %v", err)))
			return true
		}
		proxyR2File(c, imageUrl, mimeType, imageModel.FileSize, bucket, s3Client)

	case "s3":
		// 初始化S3客户端
		s3Client, err := s3.NewS3Client(setting, bucket)
		if err != nil {
			c.JSON(http.StatusInternalServerError, mediaError(500, fmt.Sprintf("S3客户端初始化失败: %v", err)))
			return true
		}
		// 代理S3/R2文件
		proxyS3File(c, imageUrl, mimeType, imageModel.FileSize, bucket, s3Client)

	case "ftp":
		proxyFTPFile(c, imageUrl, mimeType, bucket)

	default:
		c.JSON(http.StatusUnprocessableEntity, mediaError(422, fmt.Sprintf("不支持的存储类型: %s", imageModel.Storage)))
	}

	return true
}

// proxyR2File R2文件代理
func proxyR2File(c *gin.Context, objectKey, mimeType string, fileSize int64, bucket models.Buckets, s3Client *awss3.Client) {
	// 清理objectKey（去除开头的/，适配S3路径规则）
	objectKey = strings.TrimPrefix(objectKey, "/")

	// 获取存储配置
	storageConfig := buckets.ConvertToR2Bucket(bucket.Config)

	// 校验bucket和objectKey
	if storageConfig.R2Bucket == "" || objectKey == "" {
		c.JSON(http.StatusInternalServerError, mediaError(500, "R2配置缺失（Bucket或ObjectKey为空）"))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. 获取R2文件对象
	getInput := awss3.GetObjectInput{
		Bucket: aws.String(storageConfig.R2Bucket),
		Key:    aws.String(objectKey),
	}

	resp, err := s3Client.GetObject(ctx, &getInput)
	if err != nil {
		// 区分不同错误类型
		var noSuchKeyErr *types.NoSuchKey
		if errors.As(err, &noSuchKeyErr) {
			c.JSON(http.StatusNotFound, mediaError(404, "R2文件不存在"))
			return
		}

		var respErr *smithyhttp.ResponseError
		if errors.As(err, &respErr) {
			statusCode := respErr.HTTPStatusCode()
			switch statusCode {
			case http.StatusForbidden:
				c.JSON(http.StatusForbidden, mediaError(403, "R2文件访问权限不足"))
				return
			case http.StatusRequestTimeout:
				c.JSON(http.StatusGatewayTimeout, mediaError(504, "R2请求超时"))
				return
			}
		}

		log.Printf("R2获取文件失败 [key:%s, bucket:%s]: %v", objectKey, bucket.Name, err)
		c.JSON(http.StatusBadGateway, mediaError(502, "R2文件获取失败"))
		return
	}
	defer resp.Body.Close()

	// 2. 设置响应头
	c.Header("Content-Type", mimeType)
	if resp.ContentLength != nil && *resp.ContentLength > 0 {
		c.Header("Content-Length", strconv.FormatInt(*resp.ContentLength, 10))
	} else if fileSize > 0 {
		c.Header("Content-Length", strconv.FormatInt(fileSize, 10))
	}
	// 缓存控制（永久缓存）
	c.Header("Cache-Control", "public, max-age=31536000")
	// 存储类型标识
	c.Header("X-Storage-Type", bucket.Type)
	// 跨域支持（可选）
	c.Header("Access-Control-Allow-Origin", "*")

	// 3. 流式传输文件（避免内存溢出）
	// 设置响应状态码
	c.Status(http.StatusOK)
	// 分块传输，每次4KB
	buf := make([]byte, 4096)
	_, err = io.CopyBuffer(c.Writer, resp.Body, buf)
	if err != nil && err != io.EOF {
		log.Printf("S3/R2文件传输失败 [key:%s]: %v", objectKey, err)
	}
}

// proxyS3File S3文件代理
func proxyS3File(c *gin.Context, objectKey, mimeType string, fileSize int64, bucket models.Buckets, s3Client *awss3.Client) {
	// 清理objectKey（去除开头的/，适配S3路径规则）
	objectKey = strings.TrimPrefix(objectKey, "/")

	// 获取存储配置
	storageConfig := buckets.ConvertToS3Bucket(bucket.Config)

	// 校验bucket和objectKey
	if storageConfig.S3Bucket == "" || objectKey == "" {
		c.JSON(http.StatusInternalServerError, mediaError(500, "S3配置缺失（Bucket或ObjectKey为空）"))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. 获取S3文件对象
	getInput := awss3.GetObjectInput{
		Bucket: aws.String(storageConfig.S3Bucket),
		Key:    aws.String(objectKey),
	}

	resp, err := s3Client.GetObject(ctx, &getInput)
	if err != nil {
		// 区分不同错误类型
		var noSuchKeyErr *types.NoSuchKey
		if errors.As(err, &noSuchKeyErr) {
			c.JSON(http.StatusNotFound, mediaError(404, "S3文件不存在"))
			return
		}

		var respErr *smithyhttp.ResponseError
		if errors.As(err, &respErr) {
			statusCode := respErr.HTTPStatusCode()
			switch statusCode {
			case http.StatusForbidden:
				c.JSON(http.StatusForbidden, mediaError(403, "S3文件访问权限不足"))
				return
			case http.StatusRequestTimeout:
				c.JSON(http.StatusGatewayTimeout, mediaError(504, "S3请求超时"))
				return
			}
		}

		log.Printf("S3获取文件失败 [key:%s, bucket:%s]: %v", objectKey, bucket.Name, err)
		c.JSON(http.StatusBadGateway, mediaError(502, "S3文件获取失败"))
		return
	}
	defer resp.Body.Close()

	// 2. 设置响应头
	c.Header("Content-Type", mimeType)
	if resp.ContentLength != nil && *resp.ContentLength > 0 {
		c.Header("Content-Length", strconv.FormatInt(*resp.ContentLength, 10))
	} else if fileSize > 0 {
		c.Header("Content-Length", strconv.FormatInt(fileSize, 10))
	}
	// 缓存控制（永久缓存）
	c.Header("Cache-Control", "public, max-age=31536000")
	// 存储类型标识
	c.Header("X-Storage-Type", bucket.Type)
	// 跨域支持（可选）
	c.Header("Access-Control-Allow-Origin", "*")

	// 3. 流式传输文件（避免内存溢出）
	// 设置响应状态码
	c.Status(http.StatusOK)
	// 分块传输，每次4KB
	buf := make([]byte, 4096)
	_, err = io.CopyBuffer(c.Writer, resp.Body, buf)
	if err != nil && err != io.EOF {
		log.Printf("S3文件传输失败 [key:%s]: %v", objectKey, err)
	}
}

// proxyWebDAVFile WebDAV文件代理
func proxyWebDAVFile(c *gin.Context, relPath, mimeType string, fileSize int64, bucket models.Buckets) {
	// 获取存储配置
	storageConfig := buckets.ConvertToWebDavBucket(bucket.Config)

	// 初始化WebDav客户端
	if storageConfig.WebdavURL == "" {
		c.JSON(http.StatusInternalServerError, mediaError(500, "WebDAV配置未设置（WebdavURL为空）"))
		return
	}
	client := webdav.Client(webdav.Config{
		BaseURL:  storageConfig.WebdavURL,
		Username: storageConfig.WebdavUser,
		Password: storageConfig.WebdavPass,
		Timeout:  30 * time.Second,
	})
	// 验证WebDAV连接（非阻塞，仅日志）
	go func() {
		ctx := context.Background()
		if _, err := client.WebDAVStat(ctx, ""); err != nil {
			log.Printf("WebDAV连接验证失败: %v", err)
		}
	}()

	ctx := context.Background()

	// 验证文件存在
	exists, err := client.WebDAVStat(ctx, relPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, mediaError(500, "WebDAV文件状态验证失败"))
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, mediaError(404, "WebDAV文件不存在"))
		return
	}

	// 获取文件流
	resp, err := client.WebDAVGetFile(ctx, relPath)
	if err != nil {
		c.JSON(http.StatusBadGateway, mediaError(502, "WebDAV文件获取失败"))
		return
	}
	defer resp.Body.Close()

	// 校验响应状态
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, mediaError(resp.StatusCode, "WebDAV文件获取失败"))
		return
	}

	// 设置响应头
	c.Header("Content-Type", mimeType)
	if resp.ContentLength > 0 {
		c.Header("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
	} else if fileSize > 0 {
		c.Header("Content-Length", strconv.FormatInt(fileSize, 10))
	}
	c.Header("Cache-Control", "public, max-age=31536000")
	c.Header("X-Storage-Type", "webdav")
	c.Header("Access-Control-Allow-Origin", "*")

	// 流式传输文件
	_, err = io.Copy(c.Writer, resp.Body)
	if err != nil {
		log.Printf("WebDAV文件传输失败：%v", err)
	}
}

// proxyLocalFile 本地文件代理
func proxyLocalFile(c *gin.Context, realPath string, mimeType string) {
	fullPath := localProxyPath(realPath)
	// 去除第一个/和\
	fullPath = strings.TrimPrefix(fullPath, "/")
	fullPath = strings.TrimPrefix(fullPath, "\\")

	fileInfo, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, mediaError(404, "文件不存在"))
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, mediaError(500, "文件状态验证失败"))
		return
	}

	if fileInfo.IsDir() {
		c.JSON(http.StatusForbidden, mediaError(403, "文件不可访问"))
		return
	}

	// 设置响应头
	c.Header("Content-Type", mimeType)
	c.Header("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))
	c.Header("Cache-Control", "public, max-age=31536000")
	c.Header("X-Storage-Type", "default")
	c.Header("Access-Control-Allow-Origin", "*")

	// 流式传输
	c.File(fullPath)
}

func localProxyPath(realPath string) string {
	cleanPath := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(realPath)), "/")
	if strings.HasPrefix(cleanPath, "thumbnails/") {
		return filepath.Join(".", "data", cleanPath)
	}
	return filepath.Join(cleanPath)
}

func isThumbnailPath(path string) bool {
	cleanPath := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(path)), "/")
	return strings.HasPrefix(cleanPath, "thumbnails/") || strings.Contains(cleanPath, "/thumbnails/")
}

// FTP代理
func proxyFTPFile(c *gin.Context, ftpPath string, mimeType string, bucket models.Buckets) {
	c.Header("Transfer-Encoding", "chunked")
	c.Writer.Header().Del("Content-Length")

	// 清理FTP路径
	ftpPath = cleanFTPPath(ftpPath)

	// 获取存储配置
	storageConfig := buckets.ConvertToFTPBucket(bucket.Config)

	ftpUtil := ftp.NewFTPUtil(ftp.FTPConfig{
		Host:     storageConfig.FTPHost,
		Port:     storageConfig.FTPPort,
		User:     storageConfig.FTPUser,
		Password: storageConfig.FTPPass,
		Timeout:  60,
	})
	defer func() {
		if err := ftpUtil.Close(); err != nil {
			if !strings.Contains(err.Error(), "227 Entering Passive Mode") {
				log.Printf("FTP连接关闭失败：%v", err)
			}
		}
	}()

	// 获取文件流
	fileReader, _, err := ftpUtil.GetFileStreamReader(ftpPath)
	if err != nil {
		log.Printf("获取FTP文件流失败（路径：%s）：%v", ftpPath, err)
		if strings.Contains(err.Error(), "550") {
			c.AbortWithStatusJSON(http.StatusBadGateway, mediaError(502, "文件不存在或PureFTPd权限不足"))
		} else {
			c.AbortWithStatusJSON(http.StatusBadGateway, mediaError(502, "FTP文件获取失败："+err.Error()))
		}
		return
	}
	defer func() {
		if err := fileReader.Close(); err != nil {
			if !strings.Contains(err.Error(), "227 Entering Passive Mode") {
				log.Printf("FTP文件流关闭失败：%v", err)
			}
		}
	}()

	c.Header("Content-Type", mimeType)
	c.Header("Cache-Control", "public, max-age=31536000")
	c.Header("X-Storage-Type", "ftp")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Connection", "close")

	c.Status(http.StatusOK)

	buf := make([]byte, 4096)
	for {
		n, err := fileReader.Read(buf)
		if n > 0 {
			if _, writeErr := c.Writer.Write(buf[:n]); writeErr != nil {
				break
			}
			c.Writer.Flush()
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			c.Writer.WriteHeader(http.StatusInternalServerError)
			break
		}
	}
	c.Writer.Flush()
	c.Abort()
}

// 辅助函数
func cleanFTPPath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	path = strings.TrimPrefix(path, "/")
	path = strings.ReplaceAll(path, "//", "/")
	path = strings.TrimSuffix(path, "/")
	return path
}

// 辅助函数，校验来源
func checkReferer(referer string, whiteList string, selfDomain string) bool {
	if referer == "" {
		return true
	}

	refererDomain, err := extractDomainFromReferer(referer)
	if err != nil {
		return false
	}

	selfDomain = strings.TrimSpace(strings.ToLower(selfDomain))
	if selfDomain != "" {
		if refererDomain == selfDomain || strings.HasSuffix(refererDomain, "."+selfDomain) {
			return true
		}
	}

	whiteListDomains := strings.Split(strings.TrimSpace(whiteList), ",")

	domainSet := make(map[string]bool)
	for _, d := range whiteListDomains {
		domain := strings.TrimSpace(d)
		if domain != "" {
			domainSet[domain] = true
		}
	}

	for allowedDomain := range domainSet {
		if refererDomain == allowedDomain {
			return true
		}
		if strings.HasSuffix(refererDomain, "."+allowedDomain) {
			return true
		}
	}

	return false
}

func extractDomainFromReferer(referer string) (string, error) {
	if !strings.HasPrefix(referer, "http") {
		referer = "http://" + referer
	}

	// 解析URL
	parsedURL, err := url.Parse(referer)
	if err != nil {
		return "", err
	}

	host := parsedURL.Hostname()

	return strings.ToLower(host), nil
}

// 辅助函数，获取本站域名
func GetSelfDomain(c *gin.Context) string {
	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.Request.Host
	}
	domain := strings.Split(host, ":")[0]
	return strings.ToLower(domain)
}
