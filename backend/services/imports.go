package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrImportURLInvalid = errors.New("remote image URL is invalid")
	ErrImportSSRF       = errors.New("remote address is not public")
	ErrImportTooLarge   = errors.New("remote image exceeds size limit")
	ErrImportNotImage   = errors.New("remote response is not an image")
)

func DownloadRemoteImage(ctx context.Context, rawURL string, maximum int64) (*multipart.FileHeader, error) {
	parsed, err := validateRemoteURL(rawURL)
	if err != nil {
		return nil, err
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil || len(ips) == 0 {
				return nil, ErrImportURLInvalid
			}
			for _, item := range ips {
				if !isPublicIP(item.IP) {
					return nil, ErrImportSSRF
				}
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
		},
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		DisableCompression:    true,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			_, err := validateRemoteURL(request.URL.String())
			return err
		},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, errors.Join(ErrImportURLInvalid, err)
	}
	request.Header.Set("Accept", "image/*")
	request.Header.Set("User-Agent", "OneImg/1.0")
	if host := strings.ToLower(request.URL.Hostname()); host == "pximg.net" || strings.HasSuffix(host, ".pximg.net") {
		request.Header.Set("Referer", "https://www.pixiv.net/")
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("remote status %d", response.StatusCode)
	}
	if response.ContentLength > maximum {
		return nil, ErrImportTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maximum+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maximum {
		return nil, ErrImportTooLarge
	}
	contentType := http.DetectContentType(data)
	if !strings.HasPrefix(contentType, "image/") {
		return nil, ErrImportNotImage
	}
	name := filepath.Base(parsed.Path)
	if name == "" || name == "." || name == "/" {
		name = "imported-image"
	}
	name = strings.ReplaceAll(strings.ReplaceAll(name, "\r", ""), "\n", "")
	return multipartHeader(data, name, contentType)
}

func validateRemoteURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, ErrImportURLInvalid
	}
	if parsed.Hostname() == "" {
		return nil, ErrImportURLInvalid
	}
	if ip := net.ParseIP(parsed.Hostname()); ip != nil && !isPublicIP(ip) {
		return nil, ErrImportSSRF
	}
	return parsed, nil
}

func isPublicIP(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return false
	}
	if ip4 := ip.To4(); ip4 != nil && (ip4[0] == 0 || ip4[0] == 127 || (ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127)) {
		return false
	}
	return true
}

func multipartHeader(data []byte, name, contentType string) (*multipart.FileHeader, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := map[string][]string{
		"Content-Disposition": {fmt.Sprintf(`form-data; name="images"; filename="%s"`, strings.ReplaceAll(name, `"`, ""))},
		"Content-Type":        {contentType},
	}
	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	request, err := http.NewRequest(http.MethodPost, "/", bytes.NewReader(body.Bytes()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if err := request.ParseMultipartForm(int64(len(data)) + 1<<20); err != nil {
		return nil, err
	}
	files := request.MultipartForm.File["images"]
	if len(files) != 1 {
		return nil, errors.New("could not construct image upload")
	}
	return files[0], nil
}
