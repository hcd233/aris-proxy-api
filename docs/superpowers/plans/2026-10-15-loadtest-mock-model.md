# 三接口 Mock 模型与生产压测 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 chat completion / response / message 三接口提供 `mock-llm` 随机字符流模型（TTFT/TPS 随机），用 wrk 压测生产并按 `api_key_id` 精确清理压测数据。

**Architecture:** 在 `OpenAIProxyPort` / `AnthropicProxyPort` 外包一层装饰器，命中 `mock-llm` 标记时本地生成随机流、不发 HTTP——三个 usecase 零改动，7 条跨协议路径自动全覆盖，鉴权/限流/触发词/审计/会话落库全部真实执行。随机流生成器是纯函数（参数 + 三协议响应构造），装饰器只做分发。

**Tech Stack:** Go 1.25.1、`math/rand/v2`、sonic、wrk + Lua、PostgreSQL、GORM

## Global Constraints

- **mock 标记值**：`mock-llm`，常量 `transport.MockUpstreamModel`，与 Model 表 `upstream_model` 列比对
- **随机长度单位 = 字符**，范围 `[1, 16384]`；请求带 `max_tokens`（token 数）时字符上限 = `max_tokens × 4`，再与 16384 取小
- **TTFT ∈ [0, 1s]**、**TPS ∈ [20, 200] tok/s**，均为**请求级随机**（每次请求抽一次，全程匀速）
- **字符集**：纯可打印 ASCII `0x20–0x7E` + `\n`，不含中文/多字节
- **usage 口径**：`1 token ≈ 4 字符`；`output_tokens = max(1, len(chars)/4)`
- **配置开关**：`mock.upstream.enabled`，默认 `false`；仅 `true` 时装饰器生效
- **DTO 禁 `any`/`interface{}`**：用具体结构体或 `sonic.NoCopyRawMessage`
- **业务错误走 `internal/common/ierr`**，不裸用 `errors.New`
- **Go 惯用法**：`math/rand/v2`（`rand.IntN`）、内置 `min`/`max`、`math/rand/v2` 的 `*rand.Rand` 并发安全
- **测试契约**：遵循 `docs/agents/go-backend.md`；e2e 默认离线 skip，需 `BASE_URL` + `API_KEY`
- **提交信息**：中文，格式 `<type>(<scope>): <描述>`

---

### Task 1: mock 标记常量与配置开关

**Files:**
- Modify: `internal/config/config.go`（声明区 + `SetDefault` + 赋值区）
- Create: `internal/infrastructure/transport/mock_gen.go`（仅放常量，后续 task 扩充）
- Test: `test/unit/config/mock_upstream_config_test.go`

**Interfaces:**
- Produces: `transport.MockUpstreamModel`（`const = "mock-llm"`）、`config.MockUpstreamEnabled`（`bool`）

- [ ] **Step 1: 写失败测试**

`test/unit/config/mock_upstream_config_test.go`:

```go
package config

import "testing"

// TestMockUpstreamDisabledByDefault 验证 mock 开关默认关闭，
// 保证生产上不会误配 mock 模型后拿到假响应。
func TestMockUpstreamDisabledByDefault(t *testing.T) {
	t.Parallel()
	if MockUpstreamEnabled {
		t.Fatal("MockUpstreamEnabled must default to false")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
go test ./test/unit/config/ -run TestMockUpstreamDisabledByDefault -v
```
Expected: FAIL — `undefined: MockUpstreamEnabled`

- [ ] **Step 3: 加配置项**

`internal/config/config.go` 声明区（`PostgresSSLMode` 附近）加：

```go
	// MockUpstreamEnabled bool 是否启用 mock 上游（压测用）
	//	update 2026-10-15 10:00:00
	MockUpstreamEnabled bool
```

`SetDefault` 区（`cron.session.terminal_cleanup.enabled` 附近）加：

```go
	config.SetDefault("mock.upstream.enabled", false)
```

赋值区（`LogLevel = config.GetString("log.level")` 附近）加：

```go
	MockUpstreamEnabled = config.GetBool("mock.upstream.enabled")
```

- [ ] **Step 4: 加标记常量**

`internal/infrastructure/transport/mock_gen.go`:

```go
package transport

// MockUpstreamModel mock 上游标记值：Model 表 upstream_model 为该值时
// 走本地随机流生成，不向上游发起 HTTP 请求。
const MockUpstreamModel = "mock-llm"
```

- [ ] **Step 5: 跑测试确认通过**

```bash
go test ./test/unit/config/ -run TestMockUpstreamDisabledByDefault -v && go build ./...
```
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go internal/infrastructure/transport/mock_gen.go test/unit/config/mock_upstream_config_test.go
git commit -m "feat(mock): 增加 mock 上游配置开关与 mock-llm 标记常量"
```

---

### Task 2: 随机流参数与字符生成器

**Files:**
- Modify: `internal/infrastructure/transport/mock_gen.go`
- Test: `test/unit/transport/mock_gen_test.go`

**Interfaces:**
- Consumes: `transport.MockUpstreamModel`（Task 1）
- Produces: `mockSpec{chars []byte; ttft time.Duration; interval time.Duration}`、`newMockSpec(rng *rand.Rand, maxTokens int) mockSpec`、`(mockSpec).tokens() int`、`(mockSpec).chunks() []string`、`randomChars(rng *rand.Rand, n int) []byte`

- [ ] **Step 1: 写失败测试**

`test/unit/transport/mock_gen_test.go`:

```go
package transport

import (
	"math/rand/v2"
	"testing"
	"time"
)

func fixedRNG() *rand.Rand { return rand.New(rand.NewPCG(1, 2)) }

func TestNewMockSpec_Ranges(t *testing.T) {
	t.Parallel()
	rng := fixedRNG()
	for range 200 {
		s := newMockSpec(rng, 0)
		if n := len(s.chars); n < 1 || n > mockMaxChars {
			t.Fatalf("chars length %d out of [1,%d]", n, mockMaxChars)
		}
		if s.ttft < 0 || s.ttft > mockMaxTTFT {
			t.Fatalf("ttft %v out of [0,%v]", s.ttft, mockMaxTTFT)
		}
		if s.interval <= 0 {
			t.Fatalf("interval must be > 0, got %v", s.interval)
		}
		tps := time.Second / s.interval
		if tps < mockMinTPS || tps > mockMaxTPS {
			t.Fatalf("tps %d out of [%d,%d]", tps, mockMinTPS, mockMaxTPS)
		}
	}
}

func TestNewMockSpec_MaxTokensCapsLength(t *testing.T) {
	t.Parallel()
	rng := fixedRNG()
	for range 200 {
		s := newMockSpec(rng, 10) // 10 token → 最多 40 字符
		if len(s.chars) > 10*mockCharsPerTok {
			t.Fatalf("chars %d exceeds max_tokens*%d", len(s.chars), mockCharsPerTok)
		}
	}
}

func TestRandomChars_OnlyPrintableASCII(t *testing.T) {
	t.Parallel()
	rng := fixedRNG()
	chars := randomChars(rng, 500)
	for i, c := range chars {
		if c < 0x20 || c > 0x7E {
			t.Fatalf("char at %d = %q outside printable ASCII", i, c)
		}
		if c != '\n' && (c < 0x21 || c > 0x7E) {
			t.Fatalf("char at %d = %q outside allowed set", i, c)
		}
	}
}

func TestMockSpec_ChunksAndTokens(t *testing.T) {
	t.Parallel()
	s := mockSpec{chars: []byte("abcdefghijklmno")} // 15 字符
	chunks := s.chunks()
	if len(chunks) != 4 { // 4+4+4+3
		t.Fatalf("want 4 chunks, got %d", len(chunks))
	}
	if want := 3; s.tokens() != want { // 15/4 = 3
		t.Fatalf("want tokens=%d, got %d", want, s.tokens())
	}
	if empty := (mockSpec{chars: []byte("ab")}).tokens(); empty != 1 {
		t.Fatalf("short input must yield at least 1 token, got %d", empty)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
go test ./test/unit/transport/ -run "TestNewMockSpec|TestRandomChars|TestMockSpec" -v
```
Expected: FAIL — `undefined: newMockSpec`

- [ ] **Step 3: 实现生成器**

`internal/infrastructure/transport/mock_gen.go` 在常量后追加：

```go
import (
	"math/rand/v2"
	"time"
)

const (
	mockMaxChars    = 16384       // 随机内容长度上限（字符）
	mockCharsPerTok = 4           // 1 token ≈ 4 字符
	mockMinTPS      = 20          // 输出速率下限 tok/s
	mockMaxTPS      = 200         // 输出速率上限 tok/s
	mockMaxTTFT     = time.Second // 首 token 延迟上限
)

// mockSpec 描述一次 mock 响应的随机参数：内容、首 token 延迟、吐字节奏。
type mockSpec struct {
	chars    []byte        // 随机字符内容
	ttft     time.Duration // 首 token 延迟 [0,1s]
	interval time.Duration // 相邻分片间隔 = 1/TPS
}

// newMockSpec 抽取一次 mock 响应的随机参数。
// maxTokens 是请求的 max_tokens（token 数）；<=0 表示不限制。
// 随机长度单位为字符，范围 [1, 16384]；带 max_tokens 时按 maxTokens*4 收紧上限。
func newMockSpec(rng *rand.Rand, maxTokens int) mockSpec {
	maxChars := mockMaxChars
	if maxTokens > 0 {
		maxChars = min(maxTokens*mockCharsPerTok, mockMaxChars)
	}
	n := rng.IntN(maxChars) + 1
	tps := mockMinTPS + rng.IntN(mockMaxTPS-mockMinTPS+1)
	return mockSpec{
		chars:    randomChars(rng, n),
		ttft:     time.Duration(rng.IntN(int(mockMaxTTFT) + 1)),
		interval: time.Second / time.Duration(tps),
	}
}

// randomChars 生成 n 个字符：可打印 ASCII 0x21~0x7E 加换行（不含裸空格与控制字符）。
func randomChars(rng *rand.Rand, n int) []byte {
	const alphabet = "\n!\"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~"
	out := make([]byte, n)
	for i := range out {
		out[i] = alphabet[rng.IntN(len(alphabet))]
	}
	return out
}

// tokens 估算输出 token 数：1 token ≈ 4 字符，至少 1。
func (s mockSpec) tokens() int {
	return max(1, len(s.chars)/mockCharsPerTok)
}

// chunks 把内容切成 4 字符一片（≈1 token）的序列，末片可短。
func (s mockSpec) chunks() []string {
	out := make([]string, 0, (len(s.chars)+mockCharsPerTok-1)/mockCharsPerTok)
	for i := 0; i < len(s.chars); i += mockCharsPerTok {
		out = append(out, string(s.chars[i:min(i+mockCharsPerTok, len(s.chars))]))
	}
	return out
}
```

- [ ] **Step 4: 跑测试确认通过**

```bash
go test ./test/unit/transport/ -run "TestNewMockSpec|TestRandomChars|TestMockSpec" -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/transport/mock_gen.go test/unit/transport/mock_gen_test.go
git commit -m "feat(mock): 随机流参数与字符生成器（长度/TTFT/TPS 均随机）"
```

---

### Task 3: OpenAI Chat Completions 响应构造

**Files:**
- Create: `internal/infrastructure/transport/mock_openai.go`
- Test: `test/unit/transport/mock_openai_test.go`

**Interfaces:**
- Consumes: `mockSpec`、`(mockSpec).chunks()`、`(mockSpec).tokens()`（Task 2）
- Produces: `buildChatStream(spec mockSpec, model, chunkID string) []*dto.OpenAIChatCompletionChunk`、`buildChatCompletion(spec mockSpec, model string) *dto.OpenAIChatCompletion`

- [ ] **Step 1: 写失败测试**

`test/unit/transport/mock_openai_test.go`:

```go
package transport

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

func TestBuildChatStream_Shape(t *testing.T) {
	t.Parallel()
	spec := mockSpec{chars: []byte("abcdefghijklmno")} // 15 字符 → 4 片
	chunks := buildChatStream(spec, "mock-chat", "chunk-1")

	if len(chunks) != 5 { // 4 内容片 + 1 结束片
		t.Fatalf("want 5 chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if c.Model != "mock-chat" || c.ID != "chunk-1" {
			t.Fatalf("chunk %d meta mismatch: %+v", i, c)
		}
		if len(c.Choices) != 1 {
			t.Fatalf("chunk %d want 1 choice, got %d", i, len(c.Choices))
		}
	}
	if chunks[0].Choices[0].Delta.Role != enum.RoleAssistant {
		t.Fatal("first chunk must carry assistant role")
	}
	last := chunks[len(chunks)-1]
	if last.Choices[0].FinishReason == nil || *last.Choices[0].FinishReason != enum.FinishReasonStop {
		t.Fatal("last chunk must carry stop finish_reason")
	}
	if last.Usage == nil {
		t.Fatal("last chunk must carry usage")
	}
	if last.Usage.CompletionTokens != 3 {
		t.Fatalf("completion_tokens want 3, got %d", last.Usage.CompletionTokens)
	}
	if last.Usage.TotalTokens != last.Usage.PromptTokens+last.Usage.CompletionTokens {
		t.Fatal("total_tokens must equal prompt+completion")
	}
}

func TestBuildChatCompletion_NonStream(t *testing.T) {
	t.Parallel()
	spec := mockSpec{chars: []byte("abcdefgh")} // 8 字符 → 2 token
	rsp := buildChatCompletion(spec, "mock-chat")
	if rsp == nil || len(rsp.Choices) != 1 {
		t.Fatalf("want 1 choice, got %+v", rsp)
	}
	if rsp.Choices[0].Message == nil || rsp.Choices[0].Message.Content == nil {
		t.Fatal("choice message content missing")
	}
	if rsp.Choices[0].Message.Content.Text != "abcdefgh" {
		t.Fatalf("content mismatch: %q", rsp.Choices[0].Message.Content.Text)
	}
	if rsp.Choices[0].FinishReason != enum.FinishReasonStop {
		t.Fatalf("finish_reason want stop, got %s", rsp.Choices[0].FinishReason)
	}
	if rsp.Usage == nil || rsp.Usage.CompletionTokens != 2 {
		t.Fatalf("usage mismatch: %+v", rsp.Usage)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
go test ./test/unit/transport/ -run "TestBuildChat" -v
```
Expected: FAIL — `undefined: buildChatStream`

- [ ] **Step 3: 实现 Chat 构造**

`internal/infrastructure/transport/mock_openai.go`:

```go
package transport

import (
	"time"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// buildChatStream 生成 Chat Completions 流式 chunk 序列（不含时序等待）。
// 结构：首个 chunk 带 role，中间片带 content，末片带 finish_reason + usage。
func buildChatStream(spec mockSpec, model, chunkID string) []*dto.OpenAIChatCompletionChunk {
	pieces := spec.chunks()
	created := time.Now().Unix()
	out := make([]*dto.OpenAIChatCompletionChunk, 0, len(pieces)+1)

	for i, piece := range pieces {
		content := piece
		delta := &dto.OpenAIChatCompletionChunkDelta{Content: &content}
		if i == 0 {
			delta.Role = enum.RoleAssistant
		}
		out = append(out, &dto.OpenAIChatCompletionChunk{
			ID:      chunkID,
			Created: created,
			Model:   model,
			Object:  "chat.completion.chunk",
			Choices: []*dto.OpenAIChatCompletionChunkChoice{{Index: 0, Delta: delta}},
		})
	}

	stop := enum.FinishReasonStop
	out = append(out, &dto.OpenAIChatCompletionChunk{
		ID:      chunkID,
		Created: created,
		Model:   model,
		Object:  "chat.completion.chunk",
		Choices: []*dto.OpenAIChatCompletionChunkChoice{{Index: 0, FinishReason: &stop}},
		Usage:   buildChatUsage(spec),
	})
	return out
}

// buildChatCompletion 生成非流式 Chat Completions 响应。
func buildChatCompletion(spec mockSpec, model string) *dto.OpenAIChatCompletion {
	content := string(spec.chars)
	return &dto.OpenAIChatCompletion{
		ID:      "chatcmpl-mock-" + lo.RandomString(8, lo.LowerCaseLettersCharset),
		Created: time.Now().Unix(),
		Model:   model,
		Object:  "chat.completion",
		Choices: []*dto.OpenAIChatCompletionChoice{{
			Index:        0,
			Message:      &dto.OpenAIChatCompletionMessageParam{Role: enum.RoleAssistant, Content: &dto.OpenAIMessageContent{Text: content}},
			FinishReason: enum.FinishReasonStop,
		}},
		Usage: buildChatUsage(spec),
	}
}

// buildChatUsage 按 1 token ≈ 4 字符口径估算 usage。
func buildChatUsage(spec mockSpec) *dto.OpenAICompletionUsage {
	out := spec.tokens()
	return &dto.OpenAICompletionUsage{CompletionTokens: out, PromptTokens: 0, TotalTokens: out}
}
```

> **类型已核实**（勿改动）：`OpenAIChatCompletionChoice.FinishReason` 是 `enum.FinishReason` **值类型**（非指针）；`OpenAIChatCompletionMessageParam.Content` 是 `*dto.OpenAIMessageContent`（联合类型 `Text string` + `Parts`，已有 `MarshalJSON`，构造用 `&dto.OpenAIMessageContent{Text: ...}`）。

- [ ] **Step 4: 跑测试确认通过**

```bash
go test ./test/unit/transport/ -run "TestBuildChat" -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/transport/mock_openai.go test/unit/transport/mock_openai_test.go
git commit -m "feat(mock): OpenAI Chat Completions 随机流响应构造"
```

---

### Task 4: OpenAI Responses 响应构造

**Files:**
- Modify: `internal/infrastructure/transport/mock_openai.go`
- Test: `test/unit/transport/mock_openai_test.go`

**Interfaces:**
- Consumes: `mockSpec`（Task 2）
- Produces: `buildResponseStream(spec mockSpec, model, responseID string) []mockResponseEvent`、`buildResponse(spec mockSpec, model string) *dto.OpenAICreateResponseRsp`、`type mockResponseEvent struct{ Event string; Data []byte }`

- [ ] **Step 1: 写失败测试**

追加到 `test/unit/transport/mock_openai_test.go`:

```go
func TestBuildResponseStream_Events(t *testing.T) {
	t.Parallel()
	spec := mockSpec{chars: []byte("abcdefghijklmno")} // 4 片
	events := buildResponseStream(spec, "mock-response", "resp-1")

	if len(events) != 7 { // created + output_item.added + 4 delta + completed
		t.Fatalf("want 7 events, got %d", len(events))
	}
	if events[0].Event != enum.ResponseStreamEventCreated {
		t.Fatalf("first event want created, got %s", events[0].Event)
	}
	if events[len(events)-1].Event != enum.ResponseStreamEventCompleted {
		t.Fatalf("last event want completed, got %s", events[len(events)-1].Event)
	}
	deltaCount := 0
	for _, e := range events {
		if e.Event == enum.ResponseStreamEventOutputTextDelta {
			deltaCount++
		}
	}
	if deltaCount != 4 {
		t.Fatalf("want 4 delta events, got %d", deltaCount)
	}
}

func TestBuildResponse_NonStream(t *testing.T) {
	t.Parallel()
	spec := mockSpec{chars: []byte("abcdefgh")}
	rsp := buildResponse(spec, "mock-response")
	if rsp.Status != enum.ResponseStatusCompleted {
		t.Fatalf("status want completed, got %s", rsp.Status)
	}
	if rsp.Model != "mock-response" || len(rsp.Output) == 0 {
		t.Fatalf("response shape mismatch: %+v", rsp)
	}
	if rsp.Usage == nil || rsp.Usage.OutputTokens != 2 {
		t.Fatalf("usage mismatch: %+v", rsp.Usage)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
go test ./test/unit/transport/ -run "TestBuildResponse" -v
```
Expected: FAIL — `undefined: buildResponseStream`

- [ ] **Step 3: 实现 Responses 构造**

追加到 `internal/infrastructure/transport/mock_openai.go`:

```go
// mockResponseEvent 一个 Responses API 的 SSE 事件（事件名 + JSON 负载）。
type mockResponseEvent struct {
	Event string
	Data  []byte
}

// buildResponseStream 生成 Responses API 流式事件序列：
// response.created → response.output_item.added → N×response.output_text.delta → response.completed
func buildResponseStream(spec mockSpec, model, responseID string) []mockResponseEvent {
	pieces := spec.chunks()
	out := make([]mockResponseEvent, 0, len(pieces)+3)
	created := time.Now().Unix()

	out = append(out, mustMockEvent(enum.ResponseStreamEventCreated, map[string]any{
		"id": responseID, "object": "response", "status": enum.ResponseStatusInProgress,
		"created_at": created, "model": model,
	}))
	out = append(out, mustMockEvent(enum.ResponseStreamEventOutputItemAdded, map[string]any{
		"output_index": 0,
		"item":         map[string]any{"type": "message", "role": "assistant", "status": "in_progress"},
	}))
	for i, piece := range pieces {
		out = append(out, mustMockEvent(enum.ResponseStreamEventOutputTextDelta, map[string]any{
			"output_index": 0, "content_index": 0, "delta": piece, "sequence_number": i + 1,
		}))
	}
	out = append(out, mustMockEvent(enum.ResponseStreamEventCompleted, map[string]any{
		"id": responseID, "object": "response", "status": enum.ResponseStatusCompleted,
		"created_at": created, "model": model, "usage": buildResponseUsage(spec),
	}))
	return out
}

// buildResponse 生成非流式 Responses 响应。
func buildResponse(spec mockSpec, model string) *dto.OpenAICreateResponseRsp {
	responseID := "resp-mock-" + lo.RandomString(8, lo.LowerCaseLettersCharset)
	msgType, role, status := "message", "assistant", "completed"
	text := string(spec.chars)
	return &dto.OpenAICreateResponseRsp{
		ID:        responseID,
		Object:    "response",
		CreatedAt: time.Now().Unix(),
		Status:    enum.ResponseStatusCompleted,
		Model:     model,
		Output: []*dto.ResponseInputItem{{
			Type: &msgType, Role: &role, Status: &status,
			Content: &dto.ResponseInputMessageContent{Text: text},
		}},
		Usage: buildResponseUsage(spec),
	}
}

// buildResponseUsage 按 1 token ≈ 4 字符口径估算 usage。
func buildResponseUsage(spec mockSpec) *dto.ResponseUsage {
	out := spec.tokens()
	return &dto.ResponseUsage{OutputTokens: out, TotalTokens: out}
}

// mustMockEvent 把事件负载序列化为 SSE data 字段。
func mustMockEvent(event string, payload map[string]any) mockResponseEvent {
	data, err := sonic.Marshal(payload)
	if err != nil {
		panic(err) // 负载是本函数内的字面量 map，序列化失败属编程错误
	}
	return mockResponseEvent{Event: event, Data: data}
}
```

> 实现提示：`dto.ResponseInputMessageContent` 的确切形态（`Text` 是 string 还是 parts 数组）以 `internal/dto/openai/response_input.go` 为准；若它是联合类型，改用实际可表达「一条 assistant 文本消息」的最简形态。`enum.ResponseStreamEvent*` 常量名已确认存在于 `internal/common/enum/openai_response.go`。

- [ ] **Step 4: 跑测试确认通过**

```bash
go test ./test/unit/transport/ -run "TestBuildResponse" -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/transport/mock_openai.go test/unit/transport/mock_openai_test.go
git commit -m "feat(mock): OpenAI Responses 随机流响应构造"
```

---

### Task 5: Anthropic Messages 响应构造

**Files:**
- Create: `internal/infrastructure/transport/mock_anthropic.go`
- Test: `test/unit/transport/mock_anthropic_test.go`

**Interfaces:**
- Consumes: `mockSpec`（Task 2）
- Produces: `buildMessageStream(spec mockSpec, model, messageID string) []mockResponseEvent`、`buildMessage(spec mockSpec, model string) *dto.AnthropicMessage`、`buildTokensCount(spec mockSpec) *dto.AnthropicTokensCount`

- [ ] **Step 1: 写失败测试**

`test/unit/transport/mock_anthropic_test.go`:

```go
package transport

import "testing"

func TestBuildMessageStream_Events(t *testing.T) {
	t.Parallel()
	spec := mockSpec{chars: []byte("abcdefghijklmno")} // 4 片
	events := buildMessageStream(spec, "mock-messages", "msg-1")

	if len(events) != 7 { // message_start + content_block_start + 4 delta + message_stop
		t.Fatalf("want 7 events, got %d", len(events))
	}
	if events[0].Event != "message_start" {
		t.Fatalf("first event want message_start, got %s", events[0].Event)
	}
	if events[len(events)-1].Event != "message_stop" {
		t.Fatalf("last event want message_stop, got %s", events[len(events)-1].Event)
	}
}

func TestBuildMessage_NonStream(t *testing.T) {
	t.Parallel()
	spec := mockSpec{chars: []byte("abcdefgh")}
	msg := buildMessage(spec, "mock-messages")
	if msg.Model != "mock-messages" || len(msg.Content) != 1 {
		t.Fatalf("message shape mismatch: %+v", msg)
	}
	if msg.StopReason == nil || *msg.StopReason != enum.AnthropicStopReasonEndTurn {
		t.Fatal("stop_reason must be end_turn")
	}
	if msg.Usage == nil || msg.Usage.OutputTokens != 2 {
		t.Fatalf("usage mismatch: %+v", msg.Usage)
	}
}

func TestBuildTokensCount(t *testing.T) {
	t.Parallel()
	spec := mockSpec{chars: []byte("abcdefgh")}
	if got := buildTokensCount(spec).InputTokens; got != 2 {
		t.Fatalf("input_tokens want 2, got %d", got)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
go test ./test/unit/transport/ -run "TestBuildMessage|TestBuildTokensCount" -v
```
Expected: FAIL — `undefined: buildMessageStream`

- [ ] **Step 3: 实现 Anthropic 构造**

`internal/infrastructure/transport/mock_anthropic.go`:

```go
package transport

import (
	"time"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// buildMessageStream 生成 Anthropic Messages 流式事件序列：
// message_start → content_block_start → N×content_block_delta → message_delta → message_stop
func buildMessageStream(spec mockSpec, model, messageID string) []mockResponseEvent {
	pieces := spec.chunks()
	out := make([]mockResponseEvent, 0, len(pieces)+4)

	out = append(out, mustMockEvent("message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id": messageID, "type": "message", "role": "assistant", "model": model,
			"content": []any{}, "stop_reason": nil,
			"usage": map[string]any{"input_tokens": 0, "output_tokens": 0},
		},
	}))
	out = append(out, mustMockEvent("content_block_start", map[string]any{
		"type": "content_block_start", "index": 0,
		"content_block": map[string]any{"type": "text", "text": ""},
	}))
	for i, piece := range pieces {
		out = append(out, mustMockEvent("content_block_delta", map[string]any{
			"type": "content_block_delta", "index": 0, "sequence_number": i + 1,
			"delta": map[string]any{"type": "text_delta", "text": piece},
		}))
	}
	out = append(out, mustMockEvent("message_delta", map[string]any{
		"type": "message_delta",
		"delta":   map[string]any{"stop_reason": enum.AnthropicStopReasonEndTurn, "stop_sequence": nil},
		"usage":   map[string]any{"output_tokens": spec.tokens()},
	}))
	out = append(out, mustMockEvent("message_stop", map[string]any{"type": "message_stop"}))
	return out
}

// buildMessage 生成非流式 Anthropic Message 响应。
func buildMessage(spec mockSpec, model string) *dto.AnthropicMessage {
	text := string(spec.chars)
	return &dto.AnthropicMessage{
		ID:         "msg-mock-" + lo.RandomString(8, lo.LowerCaseLettersCharset),
		Type:       "message",
		Role:       "assistant",
		Model:      model,
		Content:    []*dto.AnthropicContentBlock{{Type: "text", Text: &text}},
		StopReason: lo.ToPtr(enum.AnthropicStopReasonEndTurn),
		Usage:      &dto.AnthropicUsage{OutputTokens: spec.tokens()},
	}
}

// buildTokensCount 生成 count_tokens 响应（按同口径估算）。
func buildTokensCount(spec mockSpec) *dto.AnthropicTokensCount {
	return &dto.AnthropicTokensCount{InputTokens: spec.tokens()}
}
```

- [ ] **Step 4: 跑测试确认通过**

```bash
go test ./test/unit/transport/ -run "TestBuildMessage|TestBuildTokensCount" -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/transport/mock_anthropic.go test/unit/transport/mock_anthropic_test.go
git commit -m "feat(mock): Anthropic Messages 随机流响应构造"
```

---

### Task 6: mock 装饰器（分发 + 时序 + 流式 Read）

**Files:**
- Create: `internal/infrastructure/transport/mock_proxy.go`
- Test: `test/unit/transport/mock_proxy_test.go`

**Interfaces:**
- Consumes: Task 2/3/4/5 的全部构造函数、`config.MockUpstreamEnabled`（Task 1）
- Produces: `NewMockOpenAIProxy(inner usecase.OpenAIProxyPort) usecase.OpenAIProxyPort`、`NewMockAnthropicProxy(inner usecase.AnthropicProxyPort) usecase.AnthropicProxyPort`、`mockStream`、`isMock(ep vo.UpstreamEndpoint) bool`

- [ ] **Step 1: 写失败测试**

`test/unit/transport/mock_proxy_test.go`:

```go
package transport

import (
	"context"
	"io"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
)

// fakeOpenAIProxy 记录是否被调用，用于验证非 mock 请求零透传改变。
type fakeOpenAIProxy struct{ called bool }

func (f *fakeOpenAIProxy) ForwardChatCompletion(context.Context, vo.UpstreamEndpoint, []byte) (*dto.OpenAIChatCompletion, error) {
	f.called = true
	return &dto.OpenAIChatCompletion{Object: "chat.completion"}, nil
}
func (f *fakeOpenAIProxy) OpenChatCompletionStream(context.Context, vo.UpstreamEndpoint, []byte) (io.ReadCloser, error) {
	f.called = true
	return io.NopCloser(nil), nil
}
func (f *fakeOpenAIProxy) ReadChatCompletionStream(context.Context, io.ReadCloser, func(*dto.OpenAIChatCompletionChunk) error) (*dto.OpenAIChatCompletion, error) {
	f.called = true
	return nil, nil
}
func (f *fakeOpenAIProxy) ForwardCreateResponse(context.Context, vo.UpstreamEndpoint, []byte) ([]byte, error) {
	f.called = true
	return nil, nil
}
func (f *fakeOpenAIProxy) ForwardCreateDecision(context.Context, vo.UpstreamEndpoint, []byte) ([]byte, error) {
	f.called = true
	return nil, nil
}
func (f *fakeOpenAIProxy) OpenCreateResponseStream(context.Context, vo.UpstreamEndpoint, []byte) (io.ReadCloser, error) {
	f.called = true
	return io.NopCloser(nil), nil
}
func (f *fakeOpenAIProxy) ReadCreateResponseStream(context.Context, io.ReadCloser, func(string, []byte) error) error {
	f.called = true
	return nil
}

func TestIsMock_DisabledByDefault(t *testing.T) {
	t.Parallel()
	ep := vo.NewUpstreamEndpointFromCredential(MockUpstreamModel, "k", "http://mock.invalid")
	if isMock(ep) {
		t.Fatal("isMock must be false when switch is off")
	}
}

func TestMockProxy_ForwardChatCompletion_UsesMock(t *testing.T) {
	t.Setenv("MOCK_UPSTREAM_ENABLED", "true")
	config.MockUpstreamEnabled = true
	t.Cleanup(func() { config.MockUpstreamEnabled = false })

	fake := &fakeOpenAIProxy{}
	p := NewMockOpenAIProxy(fake)
	ep := vo.NewUpstreamEndpointFromCredential(MockUpstreamModel, "k", "http://mock.invalid")

	rsp, err := p.ForwardChatCompletion(context.Background(), ep, []byte(`{"model":"mock-chat"}`))
	if err != nil {
		t.Fatal(err)
	}
	if rsp == nil || rsp.Object != "chat.completion" {
		t.Fatalf("mock response malformed: %+v", rsp)
	}
	if fake.called {
		t.Fatal("inner proxy must not be called for mock target")
	}
}

func TestMockProxy_Passthrough_RealModel(t *testing.T) {
	config.MockUpstreamEnabled = true
	t.Cleanup(func() { config.MockUpstreamEnabled = false })

	fake := &fakeOpenAIProxy{}
	p := NewMockOpenAIProxy(fake)
	ep := vo.NewUpstreamEndpointFromCredential("gpt-4o", "k", "http://real.example")

	if _, err := p.ForwardChatCompletion(context.Background(), ep, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if !fake.called {
		t.Fatal("inner proxy must be called for real target")
	}
}
```

> 注意：`fakeOpenAIProxy` 须实现 `usecase.OpenAIProxyPort` 全部 7 个方法；上方签名以 `internal/application/llmproxy/usecase/port.go` 为准。测试内改 `config.MockUpstreamEnabled` 后必须 `t.Cleanup` 还原（既有 `guard_integration_test.go` 同模式）。

- [ ] **Step 2: 跑测试确认失败**

```bash
go test ./test/unit/transport/ -run "TestIsMock|TestMockProxy" -v
```
Expected: FAIL — `undefined: isMock`

- [ ] **Step 3: 实现装饰器**

`internal/infrastructure/transport/mock_proxy.go`:

```go
package transport

import (
	"context"
	"io"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// isMock 判断该上游端点是否为 mock 目标：开关开启且 upstream model 为 mock 标记。
func isMock(ep vo.UpstreamEndpoint) bool {
	return config.MockUpstreamEnabled && ep.Model == MockUpstreamModel
}

// mockStream 承载 mock 参数的假流。Read 恒返回 EOF——真实内容由 Read*Stream
// 依 spec 本地生成，不经过字节流序列化往返。
type mockStream struct {
	io.Reader
	spec  mockSpec
	model string
}

func newMockStream(spec mockSpec, model string) *mockStream {
	return &mockStream{Reader: strings.NewReader(""), spec: spec, model: model}
}

func (s *mockStream) Close() error { return nil }

// mockOpenAIProxy 包装真实 OpenAIProxyPort，命中 mock 标记时本地生成响应。
type mockOpenAIProxy struct{ inner usecase.OpenAIProxyPort }

// NewMockOpenAIProxy 构造带 mock 能力的 OpenAI 上游代理。
func NewMockOpenAIProxy(inner usecase.OpenAIProxyPort) usecase.OpenAIProxyPort {
	return &mockOpenAIProxy{inner: inner}
}

func (p *mockOpenAIProxy) ForwardChatCompletion(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (*dto.OpenAIChatCompletion, error) {
	if !isMock(ep) {
		return p.inner.ForwardChatCompletion(ctx, ep, body)
	}
	return buildChatCompletion(newMockSpec(rand.IntN, maxTokensFromOpenAIBody(body)), ep.Model), nil
}

func (p *mockOpenAIProxy) OpenChatCompletionStream(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (io.ReadCloser, error) {
	if !isMock(ep) {
		return p.inner.OpenChatCompletionStream(ctx, ep, body)
	}
	return newMockStream(newMockSpec(rand.IntN, maxTokensFromOpenAIBody(body)), ep.Model), nil
}

func (p *mockOpenAIProxy) ReadChatCompletionStream(ctx context.Context, stream io.ReadCloser, onChunk func(*dto.OpenAIChatCompletionChunk) error) (*dto.OpenAIChatCompletion, error) {
	ms, ok := stream.(*mockStream)
	if !ok {
		return p.inner.ReadChatCompletionStream(ctx, stream, onChunk)
	}
	defer func() { _ = stream.Close() }()
	return emitChatStream(ctx, ms, onChunk)
}

func (p *mockOpenAIProxy) ForwardCreateResponse(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) ([]byte, error) {
	if !isMock(ep) {
		return p.inner.ForwardCreateResponse(ctx, ep, body)
	}
	return sonic.Marshal(buildResponse(newMockSpec(rand.IntN, maxTokensFromOpenAIBody(body)), ep.Model))
}

func (p *mockOpenAIProxy) ForwardCreateDecision(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) ([]byte, error) {
	// Decision 不在 mock 范围（mock Endpoint 不开该能力），原样透传。
	return p.inner.ForwardCreateDecision(ctx, ep, body)
}

func (p *mockOpenAIProxy) OpenCreateResponseStream(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (io.ReadCloser, error) {
	if !isMock(ep) {
		return p.inner.OpenCreateResponseStream(ctx, ep, body)
	}
	return newMockStream(newMockSpec(rand.IntN, maxTokensFromOpenAIBody(body)), ep.Model), nil
}

func (p *mockOpenAIProxy) ReadCreateResponseStream(ctx context.Context, stream io.ReadCloser, onEvent func(event string, data []byte) error) error {
	ms, ok := stream.(*mockStream)
	if !ok {
		return p.inner.ReadCreateResponseStream(ctx, stream, onEvent)
	}
	defer func() { _ = stream.Close() }()
	return emitResponseStream(ctx, ms, onEvent)
}

// emitChatStream 按 mockSpec 的 TTFT/TPS 时序吐出 Chat chunk 并聚合出 completion。
func emitChatStream(ctx context.Context, ms *mockStream, onChunk func(*dto.OpenAIChatCompletionChunk) error) (*dto.OpenAIChatCompletion, error) {
	if err := waitTTFT(ctx, ms.spec.ttft); err != nil {
		return nil, err
	}
	for _, chunk := range buildChatStream(ms.spec, ms.model, "chatcmpl-mock") {
		if err := onChunk(chunk); err != nil {
			return nil, err
		}
		if err := waitInterval(ctx, ms.spec.interval); err != nil {
			return nil, err
		}
	}
	return buildChatCompletion(ms.spec, ms.model), nil
}

// emitResponseStream 按时序吐出 Responses 事件。
func emitResponseStream(ctx context.Context, ms *mockStream, onEvent func(event string, data []byte) error) error {
	if err := waitTTFT(ctx, ms.spec.ttft); err != nil {
		return err
	}
	for _, ev := range buildResponseStream(ms.spec, ms.model, "resp-mock") {
		if err := onEvent(ev.Event, ev.Data); err != nil {
			return err
		}
		if err := waitInterval(ctx, ms.spec.interval); err != nil {
			return err
		}
	}
	return nil
}

// waitTTFT 流开始前等待首 token 延迟；ctx 取消则立即返回。
func waitTTFT(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// waitInterval 相邻分片间等待 1/TPS；ctx 取消则立即返回。
func waitInterval(ctx context.Context, d time.Duration) error {
	return waitTTFT(ctx, d)
}

// maxTokensFromOpenAIBody 从 OpenAI 请求体提取 max_tokens（缺失或非法返回 0）。
func maxTokensFromOpenAIBody(body []byte) int {
	var req struct {
		MaxTokens int `json:"max_tokens"`
	}
	if err := sonic.Unmarshal(body, &req); err != nil {
		return 0
	}
	return req.MaxTokens
}

// mockAnthropicProxy 包装真实 AnthropicProxyPort，命中 mock 标记时本地生成响应。
type mockAnthropicProxy struct{ inner usecase.AnthropicProxyPort }

// NewMockAnthropicProxy 构造带 mock 能力的 Anthropic 上游代理。
func NewMockAnthropicProxy(inner usecase.AnthropicProxyPort) usecase.AnthropicProxyPort {
	return &mockAnthropicProxy{inner: inner}
}

func (p *mockAnthropicProxy) ForwardCreateMessage(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (*dto.AnthropicMessage, error) {
	if !isMock(ep) {
		return p.inner.ForwardCreateMessage(ctx, ep, body)
	}
	return buildMessage(newMockSpec(rand.IntN, maxTokensFromAnthropicBody(body)), ep.Model), nil
}

func (p *mockAnthropicProxy) OpenCreateMessageStream(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (io.ReadCloser, error) {
	if !isMock(ep) {
		return p.inner.OpenCreateMessageStream(ctx, ep, body)
	}
	return newMockStream(newMockSpec(rand.IntN, maxTokensFromAnthropicBody(body)), ep.Model), nil
}

func (p *mockAnthropicProxy) ReadCreateMessageStream(ctx context.Context, stream io.ReadCloser, onEvent func(dto.AnthropicSSEEvent) error) (*dto.AnthropicMessage, error) {
	ms, ok := stream.(*mockStream)
	if !ok {
		return p.inner.ReadCreateMessageStream(ctx, stream, onEvent)
	}
	defer func() { _ = stream.Close() }()
	if err := waitTTFT(ctx, ms.spec.ttft); err != nil {
		return nil, err
	}
	for _, ev := range buildMessageStream(ms.spec, ms.model, "msg-mock") {
		if err := onEvent(dto.AnthropicSSEEvent{Event: ev.Event, Data: ev.Data}); err != nil {
			return nil, err
		}
		if err := waitInterval(ctx, ms.spec.interval); err != nil {
			return nil, err
		}
	}
	return buildMessage(ms.spec, ms.model), nil
}

func (p *mockAnthropicProxy) ForwardCountTokens(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (*dto.AnthropicTokensCount, error) {
	if !isMock(ep) {
		return p.inner.ForwardCountTokens(ctx, ep, body)
	}
	return buildTokensCount(newMockSpec(rand.IntN, maxTokensFromAnthropicBody(body))), nil
}

// maxTokensFromAnthropicBody 从 Anthropic 请求体提取 max_tokens（缺失或非法返回 0）。
func maxTokensFromAnthropicBody(body []byte) int {
	var req struct {
		MaxTokens int `json:"max_tokens"`
	}
	if err := sonic.Unmarshal(body, &req); err != nil {
		return 0
	}
	return req.MaxTokens
}
```

> 实现提示：`newMockSpec(rand.IntN, ...)` 的签名是 `func(int) int` 适配；若 `newMockSpec` 已定义为接收 `*rand.Rand`，改传 `rand.New(rand.NewPCG(1, 2))` 或把签名改为接收 `intN func(int) int`。`dto.AnthropicSSEEvent.Data` 是 `sonic.NoCopyRawMessage`，`[]byte` 可直接赋值。`constant` 包若未用到则删掉该 import。

- [ ] **Step 4: 跑测试确认通过**

```bash
go test ./test/unit/transport/ -run "TestIsMock|TestMockProxy" -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/transport/mock_proxy.go test/unit/transport/mock_proxy_test.go
git commit -m "feat(mock): Proxy Port 装饰器，命中 mock-llm 本地生成随机流"
```

---

### Task 7: DI 装配接线

**Files:**
- Modify: `internal/bootstrap/modules/repository.go:140-146`
- Test: `test/unit/dependency_injection/`（既有）

**Interfaces:**
- Consumes: `transport.NewMockOpenAIProxy` / `transport.NewMockAnthropicProxy`（Task 6）
- Produces: `NewOpenAIProxy` / `NewAnthropicProxy` 返回值改为装饰后的代理

- [ ] **Step 1: 改装配**

`internal/bootstrap/modules/repository.go` 的两个构造函数改为装饰：

```go
func NewOpenAIProxy(tracker *inflight.Tracker, guard *transport.EndpointGuard) usecase.OpenAIProxyPort {
	return transport.NewMockOpenAIProxy(transport.NewOpenAIProxy(tracker, guard))
}

func NewAnthropicProxy(tracker *inflight.Tracker, guard *transport.EndpointGuard) usecase.AnthropicProxyPort {
	return transport.NewMockAnthropicProxy(transport.NewAnthropicProxy(tracker, guard))
}
```

- [ ] **Step 2: 编译 + 全量测试**

```bash
go build ./... && go test ./test/unit/... 2>&1 | tail -20
```
Expected: 编译通过、既有测试全绿（装饰器默认关闭，非 mock 路径零行为改变）

- [ ] **Step 3: Commit**

```bash
git add internal/bootstrap/modules/repository.go
git commit -m "feat(mock): DI 装配改为装饰器包装，mock 开关默认关闭"
```

---

### Task 8: wrk 压测脚本（三协议）

**Files:**
- Create: `test/loadtest/loadtest_common.lua`、`test/loadtest/chat_completions.lua`、`test/loadtest/responses.lua`、`test/loadtest/messages.lua`
- Create: `test/loadtest/README.md`

**Interfaces:**
- Consumes: `mock-llm` 模型别名、压测专用 API Key
- Produces: 可执行 wrk 脚本，输出并发 SSE 连接数、延迟分位、错误率

- [ ] **Step 1: 写公共库**

`test/loadtest/loadtest_common.lua`:

```lua
-- loadtest_common.lua 三协议共用：参数注入、请求头、响应校验、汇总输出
local M = {}

M.target   = os.getenv("TARGET_URL") or "https://api.lvlvko.top"
M.api_key  = os.getenv("API_KEY") or ""
M.model    = os.getenv("MODEL") or "mock-chat"
M.max_tok  = tonumber(os.getenv("MAX_TOKENS") or "0") or 0
M.ua       = "aris-loadtest/1.0"
M.marker   = "__LOADTEST__"  -- 防 checksum 复用真实消息

function M.headers()
  return {
    ["Content-Type"]  = "application/json",
    ["X-API-Key"]     = M.api_key,
    ["User-Agent"]    = M.ua,
    ["Accept"]        = "text/event-stream",
  }
end

function M.prompt()
  -- 固定前缀 + 时间戳，保证 checksum 不与任何真实消息相同
  return M.marker .. " perf probe " .. tostring(os.time())
end

function M.build_body()
  error("override in protocol script")
end

local status_count = {}
local body_bytes = 0

function M.on_response(status, headers, body)
  status_count[status] = (status_count[status] or 0) + 1
  body_bytes = body_bytes + (body and #body or 0)
end

function M.on_done(summary, latency, requests)
  print("\n===== loadtest summary =====")
  print(string.format("duration(s)      : %.2f", summary.duration / 1e6))
  print(string.format("requests         : %d", summary.requests))
  print(string.format("errors           : %d", summary.errors.connect + summary.errors.read + summary.errors.write + summary.errors.status))
  print(string.format("latency avg(ms)  : %.2f", latency.mean / 1000))
  print(string.format("latency p50(ms)  : %.2f", latency:percentile(50) / 1000))
  print(string.format("latency p90(ms)  : %.2f", latency:percentile(90) / 1000))
  print(string.format("latency p99(ms)  : %.2f", latency:percentile(99) / 1000))
  print(string.format("body bytes       : %d", body_bytes))
  print("status codes:")
  for code, n in pairs(status_count) do
    print(string.format("  %s : %d", tostring(code), n))
  end
  print("note: 核心指标「并发 SSE 长连接数」= wrk -c 参数值；逐档加压找稳定拐点")
  print("============================")
end

return M
```

- [ ] **Step 2: 写三协议脚本**

`test/loadtest/chat_completions.lua`:

```lua
local C = require("loadtest_common")

wrk.method = "POST"
wrk.headers = C.headers()
wrk.body = [[{"model":"]] .. C.model .. [[","stream":true,"messages":[{"role":"user","content":"]] .. C.prompt() .. [["]}]]

response = C.on_response
done     = C.on_done
```

`test/loadtest/responses.lua`:

```lua
local C = require("loadtest_common")

wrk.method = "POST"
wrk.headers = C.headers()
wrk.body = [[{"model":"]] .. C.model .. [[","stream":true,"input":"]] .. C.prompt() .. [["}]]

response = C.on_response
done     = C.on_done
```

`test/loadtest/messages.lua`:

```lua
local C = require("loadtest_common")

wrk.method = "POST"
wrk.headers = C.headers()
wrk.body = [[{"model":"]] .. C.model .. [[","stream":true,"max_tokens":256,"messages":[{"role":"user","content":"]] .. C.prompt() .. [["]}]]

response = C.on_response
done     = C.on_done
```

- [ ] **Step 3: 写使用说明**

`test/loadtest/README.md`:

```markdown
# 三接口压测脚本（wrk）

## 执行
​```bash
export TARGET_URL=https://api.lvlvko.top
export API_KEY=<压测专用 Key>
export MODEL=mock-chat          # 或 mock-response / mock-messages
​```
​```bash
wrk -t4 -c100 -d5m --latency -s test/loadtest/chat_completions.lua "$TARGET_URL/api/openai/v1/chat/completions"
wrk -t4 -c100 -d5m --latency -s test/loadtest/responses.lua     "$TARGET_URL/api/openai/v1/responses"
wrk -t4 -c100 -d5m --latency -s test/loadtest/messages.lua      "$TARGET_URL/api/anthropic/v1/messages"
​```

## 口径
- **核心指标 = 并发 SSE 长连接数**（即 wrk `-c`），逐档 50/100/200/400 加压找稳定拐点。
- TTFT 与流持续时间**不在 wrk 侧测**，压测后查审计表 `model_call_audits.first_token_latency_ms` / `stream_duration_ms`。
- 窗口 5 分钟 > 最长流 205 秒；连接超时须 > 205 秒。

## 沉淀
每次压测建档 `docs/loadtest/YYYY-MM-DD-<topic>.md`，原始输出存 `docs/loadtest/raw/`。
```

- [ ] **Step 4: 语法自检**

```bash
luac -p test/loadtest/*.lua 2>&1 || echo "luac 不可用时改用 luajit -bl 或人工复查"
```
Expected: 无语法错误

- [ ] **Step 5: Commit**

```bash
git add test/loadtest/
git commit -m "test(loadtest): wrk 三协议压测脚本与使用说明"
```

---

### Task 9: 清理 SQL 脚本

**Files:**
- Create: `scripts/loadtest-cleanup.sql`

**Interfaces:**
- Consumes: 压测专用 `api_key_id`、`sessions.message_ids` / `tool_ids`（JSON 数组列）
- Produces: 幂等清理脚本（预览 → 删除 → 孤儿回收）

- [ ] **Step 1: 写脚本**

`scripts/loadtest-cleanup.sql`:

```sql
-- loadtest-cleanup.sql 清理压测数据（按 api_key_id 精确定位）
-- 用法：psql -v api_key_id=<ID> -f scripts/loadtest-cleanup.sql
-- 幂等：重复执行第二次删 0 行。

\set ON_ERROR_STOP on

BEGIN;

-- ========== Step 1 预览核对（先看再删） ==========
SELECT 'audits_to_delete' AS what, count(*) AS n
FROM model_call_audits WHERE api_key_id = :api_key_id
UNION ALL
SELECT 'sessions_to_delete', count(*)
FROM sessions WHERE api_key_id = :api_key_id AND deleted_at = 0;

-- ========== Step 2 删审计 ==========
DELETE FROM model_call_audits WHERE api_key_id = :api_key_id;

-- ========== Step 3 收集压测 session 引用的 message/tool ==========
CREATE TEMP TABLE lt_msg AS
SELECT DISTINCT jsonb_array_elements_text(message_ids::jsonb)::bigint AS id
FROM sessions WHERE api_key_id = :api_key_id;

CREATE TEMP TABLE lt_tool AS
SELECT DISTINCT jsonb_array_elements_text(tool_ids::jsonb)::bigint AS id
FROM sessions WHERE api_key_id = :api_key_id;

-- ========== Step 4 软删压测 session ==========
UPDATE sessions SET deleted_at = extract(epoch from now())::bigint
WHERE api_key_id = :api_key_id AND deleted_at = 0;

-- ========== Step 5 硬删孤儿 message/tool ==========
-- 孤儿 = 压测引用过、但不再被任何活跃 session 引用
DELETE FROM messages m
USING lt_msg l
WHERE m.id = l.id
  AND NOT EXISTS (
    SELECT 1 FROM sessions s
    WHERE s.deleted_at = 0
      AND s.message_ids::jsonb ?| ARRAY[l.id::text]  -- 占位，见下方说明
  );

DELETE FROM tools t
USING lt_tool l
WHERE t.id = l.id
  AND NOT EXISTS (
    SELECT 1 FROM sessions s
    WHERE s.deleted_at = 0
      AND s.tool_ids::jsonb ?| ARRAY[l.id::text]
  );

-- ========== Step 6 硬删已软删的压测 session ==========
DELETE FROM sessions WHERE api_key_id = :api_key_id AND deleted_at <> 0;

COMMIT;

-- ========== Step 7 复核 ==========
SELECT 'audits_left' AS what, count(*) AS n
FROM model_call_audits WHERE api_key_id = :api_key_id
UNION ALL
SELECT 'sessions_left', count(*)
FROM sessions WHERE api_key_id = :api_key_id;
```

> **重要实现提示**：Step 5 的 `?| ARRAY[l.id::text]` 是占位写法，`jsonb ?|` 判断的是**对象键**而非数组元素。数组成员判定须用 `EXISTS (SELECT 1 FROM jsonb_array_elements_text(s.message_ids::jsonb) x WHERE x::bigint = l.id)`。实现时必须按此改写，并用 `test/unit/message_dedup` 的数据形态造最小验证集确认不误删。这一步是唯一有数据安全风险的地方，必须先在本地 PG 造数据验证再上生产。

- [ ] **Step 2: 本地造数据验证**

```bash
psql "$DEV_DATABASE_URL" -v api_key_id=99999 -f scripts/loadtest-cleanup.sql
```
Expected: 预览行数符合预期；他 Key 数据不变；重复执行 0 行

- [ ] **Step 3: Commit**

```bash
git add scripts/loadtest-cleanup.sql
git commit -m "feat(loadtest): 压测数据清理脚本（api_key_id 定向 + 孤儿回收 + 幂等）"
```

---

### Task 10: E2E 用例

**Files:**
- Create: `test/e2e/loadtest_mock/loadtest_mock_test.go`

**Interfaces:**
- Consumes: `mock-llm` 模型别名、`BASE_URL` / `API_KEY` 环境变量
- Produces: 验证三接口 mock 输出 + 落库 + 清理归零

- [ ] **Step 1: 写 E2E**

`test/e2e/loadtest_mock/loadtest_mock_test.go`:

```go
package loadtest_mock

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// mustEnv 返回 (baseURL, apiKey) 或 t.Skip；e2e 默认离线 skip。
func mustEnv(t *testing.T) (string, string) {
	t.Helper()
	baseURL, apiKey := os.Getenv("BASE_URL"), os.Getenv("API_KEY")
	if baseURL == "" || apiKey == "" {
		t.Skip("BASE_URL and API_KEY are required for e2e test")
	}
	return strings.TrimRight(baseURL, "/"), apiKey
}

func newClient() *http.Client { return &http.Client{Timeout: 60 * time.Second} }

// TestMockChatCompletionStream 验证 chat 接口返回随机字符流。
func TestMockChatCompletionStream(t *testing.T) {
	baseURL, apiKey := mustEnv(t)
	body := []byte(`{"model":"mock-chat","stream":true,"messages":[{"role":"user","content":"__LOADTEST__ e2e probe"}]}`)

	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/openai/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)

	resp, err := newClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	// 流式：至少读到一个带 content 的 chunk
	scanner := bufio.NewScanner(resp.Buffer(nil, 1<<20))
	sawContent := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, `"content"`) {
			sawContent = true
			break
		}
	}
	if !sawContent {
		t.Fatal("no content chunk observed in SSE stream")
	}
}

// TestMockMessageNonStream 验证 messages 接口非流式返回随机文本。
func TestMockMessageNonStream(t *testing.T) {
	baseURL, apiKey := mustEnv(t)
	body := []byte(`{"model":"mock-messages","max_tokens":64,"messages":[{"role":"user","content":"__LOADTEST__ e2e probe"}]}`)

	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/anthropic/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)

	resp, err := newClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, _ := io.ReadAll(resp.Body)
	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, raw)
	}
	if len(out.Content) == 0 || out.Content[0].Text == "" {
		t.Fatal("empty mock content")
	}
	if out.Usage.OutputTokens <= 0 {
		t.Fatal("usage.output_tokens must be > 0")
	}
	if !strings.Contains(out.Content[0].Text, "__LOADTEST__") && len(out.Content[0].Text) < 1 {
		t.Fatal("mock content malformed")
	}
}
```

- [ ] **Step 2: 离线跑（应 skip）**

```bash
go test ./test/e2e/loadtest_mock/ -v
```
Expected: SKIP（无 `BASE_URL`/`API_KEY`）

- [ ] **Step 3: 连生产跑（需 mock 模型已配 + 开关开启）**

```bash
BASE_URL=https://api.lvlvko.top API_KEY=<key> go test ./test/e2e/loadtest_mock/ -v
```
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add test/e2e/loadtest_mock/
git commit -m "test(e2e): loadtest mock 三接口随机流验证"
```

---

### Task 11: 压测档案模板

**Files:**
- Create: `docs/loadtest/TEMPLATE.md`、`docs/loadtest/README.md`

- [ ] **Step 1: 写模板**

`docs/loadtest/TEMPLATE.md`:

```markdown
# 压测档案：<主题>

> 日期：YYYY-MM-DD ｜ 执行人：xxx ｜ 分支：`xxx`

## 1. 压测参数
| 项 | 值 |
|----|----|
| 目标 URL | |
| 模型别名 | |
| 脚本 | `test/loadtest/xxx.lua`（git commit: `xxxxxxx`） |
| wrk 命令 | `wrk -t4 -c<并发> -d5m --latency -s ...` |
| 并发档 | 50 / 100 / 200 / ... |
| stream | true |
| max_tokens | |

## 2. 环境信息
- git commit：
- 被测镜像/版本：
- pod 数 × CPU 配额：
- 执行时间（低峰期证据）：

## 3. 原始数据
- wrk 输出：`docs/loadtest/raw/<日期-主题>/`

## 4. 服务侧数据（审计表）
| 指标 | 值 | SQL |
|------|----|-----|
| TTFT p50/p90/p99 | | `SELECT ... FROM model_call_audits WHERE api_key_id=...` |
| 流持续 p50/p90/p99 | | 同上 |
| 状态码分布 | | |

## 5. 结论
- **并发 SSE 长连接数（稳定拐点）**：
- 延迟分位：
- 流时长分层（短/中/长流完成数与延迟）：
- 错误率：
- 与历史轮次对比：
```

`docs/loadtest/README.md`:

```markdown
# 三接口压测档案

每次压测建档一份 `YYYY-MM-DD-<topic>.md`（从 `TEMPLATE.md` 复制），原始输出存 `raw/YYYY-MM-DD-<topic>/`。

**核心指标口径**：并发 SSE 长连接数（服务器同时维持的流数），非「每秒完成请求数」。逐档加压找稳定拐点。

**可复现要求**：档案必须含 wrk 完整命令行、git commit、环境信息、原始输出，缺一不可。
```

- [ ] **Step 2: Commit**

```bash
git add docs/loadtest/
git commit -m "docs(loadtest): 压测档案模板与沉淀规范"
```

---

## Self-Review 记录

**1. Spec 覆盖**：spec §4.1（mock 装饰器）→ Task 1/6/7；§4.2（随机流时序）→ Task 2/3/5；§4.3（wrk + 口径）→ Task 8；§4.4（清理）→ Task 9；§4.5（沉淀）→ Task 11；§5（测试）→ Task 1-6 单测 + Task 10 E2E。无遗漏。

**2. 占位符**：Task 9 Step 5 的 `?|` 写法已显式标注为**占位并给出正确改写方向**（`jsonb_array_elements_text`），这是数据安全关键点，不能含糊。Task 3/4/5 各有一条「实现提示」注明 DTO 字段以实际代码为准——因为 `ResponseInputMessageContent` 这类联合类型的精确形态需在实现时核对，已给出明确替代方案。

**3. 类型一致性**：`mockSpec`（Task 2）→ 三协议构造（Task 3/4/5）→ 装饰器（Task 6）字段名一致；`mockResponseEvent`（Task 4）被 Task 5/6 复用；`isMock` / `mockStream`（Task 6）被 Task 7 装配引用。

**类型核实结果**（实现时勿改动，已对照 `internal/dto` 确认）：
- `OpenAIChatCompletionChoice.FinishReason` 是 `enum.FinishReason` **值类型**（非指针）
- `OpenAIChatCompletionMessageParam.Content` 是 `*dto.OpenAIMessageContent`（联合类型 `Text string` + `Parts`，已有 `MarshalJSON`，构造用 `&dto.OpenAIMessageContent{Text: ...}`）
- `ResponseInputMessageContent` 同为联合类型（`Text string` + `Parts`），已有 `MarshalJSON`，构造用 `Text` 字段
- `AnthropicSSEEvent.Data` 是 `sonic.NoCopyRawMessage`，可直接赋 `[]byte`

**唯一需实现时统一的**：`newMockSpec` 的 RNG 注入签名——Task 2 按 `*rand.Rand` 定义，Task 6 调用处写成 `intN func(int) int` 形态。实现时统一为 `*rand.Rand`（推荐），Task 6 调用处改传 `rand.New(rand.NewPCG(1, 2))` 或包级 `mockRNG`。
