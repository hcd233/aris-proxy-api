package compression

import (
	"regexp"
	"strings"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// ContentDetector 通过正则检测 tool output 的内容类型，无 ML、无 I/O。
type ContentDetector struct {
	searchPattern   *regexp.Regexp
	diffHeaderRegex *regexp.Regexp
	logLevelRegex   *regexp.Regexp
	codeRegexes     map[string]*regexp.Regexp
	htmlTagRegex    *regexp.Regexp
}

// NewContentDetector 构造 ContentDetector。
func NewContentDetector() *ContentDetector {
	return &ContentDetector{
		searchPattern:   regexp.MustCompile(`(?m)^\S+:\d+:`),
		diffHeaderRegex: regexp.MustCompile(`(?m)^(diff --git|--- a/|\+\+\+ b/|@@\s+-\d+,\d+\s+\+\d+,\d+\s+@@)`),
		logLevelRegex:   regexp.MustCompile(`(?i)\b(ERROR|WARN|FATAL|PANIC|INFO|DEBUG|TRACE)\b`),
		codeRegexes: map[string]*regexp.Regexp{
			constant.CompressionLangGo:     regexp.MustCompile(`(?m)^\s*(func|type|package|import)\s+\w+`),
			constant.CompressionLangPython: regexp.MustCompile(`(?m)^\s*(def|class|import|from)\s+\w+`),
			constant.CompressionLangRust:   regexp.MustCompile(`(?m)^\s*(fn|struct|enum|impl|mod|use|pub)\s+\w+`),
			constant.CompressionLangJava:   regexp.MustCompile(`(?m)^\s*(public|private|protected)\s+(class|interface)`),
			constant.CompressionLangJS:     regexp.MustCompile(`(?m)^\s*(function|const|let|var|class|import)\s+`),
		},
		htmlTagRegex: regexp.MustCompile(`<[^>]+>`),
	}
}

// Detect 检测内容类型。
func (d *ContentDetector) Detect(content string) enum.ContentType {
	if content == "" {
		return enum.ContentTypePlainText
	}

	// 1. JSON 数组
	if ct := d.detectJsonArray(content); ct != enum.ContentTypePlainText {
		return ct
	}

	// 2. 搜索结果（file:line: 格式，至少 2 行匹配）
	if d.countMatches(d.searchPattern, content) >= 2 {
		return enum.ContentTypeSearchResults
	}

	// 3. Git diff
	if d.diffHeaderRegex.MatchString(content) {
		return enum.ContentTypeGitDiff
	}

	// 4. 构建日志（含日志级别关键词 + 多行）
	if d.logLevelRegex.MatchString(content) && strings.Count(content, "\n") >= 2 {
		return enum.ContentTypeBuildOutput
	}

	// 5. 源码
	if d.detectCode(content) {
		return enum.ContentTypeSourceCode
	}

	// 6. HTML
	if d.htmlTagRegex.MatchString(content) && strings.Count(content, "<") >= 3 {
		return enum.ContentTypeHtml
	}

	return enum.ContentTypePlainText
}

func (d *ContentDetector) detectJsonArray(content string) enum.ContentType {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "[") {
		return enum.ContentTypePlainText
	}
	var arr []any
	if err := sonic.UnmarshalString(trimmed, &arr); err != nil {
		return enum.ContentTypePlainText
	}
	if len(arr) == 0 {
		return enum.ContentTypePlainText
	}
	return enum.ContentTypeJsonArray
}

func (d *ContentDetector) detectCode(content string) bool {
	for _, re := range d.codeRegexes {
		if re.MatchString(content) {
			return true
		}
	}
	return false
}

func (d *ContentDetector) countMatches(re *regexp.Regexp, content string) int {
	return len(re.FindAllString(content, -1))
}
