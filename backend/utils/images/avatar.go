package images

import (
	"context"

	"github.com/disintegration/imaging"
)

const (
	AvatarSize    = 256
	AvatarQuality = 85
	// AvatarMaxBytes 是上传原图的字节上限，与系统设置中的图片大小限制无关。
	AvatarMaxBytes = 10 << 20
)

// EncodeAvatar 把静态栅格图居中裁成正方形并重新编码为 WebP，丢弃原图的元数据。
// 动图、SVG 和超出像素预算的图片都会被拒绝。
func (s *ImageService) EncodeAvatar(ctx context.Context, data []byte) ([]byte, error) {
	ctx, release, err := AcquireProcessing(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	img, _, err := s.decodeBytes(data)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.convertToWebP(imaging.Fill(img, AvatarSize, AvatarSize, imaging.Center, imaging.Lanczos), AvatarQuality)
}
