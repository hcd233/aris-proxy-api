# P3-2 LLM Proxy 去 Huma 依赖实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `internal/application/llmproxy/usecase` 与 application port 从 Huma、Fiber 和 `internal/api/util` 解耦，同时保持 OpenAI/Anthropic 普通响应、SSE、跨协议转换和上游打开阶段错误透传行为不变。

**Architecture:** application usecase 只返回 transport-neutral 的协议结果、流计划和业务/上游错误；HTTP handler/API adapter 负责把这些结果转换为 Huma response，并拥有 HTTP status、headers、JSON body、SSE writer 和生命周期指标。迁移按 native stream → 跨协议 stream → Responses API 的顺序进行，每一步均先增加行为测试再修改生产代码。

**Tech Stack:** Go 1.25.1、标准库 `testing`、Huma v2、Fiber v3、现有 `internal/infrastructure/transport` Open/Read 两阶段接口、现有 OpenAI/Anthropic DTO 与 converter。

## Global Constraints

- `internal/application/llmproxy/usecase` 和 `internal/application/llmproxy/port` 不得导入 Huma、Fiber 或 `internal/api/util`。
- application 层不得接收或写入 Huma/Fiber context、HTTP writer、`bufio.Writer` 或 `huma.StreamResponse`。
- 保持 `Open*Stream` 负责建立连接、`Read*Stream` 负责消费并关闭资源的两阶段契约。
- 上游 Open 阶段返回 429、401、5xx 时，客户端必须继续收到原有 HTTP 状态、允许透传的 headers 和错误 body；不得先发送 200 SSE headers。
- 不改变 OpenAI/Anthropic JSON error envelope、SSE event/data framing、Responses lifecycle event 顺序和 header passthrough 语义。
- 测试文件只能放在 `test/unit/<topic>/` 或 `test/e2e/<topic>/`，只能使用标准库 `testing`。
- 不引入通用 HTTP response framework；结果类型只表达本任务需要的协议数据和流生命周期。
- 每个任务完成后运行该任务列出的聚焦测试，再提交一个可独立审查的 commit。

---

## 文件与职责地图

| 文件 | 责任 |
| --- | --- |
| `internal/application/llmproxy/port/response.go` | 定义 application 结果、协议错误和无 HTTP writer 的流接口 |
| `internal/application/llmproxy/port/handler.go` | 让 usecase port 使用 transport-neutral 返回类型 |
| `internal/application/llmproxy/usecase/common.go` | 返回 application 错误/结果，不构造 Huma response |
| `internal/application/llmproxy/usecase/openai.go`、`openai_chat.go` | OpenAI Chat 路径和 OpenAI↔Anthropic Chat 路径 |
| `internal/application/llmproxy/usecase/anthropic.go`、`anthropic_message.go` | Anthropic Messages 路径和 Anthropic↔OpenAI Chat 路径 |
| `internal/application/llmproxy/usecase/openai_response.go` | OpenAI Responses native/跨协议路径 |
| `internal/application/llmproxy/usecase/recorder.go` | application 侧流生命周期、审计和 usage 记录 |
| `internal/application/llmproxy/util/sse.go`、`util/anthropic.go` | 只保留协议事件/错误数据构造，删除 Huma response factory |
| `internal/handler/openai.go`、`anthropic.go` | 调用 application 并将结果适配为 Huma response |
| `internal/api/util/http.go` 或新的 focused adapter 文件 | 复用现有 HTTP/Huma response 写入逻辑，不被 application 反向依赖 |
| `test/unit/llmproxy_usecase/architecture_test.go` | 静态验证 application 依赖边界 |
| `test/unit/llmproxy_usecase/stream_result_test.go` | 验证结果类型、错误和流资源所有权 |
| `test/unit/llmproxy_usecase/openai_forward_test.go` | 锁定 OpenAI native/跨协议流行为 |
| `test/unit/llmproxy_usecase/anthropic_forward_test.go` | 锁定 Anthropic native/跨协议流行为 |
| `test/unit/llmproxy_usecase/response_forward_test.go` | 锁定 Responses lifecycle 行为 |
| `test/e2e/openai_chat_completion/*`、`test/e2e/anthropic_sse/*` | 验证真实 HTTP adapter 行为，按现有目录实际文件补充测试 |

---

### Task 1: 先锁定依赖边界和 Open/Read 行为

**Files:**
- Create: `test/unit/llmproxy_usecase/architecture_test.go`
- Create: `test/unit/llmproxy_usecase/stream_result_test.go`
- Modify: `test/unit/llmproxy_usecase/openai_forward_test.go`
- Modify: `test/unit/llmproxy_usecase/anthropic_forward_test.go`

**Interfaces:**
- 本任务不修改生产接口。
- 测试使用源码扫描和现有 mock transport，锁定后续迁移不能改变的行为。

- [ ] **Step 1: 写 application 依赖边界失败测试**

```go
func TestLLMProxyApplicationDoesNotImportHTTPTransport(t *testing.T) {
    roots := []string{
        "internal/application/llmproxy/usecase",
        "internal/application/llmproxy/port",
    }
    forbidden := []string{
        "github.com/danielgtaylor/huma/v2",
        "github.com/gofiber/fiber/v3",
        "internal/api/util",
    }
    // 使用 os.ReadDir / os.ReadFile 递归读取 .go 文件；发现 forbidden 字符串即失败。
}
```

测试必须读取当前 module 的 `internal/application/llmproxy/usecase` 和 `port` 源文件，而不是依赖编译器错误信息。

- [ ] **Step 2: 运行测试确认当前代码失败**

运行：

```bash
go test ./test/unit/llmproxy_usecase -run TestLLMProxyApplicationDoesNotImportHTTPTransport -count=1
```

预期：FAIL，并列出 `huma`、`internal/api/util` 或 Fiber 的当前引用。

- [ ] **Step 3: 增加 Open 阶段错误行为测试**

使用现有 transport mock 验证：

```text
OpenChatCompletionStream 返回 model.UpstreamError{StatusCode: 429, Headers, Body}
=> usecase 不启动 Read，不写 SSE 数据，adapter 最终可获得 429、headers、body
```

同时覆盖 Anthropic Messages 和 OpenAI Responses 的 Open 阶段错误。测试必须断言 `Read*Stream` 未被调用。

- [ ] **Step 4: 运行行为测试确认基线**

运行：

```bash
go test ./test/unit/llmproxy_usecase -run 'Test.*(Stream|Upstream|Open)' -count=1
```

预期：既有测试通过；新架构测试仍因当前依赖失败。

- [ ] **Step 5: 提交测试基线**

```bash
git add test/unit/llmproxy_usecase
git commit -m "test: lock llmproxy application transport boundary"
```

---

### Task 2: 定义最小 transport-neutral 结果与错误接口

**Files:**
- Create: `internal/application/llmproxy/port/response.go`
- Create: `test/unit/llmproxy_usecase/stream_result_test.go`
- Modify: `internal/common/model/upstream_error.go`（仅在现有字段不足以携带 headers/body 时修改）

**Interfaces:**

结果类型必须满足以下约束：

```go
type ProtocolKind uint8

const (
    ProtocolOpenAI ProtocolKind = iota
    ProtocolAnthropic
)

type ProxyError struct {
    StatusCode int
    Headers    map[string]string
    Body       []byte
    Cause      error
    Protocol   ProtocolKind
}

type StreamResult struct {
    Protocol ProtocolKind
    Headers  map[string]string
    Open     func(context.Context) (Stream, error)
}

type Stream interface {
    Read(context.Context, EventSink) error
    Close() error
}

type EventSink interface {
    WriteEvent(event string, data []byte) error
}
```

实际代码可在编译验证后调整命名，但不得增加 `WriteHTTP`、`BodyWriter`、`SetStatus`、`SendStreamWriter` 或 Huma/Fiber 类型。若现有 converter 需要结构化事件，`EventSink` 必须改为 application 协议事件接口，而不是退回 `bufio.Writer`。

- [ ] **Step 1: 写结果类型测试**

测试以下行为：

1. `ProxyError` 能保留 status、headers、body 和 cause；
2. `StreamResult.Open` 只打开一次；
3. `Stream.Close` 在消费完成和消费失败时都可被 adapter 统一调用；
4. application 类型的 import 集合不包含 HTTP 框架。

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./test/unit/llmproxy_usecase -run 'Test(ProxyError|StreamResult)' -count=1
```

预期：FAIL，因为类型尚未存在。

- [ ] **Step 3: 实现最小类型**

只创建结果和错误数据结构，不添加通用 response registry、middleware、HTTP writer 或第二套 transport framework。

- [ ] **Step 4: 运行测试确认通过**

```bash
gofmt -w internal/application/llmproxy/port/response.go test/unit/llmproxy_usecase/stream_result_test.go
go test ./test/unit/llmproxy_usecase -run 'Test(ProxyError|StreamResult)' -count=1
```

预期：PASS。

- [ ] **Step 5: 提交接口**

```bash
git add internal/application/llmproxy/port/response.go test/unit/llmproxy_usecase/stream_result_test.go
git commit -m "refactor: define transport-neutral llmproxy result"
```

---

### Task 3: 迁移 usecase port 和 HTTP adapter

**Files:**
- Modify: `internal/application/llmproxy/port/handler.go`
- Modify: `internal/handler/openai.go`
- Modify: `internal/handler/anthropic.go`
- Modify: `internal/api/util/http.go`
- Create/Modify: `test/unit/llmproxy_usecase/handler_adapter_test.go`

**Interfaces:**
- `port.OpenAIUseCase.CreateChatCompletion`、`CreateResponse` 和 `port.AnthropicUseCase.CreateMessage` 不再返回 `*huma.StreamResponse`。
- handler 仍可实现 Huma route 所需的 `*huma.StreamResponse` 返回值。
- adapter 必须能将 `ProxyError` 映射为入口协议对应的 JSON error envelope。

- [ ] **Step 1: 写 adapter 失败测试**

使用标准库 fake writer 验证：

```text
OpenAI ProxyError{StatusCode: 429, Headers: X-Request-ID, Body: upstreamBody}
=> adapter 返回 Huma response，状态为 429，保留允许透传的 header 和 body

Anthropic ProxyError
=> adapter 返回 Anthropic error envelope，不写 OpenAI envelope

StreamResult
=> adapter 只在真实 stream 开始时设置 event-stream headers 和启动 SSE gauge
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./test/unit/llmproxy_usecase -run TestHandlerAdapter -count=1
```

预期：FAIL，因为 handler 仍直接返回 usecase 的 Huma response，且 adapter 尚未存在。

- [ ] **Step 3: 修改 port 返回类型**

将 port 方法改为返回 Task 2 定义的 application 结果/错误类型；保持请求 DTO、context 和方法名不变。不要在 port 包中导入 Huma。

- [ ] **Step 4: 在 handler/API 层实现 adapter**

adapter 按以下顺序工作：

```text
调用 usecase
  -> ProxyError：在 Huma response body 开始前设置 status/header/body
  -> StreamResult：创建 Huma SSE response；在 body callback 内 Open/Read
  -> Open 错误：如果 response 尚未写出，映射为 HTTP JSON 错误；不得写成 200 SSE
  -> Read 错误：沿用既有 SSE 终止/错误事件语义
  -> defer：关闭 stream、减少 gauge、完成审计生命周期
```

HTTP header 白名单和已有 `apiutil` 行为必须复用，不要将 header copy 逻辑复制到 application。

- [ ] **Step 5: 运行 adapter 测试确认通过**

```bash
gofmt -w internal/application/llmproxy/port internal/handler internal/api/util test/unit/llmproxy_usecase
go test ./test/unit/llmproxy_usecase -run TestHandlerAdapter -count=1
```

预期：PASS。

- [ ] **Step 6: 提交 adapter 边界**

```bash
git add internal/application/llmproxy/port/handler.go internal/handler/openai.go internal/handler/anthropic.go internal/api/util test/unit/llmproxy_usecase
git commit -m "refactor: adapt llmproxy results at HTTP boundary"
```

---

### Task 4: 迁移 OpenAI Chat 与 Anthropic Messages native stream

**Files:**
- Modify: `internal/application/llmproxy/usecase/common.go`
- Modify: `internal/application/llmproxy/usecase/openai.go`
- Modify: `internal/application/llmproxy/usecase/openai_chat.go`
- Modify: `internal/application/llmproxy/usecase/anthropic.go`
- Modify: `internal/application/llmproxy/usecase/anthropic_message.go`
- Modify: `internal/application/llmproxy/util/sse.go`
- Modify: `internal/application/llmproxy/util/anthropic.go`
- Modify: `test/unit/llmproxy_usecase/openai_forward_test.go`
- Modify: `test/unit/llmproxy_usecase/anthropic_forward_test.go`

**Interfaces:**
- usecase native stream 方法返回 Task 2 的 `StreamResult` 或 `ProxyError`。
- `OpenChatCompletionStream`、`OpenCreateMessageStream` 和对应 `Read` 方法签名保持不变。
- 协议转换 callback 不接收 Huma/Fiber context 或 HTTP writer。

- [ ] **Step 1: 扩展 native stream 行为测试**

为 OpenAI 和 Anthropic 各增加：

1. successful Open → Read 事件序列；
2. Open 429 不调用 Read；
3. Read 中途错误时 stream 被关闭；
4. model-not-found/content-blocked 的协议 error body 与 status 不变。

- [ ] **Step 2: 运行测试确认旧实现基线**

```bash
go test ./test/unit/llmproxy_usecase -run 'Test(OpenAI|Anthropic).*Native|Test.*ModelNotFound|Test.*ContentBlocked' -count=1
```

预期：旧行为测试通过；架构测试仍报告 Huma/API util 依赖。

- [ ] **Step 3: 将错误 factory 改为协议数据/ProxyError**

`util/sse.go` 和 `util/anthropic.go` 删除返回 `*huma.StreamResponse` 的函数，改为返回协议错误 body 或 `ProxyError`。application 只构造数据，不设置 HTTP status/header，不调用 `BodyWriter` 或 `SendStreamWriter`。

- [ ] **Step 4: 迁移 native usecase**

将 `apiutil.WrapStreamResponse` 和 Huma response callback 移除。usecase 返回可被 adapter 打开的 `StreamResult`；stream 消费仍调用现有 `Read*Stream`、converter、recorder 和 timer。

Open 阶段错误必须原样返回，不得在 `StreamResult` 内部先发送 SSE headers。

- [ ] **Step 5: 运行 native 测试与架构测试**

```bash
gofmt -w internal/application/llmproxy/usecase internal/application/llmproxy/util test/unit/llmproxy_usecase
go test ./test/unit/llmproxy_usecase -run 'Test(OpenAI|Anthropic)|TestLLMProxyApplicationDoesNotImportHTTPTransport' -count=1
```

预期：PASS，且架构测试无输出依赖错误。

- [ ] **Step 6: 提交 native 路径**

```bash
git add internal/application/llmproxy/usecase internal/application/llmproxy/util test/unit/llmproxy_usecase
git commit -m "refactor: decouple native llmproxy streams from huma"
```

---

### Task 5: 迁移跨协议 Chat 流和 OpenAI Responses API

**Files:**
- Modify: `internal/application/llmproxy/usecase/openai_chat.go`
- Modify: `internal/application/llmproxy/usecase/anthropic_message.go`
- Modify: `internal/application/llmproxy/usecase/openai_response.go`
- Modify: `internal/application/llmproxy/usecase/recorder.go`
- Modify: `test/unit/llmproxy_usecase/openai_forward_test.go`
- Modify: `test/unit/llmproxy_usecase/anthropic_forward_test.go`
- Modify: `test/unit/llmproxy_usecase/response_forward_test.go`
- Modify: existing files under `test/e2e/openai_chat_completion/`
- Modify: existing files under `test/e2e/anthropic_sse/`

**Interfaces:**
- 跨协议 stream 与 native stream 使用同一 adapter 边界；不新增第二种 HTTP response 类型。
- Responses API 的 event callback 保留 `event` 和 `data` 原始协议语义，adapter 只负责写入 transport。

- [ ] **Step 1: 写跨协议与 Responses 回归测试**

测试必须断言：

- OpenAI→Anthropic Chat 的 chunk/event 顺序、finish reason、usage；
- Anthropic→OpenAI Chat 的 tool call 和终止事件；
- Responses native 的 `response.in_progress`、delta、`response.completed` 顺序；
- Responses 跨协议的 lifecycle event 顺序；
- 各路径 Open 429/401/5xx 在发送 SSE 前返回 HTTP 错误；
- 请求取消后 stream close、gauge 和 recorder 结果恢复。

- [ ] **Step 2: 运行测试确认基线**

```bash
go test ./test/unit/llmproxy_usecase -run 'Test.*(CrossProtocol|Response|Lifecycle|ToolCall)' -count=1
go test ./test/e2e/openai_chat_completion ./test/e2e/anthropic_sse -count=1
```

预期：基线通过。

- [ ] **Step 3: 移除跨协议 usecase 中的 Huma/API util response 构造**

将跨协议 body callback 改为 application stream/event producer；所有 status/header/body 写入移动到 Task 3 的 adapter。保持 converter 和 recorder 的输入输出不变。

- [ ] **Step 4: 迁移 Responses API**

将 `openai_response.go` 中 native 和 Anthropic-backed 两条 stream 路径都改为 transport-neutral stream producer。不得改变 `ReadCreateResponseStream` 的 raw event/data 输入，也不得把 Responses event 重新编码成 Chat Completion chunk。

- [ ] **Step 5: 运行聚焦测试**

```bash
gofmt -w internal/application/llmproxy/usecase test/unit/llmproxy_usecase
go test ./test/unit/llmproxy_usecase -run 'Test.*(CrossProtocol|Response|Lifecycle|ToolCall)' -count=1
go test ./test/e2e/openai_chat_completion ./test/e2e/anthropic_sse -count=1
```

预期：PASS。

- [ ] **Step 6: 提交跨协议和 Responses 路径**

```bash
git add internal/application/llmproxy/usecase test/unit/llmproxy_usecase test/e2e/openai_chat_completion test/e2e/anthropic_sse
git commit -m "refactor: decouple cross-protocol llmproxy streams"
```

---

### Task 6: 删除旧依赖并完成全量验证

**Files:**
- Modify/Delete: `internal/application/llmproxy/util/sse.go`、`util/anthropic.go` 中未使用的 Huma response factory
- Modify: `docs/agents/architecture.md`（只在当前架构描述需要同步时修改）
- Test: `test/unit/llmproxy_usecase/architecture_test.go`

**Interfaces:**
- application usecase/port 不再暴露任何 Huma/Fiber/API util 类型。
- handler/API adapter 是唯一的 Huma response 构造边界。

- [ ] **Step 1: 删除旧 factory 并搜索依赖**

```bash
rg -n 'huma|humafiber|internal/api/util|github.com/gofiber/fiber/v3' \
  internal/application/llmproxy/usecase \
  internal/application/llmproxy/port
```

预期：无输出。

- [ ] **Step 2: 运行架构测试**

```bash
go test ./test/unit/llmproxy_usecase -run TestLLMProxyApplicationDoesNotImportHTTPTransport -count=1
```

预期：PASS。

- [ ] **Step 3: 运行完整 Go 测试**

```bash
make test
```

预期：`./cmd/... ./internal/... ./test/...` 全部通过。

- [ ] **Step 4: 运行 lint 和 Web build**

```bash
make lint
make web-build
```

预期：LintConv、LintStatic 和 Next.js production build 全部成功。

- [ ] **Step 5: 检查 diff**

```bash
git diff --check
git diff --stat
git status --short
```

预期：无 whitespace error；只包含 P3-2 相关生产代码和测试；工作区干净。

- [ ] **Step 6: 提交清理结果**

```bash
git add internal/application/llmproxy docs/agents/architecture.md test/unit/llmproxy_usecase
git commit -m "refactor: remove huma dependency from llmproxy application"
```

实现任务必须另开分支执行；本计划文件本身不代表 P3-2 已经开发完成。

---

## 计划自检

- 已覆盖：依赖边界、Open/Read 错误透传、native stream、跨协议 stream、Responses API、adapter、资源关闭、SSE gauge、审计、全量验证。
- 未包含：新的 HTTP 框架、协议 DTO 重写、transport implementation 重写和无第二种 transport 需求支撑的通用 framework。
- 本计划没有要求在 application 层使用 Huma/Fiber、HTTP writer 或 `bufio.Writer`。
- 所有生产改动任务都包含先写测试、运行失败测试、实现、运行通过测试和独立提交步骤。
