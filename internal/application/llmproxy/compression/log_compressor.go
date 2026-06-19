package compression

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

var (
	logTimestampRegex = regexp.MustCompile(`\d{4}[-/]\d{2}[-/]\d{2}[ T]\d{2}:\d{2}:\d{2}(\.\d+)?`)
	logNumberRegex    = regexp.MustCompile(`\b\d+\b`)
	logPathRegex      = regexp.MustCompile(`(/[\w./-]+)|(\w:\\[\w\\.-]+)`)
	logHexIDRegex     = regexp.MustCompile(`\b[0-9a-f]{8,}\b`)
)

// LogCompressor 压缩日志：模板提取 + 按级别保留 + 去重。
type LogCompressor struct{}

// NewLogCompressor 构造 LogCompressor。
func NewLogCompressor() *LogCompressor {
	return &LogCompressor{}
}

// Compress 压缩日志内容。
func (l *LogCompressor) Compress(content string) ItemCompressionResult {
	bytesBefore := len(content)
	lines := strings.Split(content, "\n")
	if len(lines) <= 3 {
		return passthrough(content, constant.CompressionStrategyLogCompressor)
	}

	keepLevels := strings.Split(constant.CompressionLogKeepLevels, ",")
	var output []string
	infoTemplates := map[string]int{}

	for _, line := range lines {
		upper := strings.ToUpper(line)
		if containsAny(upper, keepLevels) {
			// ERROR/WARN/FATAL 行全部保留
			output = append(output, line)
		} else {
			// INFO/DEBUG 行按模板去重
			tmpl := templateize(line)
			infoTemplates[tmpl]++
			if infoTemplates[tmpl] == 1 && len(filterUnique(output, tmpl)) < constant.CompressionLogMaxInfoLines {
				output = append(output, line)
			}
		}
	}

	// 附加统计摘要
	dupCount := 0
	for _, count := range infoTemplates {
		if count > 1 {
			dupCount += count - 1
		}
	}
	if dupCount > 0 {
		output = append(output, fmt.Sprintf(constant.CompressionLogDedupFormat, dupCount))
	}

	result := strings.Join(output, "\n")
	if len(result) >= bytesBefore {
		return passthrough(content, constant.CompressionStrategyLogCompressor)
	}
	return ItemCompressionResult{
		Output:      result,
		Strategy:    constant.CompressionStrategyLogCompressor,
		Applied:     true,
		BytesBefore: bytesBefore,
		BytesAfter:  len(result),
	}
}

func templateize(line string) string {
	s := logTimestampRegex.ReplaceAllString(line, constant.CompressionLogTemplateTS)
	s = logPathRegex.ReplaceAllString(s, constant.CompressionLogTemplatePATH)
	s = logHexIDRegex.ReplaceAllString(s, constant.CompressionLogTemplateID)
	s = logNumberRegex.ReplaceAllString(s, constant.CompressionLogTemplateN)
	return s
}

func containsAny(s string, substrs []string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func filterUnique(lines []string, tmpl string) []string {
	return lines // placeholder; dedup logic handled by map above
}
