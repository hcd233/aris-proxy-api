# 消息级压缩信息存储与展示 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `vo.UnifiedMessage` 中存储 tool output 的压缩前后内容和策略，重构压缩管线直接在 DTO 上操作，并在 Web session 详情页展示 diff 视图。

**Architecture:** 压缩管线从操作序列化 bytes 改为直接在 typed DTO 上 in-place 修改；per-item 压缩结果通过 `tool_call_id` 关联回 `UnifiedMessage` 的新字段 `RawContent` 和 `CompressionStrategy`；Web 端在 `ToolCallCard` 中新增 diff 视图组件展示 before/after 内容。

**Tech Stack:** Go 1.25, sonic, samber/lo, dig, Next.js 16, React 19, Tailwind v4, diff npm package

---

## File Structure

| 文件 | 操作 | 职责 |
|------|------|------|
| `internal/common/vo/unified_message.go` | 修改 | 加 `RawContent` + `CompressionStrategy` 字段 |
| `internal/application/llmproxy/compression/result.go` | 修改 | `ItemCompressionResult` 加字段；`CompressionStats` 加 `Items`；`addItem` 逻辑 |
| `internal/application/llmproxy/compression/locator.go` | 修改 | 删除旧接口和函数 |
| `internal/application/llmproxy/compression/locator_openai.go` | 重写 | `CompressOpenAIChat` 直接操作 DTO |
| `internal/application/llmproxy/compression/locator_anthropic.go` | 重写 | `CompressAnthropicMessages` 直接操作 DTO |
| `internal/application/llmproxy/compression/locator_responses.go` | 重写 | `CompressOpenAIResponses` 直接操作 DTO |
| `internal/application/llmproxy/compression/apply.go` | **新建** | `ApplyResultsToMessages` 函数 |
| `internal/application/llmproxy/usecase/openai.go` | 修改 | 删除 `compressBodyIfNeeded`，新增 `compressMessagesIfNeeded` |
| `internal/application/llmproxy/usecase/anthropic.go` | 修改 | 同上 |
| `internal/application/llmproxy/usecase/openai_chat.go` | 修改 | forward 路径改用 DTO 压缩 + store 传参 |
| `internal/application/llmproxy/usecase/openai_response.go` | 修改 | 同上 |
| `internal/application/llmproxy/usecase/anthropic_message.go` | 修改 | 同上 |
| `internal/application/llmproxy/usecase/openai_store.go` | 修改 | store 方法加 `compResults` 参数 |
| `internal/application/llmproxy/usecase/anthropic_store.go` | 修改 | 同上 |
| `internal/config/config.go` | 修改 | 移除 `CompressionMinBodyBytes` |
| `env/api.env.template` | 修改 | 移除 `COMPRESSION_MIN_BODY_BYTES` |
| `test/unit/compression/*_test.go` | 修改 | 适配 DTO 输入 |
| `test/unit/compression/apply_test.go` | **新建** | `ApplyResultsToMessages` 单测 |
| `web/src/lib/types.ts` | 修改 | `UnifiedMessage` 加 2 字段 |
| `web/src/components/chat/content-extract.ts` | 修改 | `ToolResultInfo` + 返回类型 |
| `web/src/components/chat/chat-message.tsx` | 修改 | prop 类型 |
| `web/src/components/chat/assistant-message.tsx` | 修改 | prop 类型 |
| `web/src/components/chat/tool-call-card.tsx` | 修改 | 压缩展示 + diff 触发 |
| `web/src/components/chat/compression-diff.tsx` | **新建** | diff 组件 |
| `web/src/app/share/page.tsx` | 修改 | prop 类型 |
| `web/package.json` | 修改 | 加 `diff` + `@types/diff` |

---

### Task 1: 扩展 `vo.UnifiedMessage`

**Files:**
- Modify: `internal/common/vo/unified_message.go:90-98`

- [ ] **Step 1: 添加 `RawContent` 和 `CompressionStrategy` 字段**

在 `UnifiedMessage` 结构体的 `Refusal` 字段之后添加：

```go
	// 压缩相关，仅当 tool output 被压缩时设置
	RawContent          *string `json:"raw_content,omitempty" doc:"压缩前原始内容"`
	CompressionStrategy string  `json:"compression_strategy,omitempty" doc:"压缩策略"`
```

- [ ] **Step 2: 验证编译通过**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/common/vo/unified_message.go
git commit -m "feat: add RawContent and CompressionStrategy to UnifiedMessage"
```

---

### Task 2: 扩展 `ItemCompressionResult` 和 `CompressionStats`

**Files:**
- Modify: `internal/application/llmproxy/compression/result.go`

- [ ] **Step 1: 在 `ItemCompressionResult` 添加 `ToolCallID` 和 `Input` 字段**

将 `ItemCompressionResult` 结构体改为：

```go
// ItemCompressionResult 单个 tool output 的压缩结果。
type ItemCompressionResult struct {
	ToolCallID  string // 关联到存储消息的 tool_call_id
	Input       string // 压缩前原始内容
	Output      string // 压缩后内容（或跳过/失败时的原始内容）
	Strategy    string // 策略名（"smart_crusher"/"log_compressor"/"search_compressor"/"passthrough"）
	Applied     bool   // 是否实际执行了压缩
	BytesBefore int    // len(原始内容)
	BytesAfter  int    // len(Output)
}
```

- [ ] **Step 2: 在 `CompressionStats` 添加 `Items` 字段**

将 `CompressionStats` 结构体改为：

```go
// CompressionStats 一个请求的聚合压缩统计。
type CompressionStats struct {
	BytesBefore     int
	BytesAfter      int
	ItemsCompressed int
	ItemsSkipped    int
	StrategiesUsed  []string
	Items           []ItemCompressionResult // per-item 详情列表
}
```

- [ ] **Step 3: 更新 `addItem` 方法，在 `Applied` 时追加到 `Items`**

将 `addItem` 方法改为：

```go
func (s *CompressionStats) addItem(r ItemCompressionResult) {
	s.BytesBefore += r.BytesBefore
	s.BytesAfter += r.BytesAfter
	if r.Applied {
		s.ItemsCompressed++
		if s.StrategiesUsed == nil {
			s.StrategiesUsed = []string{}
		}
		s.StrategiesUsed = append(s.StrategiesUsed, r.Strategy)
		s.Items = append(s.Items, r)
	} else {
		s.ItemsSkipped++
	}
}
```

- [ ] **Step 4: 验证编译通过**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/application/llmproxy/compression/result.go
git commit -m "feat: add ToolCallID, Input to ItemCompressionResult and Items to CompressionStats"
```

---

### Task 3: 创建 `ApplyResultsToMessages` 函数 + 单测

**Files:**
- Create: `internal/application/llmproxy/compression/apply.go`
- Create: `test/unit/compression/apply_test.go`

- [ ] **Step 1: 写测试**

创建 `test/unit/compression/apply_test.go`：

```go
package compression

import (
	"testing"

	comp "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/vo"
)

func TestApplyResultsToMessages_MatchesByToolCallID(t *testing.T) {
	t.Parallel()
	rawBefore := "original content"
	messages := []*vo.UnifiedMessage{
		{
			Role:       enum.RoleTool,
			ToolCallID: "call_001",
			Content:    &vo.UnifiedContent{Text: "compressed"},
		},
	}
	results := []comp.ItemCompressionResult{
		{
			ToolCallID:  "call_001",
			Input:       rawBefore,
			Output:      "compressed",
			Strategy:    "smart_crusher",
			Applied:     true,
			BytesBefore: len(rawBefore),
			BytesAfter:  len("compressed"),
		},
	}

	comp.ApplyResultsToMessages(messages, results)

	if messages[0].RawContent == nil || *messages[0].RawContent != rawBefore {
		t.Error("expected RawContent to be set to original content")
	}
	if messages[0].CompressionStrategy != "smart_crusher" {
		t.Error("expected CompressionStrategy to be 'smart_crusher'")
	}
}

func TestApplyResultsToMessages_SkipsUnmatched(t *testing.T) {
	t.Parallel()
	messages := []*vo.UnifiedMessage{
		{
			Role:       enum.RoleTool,
			ToolCallID: "call_001",
			Content:    &vo.UnifiedContent{Text: "unchanged"},
		},
	}
	results := []comp.ItemCompressionResult{
		{
			ToolCallID: "call_999",
			Input:      "other",
			Strategy:   "smart_crusher",
			Applied:    true,
		},
	}

	comp.ApplyResultsToMessages(messages, results)

	if messages[0].RawContent != nil {
		t.Error("expected RawContent to remain nil for unmatched message")
	}
	if messages[0].CompressionStrategy != "" {
		t.Error("expected CompressionStrategy to remain empty for unmatched message")
	}
}

func TestApplyResultsToMessages_SkipsNotApplied(t *testing.T) {
	t.Parallel()
	messages := []*vo.UnifiedMessage{
		{
			Role:       enum.RoleTool,
			ToolCallID: "call_001",
			Content:    &vo.UnifiedContent{Text: "unchanged"},
		},
	}
	results := []comp.ItemCompressionResult{
		{
			ToolCallID: "call_001",
			Input:      "original",
			Strategy:   "passthrough",
			Applied:    false,
		},
	}

	comp.ApplyResultsToMessages(messages, results)

	if messages[0].RawContent != nil {
		t.Error("expected RawContent to remain nil when Applied=false")
	}
}

func TestApplyResultsToMessages_EmptyResultsNoop(t *testing.T) {
	t.Parallel()
	messages := []*vo.UnifiedMessage{
		{
			Role:       enum.RoleTool,
			ToolCallID: "call_001",
			Content:    &vo.UnifiedContent{Text: "unchanged"},
		},
	}

	comp.ApplyResultsToMessages(messages, nil)

	if messages[0].RawContent != nil {
		t.Error("expected RawContent to remain nil with empty results")
	}
}

func TestApplyResultsToMessages_SkipsEmptyToolCallID(t *testing.T) {
	t.Parallel()
	messages := []*vo.UnifiedMessage{
		{
			Role:    enum.RoleUser,
			Content: &vo.UnifiedContent{Text: "user message"},
		},
	}
	results := []comp.ItemCompressionResult{
		{
			ToolCallID: "call_001",
			Input:      "original",
			Strategy:   "smart_crusher",
			Applied:    true,
		},
	}

	comp.ApplyResultsToMessages(messages, results)

	if messages[0].RawContent != nil {
		t.Error("expected RawContent to remain nil for message without ToolCallID")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -v -count=1 -run TestApplyResultsToMessages ./test/unit/compression/`
Expected: FAIL — `ApplyResultsToMessages` undefined

- [ ] **Step 3: 实现 `ApplyResultsToMessages`**

创建 `internal/application/llmproxy/compression/apply.go`：

```go
package compression

import (
	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/vo"
)

// ApplyResultsToMessages 按 tool_call_id 将压缩结果回填到 UnifiedMessage。
// 仅处理 Applied=true 的结果：设置 RawContent(before) 和 CompressionStrategy。
// 未匹配到的消息不受影响。
func ApplyResultsToMessages(messages []*vo.UnifiedMessage, results []ItemCompressionResult) {
	if len(results) == 0 {
		return
	}
	resultMap := lo.SliceToMap(results, func(r ItemCompressionResult) (string, ItemCompressionResult) {
		return r.ToolCallID, r
	})
	for _, msg := range messages {
		if msg.ToolCallID == "" {
			continue
		}
		if result, ok := resultMap[msg.ToolCallID]; ok && result.Applied {
			msg.RawContent = &result.Input
			msg.CompressionStrategy = result.Strategy
		}
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test -v -count=1 -run TestApplyResultsToMessages ./test/unit/compression/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/application/llmproxy/compression/apply.go test/unit/compression/apply_test.go
git commit -m "feat: add ApplyResultsToMessages to backfill compression info into UnifiedMessage"
```

---

### Task 4: 重写 `locator_openai.go` — `CompressOpenAIChat`

**Files:**
- Rewrite: `internal/application/llmproxy/compression/locator_openai.go`
- Modify: `test/unit/compression/locator_openai_test.go`

- [ ] **Step 1: 重写 `locator_openai.go`**

将整个文件替换为：

```go
package compression

import (
	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// CompressOpenAIChat 扫描 OpenAI Chat Completions 消息中的 role=tool 消息，
// 压缩其 Content.Text，in-place 修改 DTO。
func CompressOpenAIChat(messages []*dto.OpenAIChatCompletionMessageParam, dispatcher *Dispatcher, minToolOutputBytes int) CompressionStats {
	stats := CompressionStats{}
	for _, msg := range messages {
		if msg.Role != enum.RoleTool || msg.Content == nil {
			continue
		}
		content := msg.Content.Text
		if len(content) < minToolOutputBytes {
			stats.addItem(ItemCompressionResult{
				ToolCallID:  lo.FromPtr(msg.ToolCallID),
				Input:       content,
				Output:      content,
				Strategy:    constant.CompressionStrategySkippedTooSmall,
				Applied:     false,
				BytesBefore: len(content),
				BytesAfter:  len(content),
			})
			continue
		}
		result := dispatcher.Compress(content)
		result.ToolCallID = lo.FromPtr(msg.ToolCallID)
		result.Input = content
		stats.addItem(result)
		if result.Applied {
			msg.Content.Text = result.Output
			msg.Content.Parts = nil
		}
	}
	return stats
}
```

- [ ] **Step 2: 重写 `locator_openai_test.go`**

将整个文件替换为适配 DTO 输入的测试：

```go
package compression

import (
	"testing"

	comp "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

func TestCompressOpenAIChat_CompressesToolOutput(t *testing.T) {
	t.Parallel()
	toolCallID := "call_001"
	largeContent := makeLargeJSONArray(20)
	messages := []*dto.OpenAIChatCompletionMessageParam{
		{
			Role:       enum.RoleTool,
			ToolCallID: &toolCallID,
			Content:    &dto.OpenAIMessageContent{Text: largeContent},
		},
	}
	dispatcher := comp.NewDispatcher()

	stats := comp.CompressOpenAIChat(messages, dispatcher, 100)

	if stats.ItemsCompressed == 0 {
		t.Fatal("expected at least 1 item compressed")
	}
	if messages[0].Content.Text == largeContent {
		t.Error("expected message content to be replaced with compressed output")
	}
	if len(stats.Items) == 0 {
		t.Fatal("expected stats.Items to contain per-item results")
	}
	if stats.Items[0].ToolCallID != toolCallID {
		t.Error("expected ToolCallID to be set in result")
	}
	if stats.Items[0].Input != largeContent {
		t.Error("expected Input to contain original content")
	}
}

func TestCompressOpenAIChat_SkipsSmallToolOutput(t *testing.T) {
	t.Parallel()
	toolCallID := "call_002"
	smallContent := "small"
	messages := []*dto.OpenAIChatCompletionMessageParam{
		{
			Role:       enum.RoleTool,
			ToolCallID: &toolCallID,
			Content:    &dto.OpenAIMessageContent{Text: smallContent},
		},
	}
	dispatcher := comp.NewDispatcher()

	stats := comp.CompressOpenAIChat(messages, dispatcher, 100)

	if stats.ItemsCompressed != 0 {
		t.Error("expected 0 items compressed for small content")
	}
	if stats.ItemsSkipped != 1 {
		t.Error("expected 1 item skipped")
	}
	if messages[0].Content.Text != smallContent {
		t.Error("expected small content to remain unchanged")
	}
}

func TestCompressOpenAIChat_SkipsNonToolMessages(t *testing.T) {
	t.Parallel()
	messages := []*dto.OpenAIChatCompletionMessageParam{
		{
			Role:    enum.RoleUser,
			Content: &dto.OpenAIMessageContent{Text: "user message"},
		},
	}
	dispatcher := comp.NewDispatcher()

	stats := comp.CompressOpenAIChat(messages, dispatcher, 0)

	if stats.ItemsCompressed != 0 || stats.ItemsSkipped != 0 {
		t.Error("expected no items processed for non-tool messages")
	}
}

func TestCompressOpenAIChat_NilContentSkipped(t *testing.T) {
	t.Parallel()
	toolCallID := "call_003"
	messages := []*dto.OpenAIChatCompletionMessageParam{
		{
			Role:       enum.RoleTool,
			ToolCallID: &toolCallID,
			Content:    nil,
		},
	}
	dispatcher := comp.NewDispatcher()

	stats := comp.CompressOpenAIChat(messages, dispatcher, 0)

	if stats.ItemsCompressed != 0 || stats.ItemsSkipped != 0 {
		t.Error("expected no items processed for nil content")
	}
}

func makeLargeJSONArray(count int) string {
	result := "["
	for i := 0; i < count; i++ {
		if i > 0 {
			result += ","
		}
		result += `{"id":` + itoa(i) + `,"name":"item_` + itoa(i) + `","data":"some data here"}`
	}
	result += "]"
	return result
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}
```

- [ ] **Step 3: 运行测试确认通过**

Run: `go test -v -count=1 -run TestCompressOpenAIChat ./test/unit/compression/`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/application/llmproxy/compression/locator_openai.go test/unit/compression/locator_openai_test.go
git commit -m "refactor: rewrite OpenAIChat locator to compress DTO in-place"
```

---

### Task 5: 重写 `locator_anthropic.go` — `CompressAnthropicMessages`

**Files:**
- Rewrite: `internal/application/llmproxy/compression/locator_anthropic.go`
- Modify: `test/unit/compression/locator_anthropic_test.go`

- [ ] **Step 1: 重写 `locator_anthropic.go`**

将整个文件替换为：

```go
package compression

import (
	"strings"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// CompressAnthropicMessages 扫描 Anthropic Messages 中的 tool_result content block，
// 压缩其 Content，in-place 修改 DTO。
func CompressAnthropicMessages(messages []*dto.AnthropicMessageParam, dispatcher *Dispatcher, minToolOutputBytes int) CompressionStats { //nolint:gocognit // tool_result content has string and array variants, inherently complex
	stats := CompressionStats{}
	for _, msg := range messages {
		if msg.Content == nil {
			continue
		}
		for _, block := range msg.Content.Blocks {
			if block.Type != constant.CompressionJSONKeyToolResult || block.Content == nil {
				continue
			}
			content := extractAnthropicToolResultText(block.Content)
			if len(content) < minToolOutputBytes {
				stats.addItem(ItemCompressionResult{
					ToolCallID:  lo.FromPtr(block.ToolUseID),
					Input:       content,
					Output:      content,
					Strategy:    constant.CompressionStrategySkippedTooSmall,
					Applied:     false,
					BytesBefore: len(content),
					BytesAfter:  len(content),
				})
				continue
			}
			result := dispatcher.Compress(content)
			result.ToolCallID = lo.FromPtr(block.ToolUseID)
			result.Input = content
			stats.addItem(result)
			if result.Applied {
				block.Content.Text = result.Output
				block.Content.Blocks = nil
			}
		}
	}
	return stats
}

// extractAnthropicToolResultText 从 AnthropicToolResultContent 中提取文本。
// 若 Text 非空则直接返回；若 Blocks 非空则提取所有 text block 的 text 合并返回。
func extractAnthropicToolResultText(content *dto.AnthropicToolResultContent) string {
	if content.Text != "" {
		return content.Text
	}
	if len(content.Blocks) == 0 {
		return ""
	}
	texts := make([]string, 0, len(content.Blocks))
	for _, block := range content.Blocks {
		if block.Text != nil {
			texts = append(texts, *block.Text)
		}
	}
	return strings.Join(texts, "\n")
}
```

- [ ] **Step 2: 重写 `locator_anthropic_test.go`**

将整个文件替换为适配 DTO 输入的测试：

```go
package compression

import (
	"testing"

	comp "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

func TestCompressAnthropicMessages_CompressesStringContent(t *testing.T) {
	t.Parallel()
	toolUseID := "toolu_001"
	largeContent := makeLargeJSONArray(20)
	messages := []*dto.AnthropicMessageParam{
		{
			Role: "user",
			Content: &dto.AnthropicMessageContent{
				Blocks: []*dto.AnthropicContentBlock{
					{
						Type:      constant.CompressionJSONKeyToolResult,
						ToolUseID: &toolUseID,
						Content:   &dto.AnthropicToolResultContent{Text: largeContent},
					},
				},
			},
		},
	}
	dispatcher := comp.NewDispatcher()

	stats := comp.CompressAnthropicMessages(messages, dispatcher, 100)

	if stats.ItemsCompressed == 0 {
		t.Fatal("expected at least 1 item compressed")
	}
	if messages[0].Content.Blocks[0].Content.Text == largeContent {
		t.Error("expected content to be replaced with compressed output")
	}
	if len(stats.Items) == 0 || stats.Items[0].ToolCallID != toolUseID {
		t.Error("expected ToolCallID to be set in result")
	}
}

func TestCompressAnthropicMessages_CompressesArrayContent(t *testing.T) {
	t.Parallel()
	toolUseID := "toolu_002"
	text1 := "line1: " + makeLargeJSONArray(10)
	text2 := "line2: " + makeLargeJSONArray(10)
	messages := []*dto.AnthropicMessageParam{
		{
			Role: "user",
			Content: &dto.AnthropicMessageContent{
				Blocks: []*dto.AnthropicContentBlock{
					{
						Type:      constant.CompressionJSONKeyToolResult,
						ToolUseID: &toolUseID,
						Content: &dto.AnthropicToolResultContent{
							Blocks: []*dto.AnthropicContentBlock{
								{Type: "text", Text: &text1},
								{Type: "text", Text: &text2},
							},
						},
					},
				},
			},
		},
	}
	dispatcher := comp.NewDispatcher()

	stats := comp.CompressAnthropicMessages(messages, dispatcher, 100)

	if stats.ItemsCompressed == 0 {
		t.Fatal("expected at least 1 item compressed for array content")
	}
}

func TestCompressAnthropicMessages_SkipsNonToolResultBlocks(t *testing.T) {
	t.Parallel()
	originalText := "some text"
	messages := []*dto.AnthropicMessageParam{
		{
			Role: "assistant",
			Content: &dto.AnthropicMessageContent{
				Blocks: []*dto.AnthropicContentBlock{
					{Type: "text", Text: &originalText},
				},
			},
		},
	}
	dispatcher := comp.NewDispatcher()

	stats := comp.CompressAnthropicMessages(messages, dispatcher, 0)

	if stats.ItemsCompressed != 0 || stats.ItemsSkipped != 0 {
		t.Error("expected no items processed for non-tool_result blocks")
	}
}

func TestCompressAnthropicMessages_NilContentSkipped(t *testing.T) {
	t.Parallel()
	messages := []*dto.AnthropicMessageParam{
		{Role: "user", Content: nil},
	}
	dispatcher := comp.NewDispatcher()

	stats := comp.CompressAnthropicMessages(messages, dispatcher, 0)

	if stats.ItemsCompressed != 0 || stats.ItemsSkipped != 0 {
		t.Error("expected no items processed for nil content")
	}
}
```

- [ ] **Step 3: 运行测试确认通过**

Run: `go test -v -count=1 -run TestCompressAnthropicMessages ./test/unit/compression/`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/application/llmproxy/compression/locator_anthropic.go test/unit/compression/locator_anthropic_test.go
git commit -m "refactor: rewrite Anthropic locator to compress DTO in-place"
```

---

### Task 6: 重写 `locator_responses.go` — `CompressOpenAIResponses`

**Files:**
- Rewrite: `internal/application/llmproxy/compression/locator_responses.go`
- Modify: `test/unit/compression/locator_responses_test.go`

- [ ] **Step 1: 重写 `locator_responses.go`**

将整个文件替换为：

```go
package compression

import (
	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// CompressOpenAIResponses 扫描 OpenAI Responses input 中的 function_call_output 项，
// 压缩其 Output.Text，in-place 修改 DTO。
func CompressOpenAIResponses(items []*dto.ResponseInputItem, dispatcher *Dispatcher, minToolOutputBytes int) CompressionStats {
	stats := CompressionStats{}
	for _, item := range items {
		if lo.FromPtr(item.Type) != constant.CompressionJSONKeyFuncCallOutput || item.Output == nil {
			continue
		}
		output := item.Output.Text
		if len(output) < minToolOutputBytes {
			stats.addItem(ItemCompressionResult{
				ToolCallID:  lo.FromPtr(item.CallID),
				Input:       output,
				Output:      output,
				Strategy:    constant.CompressionStrategySkippedTooSmall,
				Applied:     false,
				BytesBefore: len(output),
				BytesAfter:  len(output),
			})
			continue
		}
		result := dispatcher.Compress(output)
		result.ToolCallID = lo.FromPtr(item.CallID)
		result.Input = output
		stats.addItem(result)
		if result.Applied {
			item.Output.Text = result.Output
			item.Output.FunctionOutput = nil
		}
	}
	return stats
}
```

- [ ] **Step 2: 重写 `locator_responses_test.go`**

将整个文件替换为适配 DTO 输入的测试：

```go
package compression

import (
	"testing"

	comp "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

func TestCompressOpenAIResponses_CompressesFunctionCallOutput(t *testing.T) {
	t.Parallel()
	callID := "call_001"
	itemType := constant.CompressionJSONKeyFuncCallOutput
	largeContent := makeLargeJSONArray(20)
	items := []*dto.ResponseInputItem{
		{
			Type:   &itemType,
			CallID: &callID,
			Output: &dto.ResponseInputItemOutput{Text: largeContent},
		},
	}
	dispatcher := comp.NewDispatcher()

	stats := comp.CompressOpenAIResponses(items, dispatcher, 100)

	if stats.ItemsCompressed == 0 {
		t.Fatal("expected at least 1 item compressed")
	}
	if items[0].Output.Text == largeContent {
		t.Error("expected output to be replaced with compressed content")
	}
	if len(stats.Items) == 0 || stats.Items[0].ToolCallID != callID {
		t.Error("expected ToolCallID to be set in result")
	}
}

func TestCompressOpenAIResponses_SkipsSmallOutput(t *testing.T) {
	t.Parallel()
	callID := "call_002"
	itemType := constant.CompressionJSONKeyFuncCallOutput
	smallContent := "small"
	items := []*dto.ResponseInputItem{
		{
			Type:   &itemType,
			CallID: &callID,
			Output: &dto.ResponseInputItemOutput{Text: smallContent},
		},
	}
	dispatcher := comp.NewDispatcher()

	stats := comp.CompressOpenAIResponses(items, dispatcher, 100)

	if stats.ItemsCompressed != 0 {
		t.Error("expected 0 items compressed for small content")
	}
	if stats.ItemsSkipped != 1 {
		t.Error("expected 1 item skipped")
	}
	if items[0].Output.Text != smallContent {
		t.Error("expected small output to remain unchanged")
	}
}

func TestCompressOpenAIResponses_SkipsNonFunctionCallOutput(t *testing.T) {
	t.Parallel()
	msgType := "message"
	items := []*dto.ResponseInputItem{
		{
			Type: &msgType,
		},
	}
	dispatcher := comp.NewDispatcher()

	stats := comp.CompressOpenAIResponses(items, dispatcher, 0)

	if stats.ItemsCompressed != 0 || stats.ItemsSkipped != 0 {
		t.Error("expected no items processed for non-function_call_output items")
	}
}

func TestCompressOpenAIResponses_NilOutputSkipped(t *testing.T) {
	t.Parallel()
	itemType := constant.CompressionJSONKeyFuncCallOutput
	items := []*dto.ResponseInputItem{
		{
			Type:   &itemType,
			Output: nil,
		},
	}
	dispatcher := comp.NewDispatcher()

	stats := comp.CompressOpenAIResponses(items, dispatcher, 0)

	if stats.ItemsCompressed != 0 || stats.ItemsSkipped != 0 {
		t.Error("expected no items processed for nil output")
	}
}
```

- [ ] **Step 3: 运行测试确认通过**

Run: `go test -v -count=1 -run TestCompressOpenAIResponses ./test/unit/compression/`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/application/llmproxy/compression/locator_responses.go test/unit/compression/locator_responses_test.go
git commit -m "refactor: rewrite OpenAI Responses locator to compress DTO in-place"
```

---

### Task 7: 删除旧 `locator.go` + 更新 dispatcher/dispatcher_test

**Files:**
- Modify: `internal/application/llmproxy/compression/locator.go`
- Modify: `test/unit/compression/dispatcher_test.go`

- [ ] **Step 1: 清空 `locator.go` 中的旧接口和函数**

将 `locator.go` 整个文件替换为（保留 package 声明，删除所有旧代码）：

```go
package compression
```

- [ ] **Step 2: 修复 `dispatcher_test.go` 中的编译错误**

`dispatcher_test.go` 中的 `TestDispatcherCompressJSONArray` 和 `TestDispatcherCompressSearchResults` 使用了旧 `CompressBody` 函数。检查是否有引用旧 `CompressBody` 的测试，如有则改为直接调用 `Dispatcher.Compress`。

Run: `go build ./...` 检查编译错误，逐一修复。

- [ ] **Step 3: 验证所有压缩单测通过**

Run: `go test -v -count=1 ./test/unit/compression/`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/application/llmproxy/compression/locator.go test/unit/compression/dispatcher_test.go
git commit -m "refactor: remove legacy ToolOutputLocator interface and CompressBody function"
```

---

### Task 8: 移除 `CompressionMinBodyBytes` 配置

**Files:**
- Modify: `internal/config/config.go:202-203, 332`
- Modify: `env/api.env.template:77`

- [ ] **Step 1: 移除 config.go 中的变量声明**

删除 `internal/config/config.go` 中的：

```go
	// CompressionMinBodyBytes int body 小于此值跳过压缩
	CompressionMinBodyBytes int
```

- [ ] **Step 2: 移除 config.go 中的初始化**

删除 `internal/config/config.go` `initEnvironment()` 中的：

```go
	CompressionMinBodyBytes = config.GetInt("compression.min.body.bytes")
```

同时删除对应的 `config.SetDefault("compression.min.body.bytes", ...)` 行（如果存在）。

- [ ] **Step 3: 移除 env 模板中的环境变量**

删除 `env/api.env.template` 中的 `COMPRESSION_MIN_BODY_BYTES=2048` 行。

- [ ] **Step 4: 验证编译通过**

Run: `go build ./...`
Expected: PASS（如果有引用 `config.CompressionMinBodyBytes` 的地方，会在编译时报错，此时一并修复）

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go env/api.env.template
git commit -m "refactor: remove CompressionMinBodyBytes config (no longer needed with DTO approach)"
```

---

### Task 9: 重构 OpenAI usecase — 压缩逻辑移入 forward 方法

**Files:**
- Modify: `internal/application/llmproxy/usecase/openai.go:153-167`
- Modify: `internal/application/llmproxy/usecase/openai_chat.go`
- Modify: `internal/application/llmproxy/usecase/openai_response.go`
- Modify: `internal/application/llmproxy/usecase/openai_store.go`

- [ ] **Step 1: 删除 `openai.go` 中的 `compressBodyIfNeeded` 方法**

删除 `internal/application/llmproxy/usecase/openai.go` 中的整个 `compressBodyIfNeeded` 方法（第 153-167 行）。

- [ ] **Step 2: 修改 `openai_chat.go` — `forwardChatNative`**

将 `forwardChatNative` 中的：

```go
	body := proxyutil.MarshalOpenAIChatCompletionBodyForModel(req.Body, upstream.Model)
	body, compStats := u.compressBodyIfNeeded(ctx, body, enum.ProtocolOpenAIChatCompletion)
```

改为：

```go
	compStats := u.compressChatMessagesIfNeeded(req.Body.Messages)
	body := proxyutil.MarshalOpenAIChatCompletionBodyForModel(req.Body, upstream.Model)
```

- [ ] **Step 3: 在 `openai.go` 中添加 `compressChatMessagesIfNeeded` 方法**

在 `openai.go` 中（原 `compressBodyIfNeeded` 的位置）添加：

```go
func (u *openAIUseCase) compressChatMessagesIfNeeded(messages []*dto.OpenAIChatCompletionMessageParam) *compression.CompressionStats {
	if !config.CompressionEnabled || u.dispatcher == nil {
		return nil
	}
	stats := compression.CompressOpenAIChat(messages, u.dispatcher, config.CompressionMinToolOutputBytes)
	if stats.ItemsCompressed > 0 {
		return &stats
	}
	return nil
}
```

- [ ] **Step 4: 在 `openai.go` 中添加 `compressAnthropicMessagesIfNeeded` 方法**

```go
func (u *openAIUseCase) compressAnthropicMessagesIfNeeded(messages []*dto.AnthropicMessageParam) *compression.CompressionStats {
	if !config.CompressionEnabled || u.dispatcher == nil {
		return nil
	}
	stats := compression.CompressAnthropicMessages(messages, u.dispatcher, config.CompressionMinToolOutputBytes)
	if stats.ItemsCompressed > 0 {
		return &stats
	}
	return nil
}
```

- [ ] **Step 5: 在 `openai.go` 中添加 `compressResponseItemsIfNeeded` 方法**

```go
func (u *openAIUseCase) compressResponseItemsIfNeeded(items []*dto.ResponseInputItem) *compression.CompressionStats {
	if !config.CompressionEnabled || u.dispatcher == nil {
		return nil
	}
	stats := compression.CompressOpenAIResponses(items, u.dispatcher, config.CompressionMinToolOutputBytes)
	if stats.ItemsCompressed > 0 {
		return &stats
	}
	return nil
}
```

- [ ] **Step 6: 修改 `openai_chat.go` — `forwardChatViaAnthropic`**

在 `forwardChatViaAnthropic` 中，在 `body := proxyutil.MarshalAnthropicMessageBodyForModel(anthropicReq, upstream.Model)` 之前添加压缩调用：

```go
	compStats := u.compressAnthropicMessagesIfNeeded(anthropicReq.Messages)
	body := proxyutil.MarshalAnthropicMessageBodyForModel(anthropicReq, upstream.Model)
```

然后将 `compStats` 传递给 `forwardChatViaAnthropicStream` 和 `forwardChatViaAnthropicUnary`（需修改这两个方法的签名加 `compStats *compression.CompressionStats` 参数，并在其中传递给 store 方法和 audit task）。

注意：`forwardChatViaAnthropicStream` 和 `forwardChatViaAnthropicUnary` 当前不接收 `compStats`。需要在调用 `u.storeOpenAIChatFromCompletion` 时传入 `compStats`，并在 audit task 部分加入 `compStats` 处理。

- [ ] **Step 7: 修改 `openai_response.go` — 三个 forward 路径**

对 `forwardResponseNative`：
```go
	compStats := u.compressResponseItemsIfNeeded(req.Body.Input.Items)
	body := proxyutil.MarshalOpenAIResponseBodyForModel(req.Body, upstream.Model)
```

对 `forwardResponseViaChat`：
```go
	compStats := u.compressChatMessagesIfNeeded(chatReq.Messages)
	body := proxyutil.MarshalOpenAIChatCompletionBodyForModel(chatReq, upstream.Model)
```

对 `forwardResponseViaAnthropic`：
```go
	compStats := u.compressAnthropicMessagesIfNeeded(anthropicReq.Messages)
	body := proxyutil.MarshalAnthropicMessageBodyForModel(anthropicReq, upstream.Model)
```

将 `compStats` 传递到各 stream/unary 子方法，用于 audit task 和 store 方法。

- [ ] **Step 8: 修改 `openai_store.go` — store 方法加 `compResults` 参数**

将 `storeOpenAIChatFromCompletion` 签名改为：
```go
func (u *openAIUseCase) storeOpenAIChatFromCompletion(ctx context.Context, req *dto.OpenAIChatCompletionRequest, completion *dto.OpenAIChatCompletion, proxyErr error, upstreamModel string, compResults []compression.ItemCompressionResult)
```

将 `storeOpenAIChatMessages` 签名改为：
```go
func (u *openAIUseCase) storeOpenAIChatMessages(ctx context.Context, req *dto.OpenAIChatCompletionRequest, assistantMsg *dto.OpenAIChatCompletionMessageParam, upstreamModel string, usage *dto.OpenAICompletionUsage, compResults []compression.ItemCompressionResult)
```

在 `storeOpenAIChatMessages` 中，在 `SubmitMessageStoreTask` 之前添加：
```go
	compression.ApplyResultsToMessages(unifiedMessages, compResults)
```

同理修改 `storeResponseFromRsp`，加 `compResults` 参数并调用 `ApplyResultsToMessages`。

- [ ] **Step 9: 更新所有调用 store 方法的位置**

在 `openai_chat.go` 和 `openai_response.go` 中，所有调用 `storeOpenAIChatFromCompletion` 和 `storeResponseFromRsp` 的地方，传入 `compStats` 的 `Items` 字段（当 `compStats` 非 nil 时）：

```go
	var compResults []compression.ItemCompressionResult
	if compStats != nil {
		compResults = compStats.Items
	}
	u.storeOpenAIChatFromCompletion(ctx, req, completion, err, upstream.Model, compResults)
```

- [ ] **Step 10: 验证编译通过**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 11: 验证现有测试通过**

Run: `go test -count=1 ./test/unit/compression/ ./test/unit/model_call_audit/`
Expected: PASS

- [ ] **Step 12: Commit**

```bash
git add internal/application/llmproxy/usecase/openai.go internal/application/llmproxy/usecase/openai_chat.go internal/application/llmproxy/usecase/openai_response.go internal/application/llmproxy/usecase/openai_store.go
git commit -m "refactor: move OpenAI compression to DTO-level, pass compResults to store methods"
```

---

### Task 10: 重构 Anthropic usecase — 压缩逻辑移入 forward 方法

**Files:**
- Modify: `internal/application/llmproxy/usecase/anthropic.go:129-143`
- Modify: `internal/application/llmproxy/usecase/anthropic_message.go`
- Modify: `internal/application/llmproxy/usecase/anthropic_store.go`

- [ ] **Step 1: 删除 `anthropic.go` 中的 `compressBodyIfNeeded` 方法**

删除 `internal/application/llmproxy/usecase/anthropic.go` 中的整个 `compressBodyIfNeeded` 方法（第 129-143 行）。

- [ ] **Step 2: 在 `anthropic.go` 中添加 `compressMessagesIfNeeded` 和 `compressChatMessagesIfNeeded` 方法**

```go
func (u *anthropicUseCase) compressMessagesIfNeeded(messages []*dto.AnthropicMessageParam) *compression.CompressionStats {
	if !config.CompressionEnabled || u.dispatcher == nil {
		return nil
	}
	stats := compression.CompressAnthropicMessages(messages, u.dispatcher, config.CompressionMinToolOutputBytes)
	if stats.ItemsCompressed > 0 {
		return &stats
	}
	return nil
}

func (u *anthropicUseCase) compressChatMessagesIfNeeded(messages []*dto.OpenAIChatCompletionMessageParam) *compression.CompressionStats {
	if !config.CompressionEnabled || u.dispatcher == nil {
		return nil
	}
	stats := compression.CompressOpenAIChat(messages, u.dispatcher, config.CompressionMinToolOutputBytes)
	if stats.ItemsCompressed > 0 {
		return &stats
	}
	return nil
}
```

- [ ] **Step 3: 修改 `anthropic_message.go` — `forwardMessageNative`**

将：
```go
	body := proxyutil.MarshalAnthropicMessageBodyForModel(req.Body, upstream.Model)
	body, compStats := u.compressBodyIfNeeded(ctx, body, enum.ProtocolAnthropicMessage)
```
改为：
```go
	compStats := u.compressMessagesIfNeeded(req.Body.Messages)
	body := proxyutil.MarshalAnthropicMessageBodyForModel(req.Body, upstream.Model)
```

- [ ] **Step 4: 修改 `anthropic_message.go` — `forwardMessageViaChat`**

在 `body := proxyutil.MarshalOpenAIChatCompletionBodyForModel(chatReq, upstream.Model)` 之前添加：
```go
	compStats := u.compressChatMessagesIfNeeded(chatReq.Messages)
	body := proxyutil.MarshalOpenAIChatCompletionBodyForModel(chatReq, upstream.Model)
```

将 `compStats` 传递到 `forwardMessageViaChatStream` 和 `forwardMessageViaChatUnary`（需修改签名加 `compStats` 参数）。

- [ ] **Step 5: 修改 `anthropic_store.go` — store 方法加 `compResults` 参数**

将 `storeAnthropicFromMsg` 签名改为：
```go
func (u *anthropicUseCase) storeAnthropicFromMsg(ctx context.Context, req *dto.AnthropicCreateMessageRequest, msg *dto.AnthropicMessage, proxyErr error, upstreamModel string, compResults []compression.ItemCompressionResult)
```

将 `storeAnthropicMessages` 签名改为：
```go
func (u *anthropicUseCase) storeAnthropicMessages(ctx context.Context, req *dto.AnthropicCreateMessageRequest, assistantMsg *dto.AnthropicMessage, upstreamModel string, compResults []compression.ItemCompressionResult)
```

在 `storeAnthropicMessages` 中，在 `SubmitMessageStoreTask` 之前添加：
```go
	compression.ApplyResultsToMessages(unifiedMessages, compResults)
```

- [ ] **Step 6: 更新所有调用 store 方法的位置**

在 `anthropic_message.go` 中，所有调用 `storeAnthropicFromMsg` 的地方，传入 `compStats.Items`：

```go
	var compResults []compression.ItemCompressionResult
	if compStats != nil {
		compResults = compStats.Items
	}
	u.storeAnthropicFromMsg(ctx, req, anthropicMsg, err, upstream.Model, compResults)
```

- [ ] **Step 7: 验证编译通过**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 8: 运行 lint**

Run: `make lint`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add internal/application/llmproxy/usecase/anthropic.go internal/application/llmproxy/usecase/anthropic_message.go internal/application/llmproxy/usecase/anthropic_store.go
git commit -m "refactor: move Anthropic compression to DTO-level, pass compResults to store methods"
```

---

### Task 11: 安装 Web diff 依赖

**Files:**
- Modify: `web/package.json`

- [ ] **Step 1: 安装 diff 包**

Run:
```bash
cd web && npm install diff && npm install -D @types/diff
```

- [ ] **Step 2: 验证安装成功**

Run: `cd web && npm run build`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add web/package.json web/package-lock.json
git commit -m "deps: add diff and @types/diff for compression diff view"
```

---

### Task 12: 更新 Web `types.ts` 和 `content-extract.ts`

**Files:**
- Modify: `web/src/lib/types.ts:111-119`
- Modify: `web/src/components/chat/content-extract.ts`

- [ ] **Step 1: 在 `types.ts` 的 `UnifiedMessage` 接口添加字段**

在 `web/src/lib/types.ts` 的 `UnifiedMessage` 接口中，在 `tool_calls` 之后添加：

```typescript
  raw_content?: string;
  compression_strategy?: string;
```

- [ ] **Step 2: 在 `content-extract.ts` 添加 `ToolResultInfo` 接口**

在 `web/src/components/chat/content-extract.ts` 的 import 之后添加：

```typescript
export interface ToolResultInfo {
  text: string;
  rawContent?: string;
  compressionStrategy?: string;
}
```

- [ ] **Step 3: 修改 `buildToolResultsByID` 返回类型**

将 `buildToolResultsByID` 改为：

```typescript
export function buildToolResultsByID(
  messages: MessageItem[],
): Record<string, ToolResultInfo> {
  const map: Record<string, ToolResultInfo> = {};
  for (const m of messages) {
    const id = m.message.tool_call_id;
    if (!id) continue;
    if (m.message.role !== "tool" && m.message.role !== "user") continue;
    const { text } = extractContent(m.message.content);
    map[id] = {
      text,
      rawContent: m.message.raw_content,
      compressionStrategy: m.message.compression_strategy,
    };
  }
  return map;
}
```

- [ ] **Step 4: 修改 `lookupToolResult` 返回类型**

将 `lookupToolResult` 改为：

```typescript
export function lookupToolResult(
  map: Record<string, ToolResultInfo>,
  id: string,
): ToolResultInfo | undefined {
  if (id in map) return map[id];
  const normalized = normalizeToolCallID(id);
  for (const key of Object.keys(map)) {
    if (normalizeToolCallID(key) === normalized) return map[key];
  }
  return undefined;
}
```

- [ ] **Step 5: 验证 lint 通过**

Run: `cd web && npm run lint`
Expected: PASS（可能有 TS 错误因为下游组件类型还没改）

- [ ] **Step 6: Commit**

```bash
git add web/src/lib/types.ts web/src/components/chat/content-extract.ts
git commit -m "feat: add compression fields to UnifiedMessage and ToolResultInfo type"
```

---

### Task 13: 更新 Web 组件 prop 类型

**Files:**
- Modify: `web/src/components/chat/chat-message.tsx:26`
- Modify: `web/src/components/chat/assistant-message.tsx:27`
- Modify: `web/src/app/share/page.tsx`

- [ ] **Step 1: 更新 `chat-message.tsx` prop 类型**

将 `ChatMessageProps` 中的 `toolResultsByID` 类型从 `Record<string, string>` 改为 `Record<string, ToolResultInfo>`，并在 import 中加入 `ToolResultInfo`：

```typescript
import { type ToolResultInfo, buildToolResultsByID } from "./content-extract";
```

```typescript
  toolResultsByID: Record<string, ToolResultInfo>;
```

- [ ] **Step 2: 更新 `assistant-message.tsx` prop 类型**

在 import 中加入 `ToolResultInfo`：
```typescript
import { lookupToolResult, type ToolResultInfo } from "./content-extract";
```

将 `AssistantMessageProps` 中的 `toolResultsByID` 类型改为 `Record<string, ToolResultInfo>`。

- [ ] **Step 3: 更新 `share/page.tsx` prop 类型**

在 `share/page.tsx` 中，更新 `toolResultsByID` 的类型声明为 `Record<string, ToolResultInfo>`，并在 import 中加入 `ToolResultInfo`。

- [ ] **Step 4: 验证 lint 通过**

Run: `cd web && npm run lint`
Expected: PASS（`tool-call-card.tsx` 的 `result` prop 类型可能还需改，在下一个 task 处理）

- [ ] **Step 5: Commit**

```bash
git add web/src/components/chat/chat-message.tsx web/src/components/chat/assistant-message.tsx web/src/app/share/page.tsx
git commit -m "refactor: update component prop types to use ToolResultInfo"
```

---

### Task 14: 创建 `CompressionDiff` 组件

**Files:**
- Create: `web/src/components/chat/compression-diff.tsx`

- [ ] **Step 1: 创建 diff 组件**

创建 `web/src/components/chat/compression-diff.tsx`：

```tsx
"use client";

import { useState, useMemo } from "react";
import { diffLines } from "diff";
import { Columns2, Rows2 } from "lucide-react";
import { cn } from "@/lib/utils";

interface CompressionDiffProps {
  before: string;
  after: string;
  strategy: string;
}

export function CompressionDiff({ before, after, strategy }: CompressionDiffProps) {
  const [mode, setMode] = useState<"split" | "inline">("split");

  const changes = useMemo(() => diffLines(before, after), [before, after]);

  return (
    <div className="mt-2 overflow-hidden rounded-md border border-border/60">
      <div className="flex items-center justify-between border-b border-border/50 bg-muted/20 px-3 py-1.5">
        <span className="font-mono text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
          {strategy}
        </span>
        <button
          type="button"
          onClick={() => setMode((m) => (m === "split" ? "inline" : "split"))}
          className="flex items-center gap-1 rounded px-1.5 py-0.5 text-[11px] text-muted-foreground transition-colors hover:bg-muted/40 hover:text-foreground"
        >
          {mode === "split" ? (
            <>
              <Columns2 className="size-3" />
              Split
            </>
          ) : (
            <>
              <Rows2 className="size-3" />
              Inline
            </>
          )}
        </button>
      </div>

      {mode === "inline" ? (
        <InlineDiffView changes={changes} />
      ) : (
        <SplitDiffView changes={changes} />
      )}
    </div>
  );
}

type Change = { value: string; added?: boolean; removed?: boolean };

function InlineDiffView({ changes }: { changes: Change[] }) {
  const lines: { text: string; type: "added" | "removed" | "unchanged" }[] = [];
  for (const change of changes) {
    const type = change.added ? "added" : change.removed ? "removed" : "unchanged";
    const splitLines = change.value.split("\n");
    if (splitLines.length > 0 && splitLines[splitLines.length - 1] === "") {
      splitLines.pop();
    }
    for (const line of splitLines) {
      lines.push({ text: line, type });
    }
  }

  return (
    <div className="max-h-[400px] overflow-auto font-mono text-[11px] leading-relaxed">
      {lines.map((line, i) => (
        <div
          key={i}
          className={cn(
            "px-3 py-0.5",
            line.type === "added" && "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400",
            line.type === "removed" && "bg-red-500/10 text-red-700 dark:text-red-400",
            line.type === "unchanged" && "text-foreground/70",
          )}
        >
          <span className="select-none mr-2 text-muted-foreground/40">
            {line.type === "added" ? "+" : line.type === "removed" ? "-" : " "}
          </span>
          {line.text}
        </div>
      ))}
    </div>
  );
}

function SplitDiffView({ changes }: { changes: Change[] }) {
  const leftLines: { text: string; type: string }[] = [];
  const rightLines: { text: string; type: string }[] = [];

  for (const change of changes) {
    const splitLines = change.value.split("\n");
    if (splitLines.length > 0 && splitLines[splitLines.length - 1] === "") {
      splitLines.pop();
    }
    if (change.added) {
      for (const line of splitLines) {
        rightLines.push({ text: line, type: "added" });
      }
    } else if (change.removed) {
      for (const line of splitLines) {
        leftLines.push({ text: line, type: "removed" });
      }
    } else {
      const maxLen = Math.max(splitLines.length, 0);
      for (const line of splitLines) {
        leftLines.push({ text: line, type: "unchanged" });
        rightLines.push({ text: line, type: "unchanged" });
      }
      void maxLen;
    }
  }

  const maxRows = Math.max(leftLines.length, rightLines.length);

  return (
    <div className="flex max-h-[400px] overflow-auto font-mono text-[11px] leading-relaxed">
      <div className="flex-1 border-r border-border/40">
        {Array.from({ length: maxRows }).map((_, i) => (
          <div
            key={i}
            className={cn(
              "px-3 py-0.5 whitespace-pre-wrap break-all",
              leftLines[i]?.type === "removed" && "bg-red-500/10 text-red-700 dark:text-red-400",
              leftLines[i]?.type === "unchanged" && "text-foreground/70",
              !leftLines[i] && "bg-muted/10",
            )}
          >
            {leftLines[i]?.text ?? ""}
          </div>
        ))}
      </div>
      <div className="flex-1">
        {Array.from({ length: maxRows }).map((_, i) => (
          <div
            key={i}
            className={cn(
              "px-3 py-0.5 whitespace-pre-wrap break-all",
              rightLines[i]?.type === "added" && "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400",
              rightLines[i]?.type === "unchanged" && "text-foreground/70",
              !rightLines[i] && "bg-muted/10",
            )}
          >
            {rightLines[i]?.text ?? ""}
          </div>
        ))}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: 验证 lint 通过**

Run: `cd web && npm run lint`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add web/src/components/chat/compression-diff.tsx
git commit -m "feat: add CompressionDiff component for before/after diff view"
```

---

### Task 15: 更新 `ToolCallCard` 展示压缩信息

**Files:**
- Modify: `web/src/components/chat/tool-call-card.tsx`

- [ ] **Step 1: 修改 `ToolCallCard` 的 props 和 import**

在文件顶部添加 import：

```typescript
import { CompressionDiff } from "./compression-diff";
import type { ToolResultInfo } from "./content-extract";
```

将 `ToolCallCardProps` 改为：

```typescript
interface ToolCallCardProps {
  call: UnifiedToolCall;
  result?: ToolResultInfo;
}
```

- [ ] **Step 2: 修改组件逻辑，适配 `ToolResultInfo` 并添加压缩展示**

将 `ToolCallCard` 函数体改为：

```typescript
export function ToolCallCard({ call, result }: ToolCallCardProps) {
  const [open, setOpen] = useState(false);
  const [showDiff, setShowDiff] = useState(false);
  const args = prettyJSON(call.arguments);
  const out = result ? prettyJSON(result.text) : undefined;
  const preview = previewFirstArg(call.arguments);
  const hasCompression = !!result?.compressionStrategy && !!result?.rawContent;

  return (
    <div
      className={cn(
        "mt-3 overflow-hidden rounded-lg border border-border bg-card",
      )}
    >
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center gap-2.5 px-3 py-2 text-left transition-colors hover:bg-muted/30"
      >
        <div className="flex size-6 shrink-0 items-center justify-center rounded-md bg-primary/12 text-primary">
          <Wrench className="size-3.5" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className="font-mono text-[13px] font-medium text-foreground">
              {call.name || "tool"}
            </span>
            {!open && preview && (
              <span className="ml-1 flex-1 truncate font-mono text-[11px] text-muted-foreground">
                {preview}
              </span>
            )}
            {hasCompression && (
              <span className="shrink-0 rounded bg-primary/10 px-1.5 py-0.5 font-mono text-[10px] text-primary">
                cp
              </span>
            )}
          </div>
        </div>
        {open ? (
          <ChevronDown className="size-4 shrink-0 text-muted-foreground" />
        ) : (
          <ChevronRight className="size-4 shrink-0 text-muted-foreground" />
        )}
      </button>
      {open && (
        <div className="border-t border-border bg-background/40 min-w-0">
          {call.id && (
            <div className="border-b border-border/50 px-3 py-1.5">
              <span className="font-mono text-[10px] text-muted-foreground/60">
                {call.id}
              </span>
            </div>
          )}
          <div className="px-3 py-2.5">
            <p className="mb-1.5 font-mono text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
              Input
            </p>
            <pre className="overflow-x-auto rounded-md bg-muted/40 px-3 py-2.5 font-mono text-[12px] leading-relaxed text-foreground/90 max-w-full">
              {args || "{}"}
            </pre>
          </div>
          {out !== undefined && (
            <div className="border-t border-border px-3 py-2.5">
              <p className="mb-1.5 font-mono text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
                Output
              </p>
              <pre className="overflow-x-auto rounded-md bg-muted/40 px-3 py-2.5 font-mono text-[12px] leading-relaxed text-foreground/90 max-w-full">
                {out}
              </pre>
              {hasCompression && (
                <div className="mt-2">
                  {showDiff && result?.rawContent ? (
                    <CompressionDiff
                      before={result.rawContent}
                      after={result.text}
                      strategy={result.compressionStrategy!}
                    />
                  ) : (
                    <button
                      type="button"
                      onClick={() => setShowDiff(true)}
                      className="text-[11px] text-primary hover:underline"
                    >
                      查看原始内容 ({result.compressionStrategy})
                    </button>
                  )}
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 3: 验证 lint 通过**

Run: `cd web && npm run lint`
Expected: PASS

- [ ] **Step 4: 验证 build 通过**

Run: `cd web && npm run build`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/components/chat/tool-call-card.tsx
git commit -m "feat: show compression info and diff view in ToolCallCard"
```

---

### Task 16: 全量验证

**Files:**
- None (verification only)

- [ ] **Step 1: API 侧全量编译 + lint**

Run: `make build && make lint`
Expected: PASS

- [ ] **Step 2: API 侧全量单测**

Run: `go test -count=1 ./test/unit/compression/ ./test/unit/model_call_audit/`
Expected: PASS

- [ ] **Step 3: Web 侧 lint + build**

Run: `cd web && npm run lint && npm run build`
Expected: PASS

- [ ] **Step 4: Commit（如有 lint 自动修复）**

```bash
git add -A
git commit -m "chore: final verification pass" || echo "nothing to commit"
```
