package util

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
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
	return fmt.Sprintf(constant.DataURLTemplate, contentType, base64Data)
}

// MaskSecret 掩码敏感信息，保留前4和后4个字符
//
//	@param key
//	@return string
//	@author centonhuang
//	@update 2026-03-06 15:32:06
func MaskSecret(key string) string {
	if len(key) <= 8 {
		return constant.MaskSecretPlaceholder
	}
	return fmt.Sprintf("%s***%s", key[:4], key[len(key)-4:])
}

// TruncateFieldValue 截断过长的字符串值，保留前 maxLen 字符并附加截断信息
//
//	@param val 原始值
//	@param maxLen 最大长度
//	@return string 截断后的字符串
//	@author centonhuang
//	@update 2026-04-09 15:00:00
func TruncateFieldValue(val string, maxLen int) string {
	if len(val) <= maxLen {
		return val
	}
	var sb strings.Builder
	sb.WriteString(val[:maxLen])
	sb.WriteString("...(truncated, total ")
	fmt.Fprintf(&sb, "%d", len(val))
	sb.WriteString(" chars)")
	return sb.String()
}

// TruncateMapValues 递归截断 map 中过长的字符串值
//
//	@param m 原始 map
//	@param maxLen 字符串最大长度
//	@return map[string]any 截断后的 map
//	@author centonhuang
//	@update 2026-04-09 15:00:00
func TruncateMapValues(m map[string]any, maxLen int) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = truncateValue(v, maxLen)
	}
	return result
}

func truncateValue(val any, maxLen int) any {
	switch v := val.(type) {
	case string:
		return TruncateFieldValue(v, maxLen)
	case map[string]any:
		return TruncateMapValues(v, maxLen)
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = truncateValue(item, maxLen)
		}
		return result
	default:
		return val
	}
}
