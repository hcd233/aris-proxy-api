# LLM 上下文压缩实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 aris-proxy-api 的 usecase 层嵌入 tool output 压缩管线，复刻 headroom 的确定性压缩算法（SmartCrusher/LogCompressor/SearchCompressor），支持 OpenAI Chat / Anthropic Messages / OpenAI Responses 三种协议路径。

**Architecture:** 分层管道式——ContentDetector 检测内容类型 → Dispatcher 路由到对应 Compressor → ToolOutputLocator 按协议定位 body 中的 tool output 并替换。压缩在 `MarshalXxxBodyForModel` 之后、`ForwardXxx` 之前执行。任何压缩异常回退原始 body。

**Tech Stack:** Go 1.25 / sonic JSON / samber-lo / Viper 配置 / 标准库 testing

**Spec:** `docs/superpowers/specs/2026-06-18-llm-context-compression-design.md`

---

## 文件结构

### 新建文件

| 文件 | 职责 |
|------|------|
| `internal/application/llmproxy/compression/result.go` | `ContentType` 枚举、`ItemCompressionResult`、`CompressionStats` |
| `internal/application/llmproxy/compression/detector.go` | `ContentDetector`：纯正则内容类型检测 |
| `internal/application/llmproxy/compression/smart_crusher.go` | JSON 数组 → lossless CSV / lossy 采样 |
| `internal/application/llmproxy/compression/log_compressor.go` | 日志模板提取 + 去噪 |
| `internal/application/llmproxy/compression/search_compressor.go` | grep 结果去重 + 摘要 |
| `internal/application/llmproxy/compression/passthrough.go` | 兜底 passthrough 压缩器 |
| `internal/application/llmproxy/compression/compressor.go` | `Compressor` 接口 + `Dispatcher` |
| `internal/application/llmproxy/compression/locator.go` | `ToolOutputLocator` 接口 + 通用入口 `CompressBody` |
| `internal/application/llmproxy/compression/locator_openai.go` | OpenAI Chat: 扫描 `messages[role=tool]` |
| `internal/application/llmproxy/compression/locator_anthropic.go` | Anthropic: 扫描 `content[type=tool_result]` |
| `internal/application/llmproxy/compression/locator_responses.go` | OpenAI Responses: 扫描 `input[type=function_call_output]` |
| `test/unit/compression/detector_test.go` | ContentDetector 单元测试 |
| `test/unit/compression/smart_crusher_test.go` | SmartCrusher 单元测试 |
| `test/unit/compression/log_compressor_test.go` | LogCompressor 单元测试 |
| `test/unit/compression/search_compressor_test.go` | SearchCompressor 单元测试 |
| `test/unit/compression/dispatcher_test.go` | Dispatcher 路由测试 |
| `test/unit/compression/locator_openai_test.go` | OpenAI Chat Locator 测试 |
| `test/unit/compression/locator_anthropic_test.go` | Anthropic Locator 测试 |
| `test/unit/compression/locator_responses_test.go` | OpenAI Responses Locator 测试 |
| `test/unit/compression/fixtures/*.json` | 测试夹具 |
| `test/e2e/compression/compression_test.go` | 端到端测试 |

### 修改文件

| 文件 | 改动 |
|------|------|
| `internal/config/config.go` | 新增 `CompressionEnabled` / `CompressionMinBodyBytes` / `CompressionMinToolOutputBytes` |
| `env/api.env.template` | 新增 3 个压缩环境变量 |
| `internal/dto/asynctask.go` | `ModelCallAuditTask` 增加 3 个压缩字段 + `SetCompressionStats` 方法 |
| `internal/application/llmproxy/usecase/openai.go` | `openAIUseCase` 增加 `compressor` 字段 + `compressBodyIfNeeded` |
| `internal/application/llmproxy/usecase/anthropic.go` | `anthropicUseCase` 同上 |
| `internal/application/llmproxy/usecase/openai_chat.go` | forward 方法插入压缩调用 + 传 stats 到 audit |
| `internal/application/llmproxy/usecase/openai_response.go` | 同上 |
| `internal/application/llmproxy/usecase/anthropic_message.go` | 同上 |
| `internal/bootstrap/modules/application.go` | DI 注册压缩组件 |

---

## Task 1: 基础类型 + ContentDetector + Passthrough

**Files:**
- Create: `internal/application/llmproxy/compression/result.go`
- Create: `internal/application/llmproxy/compression/detector.go`
- Create: `internal/application/llmproxy/compression/passthrough.go`
- Test: `test/unit/compression/detector_test.go`

- [ ] **Step 1: 写 result.go**

```go
// Package compression 提供 LLM 代理请求体中 tool output 的上下文压缩能力。
//
// 复刻 headroom 项目的确定性压缩算法（跳过 ML 模型），在 marshal body 之后、
// 转发上游之前执行。任何压缩异常回退原始 body，不影响请求正常转发。
package compression

// ContentType 工具输出的内容类型。
type ContentType int

const (
	ContentTypeJsonArray ContentType = iota
	ContentTypeSearchResults
	ContentTypeBuildOutput
	ContentTypeSourceCode
	ContentTypeGitDiff
	ContentTypeHtml
	ContentTypePlainText
)

func (c ContentType) String() string {
	switch c {
	case ContentTypeJsonArray:
		return "json_array"
	case ContentTypeSearchResults:
		return "search_results"
	case ContentTypeBuildOutput:
		return "build_output"
	case ContentTypeSourceCode:
		return "source_code"
	case ContentTypeGitDiff:
		return "git_diff"
	case ContentTypeHtml:
		return "html"
	default:
		return "plain_text"
	}
}

// ItemCompressionResult 单个 tool output 的压缩结果。
type ItemCompressionResult struct {
	Output      string // 压缩后内容（或跳过/失败时的原始内容）
	Strategy    string // 策略名（"smart_crusher"/"log_compressor"/"search_compressor"/"passthrough"）
	Applied     bool   // 是否实际执行了压缩
	BytesBefore int    // len(原始内容)
	BytesAfter  int    // len(Output)
}

// CompressionStats 一个请求的聚合压缩统计。
type CompressionStats struct {
	BytesBefore     int
	BytesAfter      int
	ItemsCompressed int
	ItemsSkipped    int
	StrategiesUsed  []string
}

func (s *CompressionStats) addItem(r ItemCompressionResult) {
	s.BytesBefore += r.BytesBefore
	s.BytesAfter += r.BytesAfter
	if r.Applied {
		s.ItemsCompressed++
		if s.StrategiesUsed == nil {
			s.StrategiesUsed = []string{}
		}
		s.StrategiesUsed = append(s.StrategiesUsed, r.Strategy)
	} else {
		s.ItemsSkipped++
	}
}
```

- [ ] **Step 2: 写 detector.go**

```go
package compression

import (
	"regexp"
	"strings"

	"github.com/bytedance/sonic"
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
		searchPattern: regexp.MustCompile(`(?m)^\S+:\d+:`),
		diffHeaderRegex: regexp.MustCompile(`(?m)^(diff --git|--- a/|\+\+\+ b/|@@\s+-\d+,\d+\s+\+\d+,\d+\s+@@)`),
		logLevelRegex:  regexp.MustCompile(`(?i)\b(ERROR|WARN|FATAL|PANIC|INFO|DEBUG|TRACE)\b`),
		codeRegexes: map[string]*regexp.Regexp{
			"go":       regexp.MustCompile(`(?m)^\s*(func|type|package|import)\s+\w+`),
			"python":   regexp.MustCompile(`(?m)^\s*(def|class|import|from)\s+\w+`),
			"rust":     regexp.MustCompile(`(?m)^\s*(fn|struct|enum|impl|mod|use|pub)\s+\w+`),
			"java":     regexp.MustCompile(`(?m)^\s*(public|private|protected)\s+(class|interface)`),
			"js":       regexp.MustCompile(`(?m)^\s*(function|const|let|var|class|import)\s+`),
		},
		htmlTagRegex: regexp.MustCompile(`<[^>]+>`),
	}
}

// Detect 检测内容类型。
func (d *ContentDetector) Detect(content string) ContentType {
	if content == "" {
		return ContentTypePlainText
	}

	// 1. JSON 数组
	if ct := d.detectJsonArray(content); ct != ContentTypePlainText {
		return ct
	}

	// 2. 搜索结果（file:line: 格式，至少 2 行匹配）
	if d.countMatches(d.searchPattern, content) >= 2 {
		return ContentTypeSearchResults
	}

	// 3. Git diff
	if d.diffHeaderRegex.MatchString(content) {
		return ContentTypeGitDiff
	}

	// 4. 构建日志（含日志级别关键词 + 多行）
	if d.logLevelRegex.MatchString(content) && strings.Count(content, "\n") >= 2 {
		return ContentTypeBuildOutput
	}

	// 5. 源码
	if d.detectCode(content) {
		return ContentTypeSourceCode
	}

	// 6. HTML
	if d.htmlTagRegex.MatchString(content) && strings.Count(content, "<") >= 3 {
		return ContentTypeHtml
	}

	return ContentTypePlainText
}

func (d *ContentDetector) detectJsonArray(content string) ContentType {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "[") {
		return ContentTypePlainText
	}
	var arr []any
	if err := sonic.UnmarshalString(trimmed, &arr); err != nil {
		return ContentTypePlainText
	}
	if len(arr) == 0 {
		return ContentTypePlainText
	}
	return ContentTypeJsonArray
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
```

- [ ] **Step 3: 写 passthrough.go**

```go
package compression

// PassthroughCompressor 不压缩，原样返回内容。
type PassthroughCompressor struct{}

// Compress 返回原始内容，Applied=false。
func (p *PassthroughCompressor) Compress(content string) ItemCompressionResult {
	return ItemCompressionResult{
		Output:      content,
		Strategy:    "passthrough",
		Applied:     false,
		BytesBefore: len(content),
		BytesAfter:  len(content),
	}
}

// NewPassthroughCompressor 构造 PassthroughCompressor。
func NewPassthroughCompressor() *PassthroughCompressor {
	return &PassthroughCompressor{}
}
```

- [ ] **Step 4: 写 detector_test.go**

```go
package compression

import "testing"

func TestDetectJsonArray(t *testing.T) {
	d := NewContentDetector()
	content := `[{"name":"error","code":500},{"name":"warn","code":0}]`
	if got := d.Detect(content); got != ContentTypeJsonArray {
		t.Errorf("Detect() = %v, want JsonArray", got)
	}
}

func TestDetectSearchResults(t *testing.T) {
	d := NewContentDetector()
	content := "src/main.go:42:func main() {\nsrc/main.go:43:    fmt.Println()\nsrc/utils.go:10:func helper() {"
	if got := d.Detect(content); got != ContentTypeSearchResults {
		t.Errorf("Detect() = %v, want SearchResults", got)
	}
}

func TestDetectBuildOutput(t *testing.T) {
	d := NewContentDetector()
	content := "[2024-01-01 10:00:00] INFO Starting server\n[2024-01-01 10:00:01] ERROR Connection failed\n[2024-01-01 10:00:02] WARN Retrying"
	if got := d.Detect(content); got != ContentTypeBuildOutput {
		t.Errorf("Detect() = %v, want BuildOutput", got)
	}
}

func TestDetectGitDiff(t *testing.T) {
	d := NewContentDetector()
	content := "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1,3 +1,4 @@"
	if got := d.Detect(content); got != ContentTypeGitDiff {
		t.Errorf("Detect() = %v, want GitDiff", got)
	}
}

func TestDetectSourceCode(t *testing.T) {
	d := NewContentDetector()
	content := "package main\n\nfunc main() {\n    fmt.Println(\"hello\")\n}"
	if got := d.Detect(content); got != ContentTypeSourceCode {
		t.Errorf("Detect() = %v, want SourceCode", got)
	}
}

func TestDetectPlainText(t *testing.T) {
	d := NewContentDetector()
	content := "This is just a plain text message without any special format."
	if got := d.Detect(content); got != ContentTypePlainText {
		t.Errorf("Detect() = %v, want PlainText", got)
	}
}

func TestDetectEmpty(t *testing.T) {
	d := NewContentDetector()
	if got := d.Detect(""); got != ContentTypePlainText {
		t.Errorf("Detect(\"\") = %v, want PlainText", got)
	}
}

func TestPassthroughCompressor(t *testing.T) {
	p := NewPassthroughCompressor()
	result := p.Compress("hello world")
	if result.Applied {
		t.Error("Passthrough should not apply compression")
	}
	if result.Output != "hello world" {
		t.Errorf("Output = %q, want %q", result.Output, "hello world")
	}
}
```

- [ ] **Step 5: 运行测试**

Run: `go test -v -count=1 ./test/unit/compression/ -run "TestDetect|TestPassthrough"`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add internal/application/llmproxy/compression/ test/unit/compression/detector_test.go
git commit -m "feat: add compression foundation types and content detector"
```

---

## Task 2: SmartCrusher（JSON 数组压缩）

**Files:**
- Create: `internal/application/llmproxy/compression/smart_crusher.go`
- Test: `test/unit/compression/smart_crusher_test.go`

- [ ] **Step 1: 写 smart_crusher.go**

```go
package compression

import (
	"fmt"
	"strings"

	"github.com/bytedance/sonic"
)

const (
	smartCrusherStrategy       = "smart_crusher"
	smartCrusherLosslessRatio  = 0.7
	smartCrusherMaxItems       = 20
	smartCrusherErrorKeywords  = "error,exception,fail,fatal,panic,critical,timeout"
)

// SmartCrusher 压缩 JSON 数组：先尝试 lossless CSV，不够好则 lossy 采样。
type SmartCrusher struct {
	maxItems int
}

// NewSmartCrusher 构造 SmartCrusher。
func NewSmartCrusher() *SmartCrusher {
	return &SmartCrusher{maxItems: smartCrusherMaxItems}
}

// Compress 压缩 JSON 数组内容。
func (s *SmartCrusher) Compress(content string) ItemCompressionResult {
	original := content
	bytesBefore := len(content)

	trimmed := strings.TrimSpace(content)
	var arr []any
	if err := sonic.UnmarshalString(trimmed, &arr); err != nil {
		return passthrough(original, smartCrusherStrategy)
	}
	if len(arr) == 0 {
		return passthrough(original, smartCrusherStrategy)
	}

	// 只处理对象数组（map[string]any）
	objArray, ok := toObjectArray(arr)
	if !ok {
		return passthrough(original, smartCrusherStrategy)
	}

	// Step 1: lossless CSV
	csv := s.toCSV(objArray)
	if float64(len(csv)) < float64(bytesBefore)*smartCrusherLosslessRatio {
		return ItemCompressionResult{
			Output:      csv,
			Strategy:    smartCrusherStrategy,
			Applied:     true,
			BytesBefore: bytesBefore,
			BytesAfter:  len(csv),
		}
	}

	// Step 2: lossy 采样
	sampled := s.sampleRows(objArray)
	if len(sampled) >= bytesBefore {
		return passthrough(original, smartCrusherStrategy)
	}
	return ItemCompressionResult{
		Output:      sampled,
		Strategy:    smartCrusherStrategy,
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

	keywords := strings.Split(smartCrusherErrorKeywords, ",")
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
	return fmt.Sprintf("%s\n...省略 %d 行...", string(out), len(arr)-len(kept))
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
		if strings.ContainsAny(val, ",\"\n") {
			return fmt.Sprintf(`"%s"`, strings.ReplaceAll(val, `"`, `""`))
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
```

- [ ] **Step 2: 写 smart_crusher_test.go**

```go
package compression

import (
	"strings"
	"testing"
)

func TestSmartCrusherLosslessCSV(t *testing.T) {
	sc := NewSmartCrusher()
	content := `[{"name":"error","code":500},{"name":"warn","code":0},{"name":"info","code":200}]`
	result := sc.Compress(content)
	if !result.Applied {
		t.Fatal("expected compression to be applied")
	}
	if !strings.HasPrefix(result.Output, "code,name\n") {
		t.Errorf("expected CSV header, got: %s", result.Output[:20])
	}
	if result.BytesAfter >= result.BytesBefore {
		t.Errorf("expected bytes to decrease: before=%d after=%d", result.BytesBefore, result.BytesAfter)
	}
}

func TestSmartCrusherLossySampling(t *testing.T) {
	sc := NewSmartCrusher()
	// 构造 50 个对象的数组
	var items []map[string]any
	for i := 0; i < 50; i++ {
		items = append(items, map[string]any{
			"id":    i,
			"name":  "item_" + strings.Repeat("x", 20),
			"value": "some long value that makes the json verbose",
		})
	}
	content := mustJSON(items)
	result := sc.Compress(content)
	if !result.Applied {
		t.Fatal("expected compression to be applied for large array")
	}
	if result.BytesAfter >= result.BytesBefore {
		t.Errorf("expected bytes to decrease: before=%d after=%d", result.BytesBefore, result.BytesAfter)
	}
	if !strings.Contains(result.Output, "省略") {
		t.Error("expected sampling summary in output")
	}
}

func TestSmartCrusherNonArrayPassthrough(t *testing.T) {
	sc := NewSmartCrusher()
	content := `{"key": "value"}`
	result := sc.Compress(content)
	if result.Applied {
		t.Error("should not compress non-array JSON")
	}
}

func TestSmartCrusherEmptyArray(t *testing.T) {
	sc := NewSmartCrusher()
	result := sc.Compress("[]")
	if result.Applied {
		t.Error("should not compress empty array")
	}
}

func mustJSON(v any) string {
	out, err := sonic.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(out)
}
```

需要在 test 文件顶部加 sonic import:
```go
import (
	"strings"
	"testing"

	"github.com/bytedance/sonic"
)
```

- [ ] **Step 3: 运行测试**

Run: `go test -v -count=1 ./test/unit/compression/ -run TestSmartCrusher`
Expected: PASS

- [ ] **Step 4: 提交**

```bash
git add internal/application/llmproxy/compression/smart_crusher.go test/unit/compression/smart_crusher_test.go
git commit -m "feat: add SmartCrusher JSON array compressor"
```

---

## Task 3: LogCompressor（日志模板提取 + 去噪）

**Files:**
- Create: `internal/application/llmproxy/compression/log_compressor.go`
- Test: `test/unit/compression/log_compressor_test.go`

- [ ] **Step 1: 写 log_compressor.go**

```go
package compression

import (
	"regexp"
	"strings"
)

const (
	logCompressorStrategy = "log_compressor"
	logKeepLevels          = "ERROR,WARN,FATAL,PANIC,CRITICAL"
	logMaxInfoLines        = 3
)

var (
	logTimestampRegex = regexp.MustCompile(`\d{4}[-/]\d{2}[-/]\d{2}[\s T]\d{2}:\d{2}:\d{2}(\.\d+)?`)
	logNumberRegex    = regexp.MustCompile(`\b\d+\b`)
	logPathRegex      = regexp.MustCompile(`(/[\w./-]+)|([\w]:\\[\w\\.-]+)`)
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
		return passthrough(content, logCompressorStrategy)
	}

	keepLevels := strings.Split(logKeepLevels, ",")
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
			if infoTemplates[tmpl] == 1 && len(filterUnique(output, tmpl)) < logMaxInfoLines {
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
		output = append(output, fmt.Sprintf("...(去重 %d 行重复日志)...", dupCount))
	}

	result := strings.Join(output, "\n")
	if len(result) >= bytesBefore {
		return passthrough(content, logCompressorStrategy)
	}
	return ItemCompressionResult{
		Output:      result,
		Strategy:    logCompressorStrategy,
		Applied:     true,
		BytesBefore: bytesBefore,
		BytesAfter:  len(result),
	}
}

func templateize(line string) string {
	s := logTimestampRegex.ReplaceAllString(line, "<TS>")
	s = logPathRegex.ReplaceAllString(s, "<PATH>")
	s = logHexIDRegex.ReplaceAllString(s, "<ID>")
	s = logNumberRegex.ReplaceAllString(s, "<N>")
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
```

> 注意：`log_compressor.go` 需要加 `"fmt"` 到 import。

- [ ] **Step 2: 写 log_compressor_test.go**

```go
package compression

import (
	"strings"
	"testing"
)

func TestLogCompressorDedup(t *testing.T) {
	lc := NewLogCompressor()
	content := strings.Join([]string{
		"[2024-01-01 10:00:00] INFO Starting server on port 8080",
		"[2024-01-01 10:00:01] INFO Database connection established",
		"[2024-01-01 10:00:02] INFO Cache warmed with 500 entries",
		"[2024-01-01 10:00:03] INFO Request processed in 42ms",
		"[2024-01-01 10:00:04] INFO Request processed in 38ms",
		"[2024-01-01 10:00:05] INFO Request processed in 51ms",
		"[2024-01-01 10:00:06] ERROR Connection refused to database:5432",
		"[2024-01-01 10:00:07] WARN Retrying connection attempt 1",
		"[2024-01-01 10:00:08] INFO Request processed in 45ms",
	}, "\n")
	result := lc.Compress(content)
	if !result.Applied {
		t.Fatal("expected compression to be applied")
	}
	if !strings.Contains(result.Output, "ERROR") {
		t.Error("ERROR line should be preserved")
	}
	if !strings.Contains(result.Output, "WARN") {
		t.Error("WARN line should be preserved")
	}
	if result.BytesAfter >= result.BytesBefore {
		t.Errorf("expected bytes to decrease: before=%d after=%d", result.BytesBefore, result.BytesAfter)
	}
}

func TestLogCompressorShortPassthrough(t *testing.T) {
	lc := NewLogCompressor()
	content := "only one line"
	result := lc.Compress(content)
	if result.Applied {
		t.Error("short content should passthrough")
	}
}
```

- [ ] **Step 3: 运行测试**

Run: `go test -v -count=1 ./test/unit/compression/ -run TestLogCompressor`
Expected: PASS

- [ ] **Step 4: 提交**

```bash
git add internal/application/llmproxy/compression/log_compressor.go test/unit/compression/log_compressor_test.go
git commit -m "feat: add LogCompressor with template extraction and dedup"
```

---

## Task 4: SearchCompressor（grep 结果去重 + 摘要）

**Files:**
- Create: `internal/application/llmproxy/compression/search_compressor.go`
- Test: `test/unit/compression/search_compressor_test.go`

- [ ] **Step 1: 写 search_compressor.go**

```go
package compression

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	searchCompressorStrategy = "search_compressor"
	searchMaxPerFile         = 5
)

var searchLineRegex = regexp.MustCompile(`^(\S+):(\d+):(.*)$`)

// SearchCompressor 压缩 grep/ripgrep 搜索结果：按文件分组 + 每文件截断。
type SearchCompressor struct {
	maxPerFile int
}

// NewSearchCompressor 构造 SearchCompressor。
func NewSearchCompressor() *SearchCompressor {
	return &SearchCompressor{maxPerFile: searchMaxPerFile}
}

// Compress 压缩搜索结果内容。
func (s *SearchCompressor) Compress(content string) ItemCompressionResult {
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
			return passthrough(content, searchCompressorStrategy)
		}
		matches = append(matches, match{file: m[1], line: m[2], content: m[3]})
	}

	if len(matches) == 0 {
		return passthrough(content, searchCompressorStrategy)
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
		return passthrough(content, searchCompressorStrategy)
	}

	var output []string
	totalMatches := 0
	for _, file := range fileOrder {
		group := fileGroups[file]
		totalMatches += len(group)
		if len(group) <= s.maxPerFile {
			for _, m := range group {
				output = append(output, fmt.Sprintf("%s:%s:%s", m.file, m.line, m.content))
			}
		} else {
			// 前 maxPerFile 行
			for i := 0; i < s.maxPerFile; i++ {
				m := group[i]
				output = append(output, fmt.Sprintf("%s:%s:%s", m.file, m.line, m.content))
			}
			// 后 2 行
			for i := len(group) - 2; i < len(group); i++ {
				if i > s.maxPerFile {
					m := group[i]
					output = append(output, fmt.Sprintf("%s:%s:%s", m.file, m.line, m.content))
				}
			}
			output = append(output, fmt.Sprintf("  ...(省略 %d 行)...", len(group)-s.maxPerFile-2))
		}
	}
	output = append(output, fmt.Sprintf("共 %d 个文件, %d 处匹配", len(fileOrder), totalMatches))

	result := strings.Join(output, "\n")
	if len(result) >= bytesBefore {
		return passthrough(content, searchCompressorStrategy)
	}
	return ItemCompressionResult{
		Output:      result,
		Strategy:    searchCompressorStrategy,
		Applied:     true,
		BytesBefore: bytesBefore,
		BytesAfter:  len(result),
	}
}
```

- [ ] **Step 2: 写 search_compressor_test.go**

```go
package compression

import (
	"fmt"
	"strings"
	"testing"
)

func TestSearchCompressorTruncate(t *testing.T) {
	sc := NewSearchCompressor()
	var lines []string
	for i := 1; i <= 20; i++ {
		lines = append(lines, fmt.Sprintf("src/main.go:%d:    some code line number %d", i, i))
	}
	content := strings.Join(lines, "\n")
	result := sc.Compress(content)
	if !result.Applied {
		t.Fatal("expected compression to be applied")
	}
	if !strings.Contains(result.Output, "省略") {
		t.Error("expected truncation summary")
	}
	if !strings.Contains(result.Output, "共 1 个文件") {
		t.Error("expected file count summary")
	}
	if result.BytesAfter >= result.BytesBefore {
		t.Errorf("expected bytes to decrease: before=%d after=%d", result.BytesBefore, result.BytesAfter)
	}
}

func TestSearchCompressorSmallPassthrough(t *testing.T) {
	sc := NewSearchCompressor()
	content := "src/main.go:42:func main() {\nsrc/utils.go:10:func helper() {"
	result := sc.Compress(content)
	if result.Applied {
		t.Error("small result should passthrough")
	}
}

func TestSearchCompressorNonSearchPassthrough(t *testing.T) {
	sc := NewSearchCompressor()
	content := "just some text without file:line format"
	result := sc.Compress(content)
	if result.Applied {
		t.Error("non-search content should passthrough")
	}
}
```

- [ ] **Step 3: 运行测试**

Run: `go test -v -count=1 ./test/unit/compression/ -run TestSearchCompressor`
Expected: PASS

- [ ] **Step 4: 提交**

```bash
git add internal/application/llmproxy/compression/search_compressor.go test/unit/compression/search_compressor_test.go
git commit -m "feat: add SearchCompressor with per-file truncation"
```

---

## Task 5: Compressor 接口 + Dispatcher

**Files:**
- Create: `internal/application/llmproxy/compression/compressor.go`
- Test: `test/unit/compression/dispatcher_test.go`

- [ ] **Step 1: 写 compressor.go**

```go
package compression

// Compressor 压缩单个 tool output 的文本内容。永不返回 error——
// 压缩失败时返回原始内容，Applied=false。
type Compressor interface {
	Compress(content string) ItemCompressionResult
}

// Dispatcher 按 ContentType 路由到具体压缩器。
type Dispatcher struct {
	detector    *ContentDetector
	compressors map[ContentType]Compressor
}

// NewDispatcher 构造默认 Dispatcher，注册一期所有压缩器。
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		detector: NewContentDetector(),
		compressors: map[ContentType]Compressor{
			ContentTypeJsonArray:     NewSmartCrusher(),
			ContentTypeBuildOutput:   NewLogCompressor(),
			ContentTypeSearchResults: NewSearchCompressor(),
		},
	}
}

// Compress 检测内容类型并路由到对应压缩器。未注册的类型走 Passthrough。
func (d *Dispatcher) Compress(content string) ItemCompressionResult {
	ct := d.detector.Detect(content)
	compressor, ok := d.compressors[ct]
	if !ok {
		result := NewPassthroughCompressor().Compress(content)
		result.Strategy = ct.String() + ":passthrough"
		return result
	}
	return compressor.Compress(content)
}
```

- [ ] **Step 2: 写 dispatcher_test.go**

```go
package compression

import "testing"

func TestDispatcherJsonArray(t *testing.T) {
	d := NewDispatcher()
	content := `[{"a":1},{"a":2},{"a":3}]`
	result := d.Compress(content)
	if !result.Applied {
		t.Error("expected JSON array to be compressed")
	}
	if result.Strategy != "smart_crusher" {
		t.Errorf("strategy = %s, want smart_crusher", result.Strategy)
	}
}

func TestDispatcherPlainTextPassthrough(t *testing.T) {
	d := NewDispatcher()
	result := d.Compress("just some plain text")
	if result.Applied {
		t.Error("plain text should passthrough")
	}
}

func TestDispatcherSearchResults(t *testing.T) {
	d := NewDispatcher()
	content := "file1.go:1:line1\nfile1.go:2:line2\nfile1.go:3:line3\nfile1.go:4:line4\nfile1.go:5:line5\nfile1.go:6:line6\nfile1.go:7:line7"
	result := d.Compress(content)
	if !result.Applied {
		t.Error("expected search results to be compressed")
	}
}
```

- [ ] **Step 3: 运行测试**

Run: `go test -v -count=1 ./test/unit/compression/ -run TestDispatcher`
Expected: PASS

- [ ] **Step 4: 提交**

```bash
git add internal/application/llmproxy/compression/compressor.go test/unit/compression/dispatcher_test.go
git commit -m "feat: add Compressor interface and Dispatcher"
```

---

## Task 6: ToolOutputLocator 接口 + OpenAI Chat Locator

**Files:**
- Create: `internal/application/llmproxy/compression/locator.go`
- Create: `internal/application/llmproxy/compression/locator_openai.go`
- Test: `test/unit/compression/locator_openai_test.go`

- [ ] **Step 1: 写 locator.go**

```go
package compression

import "github.com/hcd233/aris-proxy-api/internal/common/enum"

// ToolOutputLocator 按协议定位 body 中的 tool output，执行压缩，返回新 body。
// 任何错误都返回原始 body 和空 stats——压缩永不阻塞请求。
type ToolOutputLocator interface {
	LocateAndCompress(body []byte, dispatcher *Dispatcher, minToolOutputBytes int) ([]byte, CompressionStats)
}

// SelectLocator 按 upstreamProtocol 选择对应的 Locator。
func SelectLocator(upstreamProtocol enum.ProtocolType) ToolOutputLocator {
	switch upstreamProtocol {
	case enum.ProtocolOpenAIChatCompletion:
		return &OpenAIChatLocator{}
	case enum.ProtocolAnthropicMessage:
		return &AnthropicMessagesLocator{}
	case enum.ProtocolOpenAIResponse:
		return &OpenAIResponsesLocator{}
	default:
		return nil
	}
}

// CompressBody 通用入口：根据 upstreamProtocol 选择 Locator 并执行压缩。
// 如果 Locator 不存在或压缩失败，返回原始 body。
func CompressBody(body []byte, upstreamProtocol enum.ProtocolType, dispatcher *Dispatcher, minToolOutputBytes int) ([]byte, *CompressionStats) {
	locator := SelectLocator(upstreamProtocol)
	if locator == nil {
		return body, nil
	}
	newBody, stats := locator.LocateAndCompress(body, dispatcher, minToolOutputBytes)
	if newBody == nil {
		return body, nil
	}
	return newBody, &stats
}
```

- [ ] **Step 2: 写 locator_openai.go**

```go
package compression

import (
	"github.com/bytedance/sonic"
	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)

// OpenAIChatLocator 扫描 OpenAI Chat Completions body 中的 messages[role=tool]。
type OpenAIChatLocator struct{}

func (l *OpenAIChatLocator) LocateAndCompress(body []byte, dispatcher *Dispatcher, minToolOutputBytes int) ([]byte, CompressionStats) {
	var bodyMap map[string]any
	if err := sonic.Unmarshal(body, &bodyMap); err != nil {
		logger.Logger().Warn("[Compression] OpenAI Chat: failed to parse body", zap.Error(err))
		return nil, CompressionStats{}
	}

	messagesRaw, ok := bodyMap["messages"]
	if !ok {
		return nil, CompressionStats{}
	}
	messages, ok := messagesRaw.([]any)
	if !ok {
		return nil, CompressionStats{}
	}

	stats := CompressionStats{}
	modified := false

	for _, msgRaw := range messages {
		msg, ok := msgRaw.(map[string]any)
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		if role != "tool" {
			continue
		}
		content, ok := msg["content"].(string)
		if !ok {
			continue
		}
		if len(content) < minToolOutputBytes {
			stats.addItem(ItemCompressionResult{
				Output:      content,
				Strategy:    "skipped:too_small",
				Applied:     false,
				BytesBefore: len(content),
				BytesAfter:  len(content),
			})
			continue
		}

		result := dispatcher.Compress(content)
		stats.addItem(result)
		if result.Applied {
			msg["content"] = result.Output
			modified = true
		}
	}

	if !modified {
		return nil, stats
	}

	newBody, err := proxyutil.MarshalUpstreamBody(bodyMap)
	if err != nil {
		logger.Logger().Warn("[Compression] OpenAI Chat: failed to re-marshal body", zap.Error(err))
		return nil, stats
	}
	return newBody, stats
}
```

- [ ] **Step 3: 写 locator_openai_test.go**

```go
package compression

import (
	"encoding/json"
	"testing"

	"github.com/bytedance/sonic"
)

func TestOpenAIChatLocatorCompressToolOutput(t *testing.T) {
	locator := &OpenAIChatLocator{}
	dispatcher := NewDispatcher()

	// 构造含 tool output 的 body
	body := map[string]any{
		"model": "gpt-4o",
		"messages": []any{
			map[string]any{"role": "system", "content": "You are a helpful assistant."},
			map[string]any{"role": "user", "content": "Search for errors"},
			map[string]any{"role": "assistant", "content": "Let me search."},
			map[string]any{
				"role":         "tool",
				"content":      `[{"name":"error","code":500,"msg":"database connection failed"},{"name":"warn","code":0,"msg":"ok"},{"name":"error","code":503,"msg":"timeout"},{"name":"info","code":200,"msg":"healthy"},{"name":"debug","code":100,"msg":"trace"}]`,
				"tool_call_id": "call_123",
			},
		},
	}
	bodyBytes, _ := sonic.Marshal(body)

	newBody, stats := locator.LocateAndCompress(bodyBytes, dispatcher, 10)
	if newBody == nil {
		t.Fatal("expected compressed body, got nil")
	}
	if stats.ItemsCompressed == 0 {
		t.Error("expected at least 1 item compressed")
	}

	// 验证非 tool 消息未被修改
	var result map[string]any
	sonic.Unmarshal(newBody, &result)
	messages := result["messages"].([]any)
	sysMsg := messages[0].(map[string]any)
	if sysMsg["content"] != "You are a helpful assistant." {
		t.Error("system message should not be modified")
	}
}

func TestOpenAIChatLocatorNoToolOutput(t *testing.T) {
	locator := &OpenAIChatLocator{}
	dispatcher := NewDispatcher()

	body := map[string]any{
		"model": "gpt-4o",
		"messages": []any{
			map[string]any{"role": "user", "content": "hello"},
		},
	}
	bodyBytes, _ := sonic.Marshal(body)

	newBody, _ := locator.LocateAndCompress(bodyBytes, dispatcher, 10)
	if newBody != nil {
		t.Error("body without tool output should return nil (no modification)")
	}
}

func TestOpenAIChatLocatorSmallToolOutputSkipped(t *testing.T) {
	locator := &OpenAIChatLocator{}
	dispatcher := NewDispatcher()

	body := map[string]any{
		"model": "gpt-4o",
		"messages": []any{
			map[string]any{"role": "tool", "content": "ok"},
		},
	}
	bodyBytes, _ := sonic.Marshal(body)

	newBody, stats := locator.LocateAndCompress(bodyBytes, dispatcher, 512)
	if newBody != nil {
		t.Error("small tool output should be skipped, no modification")
	}
	if stats.ItemsSkipped != 1 {
		t.Errorf("expected 1 skipped item, got %d", stats.ItemsSkipped)
	}
}

// suppress unused import
var _ = json.Marshal
```

- [ ] **Step 4: 运行测试**

Run: `go test -v -count=1 ./test/unit/compression/ -run TestOpenAIChatLocator`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/application/llmproxy/compression/locator.go internal/application/llmproxy/compression/locator_openai.go test/unit/compression/locator_openai_test.go
git commit -m "feat: add ToolOutputLocator interface and OpenAI Chat locator"
```

---

## Task 7: Anthropic Messages Locator

**Files:**
- Create: `internal/application/llmproxy/compression/locator_anthropic.go`
- Test: `test/unit/compression/locator_anthropic_test.go`

- [ ] **Step 1: 写 locator_anthropic.go**

```go
package compression

import (
	"github.com/bytedance/sonic"
	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)

// AnthropicMessagesLocator 扫描 Anthropic Messages body 中的 content[type=tool_result]。
type AnthropicMessagesLocator struct{}

func (l *AnthropicMessagesLocator) LocateAndCompress(body []byte, dispatcher *Dispatcher, minToolOutputBytes int) ([]byte, CompressionStats) {
	var bodyMap map[string]any
	if err := sonic.Unmarshal(body, &bodyMap); err != nil {
		logger.Logger().Warn("[Compression] Anthropic: failed to parse body", zap.Error(err))
		return nil, CompressionStats{}
	}

	messagesRaw, ok := bodyMap["messages"]
	if !ok {
		return nil, CompressionStats{}
	}
	messages, ok := messagesRaw.([]any)
	if !ok {
		return nil, CompressionStats{}
	}

	stats := CompressionStats{}
	modified := false

	for _, msgRaw := range messages {
		msg, ok := msgRaw.(map[string]any)
		if !ok {
			continue
		}
		contentRaw, ok := msg["content"]
		if !ok {
			continue
		}
		contentArr, ok := contentRaw.([]any)
		if !ok {
			continue
		}

		for _, blockRaw := range contentArr {
			block, ok := blockRaw.(map[string]any)
			if !ok {
				continue
			}
			blockType, _ := block["type"].(string)
			if blockType != "tool_result" {
				continue
			}

			// tool_result 的 content 可以是 string 或 content block 数组
			switch contentVal := block["content"].(type) {
			case string:
				if len(contentVal) < minToolOutputBytes {
					stats.addItem(ItemCompressionResult{
						Output:      contentVal,
						Strategy:    "skipped:too_small",
						Applied:     false,
						BytesBefore: len(contentVal),
						BytesAfter:  len(contentVal),
					})
					continue
				}
				result := dispatcher.Compress(contentVal)
				stats.addItem(result)
				if result.Applied {
					block["content"] = result.Output
					modified = true
				}

			case []any:
				// 数组形式：提取所有 text block 的 text，合并后压缩
				combined := extractTextFromBlocks(contentVal)
				if len(combined) < minToolOutputBytes {
					stats.addItem(ItemCompressionResult{
						Output:      combined,
						Strategy:    "skipped:too_small",
						Applied:     false,
						BytesBefore: len(combined),
						BytesAfter:  len(combined),
					})
					continue
				}
				result := dispatcher.Compress(combined)
				stats.addItem(result)
				if result.Applied {
					// 替换为单个 text block
					block["content"] = []any{
						map[string]any{"type": "text", "text": result.Output},
					}
					modified = true
				}
			}
		}
	}

	if !modified {
		return nil, stats
	}

	newBody, err := proxyutil.MarshalUpstreamBody(bodyMap)
	if err != nil {
		logger.Logger().Warn("[Compression] Anthropic: failed to re-marshal body", zap.Error(err))
		return nil, stats
	}
	return newBody, stats
}

func extractTextFromBlocks(blocks []any) string {
	var texts []string
	for _, blockRaw := range blocks {
		block, ok := blockRaw.(map[string]any)
		if !ok {
			continue
		}
		if t, _ := block["type"].(string); t == "text" {
			if text, ok := block["text"].(string); ok {
				texts = append(texts, text)
			}
		}
	}
	if len(texts) == 0 {
		return ""
	}
	if len(texts) == 1 {
		return texts[0]
	}
	result := ""
	for i, t := range texts {
		if i > 0 {
			result += "\n"
		}
		result += t
	}
	return result
}
```

- [ ] **Step 2: 写 locator_anthropic_test.go**

```go
package compression

import (
	"testing"

	"github.com/bytedance/sonic"
)

func TestAnthropicLocatorStringContent(t *testing.T) {
	locator := &AnthropicMessagesLocator{}
	dispatcher := NewDispatcher()

	body := map[string]any{
		"model": "claude-sonnet-4-5-20250929",
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "Check these errors"},
					map[string]any{
						"type":       "tool_result",
						"tool_use_id": "toolu_123",
						"content":     `[{"name":"error","code":500},{"name":"warn","code":0},{"name":"error","code":503},{"name":"info","code":200}]`,
					},
				},
			},
		},
	}
	bodyBytes, _ := sonic.Marshal(body)

	newBody, stats := locator.LocateAndCompress(bodyBytes, dispatcher, 10)
	if newBody == nil {
		t.Fatal("expected compressed body")
	}
	if stats.ItemsCompressed == 0 {
		t.Error("expected at least 1 item compressed")
	}

	// 验证 text block 未被修改
	var result map[string]any
	sonic.Unmarshal(newBody, &result)
	messages := result["messages"].([]any)
	msg := messages[0].(map[string]any)
	blocks := msg["content"].([]any)
	textBlock := blocks[0].(map[string]any)
	if textBlock["text"] != "Check these errors" {
		t.Error("text block should not be modified")
	}
}

func TestAnthropicLocatorArrayContent(t *testing.T) {
	locator := &AnthropicMessagesLocator{}
	dispatcher := NewDispatcher()

	body := map[string]any{
		"model": "claude-sonnet-4-5-20250929",
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type":       "tool_result",
						"tool_use_id": "toolu_456",
						"content": []any{
							map[string]any{"type": "text", "text": `[{"name":"error","code":500,"msg":"db failed"},{"name":"warn","code":0,"msg":"ok"},{"name":"error","code":503,"msg":"timeout"},{"name":"info","code":200,"msg":"healthy"},{"name":"debug","code":100,"msg":"trace"}]`},
						},
					},
				},
			},
		},
	}
	bodyBytes, _ := sonic.Marshal(body)

	newBody, stats := locator.LocateAndCompress(bodyBytes, dispatcher, 10)
	if newBody == nil {
		t.Fatal("expected compressed body for array content")
	}
	if stats.ItemsCompressed == 0 {
		t.Error("expected at least 1 item compressed")
	}
}

func TestAnthropicLocatorNoToolResult(t *testing.T) {
	locator := &AnthropicMessagesLocator{}
	dispatcher := NewDispatcher()

	body := map[string]any{
		"model": "claude-sonnet-4-5-20250929",
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": []any{
					map[string]any{"type": "text", "text": "hello"},
				},
			},
		},
	}
	bodyBytes, _ := sonic.Marshal(body)

	newBody, _ := locator.LocateAndCompress(bodyBytes, dispatcher, 10)
	if newBody != nil {
		t.Error("body without tool_result should return nil")
	}
}
```

- [ ] **Step 3: 运行测试**

Run: `go test -v -count=1 ./test/unit/compression/ -run TestAnthropicLocator`
Expected: PASS

- [ ] **Step 4: 提交**

```bash
git add internal/application/llmproxy/compression/locator_anthropic.go test/unit/compression/locator_anthropic_test.go
git commit -m "feat: add Anthropic Messages locator"
```

---

## Task 8: OpenAI Responses Locator

**Files:**
- Create: `internal/application/llmproxy/compression/locator_responses.go`
- Test: `test/unit/compression/locator_responses_test.go`

- [ ] **Step 1: 写 locator_responses.go**

```go
package compression

import (
	"github.com/bytedance/sonic"
	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)

// OpenAIResponsesLocator 扫描 OpenAI Responses body 中的 input[type=function_call_output]。
type OpenAIResponsesLocator struct{}

func (l *OpenAIResponsesLocator) LocateAndCompress(body []byte, dispatcher *Dispatcher, minToolOutputBytes int) ([]byte, CompressionStats) {
	var bodyMap map[string]any
	if err := sonic.Unmarshal(body, &bodyMap); err != nil {
		logger.Logger().Warn("[Compression] Responses: failed to parse body", zap.Error(err))
		return nil, CompressionStats{}
	}

	inputRaw, ok := bodyMap["input"]
	if !ok {
		return nil, CompressionStats{}
	}

	stats := CompressionStats{}
	modified := false

	switch input := inputRaw.(type) {
	case []any:
		for _, itemRaw := range input {
			item, ok := itemRaw.(map[string]any)
			if !ok {
				continue
			}
			itemType, _ := item["type"].(string)
			if itemType != "function_call_output" {
				continue
			}
			output, ok := item["output"].(string)
			if !ok {
				continue
			}
			if len(output) < minToolOutputBytes {
				stats.addItem(ItemCompressionResult{
					Output:      output,
					Strategy:    "skipped:too_small",
					Applied:     false,
					BytesBefore: len(output),
					BytesAfter:  len(output),
				})
				continue
			}
			result := dispatcher.Compress(output)
			stats.addItem(result)
			if result.Applied {
				item["output"] = result.Output
				modified = true
			}
		}

	case string:
		// input 是字符串时不处理
		return nil, CompressionStats{}
	}

	if !modified {
		return nil, stats
	}

	newBody, err := proxyutil.MarshalUpstreamBody(bodyMap)
	if err != nil {
		logger.Logger().Warn("[Compression] Responses: failed to re-marshal body", zap.Error(err))
		return nil, stats
	}
	return newBody, stats
}
```

- [ ] **Step 2: 写 locator_responses_test.go**

```go
package compression

import (
	"testing"

	"github.com/bytedance/sonic"
)

func TestResponsesLocatorCompressFunctionCallOutput(t *testing.T) {
	locator := &OpenAIResponsesLocator{}
	dispatcher := NewDispatcher()

	body := map[string]any{
		"model": "gpt-4o",
		"input": []any{
			map[string]any{"type": "message", "role": "user", "content": "Search for errors"},
			map[string]any{"type": "function_call", "name": "search", "arguments": "{}"},
			map[string]any{
				"type":    "function_call_output",
				"call_id": "call_123",
				"output":  `[{"name":"error","code":500,"msg":"database connection failed"},{"name":"warn","code":0,"msg":"ok"},{"name":"error","code":503,"msg":"timeout"},{"name":"info","code":200,"msg":"healthy"},{"name":"debug","code":100,"msg":"trace"}]`,
			},
		},
	}
	bodyBytes, _ := sonic.Marshal(body)

	newBody, stats := locator.LocateAndCompress(bodyBytes, dispatcher, 10)
	if newBody == nil {
		t.Fatal("expected compressed body")
	}
	if stats.ItemsCompressed == 0 {
		t.Error("expected at least 1 item compressed")
	}

	// 验证 message item 未被修改
	var result map[string]any
	sonic.Unmarshal(newBody, &result)
	input := result["input"].([]any)
	msgItem := input[0].(map[string]any)
	if msgItem["content"] != "Search for errors" {
		t.Error("message item should not be modified")
	}
}

func TestResponsesLocatorNoFunctionCallOutput(t *testing.T) {
	locator := &OpenAIResponsesLocator{}
	dispatcher := NewDispatcher()

	body := map[string]any{
		"model": "gpt-4o",
		"input": []any{
			map[string]any{"type": "message", "role": "user", "content": "hello"},
		},
	}
	bodyBytes, _ := sonic.Marshal(body)

	newBody, _ := locator.LocateAndCompress(bodyBytes, dispatcher, 10)
	if newBody != nil {
		t.Error("body without function_call_output should return nil")
	}
}
```

- [ ] **Step 3: 运行测试**

Run: `go test -v -count=1 ./test/unit/compression/ -run TestResponsesLocator`
Expected: PASS

- [ ] **Step 4: 提交**

```bash
git add internal/application/llmproxy/compression/locator_responses.go test/unit/compression/locator_responses_test.go
git commit -m "feat: add OpenAI Responses locator"
```

---

## Task 9: Config + Audit Task 字段

**Files:**
- Modify: `internal/config/config.go`
- Modify: `env/api.env.template`
- Modify: `internal/dto/asynctask.go`

- [ ] **Step 1: 修改 config.go — 新增变量声明**

在 `config.go` 的 var 块末尾（`CronThinkExtractEnabled` 之后）添加:

```go
	// CompressionEnabled bool 是否启用 tool output 压缩
	CompressionEnabled bool

	// CompressionMinBodyBytes int body 小于此值跳过压缩
	CompressionMinBodyBytes int

	// CompressionMinToolOutputBytes int 单个 tool output 小于此值跳过
	CompressionMinToolOutputBytes int
```

- [ ] **Step 2: 修改 config.go — 新增默认值和读取**

在 `initEnvironment()` 函数中，`config.AutomaticEnv()` 之后添加默认值:

```go
	config.SetDefault("compression.enabled", false)
	config.SetDefault("compression.min.body.bytes", 2048)
	config.SetDefault("compression.min.tool.output.bytes", 512)
```

在函数末尾（`TrustedProxies` 赋值之前）添加读取:

```go
	CompressionEnabled = config.GetBool("compression.enabled")
	CompressionMinBodyBytes = config.GetInt("compression.min.body.bytes")
	CompressionMinToolOutputBytes = config.GetInt("compression.min.tool.output.bytes")
```

- [ ] **Step 3: 修改 env/api.env.template**

在文件末尾添加:

```env
# LLM 上下文压缩
COMPRESSION_ENABLED=false
COMPRESSION_MIN_BODY_BYTES=2048
COMPRESSION_MIN_TOOL_OUTPUT_BYTES=512
```

- [ ] **Step 4: 修改 asynctask.go — 新增字段**

在 `ModelCallAuditTask` struct 中，`ErrorMessage` 之后添加:

```go
	// 压缩相关
	CompressionEnabled    bool
	CompressedTokens      int
	CompressionStrategies []string
	// 中间值，不持久化
	compressionBytesBefore int
	compressionBytesAfter  int
```

- [ ] **Step 5: 修改 asynctask.go — 新增方法**

在 `SetErrorFromResponseStatus` 方法之后添加:

```go
// SetCompressionStats 设置压缩统计中间值。
func (t *ModelCallAuditTask) SetCompressionStats(bytesBefore, bytesAfter int, strategies []string) {
	t.CompressionEnabled = true
	t.CompressionStrategies = strategies
	t.compressionBytesBefore = bytesBefore
	t.compressionBytesAfter = bytesAfter
}

// ComputeCompressedTokens 在拿到真实 input_tokens 后计算压缩节省的 token 数。
// 公式：CompressedTokens = input_tokens * (before - after) / after
func (t *ModelCallAuditTask) ComputeCompressedTokens() {
	if !t.CompressionEnabled || t.compressionBytesAfter == 0 || t.InputTokens <= 0 {
		return
	}
	if t.compressionBytesAfter >= t.compressionBytesBefore {
		return
	}
	t.CompressedTokens = t.InputTokens * (t.compressionBytesBefore - t.compressionBytesAfter) / t.compressionBytesAfter
}
```

- [ ] **Step 6: 运行编译检查**

Run: `go build ./...`
Expected: 编译通过

- [ ] **Step 7: 提交**

```bash
git add internal/config/config.go env/api.env.template internal/dto/asynctask.go
git commit -m "feat: add compression config and audit task fields"
```

---

## Task 10: UseCase 集成 — OpenAI Chat

**Files:**
- Modify: `internal/application/llmproxy/usecase/openai.go`
- Modify: `internal/application/llmproxy/usecase/openai_chat.go`

- [ ] **Step 1: 修改 openai.go — 增加 compressor 字段**

在 `openAIUseCase` struct 中增加字段:

```go
type openAIUseCase struct {
	resolver       service.EndpointResolver
	modelsQuery    ListOpenAIModels
	openAIProxy    OpenAIProxyPort
	anthropicProxy AnthropicProxyPort
	taskSubmitter  TaskSubmitter
	blockedChecker BlockedChecker
	dispatcher     *compression.Dispatcher
}
```

增加 import:
```go
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
	"github.com/hcd233/aris-proxy-api/internal/config"
```

修改 `NewOpenAIUseCase` 签名，增加 dispatcher 参数:

```go
func NewOpenAIUseCase(
	resolver service.EndpointResolver,
	modelsQuery ListOpenAIModels,
	openAIProxy OpenAIProxyPort,
	anthropicProxy AnthropicProxyPort,
	taskSubmitter TaskSubmitter,
	blockedChecker BlockedChecker,
	dispatcher *compression.Dispatcher,
) OpenAIUseCase {
	return &openAIUseCase{
		resolver:       resolver,
		modelsQuery:    modelsQuery,
		openAIProxy:    openAIProxy,
		anthropicProxy: anthropicProxy,
		taskSubmitter:  taskSubmitter,
		blockedChecker: blockedChecker,
		dispatcher:     dispatcher,
	}
}
```

- [ ] **Step 2: 修改 openai.go — 增加 compressBodyIfNeeded 方法**

在 `toTransportEndpoint` 函数之后添加:

```go
func (u *openAIUseCase) compressBodyIfNeeded(ctx context.Context, body []byte, upstreamProtocol enum.ProtocolType) ([]byte, *compression.CompressionStats) {
	if !config.CompressionEnabled || u.dispatcher == nil || len(body) < config.CompressionMinBodyBytes {
		return body, nil
	}
	newBody, stats := compression.CompressBody(body, upstreamProtocol, u.dispatcher, config.CompressionMinToolOutputBytes)
	if stats != nil && stats.ItemsCompressed > 0 {
		logger.WithCtx(ctx).Info("[Compression] OpenAI body compressed",
			zap.Int("bytesBefore", stats.BytesBefore),
			zap.Int("bytesAfter", stats.BytesAfter),
			zap.Int("itemsCompressed", stats.ItemsCompressed),
			zap.Strings("strategies", stats.StrategiesUsed),
		)
	}
	return newBody, stats
}
```

- [ ] **Step 3: 修改 openai_chat.go — 在 forwardChatNative 中插入压缩**

修改 `forwardChatNative`:

```go
func (u *openAIUseCase) forwardChatNative(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, stream bool) *huma.StreamResponse {
	body := proxyutil.MarshalOpenAIChatCompletionBodyForModel(req.Body, upstream.Model)
	body, compStats := u.compressBodyIfNeeded(ctx, body, enum.ProtocolOpenAIChatCompletion)
	if stream {
		return u.forwardChatNativeStream(ctx, req, m, ep, upstream, body, compStats)
	}
	return u.forwardChatNativeUnary(ctx, req, m, ep, upstream, body, compStats)
}
```

修改 `forwardChatViaAnthropic`:

```go
func (u *openAIUseCase) forwardChatViaAnthropic(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, ep *aggregate.Endpoint, exposedModel string) *huma.StreamResponse {
	conv := &converter.AnthropicProtocolConverter{}
	anthropicReq, convErr := conv.FromOpenAIRequest(req.Body)
	if convErr != nil {
		logger.WithCtx(ctx).Error("[OpenAIUseCase] Failed to convert chat request to anthropic", zap.Error(convErr))
		return proxyutil.SendOpenAIModelNotFoundError(exposedModel)
	}
	stream := req.Body.Stream != nil && *req.Body.Stream
	upstream := toTransportEndpoint(m, ep, true)
	body := proxyutil.MarshalAnthropicMessageBodyForModel(anthropicReq, upstream.Model)
	body, compStats := u.compressBodyIfNeeded(ctx, body, enum.ProtocolAnthropicMessage)
	if stream {
		return u.forwardChatNativeStream(ctx, req, m, ep, upstream, body, compStats)
	}
	return u.forwardChatNativeUnary(ctx, req, m, ep, upstream, body, compStats)
}
```

> 注意：`forwardChatViaAnthropic` 原来调用 `forwardChatViaAnthropicStream`/`forwardChatViaAnthropicUnary`。压缩后 body 已是 Anthropic 格式，但下游的 audit 等逻辑需要适配。一期简化为：跨协议路径暂不压缩（compressBodyIfNeeded 对 Anthropic 路径只在 native Anthropic usecase 里启用），或在 forwardChatViaAnthropic 中保留调用 `forwardChatViaAnthropicStream/Unary`。实际实现时需要把 compStats 传递到这些方法中。

为了最小化改动，一期方案调整：**压缩只在 native 路径执行**（因为跨协议转换的 body 结构已变，locator 需要匹配上游协议）。修改 `forwardChatViaAnthropic` 不插入压缩:

```go
func (u *openAIUseCase) forwardChatViaAnthropic(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, ep *aggregate.Endpoint, exposedModel string) *huma.StreamResponse {
	conv := &converter.AnthropicProtocolConverter{}
	anthropicReq, convErr := conv.FromOpenAIRequest(req.Body)
	if convErr != nil {
		logger.WithCtx(ctx).Error("[OpenAIUseCase] Failed to convert chat request to anthropic", zap.Error(convErr))
		return proxyutil.SendOpenAIModelNotFoundError(exposedModel)
	}
	stream := req.Body.Stream != nil && *req.Body.Stream
	upstream := toTransportEndpoint(m, ep, true)
	body := proxyutil.MarshalAnthropicMessageBodyForModel(anthropicReq, upstream.Model)
	if stream {
		return u.forwardChatViaAnthropicStream(ctx, req, m, upstream, exposedModel, ep.Name(), body)
	}
	return u.forwardChatViaAnthropicUnary(ctx, req, m, upstream, exposedModel, ep.Name(), body)
}
```

- [ ] **Step 4: 修改 openai_chat.go — 修改 stream/unary 方法签名增加 compStats 参数**

所有 `forwardChatNativeStream` 和 `forwardChatNativeUnary` 的签名增加 `compStats *compression.CompressionStats` 参数，并在 audit task 处添加:

```go
func (u *openAIUseCase) forwardChatNativeStream(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, body []byte, compStats *compression.CompressionStats) *huma.StreamResponse {
	// ... 现有逻辑 ...
	// 在 task 赋值后、submit 之前添加:
	if compStats != nil {
		task.SetCompressionStats(compStats.BytesBefore, compStats.BytesAfter, compStats.StrategiesUsed)
		task.ComputeCompressedTokens()
	}
	_ = u.taskSubmitter.SubmitModelCallAuditTask(task)
}
```

对 `forwardChatNativeUnary` 做同样修改。

- [ ] **Step 5: 运行编译**

Run: `go build ./internal/application/llmproxy/...`
Expected: 编译通过

- [ ] **Step 6: 运行现有测试确保不回归**

Run: `go test -count=1 ./test/unit/converter/`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add internal/application/llmproxy/usecase/openai.go internal/application/llmproxy/usecase/openai_chat.go
git commit -m "feat: integrate compression into OpenAI Chat usecase"
```

---

## Task 11: UseCase 集成 — OpenAI Responses + Anthropic Messages

**Files:**
- Modify: `internal/application/llmproxy/usecase/anthropic.go`
- Modify: `internal/application/llmproxy/usecase/anthropic_message.go`
- Modify: `internal/application/llmproxy/usecase/openai_response.go`

- [ ] **Step 1: 修改 anthropic.go — 增加 compressor 字段**

与 Task 10 Step 1 相同模式，在 `anthropicUseCase` struct 增加 `dispatcher *compression.Dispatcher` 字段，修改 `NewAnthropicUseCase` 签名。

```go
type anthropicUseCase struct {
	resolver         service.EndpointResolver
	modelsQuery      ListAnthropicModels
	countTokensQuery CountTokens
	anthropicProxy   AnthropicProxyPort
	openAIProxy      OpenAIProxyPort
	taskSubmitter    TaskSubmitter
	blockedChecker   BlockedChecker
	dispatcher       *compression.Dispatcher
}

func NewAnthropicUseCase(
	resolver service.EndpointResolver,
	modelsQuery ListAnthropicModels,
	countTokensQuery CountTokens,
	anthropicProxy AnthropicProxyPort,
	openAIProxy OpenAIProxyPort,
	taskSubmitter TaskSubmitter,
	blockedChecker BlockedChecker,
	dispatcher *compression.Dispatcher,
) AnthropicUseCase {
	return &anthropicUseCase{
		resolver:         resolver,
		modelsQuery:      modelsQuery,
		countTokensQuery: countTokensQuery,
		anthropicProxy:   anthropicProxy,
		openAIProxy:      openAIProxy,
		taskSubmitter:    taskSubmitter,
		blockedChecker:   blockedChecker,
		dispatcher:       dispatcher,
	}
}
```

- [ ] **Step 2: 修改 anthropic_message.go — 插入压缩**

在 `forwardMessageNative` 中插入压缩:

```go
func (u *anthropicUseCase) forwardMessageNative(ctx context.Context, req *dto.AnthropicCreateMessageRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, exposedModel string, stream bool) *huma.StreamResponse {
	body := proxyutil.MarshalAnthropicMessageBodyForModel(req.Body, upstream.Model)
	body, compStats := u.compressBodyIfNeeded(ctx, body, enum.ProtocolAnthropicMessage)
	if stream {
		return u.forwardMessageNativeStream(ctx, req, m, upstream, exposedModel, ep.Name(), body, compStats)
	}
	return u.forwardMessageNativeUnary(ctx, req, m, upstream, exposedModel, ep.Name(), body, compStats)
}
```

增加 `compressBodyIfNeeded` 方法（与 openai.go 中的相同模式）:

```go
func (u *anthropicUseCase) compressBodyIfNeeded(ctx context.Context, body []byte, upstreamProtocol enum.ProtocolType) ([]byte, *compression.CompressionStats) {
	if !config.CompressionEnabled || u.dispatcher == nil || len(body) < config.CompressionMinBodyBytes {
		return body, nil
	}
	newBody, stats := compression.CompressBody(body, upstreamProtocol, u.dispatcher, config.CompressionMinToolOutputBytes)
	if stats != nil && stats.ItemsCompressed > 0 {
		logger.WithCtx(ctx).Info("[Compression] Anthropic body compressed",
			zap.Int("bytesBefore", stats.BytesBefore),
			zap.Int("bytesAfter", stats.BytesAfter),
			zap.Int("itemsCompressed", stats.ItemsCompressed),
			zap.Strings("strategies", stats.StrategiesUsed),
		)
	}
	return newBody, stats
}
```

- [ ] **Step 3: 修改 anthropic_message.go — stream/unary 方法增加 compStats 参数**

`forwardMessageNativeStream` 和 `forwardMessageNativeUnary` 增加 `compStats *compression.CompressionStats` 参数，在 audit task 提交前添加:

```go
	if compStats != nil {
		task.SetCompressionStats(compStats.BytesBefore, compStats.BytesAfter, compStats.StrategiesUsed)
		task.ComputeCompressedTokens()
	}
```

- [ ] **Step 4: 修改 openai_response.go — 在 forwardResponseNative 中插入压缩**

```go
func (u *openAIUseCase) forwardResponseNative(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, stream bool) *huma.StreamResponse {
	body := proxyutil.MarshalOpenAIResponseBodyForModel(req.Body, upstream.Model)
	body, compStats := u.compressBodyIfNeeded(ctx, body, enum.ProtocolOpenAIResponse)
	if stream {
		return u.forwardResponseNativeStream(ctx, req, m, ep, upstream, body, compStats)
	}
	return u.forwardResponseNativeUnary(ctx, req, m, ep, upstream, body, compStats)
}
```

`forwardResponseNativeStream` 和 `forwardResponseNativeUnary` 增加 `compStats` 参数，在 audit task 提交前添加同样的 compression stats 设置代码。

- [ ] **Step 5: 运行编译**

Run: `go build ./internal/application/llmproxy/...`
Expected: 编译通过

- [ ] **Step 6: 提交**

```bash
git add internal/application/llmproxy/usecase/anthropic.go internal/application/llmproxy/usecase/anthropic_message.go internal/application/llmproxy/usecase/openai_response.go
git commit -m "feat: integrate compression into Anthropic Messages and OpenAI Responses usecases"
```

---

## Task 12: DI 注册

**Files:**
- Modify: `internal/bootstrap/modules/application.go`

- [ ] **Step 1: 修改 application.go — 注册 Dispatcher**

在 `ApplicationModule` 的 `fx.Provide` 列表中添加:

```go
		compression.NewDispatcher,
```

并在 import 中添加:
```go
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
```

- [ ] **Step 2: 运行编译和启动检查**

Run: `go build ./...`
Expected: 编译通过

- [ ] **Step 3: 提交**

```bash
git add internal/bootstrap/modules/application.go
git commit -m "feat: register compression Dispatcher in DI container"
```

---

## Task 13: 全量测试 + Lint

- [ ] **Step 1: 运行全量单元测试**

Run: `go test -v -count=1 ./test/unit/compression/`
Expected: 全部 PASS

- [ ] **Step 2: 运行 lint**

Run: `make lint`
Expected: 无新增 lint 错误

- [ ] **Step 3: 运行全量编译**

Run: `make build`
Expected: 编译通过

- [ ] **Step 4: 提交（如有 lint 修复）**

```bash
git add -A
git commit -m "chore: fix lint issues from compression integration"
```

---

## Task 14: E2E 测试

**Files:**
- Create: `test/e2e/compression/compression_test.go`

- [ ] **Step 1: 写 e2e 测试**

```go
package compression

import (
	"os"
	"testing"
)

// TestCompressionE2E 验证压缩后的请求能被上游正常接受并返回有效响应。
// 需要 BASE_URL 和 API_KEY 环境变量。
func TestCompressionE2E(t *testing.T) {
	baseURL := os.Getenv("BASE_URL")
	apiKey := os.Getenv("API_KEY")
	if baseURL == "" || apiKey == "" {
		t.Skip("BASE_URL and API_KEY are required for e2e test")
	}

	// TODO: 构造含 tool output 的请求，发送到代理，验证响应正常
	// 具体实现取决于项目 E2E 测试框架
	t.Skip("E2E compression test requires running proxy with COMPRESSION_ENABLED=true")
}
```

- [ ] **Step 2: 运行 e2e 测试（离线时 skip）**

Run: `go test -v -count=1 ./test/e2e/compression/`
Expected: SKIP（无 BASE_URL/API_KEY 时）

- [ ] **Step 3: 提交**

```bash
git add test/e2e/compression/
git commit -m "test: add compression e2e test skeleton"
```

---

## 自审检查清单

### Spec 覆盖
- [x] ContentDetector（7 种类型检测）— Task 1
- [x] SmartCrusher（JSON 数组 lossless CSV + lossy 采样）— Task 2
- [x] LogCompressor（模板提取 + 去噪）— Task 3
- [x] SearchCompressor（grep 去重 + 摘要）— Task 4
- [x] Dispatcher（按 ContentType 路由）— Task 5
- [x] OpenAI Chat Locator（messages[role=tool]）— Task 6
- [x] Anthropic Messages Locator（content[type=tool_result]）— Task 7
- [x] OpenAI Responses Locator（input[type=function_call_output]）— Task 8
- [x] 配置项（3 个 Viper 环境变量）— Task 9
- [x] Audit Task 字段 + CompressedTokens 计算 — Task 9
- [x] UseCase 集成（OpenAI Chat）— Task 10
- [x] UseCase 集成（Anthropic + Responses）— Task 11
- [x] DI 注册 — Task 12
- [x] E2E 测试 — Task 14
- [x] 通用兜底规则（膨胀回退、阈值跳过、错误回退）— 各压缩器和 locator 内实现
- [x] 压缩永不失败铁律 — locator 返回 nil 时 usecase 回退原始 body

### 类型一致性
- `CompressionStats` 字段名 `BytesBefore`/`BytesAfter`/`ItemsCompressed`/`ItemsSkipped`/`StrategiesUsed` 全程一致
- `ItemCompressionResult` 字段名 `Output`/`Strategy`/`Applied`/`BytesBefore`/`BytesAfter` 全程一致
- `CompressBody` 函数签名 `([]byte, enum.ProtocolType, *Dispatcher, int) ([]byte, *CompressionStats)` 全程一致
- `compressBodyIfNeeded` 方法签名在 openAIUseCase 和 anthropicUseCase 中一致
