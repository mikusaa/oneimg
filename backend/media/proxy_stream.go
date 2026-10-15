package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/gin-gonic/gin"
	"oneimg/backend/models"
	"oneimg/backend/utils/buckets"
	"oneimg/backend/utils/ftp"
	"oneimg/backend/utils/webdav"
)

// AllowReadMethod runs before any lookup or storage access, including unknown SPA paths.
func AllowReadMethod(c *gin.Context) bool {
	if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
		return true
	}
	c.Header("Allow", "GET, HEAD")
	c.AbortWithStatus(http.StatusMethodNotAllowed)
	return false
}

func mediaHeaders(c *gin.Context, mime, storage string) {
	if isSVGMIME(mime) || strings.EqualFold(filepath.Ext(c.Request.URL.Path), ".svg") {
		mime = "image/svg+xml"
		svgHeaders(c.Writer.Header())
		if _, wrapped := c.Writer.(*svgPolicyWriter); !wrapped {
			c.Writer = &svgPolicyWriter{c.Writer}
		}
	} else {
		c.Header("Cache-Control", "public, max-age=31536000")
	}
	c.Header("Content-Type", mime)
	c.Header("X-Storage-Type", storage)
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Expose-Headers", "Content-Range, Accept-Ranges, Content-Length")
}

func isSVGMIME(mime string) bool {
	return strings.EqualFold(strings.TrimSpace(strings.SplitN(mime, ";", 2)[0]), "image/svg+xml")
}

func svgHeaders(header http.Header) {
	header.Set("Content-Type", "image/svg+xml")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Content-Security-Policy", "sandbox; default-src 'none'; script-src 'none'; style-src 'unsafe-inline'; img-src data:; object-src 'none'; base-uri 'none'; form-action 'none'")
	header.Set("Cache-Control", "no-cache, max-age=0, must-revalidate")
}

// ServeContent removes Content-Type on 304. Reinstate the policy at the final write boundary.
type svgPolicyWriter struct{ gin.ResponseWriter }

func (w *svgPolicyWriter) WriteHeader(code int) {
	svgHeaders(w.Header())
	w.ResponseWriter.WriteHeader(code)
}
func (w *svgPolicyWriter) WriteHeaderNow() { svgHeaders(w.Header()); w.ResponseWriter.WriteHeaderNow() }
func (w *svgPolicyWriter) Write(data []byte) (int, error) {
	svgHeaders(w.Header())
	return w.ResponseWriter.Write(data)
}

// singleRange ignores unsupported units and multi-range requests. Invalid byte ranges return 416.
func singleRange(raw string, size int64) (start, length int64, partial bool, err error) {
	length = size
	if raw == "" || !strings.HasPrefix(raw, "bytes=") || strings.Contains(raw, ",") {
		return
	}
	parts := strings.Split(strings.TrimSpace(strings.TrimPrefix(raw, "bytes=")), "-")
	if len(parts) != 2 || size <= 0 {
		err = errors.New("invalid range")
		return
	}
	if parts[0] == "" {
		suffix, e := strconv.ParseInt(parts[1], 10, 64)
		if e != nil || suffix <= 0 {
			err = errors.New("invalid range")
			return
		}
		if suffix > size {
			suffix = size
		}
		start = size - suffix
		length = suffix
	} else {
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil || start < 0 || start >= size {
			err = errors.New("invalid range")
			return
		}
		end := size - 1
		if parts[1] != "" {
			end, err = strconv.ParseInt(parts[1], 10, 64)
			if err != nil || end < start {
				err = errors.New("invalid range")
				return
			}
			if end >= size {
				end = size - 1
			}
		}
		length = end - start + 1
	}
	partial = true
	return
}

func requestedRange(c *gin.Context) string {
	if c.Request.Method != http.MethodGet {
		return ""
	}
	raw := c.GetHeader("Range")
	if strings.Contains(raw, ",") || !strings.HasPrefix(raw, "bytes=") {
		return ""
	}
	return raw
}

func rangeError(c *gin.Context, size int64) {
	c.Header("Content-Range", fmt.Sprintf("bytes */%d", size))
	c.Status(http.StatusRequestedRangeNotSatisfiable)
}

func proxyR2File(c *gin.Context, key, mime string, _ int64, bucket models.Buckets, client *awss3.Client) {
	proxyObject(c, key, mime, bucket, client, buckets.ConvertToR2Bucket(bucket.Config).R2Bucket)
}
func proxyS3File(c *gin.Context, key, mime string, _ int64, bucket models.Buckets, client *awss3.Client) {
	proxyObject(c, key, mime, bucket, client, buckets.ConvertToS3Bucket(bucket.Config).S3Bucket)
}

func objectError(c *gin.Context, err error) {
	status := http.StatusBadGateway
	var response *smithyhttp.ResponseError
	if errors.As(err, &response) {
		switch response.HTTPStatusCode() {
		case 403, 404, 416:
			status = response.HTTPStatusCode()
		}
	}
	c.Status(status)
}

func proxyObject(c *gin.Context, key, mime string, bucket models.Buckets, client *awss3.Client, name string) {
	key = strings.TrimPrefix(key, "/")
	if key == "" || name == "" {
		c.Status(http.StatusInternalServerError)
		return
	}
	ctx := c.Request.Context()
	meta, err := client.HeadObject(ctx, &awss3.HeadObjectInput{Bucket: aws.String(name), Key: aws.String(key)})
	if err != nil {
		objectError(c, err)
		return
	}
	size := aws.ToInt64(meta.ContentLength)
	if isSVGMIME(aws.ToString(meta.ContentType)) {
		mime = "image/svg+xml"
	}
	mediaHeaders(c, mime, bucket.Type)
	c.Header("Accept-Ranges", "bytes")
	c.Header("ETag", aws.ToString(meta.ETag))
	if meta.LastModified != nil {
		c.Header("Last-Modified", meta.LastModified.UTC().Format(http.TimeFormat))
	}
	raw := requestedRange(c)
	if condition := c.GetHeader("If-Range"); condition != "" && condition != aws.ToString(meta.ETag) && condition != c.Writer.Header().Get("Last-Modified") {
		raw = ""
	}
	start, length, partial, err := singleRange(raw, size)
	if err != nil {
		rangeError(c, size)
		return
	}
	if c.Request.Method == http.MethodHead {
		c.Header("Content-Length", strconv.FormatInt(size, 10))
		c.Status(http.StatusOK)
		return
	}
	input := &awss3.GetObjectInput{Bucket: aws.String(name), Key: aws.String(key)}
	if partial {
		input.Range = aws.String(fmt.Sprintf("bytes=%d-%d", start, start+length-1))
	}
	resp, err := client.GetObject(ctx, input)
	if err != nil {
		if partial {
			c.Header("Content-Range", fmt.Sprintf("bytes */%d", size))
		}
		objectError(c, err)
		return
	}
	defer resp.Body.Close()
	if resp.ContentLength != nil {
		c.Header("Content-Length", strconv.FormatInt(*resp.ContentLength, 10))
	}
	status := http.StatusOK
	if value := aws.ToString(resp.ContentRange); value != "" {
		c.Header("Content-Range", value)
		status = http.StatusPartialContent
	}
	c.Status(status)
	_, _ = io.Copy(c.Writer, resp.Body)
}

func proxyWebDAVFile(c *gin.Context, path, mime string, _ int64, bucket models.Buckets) {
	cfg := buckets.ConvertToWebDavBucket(bucket.Config)
	if cfg.WebdavURL == "" {
		c.Status(http.StatusInternalServerError)
		return
	}
	client := webdav.Client(webdav.Config{BaseURL: cfg.WebdavURL, Username: cfg.WebdavUser, Password: cfg.WebdavPass, Timeout: 30 * time.Second})
	resp, err := client.WebDAVMedia(c.Request.Context(), path, c.Request.Method, requestedRange(c), c.GetHeader("If-Range"), c.GetHeader("If-None-Match"), c.GetHeader("If-Modified-Since"))
	if err != nil {
		c.Status(http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if isSVGMIME(resp.Header.Get("Content-Type")) {
		mime = "image/svg+xml"
	}
	mediaHeaders(c, mime, "webdav")
	for _, key := range []string{"Content-Range", "Accept-Ranges", "ETag", "Last-Modified"} {
		if value := resp.Header.Get(key); value != "" {
			c.Header(key, value)
		}
	}
	if resp.ContentLength >= 0 {
		c.Header("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
	}
	c.Status(resp.StatusCode)
	if c.Request.Method != http.MethodHead {
		_, _ = io.Copy(c.Writer, resp.Body)
	}
}

func proxyFTPFile(c *gin.Context, path, mime string, bucket models.Buckets) {
	cfg := buckets.ConvertToFTPBucket(bucket.Config)
	util := ftp.NewFTPUtil(ftp.FTPConfig{Context: c.Request.Context(), Host: cfg.FTPHost, Port: cfg.FTPPort, User: cfg.FTPUser, Password: cfg.FTPPass, Timeout: 30})
	defer util.Close()
	client, err := util.GetClient()
	if err != nil {
		c.Status(http.StatusBadGateway)
		return
	}
	path = cleanFTPPath(path)
	size, err := client.FileSize(path)
	if err != nil {
		c.Status(http.StatusBadGateway)
		return
	}
	raw := requestedRange(c)
	// No stable validator is available for FTP; an If-Range condition cannot be established.
	if c.GetHeader("If-Range") != "" {
		raw = ""
	}
	start, length, partial, err := singleRange(raw, size)
	mediaHeaders(c, mime, "ftp")
	if err != nil {
		rangeError(c, size)
		return
	}
	if c.Request.Method == http.MethodHead {
		c.Header("Content-Length", strconv.FormatInt(size, 10))
		c.Status(http.StatusOK)
		return
	}
	reader, err := client.RetrFrom(path, uint64(start))
	if err != nil && partial {
		reader, err = client.Retr(path)
		partial = false
		start = 0
		length = size
	}
	if err != nil {
		c.Status(http.StatusBadGateway)
		return
	}
	// SetDeadline interrupts blocked data reads without racing Response.Close's control commands.
	stop := context.AfterFunc(c.Request.Context(), func() { _ = reader.SetDeadline(time.Now()) })
	defer stop()
	defer reader.Close()
	c.Header("Content-Length", strconv.FormatInt(length, 10))
	status := http.StatusOK
	if partial {
		c.Header("Accept-Ranges", "bytes")
		c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, start+length-1, size))
		status = http.StatusPartialContent
	}
	c.Status(status)
	_, _ = io.CopyN(c.Writer, reader, length)
}
