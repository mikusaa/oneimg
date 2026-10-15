package services

import (
	"errors"
	"net/url"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"gorm.io/gorm"
	"oneimg/backend/models"
	"oneimg/backend/utils/secureconfig"
	"oneimg/backend/utils/storageobject"
)

type identityLock struct {
	mu   sync.Mutex
	refs int
}

var objectLocks = struct {
	sync.Mutex
	entries map[string]*identityLock
}{entries: map[string]*identityLock{}}

type bucketLock struct {
	mu   sync.RWMutex
	refs int
}

var bucketLocks = struct {
	sync.Mutex
	entries map[int]*bucketLock
}{entries: map[int]*bucketLock{}}

func lockBucket(id int, exclusive bool) func() {
	bucketLocks.Lock()
	entry := bucketLocks.entries[id]
	if entry == nil {
		entry = &bucketLock{}
		bucketLocks.entries[id] = entry
	}
	entry.refs++
	bucketLocks.Unlock()
	if exclusive {
		entry.mu.Lock()
	} else {
		entry.mu.RLock()
	}
	return func() {
		if exclusive {
			entry.mu.Unlock()
		} else {
			entry.mu.RUnlock()
		}
		bucketLocks.Lock()
		defer bucketLocks.Unlock()
		entry.refs--
		if entry.refs == 0 {
			delete(bucketLocks.entries, id)
		}
	}
}

func lockObjects(keys []string) func() {
	sort.Strings(keys)
	var releases []func()
	last := ""
	for _, key := range keys {
		if key == last {
			continue
		}
		last = key
		objectLocks.Lock()
		entry := objectLocks.entries[key]
		if entry == nil {
			entry = &identityLock{}
			objectLocks.entries[key] = entry
		}
		entry.refs++
		objectLocks.Unlock()
		entry.mu.Lock()
		releases = append(releases, func() {
			entry.mu.Unlock()
			objectLocks.Lock()
			defer objectLocks.Unlock()
			entry.refs--
			if entry.refs == 0 {
				delete(objectLocks.entries, key)
			}
		})
	}
	return func() {
		for i := len(releases) - 1; i >= 0; i-- {
			releases[i]()
		}
	}
}

func localObjectPath(value string) string {
	return storageobject.LocalPath(value)
}

func physicalIdentity(bucket models.Buckets, key string) (string, error) {
	if bucket.Type == "default" {
		full, err := filepath.Abs(localObjectPath(key))
		return "local:" + full, err
	}
	get := func(key string) string { return secureconfig.GetString(bucket.Config, key) }
	if bucket.Type == "s3" || bucket.Type == "r2" {
		prefix := bucket.Type
		endpoint, err := normalizedPhysicalEndpoint(get(prefix + "_endpoint"))
		if err != nil || get(prefix+"_bucket") == "" {
			return "", errors.New("unknown physical bucket identity")
		}
		return "object:" + endpoint + "/" + get(prefix+"_bucket") + "/" + strings.TrimPrefix(key, "/"), nil
	}
	if bucket.Type == "webdav" {
		u, err := url.Parse(get("webdav_url"))
		if err != nil {
			return "", err
		}
		endpoint, err := normalizedPhysicalEndpoint(u.Scheme + "://" + u.Host)
		if err != nil {
			return "", err
		}
		return "webdav:" + endpoint + path.Clean("/"+strings.Trim(u.Path, "/")+"/"+strings.ReplaceAll(key, "\\", "/")), nil
	}
	if bucket.Type == "ftp" {
		host := strings.ToLower(get("ftp_host"))
		port := secureconfig.GetInt(bucket.Config, "ftp_port")
		if port == 0 {
			port = 21
		}
		if host == "" {
			return "", errors.New("unknown FTP identity")
		}
		// Ignore account identity conservatively: different accounts may expose the same filesystem.
		return "ftp:" + host + ":" + strconv.Itoa(port) + path.Clean("/"+strings.ReplaceAll(key, "\\", "/")), nil
	}
	return "", errors.New("unsupported physical identity")
}

func normalizedPhysicalEndpoint(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", errors.New("invalid physical endpoint")
	}
	host := strings.ToLower(u.Host)
	if (u.Scheme == "http" && u.Port() == "80") || (u.Scheme == "https" && u.Port() == "443") {
		host = strings.ToLower(u.Hostname())
	}
	return u.Scheme + "://" + host + strings.TrimRight(u.EscapedPath(), "/"), nil
}

func imageObjectKeys(image models.Image, bucket models.Buckets) ([]string, error) {
	var keys []string
	for _, value := range []string{image.Url, image.Thumbnail} {
		if value == "" {
			continue
		}
		key, err := physicalIdentity(bucket, value)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, nil
}

// Cross-check both columns and all logical buckets: a legacy key may be shared across owners.
func unreferencedImageObjects(db *gorm.DB, image models.Image, bucket models.Buckets, excluded []int) (models.Image, error) {
	target := image
	var records []models.Image
	if err := db.Where("id NOT IN ?", excluded).Find(&records).Error; err != nil {
		return target, err
	}
	var buckets []models.Buckets
	if err := db.Find(&buckets).Error; err != nil {
		return target, err
	}
	byID := map[int]models.Buckets{}
	for _, item := range buckets {
		byID[item.Id] = item
	}
	live := map[string]bool{}
	for _, record := range records {
		owner, ok := byID[record.BucketId]
		if !ok {
			continue
		}
		objectStorage := func(kind string) bool { return kind == "s3" || kind == "r2" }
		if owner.Type != bucket.Type && !(objectStorage(owner.Type) && objectStorage(bucket.Type)) {
			continue
		}
		keys, err := imageObjectKeys(record, owner)
		if err != nil {
			return target, err
		}
		for _, key := range keys {
			live[key] = true
		}
	}
	for _, field := range []*string{&target.Url, &target.Thumbnail} {
		if *field == "" {
			continue
		}
		key, err := physicalIdentity(bucket, *field)
		if err != nil {
			return target, err
		}
		if live[key] {
			*field = ""
		}
	}
	return target, nil
}

func distinctDeletionTarget(image models.Image, bucket models.Buckets, seen map[string]bool) (models.Image, error) {
	for _, field := range []*string{&image.Url, &image.Thumbnail} {
		if *field == "" {
			continue
		}
		key, err := physicalIdentity(bucket, *field)
		if err != nil {
			return image, err
		}
		if seen[key] {
			*field = ""
		} else {
			seen[key] = true
		}
	}
	return image, nil
}

// Report only; never move, copy, rename or deduplicate historical objects automatically.
func CountSharedPhysicalObjects(db *gorm.DB) (int, error) {
	var records []models.Image
	if err := db.Find(&records).Error; err != nil {
		return 0, err
	}
	var buckets []models.Buckets
	if err := db.Find(&buckets).Error; err != nil {
		return 0, err
	}
	byID := map[int]models.Buckets{}
	for _, bucket := range buckets {
		byID[bucket.Id] = bucket
	}
	counts := map[string]int{}
	for _, record := range records {
		keys, err := imageObjectKeys(record, byID[record.BucketId])
		if err != nil {
			continue
		}
		seen := map[string]bool{}
		for _, key := range keys {
			if !seen[key] {
				counts[key]++
				seen[key] = true
			}
		}
	}
	total := 0
	for _, count := range counts {
		if count > 1 {
			total++
		}
	}
	return total, nil
}
