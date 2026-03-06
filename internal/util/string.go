package util

import (
	"encoding/base64"
	"fmt"
)

// ToDataURL 将文件转换为 data URL
//
//	@param contentType 文件类型
//	@param bytes
//	@return string
//	@author centonhuang
//	@update 2025-11-13 17:49:49
func ToDataURL(contentType string, bytes []byte) string {
	base64Data := base64.StdEncoding.EncodeToString(bytes)
	return fmt.Sprintf("data:%s;base64,%s", contentType, base64Data)
}

// MaskSecret 掩码敏感信息，保留前4和后4个字符
//
//	@param key
//	@return string
//	@author centonhuang
//	@update 2026-03-06 15:32:06
func MaskSecret(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return fmt.Sprintf("%s***%s", key[:4], key[len(key)-4:])
}
