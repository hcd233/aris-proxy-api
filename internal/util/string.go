package util

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"

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

// TruncateFieldValue 截断过长的字符串值，保留前 maxLen 字节并附加截断信息
//
// maxLen 是字节上限；截断点回退到 rune 边界，避免把中文/emoji 切成非法 UTF-8
// （否则日志里是乱码，JSON 编码器还会把残字节替换成 U+FFFD）。
//
//	@param val 原始值
//	@param maxLen 最大字节数
//	@return string 截断后的字符串
//	@author centonhuang
//	@update 2026-10-22 20:00:00
func TruncateFieldValue(val string, maxLen int) string {
	if len(val) <= maxLen {
		return val
	}
	cut := maxLen
	for cut > 0 && !utf8.RuneStart(val[cut]) {
		cut--
	}
	var sb strings.Builder
	sb.WriteString(val[:cut])
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

// SafeSortField 校验排序字段是否仅由 [a-zA-Z0-9_] 构成，防止 ORDER BY SQL 注入。
// 合法则原样返回，否则返回空字符串。
//
//	@param field string
//	@return string
//	@author centonhuang
//	@update 2026-06-24 19:00:00
func SafeSortField(field string) string {
	if field == "" {
		return ""
	}
	if lo.EveryBy([]rune(field), func(c rune) bool {
		return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
	}) {
		return field
	}
	return ""
}

// likePatternEscaper 预编译的 LIKE/ILIKE 通配符转义器（单遍替换，不会递归转义）。
var likePatternEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// EscapeLikePattern 转义 SQL LIKE/ILIKE 模式中的通配符（\ % _），让用户输入按字面匹配。
// 需配合 SQL 侧 ESCAPE '\' 子句（PG 的 LIKE 默认转义符就是反斜杠，显式声明防语义丢失）。
//
//	@param s string 用户输入的搜索词
//	@return string
//	@author centonhuang
//	@update 2026-10-16 16:00:00
func EscapeLikePattern(s string) string {
	return likePatternEscaper.Replace(s)
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

// NormalizeKeyword 归一化 keyword（trim + 小写），对齐 ILIKE 的大小写不敏感语义。
func NormalizeKeyword(keyword string) string {
	return strings.ToLower(strings.TrimSpace(keyword))
}

// KeywordCacheKeyFragment 归一化 keyword 并取 SHA-256 摘要作为缓存 key 片段。
// 摘要对齐 modelListCacheKey 的键构造风格：用户输入不原样进 Redis key，
// 键长固定且无特殊字符。
func KeywordCacheKeyFragment(keyword string) string {
	sum := sha256.Sum256([]byte(NormalizeKeyword(keyword)))
	return hex.EncodeToString(sum[:])
}

// KeywordLikePattern 把 keyword 归一化为 LIKE/ILIKE 模式：归一化 + 通配符转义 + 两侧 %。
//
// 必须与 KeywordCacheKeyFragment 共用 NormalizeKeyword：命中集缓存按归一化值共享，
// 模式与 key 不同源时同一缓存条目会承载不同结果集（如 "  foo " 与 "foo"）。
func KeywordLikePattern(keyword string) string {
	return "%" + EscapeLikePattern(NormalizeKeyword(keyword)) + "%"
}
