package util

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// ImageMediaTypeExtensions maps MIME types to file extensions for image storage
var ImageMediaTypeExtensions = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// ComputeImageChecksum 计算原始图片字节的 SHA256 校验和
//
//	@param data []byte
//	@return string
//	@author centonhuang
//	@update 2026-04-07 10:00:00
func ComputeImageChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// ParseDataURL 解析 data URL 为 media type 和 base64 数据
//
//	@param dataURL string "data:image/png;base64,..."
//	@return mediaType string
//	@return base64Data string
//	@return err error
//	@author centonhuang
//	@update 2026-04-07 10:00:00
func ParseDataURL(dataURL string) (mediaType, base64Data string, err error) {
	parts := strings.SplitN(dataURL, ";base64,", 2)
	if len(parts) != 2 {
		return "", "", ierr.New(ierr.ErrBadRequest, "invalid data URL format")
	}
	mediaType = strings.TrimPrefix(parts[0], "data:")
	base64Data = parts[1]
	if mediaType == "" || base64Data == "" {
		return "", "", ierr.New(ierr.ErrBadRequest, "empty media type or data in data URL")
	}
	return mediaType, base64Data, nil
}
