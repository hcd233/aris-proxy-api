# 架构修复实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 CodeGraph CR 发现的 4 个结构性问题，使架构符合 Clean Architecture 端口适配器模式，消除反向依赖，拆分过大的文件。

**Architecture:** 将 Infrastructure 层接口定义上移到 Application 层作为端口；将 `internal/util` 中协议相关工具下沉到 `application/llmproxy/util/`，HTTP 响应包装迁移到 `internal/api/util/`；按 API 类型和流式/非流式拆分 converter 和 usecase 文件。

**Tech Stack:** Go 1.25.1, dig DI, Fiber/Huma, GORM, sonic JSON

---

## 文件结构映射

### 新建文件

| 文件路径 | 职责 |
|----------|------|
| `internal/application/llmproxy/usecase/port.go` | 扩展：新增 `AnthropicProxyPort` 和 `OpenAIProxyPort` 接口 |
| `internal/application/llmproxy/util/anthropic.go` | Anthropic 协议工具函数（从 `internal/util/anthropic.go` 迁移） |
| `internal/application/llmproxy/util/openai.go` | OpenAI 协议工具函数（从 `internal/util/openai.go` 迁移） |
| `internal/application/llmproxy/util/sse.go` | SSE 工具函数（从 `internal/util/sse.go` 迁移） |
| `internal/application/llmproxy/util/model.go` | DTO 序列化/替换工具（从 `internal/util/model.go` 迁移） |
| `internal/application/llmproxy/util/openai_stream.go` | OpenAI 流式归一化工具（从 `internal/util/openai_stream.go` 迁移） |
| `internal/api/util/http.go` | HTTP 响应包装工具（从 `internal/util/http.go` 迁移） |
| `internal/application/llmproxy/converter/anthropic_chat.go` | Anthropic↔OpenAI Chat 转换（从 `anthropic.go` 拆分） |
| `internal/application/llmproxy/converter/anthropic_response.go` | Anthropic↔Response 转换（从 `anthropic.go` 拆分） |
| `internal/application/llmproxy/converter/anthropic_sse.go` | Anthropic SSE 转换辅助函数（从 `anthropic.go` 拆分） |
| `internal/application/llmproxy/converter/openai_chat.go` | OpenAI↔Anthropic Chat 转换（从 `openai.go` 拆分） |
| `internal/application/llmproxy/converter/openai_sse.go` | OpenAI SSE 转换辅助函数（从 `openai.go` 拆分） |
| `internal/application/llmproxy/usecase/openai_chat_stream.go` | Chat 流式处理（从 `openai_chat.go` 拆分） |
| `internal/application/llmproxy/usecase/openai_chat_unary.go` | Chat 非流式处理（从 `openai_chat.go` 拆分） |
| `internal/application/llmproxy/usecase/openai_response_stream.go` | Response 流式处理（从 `openai_response.go` 拆分） |
| `internal/application/llmproxy/usecase/openai_response_unary.go` | Response 非流式处理（从 `openai_response.go` 拆分） |
| `internal/application/llmproxy/usecase/anthropic_message_stream.go` | Message 流式处理（从 `anthropic_message.go` 拆分） |
| `internal/application/llmproxy/usecase/anthropic_message_unary.go` | Message 非流式处理（从 `anthropic_message.go` 拆分） |

### 删除文件（迁移后清空）

| 文件路径 | 原因 |
|----------|------|
| `internal/util/anthropic.go` | 迁移至 `application/llmproxy/util/anthropic.go` |
| `internal/util/openai.go` | 迁移至 `application/llmproxy/util/openai.go` |
| `internal/util/sse.go` | 迁移至 `application/llmproxy/util/sse.go` |
| `internal/util/model.go` | 迁移至 `application/llmproxy/util/model.go` |
| `internal/util/openai_stream.go` | 迁移至 `application/llmproxy/util/openai_stream.go` |
| `internal/util/http.go` | 迁移至 `internal/api/util/http.go` |

### 修改文件

| 文件路径 | 变更内容 |
|----------|----------|
| `internal/application/llmproxy/usecase/port.go` | 新增 `AnthropicProxyPort` 和 `OpenAIProxyPort` |
| `internal/infrastructure/transport/anthropic.go` | 删除旧接口，实现新端口，更新导入 |
| `internal/infrastructure/transport/openai.go` | 删除旧接口，实现新端口，更新导入 |
| `internal/application/llmproxy/usecase/anthropic.go` | 使用新端口类型替代 `transport.AnthropicProxy` |
| `internal/application/llmproxy/usecase/openai.go` | 使用新端口类型替代 `transport.OpenAIProxy` |
| `internal/application/llmproxy/usecase/query.go` | 使用新端口类型，更新 util 导入路径 |
| `internal/application/llmproxy/usecase/common.go` | 更新 util 导入路径 |
| `internal/application/llmproxy/usecase/openai_chat.go` | 拆分后保留入口和路由方法，更新导入 |
| `internal/application/llmproxy/usecase/openai_response.go` | 拆分后保留入口和路由方法，更新导入 |
| `internal/application/llmproxy/usecase/anthropic_message.go` | 拆分后保留入口和路由方法，更新导入 |
| `internal/application/llmproxy/converter/anthropic.go` | 拆分后保留类型定义和公共方法 |
| `internal/application/llmproxy/converter/openai.go` | 拆分后保留类型定义和公共方法 |
| `internal/bootstrap/container.go` | 更新 DI 注册的导入和类型引用 |
| `internal/handler/anthropic.go` | 更新 util 导入路径 |
| `internal/handler/openai.go` | 更新 util 导入路径 |
| `internal/handler/ping.go` | 更新 util 导入路径（如有） |
| `internal/middleware/log.go` | 更新 util 导入路径（如有） |
| `internal/middleware/trace.go` | 更新 util 导入路径（如有） |
| `internal/infrastructure/transport/header_passthrough.go` | 更新 util 导入路径（如有） |
| `test/unit/converter/converter_test.go` | 更新导入路径 |
| `test/unit/llmproxy_usecase/*.go` | 更新导入路径 |

---

## Task 1: 端口迁移 — AnthropicProxy / OpenAIProxy 接口上移

**Files:**
- Modify: `internal/application/llmproxy/usecase/port.go`
- Modify: `internal/infrastructure/transport/anthropic.go`
- Modify: `internal/infrastructure/transport/openai.go`
- Modify: `internal/application/llmproxy/usecase/anthropic.go`
- Modify: `internal/application/llmproxy/usecase/openai.go`
- Modify: `internal/application/llmproxy/usecase/query.go`
- Modify: `internal/bootstrap/container.go`

- [ ] **Step 1: 在 port.go 中新增 AnthropicProxyPort 和 OpenAIProxyPort 接口**

在 `internal/application/llmproxy/usecase/port.go` 末尾添加两个接口定义，方法签名与 `transport.AnthropicProxy` 和 `transport.OpenAIProxy` 完全一致：

```go
type AnthropicProxyPort interface {
	ForwardCreateMessage(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (*dto.AnthropicMessage, error)
	ForwardCreateMessageStream(ctx context.Context, ep vo.UpstreamEndpoint, body []byte, onEvent func(dto.AnthropicSSEEvent) error) (*dto.AnthropicMessage, error)
	ForwardCountTokens(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (*dto.AnthropicTokensCount, error)
}

type OpenAIProxyPort interface {
	ForwardChatCompletion(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (*dto.OpenAIChatCompletion, error)
	ForwardChatCompletionStream(ctx context.Context, ep vo.UpstreamEndpoint, body []byte, onChunk func(*dto.OpenAIChatCompletionChunk) error) (*dto.OpenAIChatCompletion, error)
	ForwardCreateResponse(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) ([]byte, error)
	ForwardCreateResponseStream(ctx context.Context, ep vo.UpstreamEndpoint, body []byte, onEvent func(event string, data []byte) error) error
}
```

需要在 port.go 的 import 中添加：
```go
import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)
```

- [ ] **Step 2: 修改 usecase 层引用 — anthropic.go**

将 `internal/application/llmproxy/usecase/anthropic.go` 中所有 `transport.AnthropicProxy` 替换为 `AnthropicProxyPort`，`transport.OpenAIProxy` 替换为 `OpenAIProxyPort`。

具体变更：
- 删除 import `"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"`
- struct 字段类型：`anthropicProxy transport.AnthropicProxy` → `anthropicProxy AnthropicProxyPort`
- struct 字段类型：`openAIProxy transport.OpenAIProxy` → `openAIProxy OpenAIProxyPort`
- 构造函数参数同理

- [ ] **Step 3: 修改 usecase 层引用 — openai.go**

将 `internal/application/llmproxy/usecase/openai.go` 中所有 `transport.OpenAIProxy` 替换为 `OpenAIProxyPort`，`transport.AnthropicProxy` 替换为 `AnthropicProxyPort`。

具体变更：
- 删除 import `"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"`
- struct 字段类型：`openAIProxy transport.OpenAIProxy` → `openAIProxy OpenAIProxyPort`
- struct 字段类型：`anthropicProxy transport.AnthropicProxy` → `anthropicProxy AnthropicProxyPort`
- 构造函数参数同理

- [ ] **Step 4: 修改 usecase 层引用 — query.go**

将 `internal/application/llmproxy/usecase/query.go` 中 `transport.AnthropicProxy` 替换为 `AnthropicProxyPort`。

具体变更：
- 删除 import `"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"`
- countTokens struct 字段：`proxy transport.AnthropicProxy` → `proxy AnthropicProxyPort`
- `NewCountTokens` 参数同理

- [ ] **Step 5: 在 transport 实现层添加编译期接口断言**

在 `internal/infrastructure/transport/anthropic.go` 中添加：
```go
var _ usecase.AnthropicProxyPort = (*anthropicProxy)(nil)
```

在 `internal/infrastructure/transport/openai.go` 中添加：
```go
var _ usecase.OpenAIProxyPort = (*openAIProxy)(nil)
```

注意：transport 文件需要新增 import `"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"`。保留 transport 包中的旧接口定义（`type AnthropicProxy interface` 和 `type OpenAIProxy interface`），但将其标记为 deprecated 或直接删除（因为 usecase 不再引用它们）。如果 transport 包内部无其他消费者，删除旧接口。

- [ ] **Step 6: 修改 container.go DI 注册**

`internal/bootstrap/container.go` 中 `provideInfrastructure` 的 `transport.NewOpenAIProxy` 和 `transport.NewAnthropicProxy` 注册无需变更（dig 按返回类型解析，返回类型仍为具体结构体指针，会自动满足 `usecase.AnthropicProxyPort` 和 `usecase.OpenAIProxyPort` 接口）。

但需确认 `NewOpenAIProxy` 和 `NewAnthropicProxy` 的返回类型。如果返回的是接口类型（`transport.OpenAIProxy`），需改为返回具体类型指针（`*openAIProxy`）或新端口类型（`usecase.OpenAIProxyPort`）。

检查 `transport.NewOpenAIProxy` 签名：
- 如果是 `func NewOpenAIProxy() OpenAIProxy` → 改为 `func NewOpenAIProxy() usecase.OpenAIProxyPort`
- 如果是 `func NewOpenAIProxy() *openAIProxy` → 无需变更

- [ ] **Step 7: 编译验证**

Run: `go build ./...`
Expected: 编译成功，零错误

- [ ] **Step 8: Lint 验证**

Run: `go run main.go lint conv`
Expected: `All convention checks passed!`

- [ ] **Step 9: 测试验证**

Run: `go test -count=1 ./...`
Expected: 全部通过

- [ ] **Step 10: Commit**

```bash
git add -A
git commit -m "refactor: migrate AnthropicProxy/OpenAIProxy interfaces to application layer ports"
```

---

## Task 2: Util 包拆分 — 协议工具迁移到 application/llmproxy/util/

**Files:**
- Create: `internal/application/llmproxy/util/` 目录
- Create: `internal/application/llmproxy/util/anthropic.go`
- Create: `internal/application/llmproxy/util/openai.go`
- Create: `internal/application/llmproxy/util/sse.go`
- Create: `internal/application/llmproxy/util/model.go`
- Create: `internal/application/llmproxy/util/openai_stream.go`
- Delete: `internal/util/anthropic.go`
- Delete: `internal/util/openai.go`
- Delete: `internal/util/sse.go`
- Delete: `internal/util/model.go`
- Delete: `internal/util/openai_stream.go`
- Modify: 所有引用上述文件的文件

- [ ] **Step 1: 创建 application/llmproxy/util/ 目录并迁移文件**

将以下文件从 `internal/util/` 迁移到 `internal/application/llmproxy/util/`，包名从 `util` 改为 `proxyutil`（避免与标准库 util 和 common/util 冲突）：

| 源文件 | 目标文件 |
|--------|----------|
| `internal/util/anthropic.go` | `internal/application/llmproxy/util/anthropic.go` |
| `internal/util/openai.go` | `internal/application/llmproxy/util/openai.go` |
| `internal/util/sse.go` | `internal/application/llmproxy/util/sse.go` |
| `internal/util/model.go` | `internal/application/llmproxy/util/model.go` |
| `internal/util/openai_stream.go` | `internal/application/llmproxy/util/openai_stream.go` |

每个文件的变更：
- `package util` → `package proxyutil`
- 所有导出函数保持原名称不变（`SendAnthropicModelNotFoundError` 等）

- [ ] **Step 2: 更新 usecase 层的导入路径**

所有 `internal/application/llmproxy/usecase/` 下的文件：
- `"github.com/hcd233/aris-proxy-api/internal/util"` → `"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"`
- 调用处 `util.Xxx` → `proxyutil.Xxx`（因为包名从 `util` 改为 `proxyutil`）

涉及的 usecase 文件：
- `anthropic.go`
- `openai.go`
- `anthropic_message.go`
- `openai_chat.go`
- `openai_response.go`
- `query.go`
- `common.go`

- [ ] **Step 3: 更新 transport 层的导入路径**

`internal/infrastructure/transport/anthropic.go` 和 `openai.go`：
- `"github.com/hcd233/aris-proxy-api/internal/util"` → `"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"`
- `util.Xxx` → `proxyutil.Xxx`

- [ ] **Step 4: 更新 handler 层的导入路径**

检查 `internal/handler/` 下所有引用 `internal/util` 中已迁移函数的文件，更新导入路径。

- [ ] **Step 5: 更新其他引用文件**

用 `grep -r '"github.com/hcd233/aris-proxy-api/internal/util"' internal/` 搜索所有剩余引用，逐文件更新。注意区分：
- 引用**已迁移函数**的文件 → 更新为 `proxyutil` 导入
- 引用**未迁移函数**（context.go、string.go、user.go、header_log.go）的文件 → 保持 `internal/util` 导入

- [ ] **Step 6: 删除原文件**

确认无引用后删除：
- `internal/util/anthropic.go`
- `internal/util/openai.go`
- `internal/util/sse.go`
- `internal/util/model.go`
- `internal/util/openai_stream.go`

- [ ] **Step 7: 编译验证**

Run: `go build ./...`
Expected: 编译成功

- [ ] **Step 8: Lint 验证**

Run: `go run main.go lint conv`
Expected: `All convention checks passed!`

- [ ] **Step 9: 测试验证**

Run: `go test -count=1 ./...`
Expected: 全部通过

- [ ] **Step 10: Commit**

```bash
git add -A
git commit -m "refactor: move protocol utils from internal/util to application/llmproxy/util"
```

---

## Task 3: Util 包拆分 — HTTP 响应包装迁移到 api/util/

**Files:**
- Create: `internal/api/util/http.go`
- Delete: `internal/util/http.go`
- Modify: 所有引用 `internal/util` 中 HTTP 响应函数的文件

- [ ] **Step 1: 创建 api/util/ 目录并迁移 http.go**

将 `internal/util/http.go` 迁移到 `internal/api/util/http.go`，包名从 `util` 改为 `apiutil`。

函数列表（全部迁移）：
- `WrapHTTPResponse`
- `WriteErrorResponse`
- `WriteErrorHTTPResponse`
- `WrapStreamResponse`
- `JSONResponseWriter`（类型及其方法）
- `WrapJSONResponse`
- `WriteUpstreamError`
- `ExtractUpstreamStatusAndError`

- [ ] **Step 2: 更新 handler 层的导入**

所有 `internal/handler/` 下引用这些函数的文件：
- 新增 `"github.com/hcd233/aris-proxy-api/internal/api/util"`
- `util.WrapHTTPResponse` → `apiutil.WrapHTTPResponse`
- `util.WrapStreamResponse` → `apiutil.WrapStreamResponse`
- 其他同理

- [ ] **Step 3: 更新其他引用文件**

搜索所有引用 `internal/util` 中 HTTP 函数的文件并更新。

- [ ] **Step 4: 删除原文件**

删除 `internal/util/http.go`。

- [ ] **Step 5: 编译验证**

Run: `go build ./...`
Expected: 编译成功

- [ ] **Step 6: Lint 验证**

Run: `go run main.go lint conv`
Expected: `All convention checks passed!`

- [ ] **Step 7: 测试验证**

Run: `go test -count=1 ./...`
Expected: 全部通过

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "refactor: move HTTP response utils from internal/util to internal/api/util"
```

---

## Task 4: Converter 文件拆分

**Files:**
- Modify: `internal/application/llmproxy/converter/anthropic.go` → 拆分为 3 个文件
- Modify: `internal/application/llmproxy/converter/openai.go` → 拆分为 2 个文件
- `internal/application/llmproxy/converter/response.go` 保持不变

- [ ] **Step 1: 拆分 anthropic.go**

将 `internal/application/llmproxy/converter/anthropic.go`（~1009 行）拆分为：

**`converter/anthropic.go`** — 保留类型定义和 Chat 相关的公开方法：
- `AnthropicProtocolConverter` 结构体定义
- `FromOpenAIRequest` 方法
- `ToAnthropicResponse` 方法
- `SSEContentBlockTracker` 类型及相关方法
- `ToAnthropicSSEResponse` 方法（SSE 核心入口）
- 包级 doc comment

**`converter/anthropic_response.go`** — Response API 相关转换：
- `FromResponseAPIRequest` 方法
- 所有 `convertResponse*` 开头的私有函数

**`converter/anthropic_sse.go`** — SSE 转换辅助函数：
- `newContentBlockStartEvent`
- `newTextDeltaEvent`
- `newThinkingDeltaEvent`
- `newInputJSONDeltaEvent`
- `convertChunkUsageToAnthropic`
- `convertOpenAIFinishReasonToAnthropic`

所有文件保持 `package converter`，无需修改导入路径。

- [ ] **Step 2: 拆分 openai.go**

将 `internal/application/llmproxy/converter/openai.go`（~663 行）拆分为：

**`converter/openai.go`** — 保留类型定义和 Chat 相关的公开方法：
- `OpenAIProtocolConverter` 结构体定义
- `FromAnthropicRequest` 方法
- `ToOpenAISSEResponse` 方法（SSE 核心入口）
- `ToOpenAIResponse` 方法
- 包级 doc comment

**`converter/openai_sse.go`** — SSE 转换辅助函数：
- 所有 `convertAnthropic*` 开头的私有函数
- 所有 `convertContentBlock*` 开头的私有函数
- 其他私有 helper 函数

- [ ] **Step 3: 编译验证**

Run: `go build ./...`
Expected: 编译成功（同包内拆分，无导入变更）

- [ ] **Step 4: Lint 验证**

Run: `go run main.go lint conv`
Expected: `All convention checks passed!`

- [ ] **Step 5: 测试验证**

Run: `go test -count=1 ./...`
Expected: 全部通过

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor: split converter files by API type for maintainability"
```

---

## Task 5: Usecase 文件拆分 — 按流式/非流式分离

**Files:**
- Modify: `internal/application/llmproxy/usecase/openai_chat.go` → 拆分为 3 个文件
- Modify: `internal/application/llmproxy/usecase/openai_response.go` → 拆分为 3 个文件
- Modify: `internal/application/llmproxy/usecase/anthropic_message.go` → 拆分为 3 个文件

- [ ] **Step 1: 拆分 openai_chat.go**

将 `internal/application/llmproxy/usecase/openai_chat.go` 拆分为：

**`usecase/openai_chat.go`** — 保留入口路由方法：
- `forwardChatNative` 方法（路由：根据 stream 参数分发）
- `forwardChatViaAnthropic` 方法（路由：根据 stream 参数分发）

**`usecase/openai_chat_stream.go`** — 流式处理：
- `forwardChatNativeStream` 方法
- `forwardChatViaAnthropicStream` 方法

**`usecase/openai_chat_unary.go`** — 非流式处理：
- `forwardChatNativeUnary` 方法
- `forwardChatViaAnthropicUnary` 方法

- [ ] **Step 2: 拆分 openai_response.go**

将 `internal/application/llmproxy/usecase/openai_response.go` 拆分为：

**`usecase/openai_response.go`** — 保留入口路由方法：
- `forwardResponseNative` 方法
- `forwardResponseViaChat` 方法
- `forwardResponseViaAnthropic` 方法

**`usecase/openai_response_stream.go`** — 流式处理：
- `forwardResponseNativeStream` 方法
- `forwardResponseViaChatStream` 方法
- `forwardResponseViaAnthropicStream` 方法

**`usecase/openai_response_unary.go`** — 非流式处理：
- `forwardResponseNativeUnary` 方法
- `forwardResponseViaChatUnary` 方法
- `forwardResponseViaAnthropicUnary` 方法

- [ ] **Step 3: 拆分 anthropic_message.go**

将 `internal/application/llmproxy/usecase/anthropic_message.go` 拆分为：

**`usecase/anthropic_message.go`** — 保留入口路由方法：
- `forwardMessageNative` 方法
- `forwardMessageViaChat` 方法

**`usecase/anthropic_message_stream.go`** — 流式处理：
- `forwardMessageNativeStream` 方法
- `forwardMessageViaChatStream` 方法

**`usecase/anthropic_message_unary.go`** — 非流式处理：
- `forwardMessageNativeUnary` 方法
- `forwardMessageViaChatUnary` 方法

- [ ] **Step 4: 编译验证**

Run: `go build ./...`
Expected: 编译成功（同包内拆分，无导入变更）

- [ ] **Step 5: Lint 验证**

Run: `go run main.go lint conv`
Expected: `All convention checks passed!`

- [ ] **Step 6: 测试验证**

Run: `go test -count=1 ./...`
Expected: 全部通过

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "refactor: split usecase files by stream/unary for maintainability"
```

---

## Task 6: 最终验证与清理

**Files:**
- Possibly delete: `internal/util/` 中剩余无引用的文件
- Verify: 全量编译 + lint + 测试

- [ ] **Step 1: 检查 internal/util 剩余文件**

`internal/util/` 应剩余以下文件（通用工具，不依赖 dto）：
- `context.go` — context 工具
- `string.go` — 字符串工具
- `user.go` — 用户工具
- `header_log.go` — HTTP 头日志脱敏

确认这些文件无 `internal/dto` 导入。如有，需处理。

- [ ] **Step 2: 更新 lintconv 规则（如需要）**

如果 `internal/util` 中不再有任何文件导入 `internal/dto`，则 lintconv 规则无需变更。

如果 `internal/util` 中仍有文件导入 `internal/dto`，需要：
- 评估是否可以进一步迁移
- 或者在 `rules_architecture.go` 中添加新规则检测 `internal/util` → `internal/dto` 的反向依赖

- [ ] **Step 3: 全量编译**

Run: `go build ./...`
Expected: 编译成功

- [ ] **Step 4: 全量 Lint**

Run: `go run main.go lint conv`
Expected: `All convention checks passed!`

- [ ] **Step 5: 全量测试**

Run: `go test -count=1 ./...`
Expected: 全部通过

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor: final cleanup after architecture refactoring"
```

---

## 执行顺序与依赖关系

```
Task 1 (端口迁移)
  ↓
Task 2 (协议工具迁移)
  ↓
Task 3 (HTTP 工具迁移)
  ↓  （可并行，但建议串行以减少冲突）
Task 4 (Converter 拆分)
  ↓  （同包拆分，无跨包影响）
Task 5 (Usecase 拆分)
  ↓  （同包拆分，无跨包影响）
Task 6 (最终验证)
```

Task 1-3 涉及跨包迁移，必须串行执行。Task 4-5 是同包拆分，互不影响，理论上可并行。
