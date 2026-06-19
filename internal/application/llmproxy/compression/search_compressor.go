package compression

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

var searchLineRegex = regexp.MustCompile(`^(\S+):(\d+):(.*)$`)

// SearchCompressor 压缩 grep/ripgrep 搜索结果：按文件分组 + 每文件截断。
type SearchCompressor struct {
	maxPerFile int
}

// NewSearchCompressor 构造 SearchCompressor。
func NewSearchCompressor() *SearchCompressor {
	return &SearchCompressor{maxPerFile: constant.CompressionSearchMaxPerFile}
}

// Compress 压缩搜索结果内容。
func (s *SearchCompressor) Compress(content string) ItemCompressionResult { //nolint:gocognit // search result compression has inherent grouping + truncation logic
	bytesBefore := len(content)
	lines := strings.Split(content, "\n")

	type match struct {
		file    string
		line    string
		content string
	}

	var matches []match
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := searchLineRegex.FindStringSubmatch(line)
		if m == nil {
			return passthrough(content, constant.CompressionStrategySearchCompressor)
		}
		matches = append(matches, match{file: m[1], line: m[2], content: m[3]})
	}

	if len(matches) == 0 {
		return passthrough(content, constant.CompressionStrategySearchCompressor)
	}

	// 按文件分组
	fileGroups := map[string][]match{}
	fileOrder := []string{}
	for _, m := range matches {
		if _, exists := fileGroups[m.file]; !exists {
			fileOrder = append(fileOrder, m.file)
		}
		fileGroups[m.file] = append(fileGroups[m.file], m)
	}

	// 检查是否需要截断
	needsTruncation := false
	for _, group := range fileGroups {
		if len(group) > s.maxPerFile {
			needsTruncation = true
			break
		}
	}
	if !needsTruncation {
		return passthrough(content, constant.CompressionStrategySearchCompressor)
	}

	var output []string
	totalMatches := 0
	for _, file := range fileOrder {
		group := fileGroups[file]
		totalMatches += len(group)
		if len(group) <= s.maxPerFile {
			for _, m := range group {
				output = append(output, fmt.Sprintf(constant.CompressionSearchLineFormat, m.file, m.line, m.content))
			}
		} else {
			// 前 maxPerFile 行
			for i := range s.maxPerFile {
				m := group[i]
				output = append(output, fmt.Sprintf(constant.CompressionSearchLineFormat, m.file, m.line, m.content))
			}
			// 后 2 行
			for i := len(group) - 2; i < len(group); i++ {
				if i > s.maxPerFile {
					m := group[i]
					output = append(output, fmt.Sprintf(constant.CompressionSearchLineFormat, m.file, m.line, m.content))
				}
			}
			output = append(output, fmt.Sprintf(constant.CompressionSearchTruncationFormat, len(group)-s.maxPerFile-2))
		}
	}
	output = append(output, fmt.Sprintf(constant.CompressionSearchSummaryFormat, len(fileOrder), totalMatches))

	result := strings.Join(output, "\n")
	if len(result) >= bytesBefore {
		return passthrough(content, constant.CompressionStrategySearchCompressor)
	}
	return ItemCompressionResult{
		Output:      result,
		Strategy:    constant.CompressionStrategySearchCompressor,
		Applied:     true,
		BytesBefore: bytesBefore,
		BytesAfter:  len(result),
	}
}
