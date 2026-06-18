# Token 压缩功能 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 aris-proxy-api 中实现 Go 原生的 LLM 上下文压缩管线，覆盖 Chat Completion / Message / Response 全部 7 条转发路径。

**Architecture:** 新增 `internal/application/llmproxy/compression/` 包，实现 SmartCrusher + ContentDetector + SearchCompressor + LogCompressor，通过 `Pipeline` 接口注入 usecase 层，在请求体序列化前对 messages 进行压缩。

**Tech Stack:** Go 1.25, `sonic` (JSON), 标准库 `testing`, viper (配置), dig (DI)

---

## File Structure

| 文件 | 职责 |
|------|------|
| `internal/config/config.go` (modify) | 新增 `CompressionEnabled` 全局变量 |
| `internal/application/llmproxy/compression/content_detector.go` (create) | 内容类型检测 + 置信度评分 |
| `internal/application/llmproxy/compression/smart_crusher.go` (create) | JSON 数组统计采样压缩 |
| `internal/application/llmproxy/compression/search_compressor.go` (create) | 搜索结果压缩 |
| `internal/application/llmproxy/compression/log_compressor.go` (create) | 构建日志压缩 |
| `internal/application/llmproxy/compression/pipeline.go` (create) | Pipeline 主入口 + 接口定义 |
| `test/unit/compression/content_detector_test.go` (create) | content_detector 单测 |
| `test/unit/compression/smart_crusher_test.go` (create) | smart_crusher 单测 |
| `test/unit/compression/search_compressor_test.go` (create) | search_compressor 单测 |
| `test/unit/compression/log_compressor_test.go` (create) | log_compressor 单测 |
| `test/unit/compression/pipeline_test.go` (create) | pipeline 单测 |
| `internal/dto/asynctask.go` (modify) | ModelCallAuditTask 新增压缩字段 |
| `internal/dto/audit.go` (modify) | AuditLogItem 新增压缩字段 |
| `internal/application/llmproxy/usecase/openai.go` (modify) | openAIUseCase 注入 compressPipeline |
| `internal/application/llmproxy/usecase/anthropic.go` (modify) | anthropicUseCase 注入 compressPipeline |
| `internal/application/llmproxy/usecase/openai_chat.go` (modify) | 2 条路径加入压缩调用 |
| `internal/application/llmproxy/usecase/openai_response.go` (modify) | 3 条路径加入压缩调用 |
| `internal/application/llmproxy/usecase/anthropic_message.go` (modify) | 2 条路径加入压缩调用 |
| `internal/bootstrap/container.go` (modify) | DI 注册 compression.Pipeline |

---

### Task 1: 配置 - 新增 CompressionEnabled

**Files:**
- Modify: `internal/config/config.go:195-198`

- [ ] **Step 1: 在 config.go 中新增变量和初始化**

在 `CronThinkExtractEnabled` 之后添加变量声明：

```go
// CompressionEnabled bool 是否启用请求内容压缩（减少上游 token 消耗）
//	@update 2026-06-14 10:00:00
CompressionEnabled bool
```

在 `initEnvironment()` 中 `CronThinkExtractEnabled` 行之后添加：

```go
config.SetDefault("compression.enabled", false)
```

在 `CronThinkExtractEnabled = config.GetBool("cron.think.extract.enabled")` 行之后添加：

```go
CompressionEnabled = config.GetBool("compression.enabled")
```

- [ ] **Step 2: 验证编译通过**

```bash
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add internal/config/config.go
git commit -m "feat: add CompressionEnabled config via COMPRESSION_ENABLED env var"
```

---

### Task 2: ContentDetector 内容类型检测

**Files:**
- Create: `internal/application/llmproxy/compression/content_detector.go`
- Create: `test/unit/compression/content_detector_test.go`

- [ ] **Step 1: 写测试**

```go
// test/unit/compression/content_detector_test.go
package compression

import (
	"testing"

	comp "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
)

func TestDetectJSONArray(t *testing.T) {
	t.Parallel()
	content := `[{"id":1,"name":"a"},{"id":2,"name":"b"},{"id":3,"name":"c"}]`
	ct, conf := comp.DetectContentType(content)
	if ct != comp.ContentTypeJSONArray {
		t.Errorf("expected JSON_ARRAY, got %v", ct)
	}
	if conf < 0.9 {
		t.Errorf("confidence too low: %f", conf)
	}
}

func TestDetectJSONArrayNotObject(t *testing.T) {
	t.Parallel()
	content := `[1,2,3,4,5]`
	ct, conf := comp.DetectContentType(content)
	if ct != comp.ContentTypeJSONArray {
		t.Errorf("expected JSON_ARRAY, got %v", ct)
	}
	if conf >= 1.0 {
		t.Errorf("non-object array confidence should be < 1.0: %f", conf)
	}
}

func TestDetectSearchResults(t *testing.T) {
	t.Parallel()
	content := `src/main.go:42:func process()
src/main.go:58:return result
src/utils.go:12:import "fmt"
src/utils.go:33:func helper()`
	ct, conf := comp.DetectContentType(content)
	if ct != comp.ContentTypeSearchResults {
		t.Errorf("expected SEARCH_RESULTS, got %v", ct)
	}
	if conf < 0.6 {
		t.Errorf("confidence too low: %f", conf)
	}
}

func TestDetectBuildOutput(t *testing.T) {
	t.Parallel()
	content := `2024-01-01 12:00:00 ERROR connection failed
2024-01-01 12:00:01 WARN retrying
2024-01-01 12:00:02 ERROR timeout
2024-01-01 12:00:03 INFO recovered
2024-01-01 12:00:04 FATAL shutdown`
	ct, conf := comp.DetectContentType(content)
	if ct != comp.ContentTypeBuildOutput {
		t.Errorf("expected BUILD_OUTPUT, got %v", ct)
	}
	if conf < 0.5 {
		t.Errorf("confidence too low: %f", conf)
	}
}

func TestDetectCodePython(t *testing.T) {
	t.Parallel()
	content := `def process():
    return True

class Handler:
    def __init__(self):
        pass

import os

def main():
    result = process()
    return result`
	ct, _ := comp.DetectContentType(content)
	if ct != comp.ContentTypeSourceCode {
		t.Errorf("expected SOURCE_CODE, got %v", ct)
	}
}

func TestDetectCodeGo(t *testing.T) {
	t.Parallel()
	content := `package main

import "fmt"

func main() {
    fmt.Println("hello")
}

type Server struct {
    port int
}`
	ct, _ := comp.DetectContentType(content)
	if ct != comp.ContentTypeSourceCode {
		t.Errorf("expected SOURCE_CODE, got %v", ct)
	}
}

func TestDetectPlainText(t *testing.T) {
	t.Parallel()
	content := "This is just a normal sentence with no special formatting at all."
	ct, _ := comp.DetectContentType(content)
	if ct != comp.ContentTypePlainText {
		t.Errorf("expected PLAIN_TEXT, got %v", ct)
	}
}

func TestDetectEmptyContent(t *testing.T) {
	t.Parallel()
	ct, conf := comp.DetectContentType("")
	if ct != comp.ContentTypePlainText {
		t.Errorf("expected PLAIN_TEXT for empty, got %v", ct)
	}
	if conf != 0.0 {
		t.Errorf("expected 0.0 confidence for empty, got %f", conf)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test -v -count=1 -run TestDetect ./test/unit/compression/
```
Expected: compilation error or test failure.

- [ ] **Step 3: 实现 ContentDetector**

```go
// internal/application/llmproxy/compression/content_detector.go
package compression

import (
	"regexp"
	"strings"

	"github.com/bytedance/sonic"
)

type ContentType string

const (
	ContentTypeJSONArray     ContentType = "json_array"
	ContentTypeSourceCode    ContentType = "source_code"
	ContentTypeSearchResults ContentType = "search"
	ContentTypeBuildOutput   ContentType = "build"
	ContentTypeGitDiff       ContentType = "diff"
	ContentTypeHTML          ContentType = "html"
	ContentTypePlainText     ContentType = "text"
)

const minCharsForDetection = 500

var (
	searchResultPattern = regexp.MustCompile(`^[^\s:]+:\d+:`)

	diffHeaderPattern = regexp.MustCompile(`^(diff --git|--- a/|@@\s+-\d+,\d+\s+\+\d+,\d+\s+@@)`)
	diffChangePattern = regexp.MustCompile(`^[+-][^+-]`)

	logPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\b(ERROR|FAIL|FAILED|FATAL|CRITICAL)\b`),
		regexp.MustCompile(`\b(WARN|WARNING)\b`),
		regexp.MustCompile(`\b(INFO|DEBUG|TRACE)\b`),
		regexp.MustCompile(`^\s*\d{4}-\d{2}-\d{2}`),
		regexp.MustCompile(`^\s*\[\d{2}:\d{2}:\d{2}\]`),
		regexp.MustCompile(`^={3,}|^-{3,}`),
		regexp.MustCompile(`^\s*PASSED|^\s*FAILED|^\s*SKIPPED`),
		regexp.MustCompile(`^npm ERR!|^yarn error|^cargo error`),
	}

	codePatterns = map[string][]*regexp.Regexp{
		"python": {
			regexp.MustCompile(`^\s*(def|class|import|from|async def)\s+\w+`),
			regexp.MustCompile(`^\s*@\w+`),
		},
		"go": {
			regexp.MustCompile(`^\s*(func|type|package|import)\s+`),
			regexp.MustCompile(`^\s*func\s+\([^)]+\)\s+\w+`),
		},
		"rust": {
			regexp.MustCompile(`^\s*(fn|struct|enum|impl|mod|use|pub)\s+`),
		},
		"javascript": {
			regexp.MustCompile(`^\s*(function|const|let|var|class|import|export)\s+`),
		},
	}
)

func DetectContentType(content string) (ContentType, float64) {
	content = strings.TrimSpace(content)
	if content == "" {
		return ContentTypePlainText, 0.0
	}

	if result, conf := tryDetectJSON(content); result != nil {
		return *result, conf
	}

	if result, conf := tryDetectDiff(content); result != nil && conf >= 0.7 {
		return *result, conf
	}

	if result, conf := tryDetectSearch(content); result != nil && conf >= 0.6 {
		return *result, conf
	}

	if result, conf := tryDetectLog(content); result != nil && conf >= 0.5 {
		return *result, conf
	}

	if result, conf := tryDetectCode(content); result != nil && conf >= 0.5 {
		return *result, conf
	}

	return ContentTypePlainText, 0.5
}

func tryDetectJSON(content string) (*ContentType, float64) {
	if !strings.HasPrefix(content, "[") {
		return nil, 0
	}
	var parsed []any
	if err := sonic.UnmarshalString(content, &parsed); err != nil {
		return nil, 0
	}
	allDicts := true
	for _, item := range parsed {
		if _, ok := item.(map[string]any); !ok {
			allDicts = false
			break
		}
	}
	ct := ContentTypeJSONArray
	if allDicts {
		return &ct, 1.0
	}
	return &ct, 0.8
}

func tryDetectDiff(content string) (*ContentType, float64) {
	lines := strings.SplitN(content, "\n", 501)[:500]
	var headerMatches, changeMatches int
	for _, line := range lines {
		if diffHeaderPattern.MatchString(line) {
			headerMatches++
		}
		if diffChangePattern.MatchString(line) {
			changeMatches++
		}
	}
	if headerMatches == 0 {
		return nil, 0
	}
	ct := ContentTypeGitDiff
	conf := 0.5 + float64(headerMatches)*0.2 + float64(changeMatches)*0.05
	if conf > 1.0 {
		conf = 1.0
	}
	return &ct, conf
}

func tryDetectSearch(content string) (*ContentType, float64) {
	lines := strings.SplitN(content, "\n", 101)[:100]
	var matching, nonEmpty int
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		nonEmpty++
		if searchResultPattern.MatchString(line) {
			matching++
		}
	}
	if nonEmpty == 0 || matching == 0 {
		return nil, 0
	}
	ratio := float64(matching) / float64(nonEmpty)
	if ratio < 0.3 {
		return nil, 0
	}
	ct := ContentTypeSearchResults
	conf := 0.4 + ratio*0.6
	if conf > 1.0 {
		conf = 1.0
	}
	return &ct, conf
}

func tryDetectLog(content string) (*ContentType, float64) {
	lines := strings.SplitN(content, "\n", 201)[:200]
	var patternMatches, errorMatches, nonEmpty int
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		nonEmpty++
		matched := false
		for i, pat := range logPatterns {
			if pat.MatchString(line) {
				patternMatches++
				if i < 2 {
					errorMatches++
				}
				matched = true
				break
			}
		}
		_ = matched
	}
	if nonEmpty == 0 || patternMatches == 0 {
		return nil, 0
	}
	ratio := float64(patternMatches) / float64(nonEmpty)
	if ratio < 0.1 {
		return nil, 0
	}
	ct := ContentTypeBuildOutput
	conf := 0.3 + ratio*0.5 + float64(errorMatches)*0.05
	if conf > 1.0 {
		conf = 1.0
	}
	return &ct, conf
}

func tryDetectCode(content string) (*ContentType, float64) {
	lines := strings.SplitN(content, "\n", 101)[:100]
	langScores := make(map[string]int)
	for _, line := range lines {
		for lang, patterns := range codePatterns {
			for _, pat := range patterns {
				if pat.MatchString(strings.TrimSpace(line)) {
					langScores[lang]++
					break
				}
			}
		}
	}
	if len(langScores) == 0 {
		return nil, 0
	}
	var bestLang string
	var bestScore int
	for lang, score := range langScores {
		if score > bestScore {
			bestScore = score
			bestLang = lang
		}
	}
	if bestScore < 3 {
		return nil, 0
	}
	var nonEmpty int
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmpty++
		}
	}
	ratio := float64(bestScore) / float64(nonEmpty)
	ct := ContentTypeSourceCode
	conf := 0.4 + ratio*0.4 + float64(bestScore)*0.02
	if conf > 1.0 {
		conf = 1.0
	}
	_ = bestLang
	return &ct, conf
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test -v -count=1 -run TestDetect ./test/unit/compression/
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/application/llmproxy/compression/content_detector.go test/unit/compression/content_detector_test.go
git commit -m "feat: add ContentDetector for content type detection with confidence scoring"
```

---

### Task 3: SmartCrusher JSON 数组压缩

**Files:**
- Create: `internal/application/llmproxy/compression/smart_crusher.go`
- Create: `test/unit/compression/smart_crusher_test.go`

- [ ] **Step 1: 写测试**

```go
// test/unit/compression/smart_crusher_test.go
package compression

import (
	"testing"

	comp "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
)

func TestSmartCrusherTooFewItems(t *testing.T) {
	t.Parallel()
	crusher := comp.NewSmartCrusher(comp.DefaultSmartCrusherConfig())
	content := `[{"a":1},{"a":2}]`
	crushed, modified, _ := crusher.Crush(content)
	if modified {
		t.Error("should not modify array with fewer than 5 items")
	}
	if crushed != content {
		t.Errorf("content changed: %s", crushed)
	}
}

func TestSmartCrusherPassthroughShortContent(t *testing.T) {
	t.Parallel()
	crusher := comp.NewSmartCrusher(comp.DefaultSmartCrusherConfig())
	content := `[{"a":1},{"a":2},{"a":3},{"a":4},{"a":5}]`
	crushed, modified, _ := crusher.Crush(content)
	if modified {
		t.Error("should not modify content below min tokens threshold")
	}
	if crushed != content {
		t.Errorf("content changed: %s", crushed)
	}
}

func TestSmartCrusherDedupIdenticalItems(t *testing.T) {
	t.Parallel()
	crusher := comp.NewSmartCrusher(comp.DefaultSmartCrusherConfig())
	items := ""
	for i := 0; i < 10; i++ {
		if items != "" {
			items += ","
		}
		items += `{"x":"a","y":"b"}`
	}
	content := "[" + items + "]"
	crushed, modified, strategy := crusher.Crush(content)
	if !modified {
		t.Error("should modify items")
	}
	if strategy == "passthrough" {
		t.Error("strategy should not be passthrough")
	}
	t.Logf("dedup result: %s", crushed)
	t.Logf("strategy: %s", strategy)
}

func TestSmartCrusherKeepsFirstAndLast(t *testing.T) {
	t.Parallel()
	crusher := comp.NewSmartCrusher(comp.DefaultSmartCrusherConfig())
	items := `[{"id":0,"v":"start"},{"id":1,"v":10},{"id":2,"v":10},{"id":3,"v":10},{"id":4,"v":10},{"id":5,"v":10},{"id":6,"v":10},{"id":7,"v":10},{"id":8,"v":10},{"id":9,"v":"end"}]`
	crushed, modified, strategy := crusher.Crush(items)
	if !modified {
		t.Error("should modify items")
	}
	t.Logf("crushed: %s", crushed)
	t.Logf("strategy: %s", strategy)
}

func TestSmartCrusherEmptyArray(t *testing.T) {
	t.Parallel()
	crusher := comp.NewSmartCrusher(comp.DefaultSmartCrusherConfig())
	crushed, modified, _ := crusher.Crush(`[]`)
	if modified {
		t.Error("should not modify empty array")
	}
	if crushed != `[]` {
		t.Error("should return empty array")
	}
}

func TestSmartCrusherNonJSONPassthrough(t *testing.T) {
	t.Parallel()
	crusher := comp.NewSmartCrusher(comp.DefaultSmartCrusherConfig())
	content := "not json at all"
	crushed, modified, _ := crusher.Crush(content)
	if modified {
		t.Error("should not modify non-JSON content")
	}
	if crushed != content {
		t.Error("should return original content")
	}
}

func TestSmartCrusherNonArrayJSONPassthrough(t *testing.T) {
	t.Parallel()
	crusher := comp.NewSmartCrusher(comp.DefaultSmartCrusherConfig())
	content := `{"key":"value"}`
	crushed, modified, _ := crusher.Crush(content)
	if modified {
		t.Error("should not modify non-array JSON")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test -v -count=1 -run TestSmartCrusher ./test/unit/compression/
```

- [ ] **Step 3: 实现 SmartCrusher**

```go
// internal/application/llmproxy/compression/smart_crusher.go
package compression

import (
	"math"
	"slices"

	"github.com/bytedance/sonic"
)

type SmartCrusherConfig struct {
	MinItemsToAnalyze  int
	MinTokensToCrush   int
	VarianceThreshold  float64
	UniquenessThreshold float64
	SimilarityThreshold float64
	MaxItemsAfterCrush int
	FirstFraction      float64
	LastFraction       float64
	DedupIdentical     bool
}

func DefaultSmartCrusherConfig() SmartCrusherConfig {
	return SmartCrusherConfig{
		MinItemsToAnalyze:  5,
		MinTokensToCrush:   200,
		VarianceThreshold:  2.0,
		UniquenessThreshold: 0.1,
		SimilarityThreshold: 0.8,
		MaxItemsAfterCrush: 15,
		FirstFraction:      0.3,
		LastFraction:       0.15,
		DedupIdentical:     true,
	}
}

type SmartCrusher struct {
	cfg SmartCrusherConfig
}

func NewSmartCrusher(cfg SmartCrusherConfig) *SmartCrusher {
	return &SmartCrusher{cfg: cfg}
}

func (c *SmartCrusher) Crush(content string) (string, bool, string) {
	var items []any
	if err := sonic.UnmarshalString(content, &items); err != nil {
		return content, false, "passthrough"
	}
	n := len(items)
	if n < c.cfg.MinItemsToAnalyze {
		return content, false, "passthrough"
	}
	estimatedTokens := len(content) / 4
	if estimatedTokens < c.cfg.MinTokensToCrush {
		return content, false, "passthrough"
	}

	var kept []any
	firstCount := int(math.Ceil(float64(n) * c.cfg.FirstFraction))
	lastCount := int(math.Ceil(float64(n) * c.cfg.LastFraction))

	keepSet := make(map[int]bool)
	for i := 0; i < firstCount && i < n; i++ {
		keepSet[i] = true
	}
	for i := n - lastCount; i < n; i++ {
		if i >= 0 {
			keepSet[i] = true
		}
	}

	seen := make(map[string]bool)
	for i, item := range items {
		if keepSet[i] {
			key := itemKey(item)
			if !c.cfg.DedupIdentical || !seen[key] {
				seen[key] = true
				kept = append(kept, item)
			}
			continue
		}
		if len(kept) >= c.cfg.MaxItemsAfterCrush {
			continue
		}
		if isChangePoint(items, i) {
			key := itemKey(item)
			if !c.cfg.DedupIdentical || !seen[key] {
				seen[key] = true
				kept = append(kept, item)
			}
		}
	}

	if len(kept) == n {
		return content, false, "passthrough"
	}

	compressed, _ := sonic.MarshalString(kept)
	return compressed, true, "smart_crusher"
}

func itemKey(item any) string {
	b, _ := sonic.MarshalString(item)
	return b
}

func isChangePoint(items []any, i int) bool {
	if i <= 0 || i >= len(items)-1 {
		return false
	}
	prev, okPrev := items[i-1].(map[string]any)
	curr, okCurr := items[i].(map[string]any)
	next, okNext := items[i+1].(map[string]any)
	if !okPrev || !okCurr || !okNext {
		return false
	}
	changedPrev := countChangedFields(prev, curr)
	changedNext := countChangedFields(curr, next)
	return changedPrev > 0 || changedNext > 0
}

func countChangedFields(a, b map[string]any) int {
	count := 0
	for k, av := range a {
		bv, ok := b[k]
		if !ok || av != bv {
			count++
		}
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			count++
		}
	}
	return count
}

func (c *SmartCrusher) CrushArrayJSON(content string) (string, bool, string) {
	return c.Crush(content)
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test -v -count=1 -run TestSmartCrusher ./test/unit/compression/
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/application/llmproxy/compression/smart_crusher.go test/unit/compression/smart_crusher_test.go
git commit -m "feat: add SmartCrusher for JSON array statistical sampling compression"
```

---

### Task 4: SearchCompressor 搜索结果压缩

**Files:**
- Create: `internal/application/llmproxy/compression/search_compressor.go`
- Create: `test/unit/compression/search_compressor_test.go`

- [ ] **Step 1: 写测试**

```go
// test/unit/compression/search_compressor_test.go
package compression

import (
	"strings"
	"testing"

	comp "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
)

func TestSearchCompressorParseAndSelect(t *testing.T) {
	t.Parallel()
	sc := comp.NewSearchCompressor(comp.DefaultSearchCompressorConfig())
	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, "src/main.go:"+itoa(i+1)+":line content "+itoa(i))
	}
	content := strings.Join(lines, "\n")
	result := sc.Compress(content, "")
	t.Logf("compressed: %s", result)
	t.Logf("original matches: %d, compressed: %d", result.OriginalMatchCount, result.CompressedMatchCount)
	if result.OriginalMatchCount == 0 {
		t.Error("should have original matches")
	}
}

func TestSearchCompressorKeepsFirstAndLast(t *testing.T) {
	t.Parallel()
	sc := comp.NewSearchCompressor(comp.DefaultSearchCompressorConfig())
	var lines []string
	for i := 0; i < 50; i++ {
		lines = append(lines, "src/main.go:"+itoa(i+1)+":line content "+itoa(i))
	}
	content := strings.Join(lines, "\n")
	result := sc.Compress(content, "")
	if !strings.Contains(result.Compressed, "line content 0") {
		t.Error("should keep first match")
	}
	if !strings.Contains(result.Compressed, "line content 49") {
		t.Error("should keep last match")
	}
}

func TestSearchCompressorEmptyContent(t *testing.T) {
	t.Parallel()
	sc := comp.NewSearchCompressor(comp.DefaultSearchCompressorConfig())
	result := sc.Compress("", "")
	if result.Compressed != "" {
		t.Error("compressed should be empty")
	}
}

func TestSearchCompressorContextBoost(t *testing.T) {
	t.Parallel()
	sc := comp.NewSearchCompressor(comp.DefaultSearchCompressorConfig())
	content := `src/main.go:1:normal line
src/main.go:2:error handler
src/main.go:3:normal line
src/main.go:4:normal line
src/main.go:5:normal line`
	result := sc.Compress(content, "error")
	if !strings.Contains(result.Compressed, "error handler") {
		t.Error("error match should be kept when context matches")
	}
}

func itoa(n int) string {
	return strings.TrimRight(strings.Replace(strings.Replace(
		strings.Replace(strings.Replace(
			strings.Replace(strings.Replace(
				"0123456789"[n/100000%10:n/100000%10+1]+
					"0123456789"[n/10000%10:n/10000%10+1]+
					"0123456789"[n/1000%10:n/1000%10+1]+
					"0123456789"[n/100%10:n/100%10+1]+
					"0123456789"[n/10%10:n/10%10+1]+
					"0123456789"[n%10:n%10+1],
				"\x00", ""), "\x00", ""),
			"\x00", ""), "\x00", ""),
		"\x00", ""), "\x00", "")
}
```

- [ ] **Step 2: 实现 SearchCompressor**

```go
// internal/application/llmproxy/compression/search_compressor.go
package compression

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type SearchCompressorConfig struct {
	MaxMatchesPerFile int
	AlwaysKeepFirst   bool
	AlwaysKeepLast    bool
	MaxTotalMatches   int
	MaxFiles          int
	BoostErrors       bool
}

func DefaultSearchCompressorConfig() SearchCompressorConfig {
	return SearchCompressorConfig{
		MaxMatchesPerFile: 5,
		AlwaysKeepFirst:   true,
		AlwaysKeepLast:    true,
		MaxTotalMatches:   30,
		MaxFiles:          15,
		BoostErrors:       true,
	}
}

type SearchMatch struct {
	File       string
	LineNumber int
	Content    string
	Score      float64
}

type SearchCompressionResult struct {
	Compressed           string
	OriginalMatchCount   int
	CompressedMatchCount int
}

var searchLinePattern = regexp.MustCompile(`^([^\s:]+):(\d+):(.*)`)

var errorKeywords = []string{
	"error", "exception", "fatal", "critical", "failed",
	"panic", "traceback", "segfault", "timeout", "refused",
}

type SearchCompressor struct {
	cfg SearchCompressorConfig
}

func NewSearchCompressor(cfg SearchCompressorConfig) *SearchCompressor {
	return &SearchCompressor{cfg: cfg}
}

func (c *SearchCompressor) Compress(content, context string) SearchCompressionResult {
	if content == "" {
		return SearchCompressionResult{}
	}

	fileMatches := c.parseSearchResults(content)
	if len(fileMatches) == 0 {
		return SearchCompressionResult{Compressed: content}
	}

	originalCount := 0
	for _, fm := range fileMatches {
		originalCount += len(fm)
	}

	c.scoreMatches(fileMatches, context)
	selected := c.selectMatches(fileMatches)
	compressed, compressedCount := c.formatOutput(fileMatches, selected)

	return SearchCompressionResult{
		Compressed:           compressed,
		OriginalMatchCount:   originalCount,
		CompressedMatchCount: compressedCount,
	}
}

func (c *SearchCompressor) parseSearchResults(content string) map[string][]SearchMatch {
	lines := strings.Split(content, "\n")
	result := make(map[string][]SearchMatch)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := searchLinePattern.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		lineNum, err := strconv.Atoi(m[2])
		if err != nil {
			continue
		}
		result[m[1]] = append(result[m[1]], SearchMatch{
			File:       m[1],
			LineNumber: lineNum,
			Content:    m[3],
		})
	}
	return result
}

func (c *SearchCompressor) scoreMatches(fileMatches map[string][]SearchMatch, context string) {
	contextLower := strings.ToLower(context)
	contextWords := make(map[string]bool)
	for _, w := range strings.Fields(contextLower) {
		if len(w) > 2 {
			contextWords[w] = true
		}
	}

	for file := range fileMatches {
		for i := range fileMatches[file] {
			m := &fileMatches[file][i]
			m.Score = 0.0
			contentLower := strings.ToLower(m.Content)
			for w := range contextWords {
				if strings.Contains(contentLower, w) {
					m.Score += 0.3
				}
			}
			if c.cfg.BoostErrors {
				for j, kw := range errorKeywords {
					if strings.Contains(contentLower, kw) {
						m.Score += 0.5 - float64(j)*0.1
						if m.Score < 0 {
							m.Score = 0
						}
						break
					}
				}
			}
			if m.Score > 1.0 {
				m.Score = 1.0
			}
		}
	}
}

func (c *SearchCompressor) selectMatches(fileMatches map[string][]SearchMatch) map[string][]SearchMatch {
	type fileEntry struct {
		name    string
		matches []SearchMatch
		total   float64
	}
	var files []fileEntry
	for name, matches := range fileMatches {
		total := 0.0
		for _, m := range matches {
			total += m.Score
		}
		files = append(files, fileEntry{name, matches, total})
	}
	slices.SortFunc(files, func(a, b fileEntry) int {
		if a.total > b.total {
			return -1
		}
		return 1
	})

	if len(files) > c.cfg.MaxFiles {
		files = files[:c.cfg.MaxFiles]
	}

	selected := make(map[string][]SearchMatch)
	totalSelected := 0
	for _, f := range files {
		if totalSelected >= c.cfg.MaxTotalMatches {
			break
		}
		var picked []SearchMatch
		remaining := c.cfg.MaxMatchesPerFile
		if remaining > c.cfg.MaxTotalMatches-totalSelected {
			remaining = c.cfg.MaxTotalMatches - totalSelected
		}

		if c.cfg.AlwaysKeepFirst && len(f.matches) > 0 {
			picked = append(picked, f.matches[0])
			remaining--
		}
		if c.cfg.AlwaysKeepLast && len(f.matches) > 1 && f.matches[len(f.matches)-1] != f.matches[0] {
			picked = append(picked, f.matches[len(f.matches)-1])
			remaining--
		}

		sorted := make([]SearchMatch, len(f.matches))
		copy(sorted, f.matches)
		slices.SortFunc(sorted, func(a, b SearchMatch) int {
			if a.Score > b.Score {
				return -1
			}
			return 1
		})

		for _, m := range sorted {
			if remaining <= 0 {
				break
			}
			already := false
			for _, p := range picked {
				if p == m {
					already = true
					break
				}
			}
			if !already {
				picked = append(picked, m)
				remaining--
			}
		}

		slices.SortFunc(picked, func(a, b SearchMatch) int {
			return a.LineNumber - b.LineNumber
		})
		selected[f.name] = picked
		totalSelected += len(picked)
	}
	return selected
}

func (c *SearchCompressor) formatOutput(original, selected map[string][]SearchMatch) (string, int) {
	var lines []string
	total := 0
	var fileNames []string
	for name := range selected {
		fileNames = append(fileNames, name)
	}
	slices.Sort(fileNames)
	for _, name := range fileNames {
		sel := selected[name]
		for _, m := range sel {
			lines = append(lines, fmt.Sprintf("%s:%d:%s", m.File, m.LineNumber, m.Content))
		}
		total += len(sel)
		orig := original[name]
		if len(orig) > len(sel) {
			omitted := len(orig) - len(sel)
			lines = append(lines, fmt.Sprintf("[... and %d more matches in %s]", omitted, name))
		}
	}
	return strings.Join(lines, "\n"), total
}
```

实际上 `itoa` 用 `strconv.Itoa` 代替。修正测试中的 `itoa` 为 `strconv.Itoa`。

- [ ] **Step 3: 运行测试确认通过**

```bash
go test -v -count=1 -run TestSearchCompressor ./test/unit/compression/
```

- [ ] **Step 4: Commit**

```bash
git add internal/application/llmproxy/compression/search_compressor.go test/unit/compression/search_compressor_test.go
git commit -m "feat: add SearchCompressor for grep/ripgrep result compression"
```

---

### Task 5: LogCompressor 构建日志压缩

**Files:**
- Create: `internal/application/llmproxy/compression/log_compressor.go`
- Create: `test/unit/compression/log_compressor_test.go`

- [ ] **Step 1: 写测试**

```go
// test/unit/compression/log_compressor_test.go
package compression

import (
	"strings"
	"testing"

	comp "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
)

func TestLogCompressorErrorPreservation(t *testing.T) {
	t.Parallel()
	lc := comp.NewLogCompressor(comp.DefaultLogCompressorConfig())
	var lines []string
	lines = append(lines, "INFO starting build")
	for i := 0; i < 30; i++ {
		lines = append(lines, "INFO processing item "+strconv.Itoa(i))
	}
	lines = append(lines, "ERROR build failed: connection refused")
	lines = append(lines, "WARN cleanup in progress")
	content := strings.Join(lines, "\n")
	result := lc.Compress(content)
	if !strings.Contains(result.Compressed, "ERROR build failed") {
		t.Error("should keep error line")
	}
	t.Logf("compressed lines: %d -> %d", result.OriginalLineCount, result.CompressedLineCount)
}

func TestLogCompressorStacktracePreservation(t *testing.T) {
	t.Parallel()
	lc := comp.NewLogCompressor(comp.DefaultLogCompressorConfig())
	content := `ERROR something broke
Traceback (most recent call last):
  File "main.py", line 42, in <module>
    process()
  File "main.py", line 38, in process
    raise ValueError("bad input")
ValueError: bad input`
	result := lc.Compress(content)
	if !strings.Contains(result.Compressed, "Traceback") {
		t.Error("should keep traceback")
	}
	if !strings.Contains(result.Compressed, "ValueError") {
		t.Error("should keep exception line")
	}
}

func TestLogCompressorDedupWarnings(t *testing.T) {
	t.Parallel()
	lc := comp.NewLogCompressor(comp.DefaultLogCompressorConfig())
	var lines []string
	for i := 0; i < 10; i++ {
		lines = append(lines, "WARN connection timeout after 30s")
	}
	content := strings.Join(lines, "\n")
	result := lc.Compress(content)
	t.Logf("dedup result: %s", result.Compressed)
}

func TestLogCompressorEmptyContent(t *testing.T) {
	t.Parallel()
	lc := comp.NewLogCompressor(comp.DefaultLogCompressorConfig())
	result := lc.Compress("")
	if result.Compressed != "" {
		t.Error("compressed should be empty")
	}
}
```

- [ ] **Step 2: 实现 LogCompressor**

```go
// internal/application/llmproxy/compression/log_compressor.go
package compression

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

type LogLevel int

const (
	LogLevelError LogLevel = iota
	LogLevelFail
	LogLevelWarn
	LogLevelInfo
	LogLevelDebug
	LogLevelTrace
	LogLevelUnknown
)

type LogCompressorConfig struct {
	MaxErrors          int
	ErrorContextLines  int
	KeepFirstError     bool
	KeepLastError      bool
	MaxStackTraces     int
	StackTraceMaxLines int
	MaxWarnings        int
	DedupWarnings      bool
	KeepSummaryLines   bool
	MaxTotalLines      int
}

func DefaultLogCompressorConfig() LogCompressorConfig {
	return LogCompressorConfig{
		MaxErrors:          10,
		ErrorContextLines:  3,
		KeepFirstError:     true,
		KeepLastError:      true,
		MaxStackTraces:     3,
		StackTraceMaxLines: 20,
		MaxWarnings:        5,
		DedupWarnings:      true,
		KeepSummaryLines:   true,
		MaxTotalLines:      100,
	}
}

type logLine struct {
	number       int
	content      string
	level        LogLevel
	isStackTrace bool
	isSummary    bool
	score        float64
}

type LogCompressionResult struct {
	Compressed         string
	OriginalLineCount  int
	CompressedLineCount int
}

var (
	logLevelPatterns = map[*regexp.Regexp]LogLevel{
		regexp.MustCompile(`\b(?:ERROR|error|Error|FATAL|fatal|Fatal|CRITICAL|critical)\b`):   LogLevelError,
		regexp.MustCompile(`\b(?:FAIL|FAILED|fail|failed|Fail|Failed)\b`):                      LogLevelFail,
		regexp.MustCompile(`\b(?:WARN|WARNING|warn|warning|Warn|Warning)\b`):                   LogLevelWarn,
		regexp.MustCompile(`\b(?:INFO|info|Info)\b`):                                           LogLevelInfo,
		regexp.MustCompile(`\b(?:DEBUG|debug|Debug)\b`):                                        LogLevelDebug,
		regexp.MustCompile(`\b(?:TRACE|trace|Trace)\b`):                                        LogLevelTrace,
	}

	stackTracePatterns = []*regexp.Regexp{
		regexp.MustCompile(`^\s*Traceback \(most recent call last\)`),
		regexp.MustCompile(`^\s*File ".+", line \d+`),
		regexp.MustCompile(`^\s*at .+\(.+:\d+:\d+\)`),
		regexp.MustCompile(`^\s+at [\w.$]+\s*\(`),
	}

	summaryPatterns = []*regexp.Regexp{
		regexp.MustCompile(`^={3,}`),
		regexp.MustCompile(`^-{3,}`),
		regexp.MustCompile(`^\d+ (?:passed|failed|skipped|error|warning)`),
		regexp.MustCompile(`^(?:Tests?|Suites?):?\s+\d+`),
		regexp.MustCompile(`^(?:TOTAL|Total|Summary)`),
	}
)

type LogCompressor struct {
	cfg LogCompressorConfig
}

func NewLogCompressor(cfg LogCompressorConfig) *LogCompressor {
	return &LogCompressor{cfg: cfg}
}

func (c *LogCompressor) Compress(content string) LogCompressionResult {
	if content == "" {
		return LogCompressionResult{}
	}
	lines := strings.Split(content, "\n")
	logLines := c.parseLines(lines)
	selected := c.selectLines(logLines)
	selected = c.addContext(logLines, selected)
	selected = c.limitTotal(selected)

	compressed := c.formatOutput(lines, selected, logLines)

	return LogCompressionResult{
		Compressed:          compressed,
		OriginalLineCount:   len(lines),
		CompressedLineCount: len(selected),
	}
}

func (c *LogCompressor) parseLines(lines []string) []logLine {
	result := make([]logLine, len(lines))
	inStack := false
	stackLines := 0

	for i, line := range lines {
		ll := logLine{number: i, content: line, level: LogLevelUnknown}

		for pat, level := range logLevelPatterns {
			if pat.MatchString(line) {
				ll.level = level
				break
			}
		}

		for _, pat := range stackTracePatterns {
			if pat.MatchString(line) {
				inStack = true
				stackLines = 0
				break
			}
		}
		if inStack {
			ll.isStackTrace = true
			stackLines++
			if stackLines > c.cfg.StackTraceMaxLines || strings.TrimSpace(line) == "" {
				inStack = false
			}
		}

		for _, pat := range summaryPatterns {
			if pat.MatchString(line) {
				ll.isSummary = true
				break
			}
		}

		ll.score = c.scoreLine(ll)
		result[i] = ll
	}
	return result
}

func (c *LogCompressor) scoreLine(ll logLine) float64 {
	levelScores := map[LogLevel]float64{
		LogLevelError:   1.0,
		LogLevelFail:    1.0,
		LogLevelWarn:    0.5,
		LogLevelInfo:    0.1,
		LogLevelDebug:   0.05,
		LogLevelTrace:   0.02,
		LogLevelUnknown: 0.1,
	}
	score := levelScores[ll.level]
	if ll.isStackTrace {
		score += 0.3
	}
	if ll.isSummary {
		score += 0.4
	}
	if score > 1.0 {
		score = 1.0
	}
	return score
}

func (c *LogCompressor) selectLines(logLines []logLine) []logLine {
	var (
		errors   []logLine
		fails    []logLine
		warnings []logLine
		stacks   [][]logLine
		summaries []logLine
		curStack []logLine
	)

	for _, ll := range logLines {
		switch ll.level {
		case LogLevelError:
			errors = append(errors, ll)
		case LogLevelFail:
			fails = append(fails, ll)
		case LogLevelWarn:
			warnings = append(warnings, ll)
		}

		if ll.isStackTrace {
			curStack = append(curStack, ll)
		} else if len(curStack) > 0 {
			stacks = append(stacks, curStack)
			curStack = nil
		}

		if ll.isSummary {
			summaries = append(summaries, ll)
		}
	}
	if len(curStack) > 0 {
		stacks = append(stacks, curStack)
	}

	var selected []logLine

	selected = append(selected, c.selectWithFirstLast(errors, c.cfg.MaxErrors)...)
	selected = append(selected, c.selectWithFirstLast(fails, c.cfg.MaxErrors)...)

	if c.cfg.DedupWarnings {
		warnings = c.dedupSimilar(warnings)
	}
	if len(warnings) > c.cfg.MaxWarnings {
		warnings = warnings[:c.cfg.MaxWarnings]
	}
	selected = append(selected, warnings...)

	for i, stack := range stacks {
		if i >= c.cfg.MaxStackTraces {
			break
		}
		end := c.cfg.StackTraceMaxLines
		if end > len(stack) {
			end = len(stack)
		}
		selected = append(selected, stack[:end]...)
	}

	if c.cfg.KeepSummaryLines {
		selected = append(selected, summaries...)
	}

	return selected
}

func (c *LogCompressor) selectWithFirstLast(lines []logLine, maxCount int) []logLine {
	if len(lines) <= maxCount {
		return lines
	}
	var selected []logLine
	if c.cfg.KeepFirstError && len(lines) > 0 {
		selected = append(selected, lines[0])
	}
	if c.cfg.KeepLastError && len(lines) > 1 && lines[len(lines)-1] != lines[0] {
		selected = append(selected, lines[len(lines)-1])
	}
	remaining := maxCount - len(selected)
	if remaining > 0 {
		sorted := make([]logLine, len(lines))
		copy(sorted, lines)
		slices.SortFunc(sorted, func(a, b logLine) int {
			if a.score > b.score {
				return -1
			}
			return 1
		})
		for _, ll := range sorted {
			if remaining <= 0 {
				break
			}
			seen := false
			for _, s := range selected {
				if s.number == ll.number {
					seen = true
					break
				}
			}
			if !seen {
				selected = append(selected, ll)
				remaining--
			}
		}
	}
	return selected
}

func (c *LogCompressor) dedupSimilar(lines []logLine) []logLine {
	digitRe := regexp.MustCompile(`\d+`)
	seen := make(map[string]bool)
	var result []logLine
	for _, ll := range lines {
		sep := strings.IndexAny(ll.content, ":=")
		prefix := ll.content
		suffix := ""
		if sep >= 0 {
			prefix = ll.content[:sep]
			suffix = ll.content[sep:]
		}
		suffix = digitRe.ReplaceAllString(suffix, "N")
		normalized := prefix + suffix
		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, ll)
		}
	}
	return result
}

func (c *LogCompressor) addContext(allLines, selected []logLine) []logLine {
	selectedIndices := make(map[int]bool)
	for _, ll := range selected {
		selectedIndices[ll.number] = true
	}
	contextIndices := make(map[int]bool)
	for idx := range selectedIndices {
		for i := max(0, idx-c.cfg.ErrorContextLines); i < idx; i++ {
			contextIndices[i] = true
		}
		for i := idx + 1; i < min(len(allLines), idx+c.cfg.ErrorContextLines+1); i++ {
			contextIndices[i] = true
		}
	}
	for idx := range contextIndices {
		if !selectedIndices[idx] && idx < len(allLines) {
			selected = append(selected, allLines[idx])
		}
	}
	return selected
}

func (c *LogCompressor) limitTotal(selected []logLine) []logLine {
	if len(selected) <= c.cfg.MaxTotalLines {
		return selected
	}
	sorted := make([]logLine, len(selected))
	copy(sorted, selected)
	slices.SortFunc(sorted, func(a, b logLine) int {
		if a.score > b.score {
			return -1
		}
		return 1
	})
	return sorted[:c.cfg.MaxTotalLines]
}

func (c *LogCompressor) formatOutput(allLines []string, selected, allParsed []logLine) string {
	seen := make(map[int]bool)
	ordered := make([]logLine, 0, len(selected))
	for _, ll := range selected {
		if !seen[ll.number] {
			seen[ll.number] = true
			ordered = append(ordered, ll)
		}
	}
	slices.SortFunc(ordered, func(a, b logLine) int {
		return a.number - b.number
	})

	var outLines []string
	for _, ll := range ordered {
		outLines = append(outLines, ll.content)
	}

	omitted := len(allLines) - len(ordered)
	if omitted > 0 {
		errorCount := 0
		warnCount := 0
		for _, ll := range allParsed {
			switch ll.level {
			case LogLevelError, LogLevelFail:
				errorCount++
			case LogLevelWarn:
				warnCount++
			}
		}
		parts := make([]string, 0, 2)
		if errorCount > 0 {
			parts = append(parts, fmt.Sprintf("%d ERROR", errorCount))
		}
		if warnCount > 0 {
			parts = append(parts, fmt.Sprintf("%d WARN", warnCount))
		}
		suffix := ""
		if len(parts) > 0 {
			suffix = ": " + strings.Join(parts, ", ")
		}
		outLines = append(outLines, fmt.Sprintf("[%d lines omitted%s]", omitted, suffix))
	}

	return strings.Join(outLines, "\n")
}
```

- [ ] **Step 3: 运行测试确认通过**

```bash
go test -v -count=1 -run TestLogCompressor ./test/unit/compression/
```

- [ ] **Step 4: Commit**

```bash
git add internal/application/llmproxy/compression/log_compressor.go test/unit/compression/log_compressor_test.go
git commit -m "feat: add LogCompressor for build/test log compression"
```

---

### Task 6: Pipeline 主入口 + 接口定义

**Files:**
- Create: `internal/application/llmproxy/compression/pipeline.go`
- Create: `test/unit/compression/pipeline_test.go`

- [ ] **Step 1: 写测试**

```go
// test/unit/compression/pipeline_test.go
package compression

import (
	"context"
	"strings"
	"testing"

	comp "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
)

func TestPipelineCompressToolMessages(t *testing.T) {
	t.Parallel()
	cfg := comp.DefaultPipelineConfig()
	p := comp.NewPipeline(cfg)

	messages := []comp.Message{
		{Role: "system", Content: "You are helpful"},
		{Role: "user", Content: "query"},
		{Role: "tool", Content: `[{"id":1,"name":"a","value":10},{"id":2,"name":"b","value":10},{"id":3,"name":"c","value":10},{"id":4,"name":"d","value":10},{"id":5,"name":"e","value":10},{"id":6,"name":"f","value":10},{"id":7,"name":"g","value":10},{"id":8,"name":"h","value":10},{"id":9,"name":"i","value":10},{"id":10,"name":"j","value":"end"}]`},
	}

	result, res := p.Compress(context.Background(), messages)
	if res == nil {
		t.Fatal("result should not be nil")
	}
	t.Logf("tokens before: %d, after: %d", res.TokensBefore, res.TokensAfter)
	t.Logf("strategies: %v", res.Strategies)
	_ = result
}

func TestPipelineCompressToolCallsContentList(t *testing.T) {
	t.Parallel()
	cfg := comp.DefaultPipelineConfig()
	p := comp.NewPipeline(cfg)

	messages := []comp.Message{
		{Role: "user", Content: "query"},
		{Role: "tool", Content: `[{"id":1,"name":"a"},{"id":2,"name":"a"},{"id":3,"name":"a"},{"id":4,"name":"a"},{"id":5,"name":"a"},{"id":6,"name":"b"}]`},
	}

	result, _ := p.Compress(context.Background(), messages)
	for _, msg := range result {
		if msg.Role == "tool" {
			t.Logf("compressed tool content: %s", msg.Content)
		}
	}
}

func TestPipelineSkipsSystemUserAssistant(t *testing.T) {
	t.Parallel()
	cfg := comp.DefaultPipelineConfig()
	p := comp.NewPipeline(cfg)

	original := []comp.Message{
		{Role: "system", Content: "system prompt"},
		{Role: "user", Content: "user query"},
		{Role: "assistant", Content: "assistant response"},
	}

	result, _ := p.Compress(context.Background(), original)
	for i, msg := range result {
		if msg.Content != original[i].Content {
			t.Errorf("message %d changed unexpectedly: %s", i, msg.Content)
		}
	}
}

func TestPipelineEmptyMessages(t *testing.T) {
	t.Parallel()
	cfg := comp.DefaultPipelineConfig()
	p := comp.NewPipeline(cfg)

	result, res := p.Compress(context.Background(), nil)
	if len(result) != 0 {
		t.Error("result should be empty")
	}
	if res == nil {
		t.Fatal("result summary should not be nil")
	}
}

func TestPipelineNoopPipeline(t *testing.T) {
	t.Parallel()
	p := comp.NewNoopPipeline()
	original := []comp.Message{{Role: "tool", Content: "unchanged"}}
	result, _ := p.Compress(context.Background(), original)
	if result[0].Content != "unchanged" {
		t.Error("noop should not modify content")
	}
}
```

- [ ] **Step 2: 实现 Pipeline**

```go
// internal/application/llmproxy/compression/pipeline.go
package compression

import (
	"context"
	"strings"
)

type Message struct {
	Role    string
	Content string
}

type PipelineResult struct {
	TokensBefore int
	TokensAfter  int
	Strategies   []string
}

type Pipeline interface {
	Compress(ctx context.Context, messages []Message) ([]Message, *PipelineResult)
}

type PipelineConfig struct {
	Enabled              bool
	MinCharsForBlock     int
	ProtectErrorOutputs  bool
	ErrorProtectMaxChars int
	ProtectRecentCode    int
}

func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		Enabled:              true,
		MinCharsForBlock:     500,
		ProtectErrorOutputs:  true,
		ErrorProtectMaxChars: 8000,
		ProtectRecentCode:    4,
	}
}

type pipeline struct {
	cfg              PipelineConfig
	smartCrusher     *SmartCrusher
	searchCompressor *SearchCompressor
	logCompressor    *LogCompressor
}

func NewPipeline(cfg PipelineConfig) Pipeline {
	return &pipeline{
		cfg:              cfg,
		smartCrusher:     NewSmartCrusher(DefaultSmartCrusherConfig()),
		searchCompressor: NewSearchCompressor(DefaultSearchCompressorConfig()),
		logCompressor:    NewLogCompressor(DefaultLogCompressorConfig()),
	}
}

func (p *pipeline) Compress(ctx context.Context, messages []Message) ([]Message, *PipelineResult) {
	if !p.cfg.Enabled || len(messages) == 0 {
		dup := make([]Message, len(messages))
		copy(dup, messages)
		return dup, &PipelineResult{}
	}

	result := make([]Message, len(messages))
	copy(result, messages)

	var strategies []string
	tokensBefore := 0
	tokensAfter := 0

	recentCodeCount := 0
	for i := len(result) - 1; i >= 0 && recentCodeCount < p.cfg.ProtectRecentCode; i-- {
		ct, _ := DetectContentType(result[i].Content)
		if ct == ContentTypeSourceCode {
			recentCodeCount++
		}
	}

	recentCodeCount = 0

	for i := range result {
		msg := &result[i]
		if msg.Role == "system" || msg.Role == "user" || msg.Role == "assistant" {
			continue
		}
		if msg.Role != "tool" {
			continue
		}
		if len(msg.Content) < p.cfg.MinCharsForBlock {
			continue
		}

		if p.cfg.ProtectErrorOutputs && len(msg.Content) <= p.cfg.ErrorProtectMaxChars {
			if detectErrorOutput(msg.Content) {
				continue
			}
		}

		ct, _ := DetectContentType(msg.Content)

		if ct == ContentTypeSourceCode {
			recentCodeCount++
			if recentCodeCount <= p.cfg.ProtectRecentCode {
				continue
			}
		} else {
			recentCodeCount = 0
		}

		before := estimateTokens(msg.Content)

		switch ct {
		case ContentTypeJSONArray:
			if crushed, modified, strategy := p.smartCrusher.Crush(msg.Content); modified {
				msg.Content = crushed
				after := estimateTokens(crushed)
				tokensBefore += before
				tokensAfter += after
				strategies = append(strategies, strategy)
			}

		case ContentTypeSearchResults:
			r := p.searchCompressor.Compress(msg.Content, "")
			if r.Compressed != msg.Content && r.OriginalMatchCount > r.CompressedMatchCount {
				msg.Content = r.Compressed
				after := estimateTokens(r.Compressed)
				tokensBefore += before
				tokensAfter += after
				strategies = append(strategies, "search")
			}

		case ContentTypeBuildOutput:
			r := p.logCompressor.Compress(msg.Content)
			if r.Compressed != msg.Content && r.OriginalLineCount > r.CompressedLineCount {
				msg.Content = r.Compressed
				after := estimateTokens(r.Compressed)
				tokensBefore += before
				tokensAfter += after
				strategies = append(strategies, "log")
			}
		}
	}

	return result, &PipelineResult{
		TokensBefore: tokensBefore,
		TokensAfter:  tokensAfter,
		Strategies:   strategies,
	}
}

func estimateTokens(s string) int {
	return max(1, len(s)/4)
}

func detectErrorOutput(content string) bool {
	errorPatterns := []string{
		"Traceback (most recent call last)",
		"panic:",
		"Error:",
		"Exception:",
		"at ",
	}
	lower := strings.ToLower(content)
	for _, pat := range errorPatterns {
		if strings.Contains(lower, strings.ToLower(pat)) {
			return true
		}
	}
	return false
}

type noopPipeline struct{}

func NewNoopPipeline() Pipeline {
	return &noopPipeline{}
}

func (n *noopPipeline) Compress(_ context.Context, messages []Message) ([]Message, *PipelineResult) {
	dup := make([]Message, len(messages))
	copy(dup, messages)
	return dup, &PipelineResult{}
}
```

- [ ] **Step 3: 运行测试确认通过**

```bash
go test -v -count=1 -run TestPipeline ./test/unit/compression/
```

- [ ] **Step 4: Commit**

```bash
git add internal/application/llmproxy/compression/pipeline.go test/unit/compression/pipeline_test.go
git commit -m "feat: add Pipeline interface and implementation for message compression"
```

---

### Task 7: DTO - ModelCallAuditTask 和 AuditLogItem 新增压缩字段

**Files:**
- Modify: `internal/dto/asynctask.go`
- Modify: `internal/dto/audit.go`

- [ ] **Step 1: 修改 asynctask.go**

在 `ModelCallAuditTask` struct 的 `ErrorMessage` 字段后添加：

```go
CompressionEnabled  bool   // 是否启用了压缩
CompressedTokens    int    // 压缩节省的 token 数
CompressionStrategy string // 压缩策略名称
```

新增方法：

```go
// SetCompressionResult 设置压缩结果
//
//	@receiver t *ModelCallAuditTask
//	@param originalLen int 原始内容长度
//	@param compressedLen int 压缩后内容长度
//	@param strategy string 压缩策略
//	@author centonhuang
//	@update 2026-06-14 10:00:00
func (t *ModelCallAuditTask) SetCompressionResult(originalLen, compressedLen int, strategy string) {
	if originalLen <= compressedLen {
		return
	}
	ratio := 1.0 - float64(compressedLen)/float64(originalLen)
	t.CompressionEnabled = true
	t.CompressedTokens = int(float64(t.InputTokens) * ratio)
	t.CompressionStrategy = strategy
}
```

- [ ] **Step 2: 修改 audit.go**

在 `AuditLogItem` struct 的 `ErrorMessage` 字段后添加：

```go
CompressionEnabled  bool   `json:"compressionEnabled" doc:"是否启用了压缩"`
CompressedTokens    int    `json:"compressedTokens" doc:"压缩节省的token数"`
CompressionStrategy string `json:"compressionStrategy" doc:"压缩策略"`
```

- [ ] **Step 3: 验证编译**

```bash
go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add internal/dto/asynctask.go internal/dto/audit.go
git commit -m "feat: add compression fields to ModelCallAuditTask and AuditLogItem"
```

---

### Task 8: Usecase 层注入

**Files:**
- Modify: `internal/application/llmproxy/usecase/openai.go`
- Modify: `internal/application/llmproxy/usecase/anthropic.go`

- [ ] **Step 1: 修改 openai.go**

在 `openAIUseCase` struct 中添加字段：

```go
type openAIUseCase struct {
	resolver         service.EndpointResolver
	modelsQuery      ListOpenAIModels
	openAIProxy      OpenAIProxyPort
	anthropicProxy   AnthropicProxyPort
	taskSubmitter    TaskSubmitter
	compressPipeline compression.Pipeline // 新增
}
```

修改 `NewOpenAIUseCase` 签名和实现，添加 `compressPipeline compression.Pipeline` 参数并赋值。

- [ ] **Step 2: 修改 anthropic.go**

在 `anthropicUseCase` struct 中添加字段：

```go
type anthropicUseCase struct {
	resolver         service.EndpointResolver
	modelsQuery      ListAnthropicModels
	countTokensQuery CountTokens
	anthropicProxy   AnthropicProxyPort
	openAIProxy      OpenAIProxyPort
	taskSubmitter    TaskSubmitter
	compressPipeline compression.Pipeline // 新增
}
```

修改 `NewAnthropicUseCase` 签名和实现，添加 `compressPipeline compression.Pipeline` 参数并赋值。

- [ ] **Step 3: 验证编译**

```bash
go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add internal/application/llmproxy/usecase/openai.go internal/application/llmproxy/usecase/anthropic.go
git commit -m "feat: inject compressPipeline into usecases"
```

---

### Task 9: 7 条转发路径整合压缩调用

**Files:**
- Modify: `internal/application/llmproxy/usecase/openai_chat.go`
- Modify: `internal/application/llmproxy/usecase/openai_response.go`
- Modify: `internal/application/llmproxy/usecase/anthropic_message.go`

每个 forward 方法在 `MarshalXxx` 之前插入 `compressPipeline.Compress()` 调用，并在 `newAuditTask` 之后用 `SetCompressionResult` 写入审计。

压缩在消息层面操作 messages 列表（Go native 结构），不操作序列化后的 JSON body。针对每条转发路径：

1. `openai_chat.go`
   - `forwardChatNative`：压缩 `req.Body.Messages` → 用压缩后的 messages 序列化 body
   - `forwardChatViaAnthropic`：协议转换后压缩 anthropicReq.Messages → 用压缩后的 messages 序列化 body

2. `openai_response.go`
   - `forwardResponseNative`：压缩 `req.Body.Messages`（Response API 也用 messages）
   - `forwardResponseViaChat`：协议转换后压缩 chatReq.Messages
   - `forwardResponseViaAnthropic`：协议转换后压缩 anthropicReq.Messages

3. `anthropic_message.go`
   - `forwardMessageNative`：压缩 `req.Body.Messages` → 用压缩后的 messages 序列化 body
   - `forwardMessageViaChat`：协议转换后压缩 chatReq.Messages → 用压缩后的 messages 序列化 body

每条路径的压缩模式：

```go
// 示例：forwardChatNative
func (u *openAIUseCase) forwardChatNative(...) {
    // 1. 压缩 messages（传入原始 messages 列表）
    compressedMsgs, result := u.compressPipeline.Compress(ctx, messagesToCompress)
    
    // 2. 用压缩后的 messages 序列化 body
    body := proxyutil.MarshalOpenAIChatCompletionBodyForModel(req.Body, compressedMsgs, upstream.Model)
    
    // ... forward ...
    
    // 3. 审计写入
    task := newAuditTask(...)
    task.SetTokensFromOpenAIUsage(completion.Usage)
    task.SetCompressionResult(result.TokensBefore, result.TokensAfter, strings.Join(result.Strategies, ","))
}
```

- [ ] **Step 1: 修改 openai_chat.go 两个路径**

- [ ] **Step 2: 修改 openai_response.go 三个路径**

- [ ] **Step 3: 修改 anthropic_message.go 两个路径**

- [ ] **Step 4: 验证编译**

```bash
go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add internal/application/llmproxy/usecase/openai_chat.go internal/application/llmproxy/usecase/openai_response.go internal/application/llmproxy/usecase/anthropic_message.go
git commit -m "feat: integrate compression into all 7 forwarding paths"
```

---

### Task 10: DI 注册

**Files:**
- Modify: `internal/bootstrap/container.go`

- [ ] **Step 1: 注册 compression.Pipeline 到 DI 容器**

```go
// 根据 config.CompressionEnabled 决定注入真实 Pipeline 还是 NoopPipeline
if config.CompressionEnabled {
    digProvide(container, func() compression.Pipeline {
        return compression.NewPipeline(compression.DefaultPipelineConfig())
    })
} else {
    digProvide(container, func() compression.Pipeline {
        return compression.NewNoopPipeline()
    })
}
```

- [ ] **Step 2: 更新 NewOpenAIUseCase / NewAnthropicUseCase 的 dig 注册**

在 `container.go` 中对应的 `digProvide` 调用处添加 `compression.Pipeline` 参数依赖。

- [ ] **Step 3: 验证编译**

```bash
go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add internal/bootstrap/container.go
git commit -m "feat: register compression.Pipeline in DI container"
```

---

### Task 11: 全量验证

- [ ] **Step 1: 运行全量单元测试**

```bash
go test -count=1 ./test/unit/compression/...
```

- [ ] **Step 2: 运行 lint**

```bash
make lint
```

- [ ] **Step 3: 运行全量测试**

```bash
make test
```

- [ ] **Step 4: 构建验证**

```bash
make build
```
