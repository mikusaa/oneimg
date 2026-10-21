package services

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"oneimg/backend/models"
	"oneimg/backend/utils/images"

	"gorm.io/gorm"
)

var ErrAvatarNotFound = errors.New("avatar does not exist")

// 只接受服务端生成的文件名，数据库中的异常值不会被拼进路径。
var avatarFilePattern = regexp.MustCompile(`^[0-9a-f]{32}\.webp$`)

// AvatarService 把头像保存在数据目录下，不进入用户存储，也不参与配额和统计。
type AvatarService struct {
	db      *gorm.DB
	dir     string
	encoder *images.ImageService
	// mu 串行化头像字段的读改写，避免并发替换或删除用户时留下孤儿文件。
	mu sync.Mutex
}

func NewAvatarService(db *gorm.DB, dir string) *AvatarService {
	return &AvatarService{db: db, dir: dir, encoder: &images.ImageService{}}
}

// AvatarVersion 返回头像版本，用于前端地址的缓存失效；没有头像时返回空字符串。
func AvatarVersion(user models.User) string {
	if user.Avatar == nil || !avatarFilePattern.MatchString(*user.Avatar) {
		return ""
	}
	return strings.TrimSuffix(*user.Avatar, ".webp")
}

// Set 重新编码并保存新头像，成功后删除旧文件。调用方可以提前持有图像处理槽位。
func (s *AvatarService) Set(ctx context.Context, userID int, data []byte) (models.User, error) {
	encoded, err := s.encoder.EncodeAvatar(ctx, data)
	if err != nil {
		return models.User{}, err
	}
	id, err := images.NewObjectID()
	if err != nil {
		return models.User{}, err
	}
	name := id + ".webp"
	if err := s.writeFile(name, encoded); err != nil {
		return models.User{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		s.removeFile(name)
		return models.User{}, err
	}
	// 先复制旧值：GORM 更新时会改写指针指向的字符串。
	previous := ""
	if user.Avatar != nil {
		previous = *user.Avatar
	}
	result := s.db.Model(&user).Update("avatar", name)
	if result.Error == nil && result.RowsAffected == 0 {
		result.Error = gorm.ErrRecordNotFound
	}
	if result.Error != nil {
		s.removeFile(name)
		return models.User{}, result.Error
	}
	if previous != "" {
		s.removeFile(previous)
	}
	user.Avatar = &name
	return user, nil
}

// Remove 清除头像并删除文件；没有头像时直接返回当前用户。
func (s *AvatarService) Remove(userID int) (models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return models.User{}, err
	}
	if user.Avatar == nil {
		return user, nil
	}
	previous := *user.Avatar
	if err := s.db.Model(&user).Update("avatar", nil).Error; err != nil {
		return models.User{}, err
	}
	s.removeFile(previous)
	user.Avatar = nil
	return user, nil
}

// Open 返回用户头像文件和版本；用户不存在时返回 gorm.ErrRecordNotFound。
func (s *AvatarService) Open(userID int) (*os.File, string, error) {
	var user models.User
	if err := s.db.Select("id", "avatar").First(&user, userID).Error; err != nil {
		return nil, "", err
	}
	version := AvatarVersion(user)
	if version == "" {
		return nil, "", ErrAvatarNotFound
	}
	file, err := os.Open(filepath.Join(s.dir, *user.Avatar))
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", ErrAvatarNotFound
	}
	return file, version, err
}

func (s *AvatarService) writeFile(name string, data []byte) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(s.dir, name)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(path)
	}
	return err
}

func (s *AvatarService) removeFile(name string) {
	if !avatarFilePattern.MatchString(name) {
		return
	}
	if err := os.Remove(filepath.Join(s.dir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("删除头像文件失败，需核查孤儿文件 name=%q: %v", name, err)
	}
}
