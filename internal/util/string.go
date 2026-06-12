package util

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/vo"
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
	sb.WriteString(constant.TruncateSuffixPrefix)
	fmt.Fprintf(&sb, constant.FormatDecimal, len(val))
	sb.WriteString(constant.TruncateSuffixPostfix)
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
	return lo.MapValues(m, func(v any, _ string) any { return truncateValue(v, maxLen) })
}

func truncateValue(val any, maxLen int) any {
	switch v := val.(type) {
	case string:
		return TruncateFieldValue(v, maxLen)
	case map[string]any:
		return TruncateMapValues(v, maxLen)
	case []any:
		return lo.Map(v, func(item any, _ int) any { return truncateValue(item, maxLen) })
	default:
		return val
	}
}

// ExtractMessageText 从 UnifiedContent 中提取纯文本内容
//
//	@param c *vo.UnifiedContent
//	@return string
//	@author centonhuang
//	@update 2026-06-09 10:00:00
func ExtractMessageText(c *vo.UnifiedContent) string {
	if c == nil {
		return ""
	}
	if c.Text != "" {
		return c.Text
	}
	if part, found := lo.Find(c.Parts, func(p *vo.UnifiedContentPart) bool {
		return p.Type == enum.ContentPartTypeText && p.Text != ""
	}); found {
		return part.Text
	}
	return ""
}
