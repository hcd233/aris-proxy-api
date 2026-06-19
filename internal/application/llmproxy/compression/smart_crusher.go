package compression

import (
	"fmt"
	"strings"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// SmartCrusher 压缩 JSON 数组：先尝试 lossless CSV，不够好则 lossy 采样。
type SmartCrusher struct {
	maxItems int
}

// NewSmartCrusher 构造 SmartCrusher。
func NewSmartCrusher() *SmartCrusher {
	return &SmartCrusher{maxItems: constant.CompressionSmartCrusherMaxItems}
}

// Compress 压缩 JSON 数组内容。
func (s *SmartCrusher) Compress(content string) ItemCompressionResult {
	original := content
	bytesBefore := len(content)

	trimmed := strings.TrimSpace(content)
	var arr []any
	if err := sonic.UnmarshalString(trimmed, &arr); err != nil {
		return passthrough(original, constant.CompressionStrategySmartCrusher)
	}
	if len(arr) == 0 {
		return passthrough(original, constant.CompressionStrategySmartCrusher)
	}

	// 只处理对象数组（map[string]any）
	objs, ok := toObjectArray(arr)
	if !ok {
		return passthrough(original, constant.CompressionStrategySmartCrusher)
	}

	// Step 1: lossless CSV
	csv := s.toCSV(objs)
	if float64(len(csv)) < float64(bytesBefore)*constant.CompressionSmartCrusherLosslessRatio {
		return ItemCompressionResult{
			Output:      csv,
			Strategy:    constant.CompressionStrategySmartCrusher,
			Applied:     true,
			BytesBefore: bytesBefore,
			BytesAfter:  len(csv),
		}
	}

	// Step 2: lossy 采样
	sampled := s.sampleRows(objs)
	if len(sampled) >= bytesBefore {
		return passthrough(original, constant.CompressionStrategySmartCrusher)
	}
	return ItemCompressionResult{
		Output:      sampled,
		Strategy:    constant.CompressionStrategySmartCrusher,
		Applied:     true,
		BytesBefore: bytesBefore,
		BytesAfter:  len(sampled),
	}
}

func (s *SmartCrusher) toCSV(arr []map[string]any) string {
	// 提取 schema（所有 key 的并集，按字典序）
	schemaSet := map[string]bool{}
	for _, obj := range arr {
		for k := range obj {
			schemaSet[k] = true
		}
	}
	cols := sortedKeys(schemaSet)

	var buf strings.Builder
	buf.WriteString(strings.Join(cols, ","))
	buf.WriteString("\n")
	for _, obj := range arr {
		row := make([]string, len(cols))
		for i, col := range cols {
			if v, ok := obj[col]; ok {
				row[i] = csvCell(v)
			}
		}
		buf.WriteString(strings.Join(row, ","))
		buf.WriteString("\n")
	}
	return buf.String()
}

func (s *SmartCrusher) sampleRows(arr []map[string]any) string {
	if len(arr) <= s.maxItems {
		// 数组不大，直接重新序列化（紧凑格式）
		out, err := sonic.Marshal(arr)
		if err != nil {
			return ""
		}
		return string(out)
	}

	keywords := strings.Split(constant.CompressionSmartCrusherErrorKeywords, ",")
	kept := make([]map[string]any, 0, s.maxItems)
	seen := map[int]bool{}

	// 保留含 error 关键词的行
	for i, obj := range arr {
		if len(kept) >= s.maxItems/2 {
			break
		}
		if hasKeyword(obj, keywords) {
			kept = append(kept, obj)
			seen[i] = true
		}
	}

	// 填充前 N 行
	headN := s.maxItems / 4
	for i := 0; i < headN && i < len(arr) && len(kept) < s.maxItems-2; i++ {
		if !seen[i] {
			kept = append(kept, arr[i])
			seen[i] = true
		}
	}

	// 填充尾部 N 行
	tailN := s.maxItems / 4
	for i := len(arr) - tailN; i < len(arr) && len(kept) < s.maxItems; i++ {
		if i >= 0 && !seen[i] {
			kept = append(kept, arr[i])
			seen[i] = true
		}
	}

	out, err := sonic.Marshal(kept)
	if err != nil {
		return ""
	}
	return fmt.Sprintf(constant.CompressionSmartCrusherOmitFormat, string(out), len(arr)-len(kept))
}

func toObjectArray(arr []any) ([]map[string]any, bool) {
	result := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, false
		}
		result = append(result, obj)
	}
	return result, true
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	return keys
}

func csvCell(v any) string {
	switch val := v.(type) {
	case string:
		if strings.ContainsAny(val, constant.CompressionCSVSpecialChars) {
			return fmt.Sprintf(`"%s"`, strings.ReplaceAll(val, `"`, `""`)) //nolint:gocritic // CSV quoting needs manual double-quote escaping, %q uses Go escaping
		}
		return val
	case nil:
		return ""
	default:
		out, err := sonic.Marshal(val)
		if err != nil {
			return ""
		}
		return string(out)
	}
}

func hasKeyword(obj map[string]any, keywords []string) bool {
	for _, v := range obj {
		s, ok := v.(string)
		if !ok {
			continue
		}
		lower := strings.ToLower(s)
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				return true
			}
		}
	}
	return false
}

func passthrough(content, strategy string) ItemCompressionResult {
	return ItemCompressionResult{
		Output:      content,
		Strategy:    strategy,
		Applied:     false,
		BytesBefore: len(content),
		BytesAfter:  len(content),
	}
}
