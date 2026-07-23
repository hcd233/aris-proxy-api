# P3-2 LLM Proxy Application 层去 Huma 依赖设计

## 1. 背景与目标

当前 `internal/application/llmproxy/usecase` 直接返回 `*huma.StreamResponse`，并通过 `internal/api/util` 构造 HTTP/SSE 响应；部分响应路径还依赖 Fiber/Huma adapter。这样使 application 层绑定了 HTTP 实现，违反项目中 handler → application → domain/infrastructure 的分层方向。

本设计只定义后续重构边界，不在本次变更中实现。目标是：

1. `internal/application/llmproxy/usecase` 和 `internal/application/llmproxy/port` 不再依赖 Huma、Fiber 或 `internal/api/util`。
2. 保留 OpenAI Chat Completions、OpenAI Responses、Anthropic Messages 的普通响应、SSE 流式响应、跨协议转换和上游错误透传行为。
3. 将 HTTP status、header、JSON 错误体、SSE framing 等 transport 责任收敛到 handler/API adapter。
4. 通过分阶段迁移降低流式链路回归风险。

## 2. 非目标

- 本次不修改 `internal/infrastructure/transport` 的上游 HTTP/SSE 协议实现。
- 不重新设计 OpenAI 或 Anthropic DTO。
- 不引入第二种传输协议；transport-neutral 设计只保留必要的扩展能力。
- 不改变错误码、错误 JSON 结构、SSE event/data 格式和 header passthrough 语义。
- 不在 application 层建立新的 HTTP 框架抽象。

## 3. 当前问题与影响范围

### 3.1 入口

当前 HTTP 入口为：

```text
Huma route
  -> internal/handler/openai.go
       -> port.OpenAIUseCase
          -> usecase.CreateChatCompletion/CreateResponse
  -> internal/handler/anthropic.go
       -> port.AnthropicUseCase
          -> usecase.CreateMessage
```

handler 当前同时负责少量 SSE 生命周期指标，但 usecase 返回 Huma response，使真正的 HTTP response 构造责任落入 application 层。

### 3.2 直接依赖位置

需纳入后续迁移的代码包括：

- `internal/application/llmproxy/port/handler.go`：port 方法返回 `*huma.StreamResponse`。
- `internal/application/llmproxy/usecase/common.go`：构造上游错误的 Huma response。
- `internal/application/llmproxy/usecase/openai.go`、`openai_chat.go`、`openai_response.go`：OpenAI 普通、流式及跨协议路径。
- `internal/application/llmproxy/usecase/anthropic.go`、`anthropic_message.go`：Anthropic 普通、流式及跨协议路径。
- `internal/application/llmproxy/usecase/recorder.go`：流生命周期记录与 response callback 相关编排。
- `internal/application/llmproxy/util/sse.go`、`util/anthropic.go`：模型不存在、内容拦截、内部错误和 SSE 错误响应构造。
- `internal/handler/openai.go`、`internal/handler/anthropic.go`：将 application 结果适配为 Huma response。

`internal/application/llmproxy/util` 中的协议转换、SSE 编码和 chunk 处理逻辑不应因为本任务整体搬迁；仅迁移其中与 HTTP response 构造直接相关的部分。

## 4. 设计原则

### 4.1 application 返回业务结果，不返回 HTTP response

application 只描述：

- 请求应进入哪条代理路径；
- 上游是否成功打开；
- 流如何消费和生成协议事件；
- 业务错误的类型、状态语义和协议错误信息。

application 不描述：

- Huma response 类型；
- Fiber context；
- HTTP writer；
- HTTP header 写入；
- SSE flush 和 HTTP body writer。

### 4.2 保持 Open/Read 两阶段

transport port 已经将流式调用拆成 Open/Read 两阶段。该边界必须保留：

- `Open*Stream` 只负责建立上游流并暴露打开阶段错误；
- `Read*Stream` 消费已打开流并负责关闭资源；
- 打开阶段的上游 429、401、5xx 等错误必须在 HTTP response 尚未开始写入时被 handler adapter 转换为对应状态码。

### 4.3 不以“自定义 Huma 替代品”为目标

不能简单将 `*huma.StreamResponse` 替换为一个含有 `Body func(*bufio.Writer)` 的自定义类型，再由 application 继续写 SSE。这样的实现只会把 Huma 依赖变成自定义 transport 依赖，无法达到分层目标。

## 5. 推荐架构

### 5.1 结果模型

建议在 `internal/application/llmproxy/port` 或一个只属于 application 的 focused package 中定义有限的结果模型：

```go
type ProxyResult struct {
    Kind       ResultKind
    Headers    map[string]string
    JSONBody   []byte
    Stream     StreamPlan
}

type ResultKind uint8

const (
    ResultJSON ResultKind = iota
    ResultSSE
)

type StreamPlan struct {
    Open func(context.Context) (StreamReader, error)
}
```

实际实现时不应直接采用以上草案而跳过验证；需要先按现有三类协议分别梳理共享字段。核心约束是：

- `JSONBody` 是协议层已经确定的 JSON 字节，不暴露 Huma；
- `Headers` 只表达允许透传的上游/协议 header，不携带 HTTP writer；
- `StreamPlan` 以 application 定义的事件/reader 接口表达流内容，不能接收 Fiber/Huma context；
- 状态码和错误使用 application error/result 字段表达，由 adapter 最终映射到 HTTP。

若一次性引入 `ProxyResult` 会导致多个不相关路径同时改动，应优先分阶段使用三个明确的结果类型：`JSONResult`、`SSEStreamResult`、`ProxyError`，避免过度通用的 union。

### 5.2 错误模型

打开阶段错误应继续携带：

```text
status code
response headers
upstream response body
cause / internal error
```

建议复用现有 `model.UpstreamError` 作为 application 到 adapter 的错误载体，不在 application 中把它转换成 HTTP response。adapter 负责：

- OpenAI 错误 JSON：保留 OpenAI `error` 结构；
- Anthropic 错误 JSON：保留 Anthropic `type` 和 `error` 结构；
- 上游 body 可直接透传时，保留当前 header/body 语义；
- 没有可透传 body 时，使用现有协议内部错误 fallback；
- 非上游错误按当前状态映射规则处理，不改变客户端可见结果。

### 5.3 adapter 边界

handler/API adapter 负责：

1. 调用 usecase；
2. 将 application result 的 header/status/body 映射成 Huma response；
3. 在 Huma response body callback 中启动并消费流；
4. 在真实开始写流时维护 SSE gauge；
5. 处理 Huma/Fiber 特有的 writer、flush 和生命周期。

application 负责：

1. endpoint/model 查找和权限语义；
2. request 转换；
3. upstream Open/Read；
4. 跨协议事件转换；
5. 审计和 usage 记录；
6. 返回 transport-neutral 错误或流计划。

## 6. 分阶段迁移

### 阶段一：锁定行为与架构约束

先增加标准库 `testing` 测试：

- 静态检查 usecase 和 application port 不导入 Huma、Fiber、`internal/api/util`；
- Open 阶段 429/401/5xx 的 status、headers、body 透传；
- OpenAI native stream、Anthropic native stream、两个跨协议 stream 的关键事件序列；
- stream 未启动时错误仍为普通 JSON response；
- stream 启动后错误仍按原协议发送 SSE 错误/终止事件。

### 阶段二：先迁移 port 与 handler 接口

将 `port.OpenAIUseCase` 和 `port.AnthropicUseCase` 的流式方法返回值改成 application 结果类型。此阶段只允许编译错误驱动迁移，不改变 transport 行为。

handler 内增加 focused adapter，将结果转换为 `*huma.StreamResponse`。adapter 先覆盖一个协议的一条 native stream 路径，验证接口方向后再扩展。

### 阶段三：迁移错误 response factory

把 `util/sse.go` 和 `util/anthropic.go` 中的 Huma response factory 拆成：

- application：构造协议错误数据或 `ProxyError`；
- adapter：将协议错误数据写入 Huma JSON response。

协议错误常量和 JSON schema 不迁移，避免同时改变客户端契约。

### 阶段四：按风险迁移流式路径

推荐顺序：

1. OpenAI native Chat Completions；
2. Anthropic native Messages；
3. OpenAI ↔ Anthropic Chat Completions 跨协议；
4. OpenAI Responses native；
5. OpenAI Responses 跨协议。

每条路径独立完成测试、运行聚焦测试并提交，避免五条流式链路同时处于半迁移状态。

### 阶段五：删除旧依赖

确认以下命令无输出后，删除旧 response factory 和无调用的 adapter：

```bash
rg -n 'huma|humafiber|internal/api/util|github.com/gofiber/fiber/v3' \\
  internal/application/llmproxy/usecase \\
  internal/application/llmproxy/port
```

允许 `internal/application/llmproxy/util` 保留协议编码所需的 `bufio.Writer`，但不得保留 Huma/Fiber 类型或 HTTP context。

## 7. 兼容性要求

必须保持：

- 成功 JSON response 的状态码、Content-Type 和 body schema；
- SSE 的 Content-Type、Cache-Control、event/data framing；
- 上游 response header passthrough；
- 上游打开阶段错误的 HTTP status 和错误 body；
- 客户端取消请求时的 stream 关闭和审计记录；
- SSE gauge 只在真实流生命周期内递增/递减；
- OpenAI Responses lifecycle 事件顺序；
- Anthropic `message_start`、content block、`message_delta`、`message_stop` 等事件顺序。

## 8. 验收标准

P3-2 实现完成时应满足：

1. `go list -deps` 或静态检查证明 usecase/port 不依赖 Huma、Fiber、`internal/api/util`；
2. application package 可在不创建 Fiber/Huma server 的测试中测试 Open/Read 和协议转换；
3. handler adapter 测试覆盖 JSON 错误、stream response、header passthrough；
4. 所有既有 LLM proxy unit/E2E 测试通过；
5. `go test -count=1 ./cmd/... ./internal/... ./test/...`、`make lint`、Web build 通过；
6. diff 中没有新增“通用 response framework”或第二套 HTTP abstraction；
7. 文档明确标注本设计不包含实现，实际实现必须另开任务并遵循阶段顺序。

## 9. 风险控制

- 每次只迁移一条代理路径；
- 保留 Open/Read 两阶段，避免把打开阶段错误延迟到 SSE body；
- 先锁定行为测试，再修改返回类型；
- 不在 application 引入 `bufio.Writer`、Huma context 或 Fiber context；
- 任何无法在 adapter 中表达的字段，先回到结果模型设计，不通过偷偷暴露 HTTP writer 解决；
- 若没有第二种 transport 的实际需求，允许评审决定只完成依赖边界和错误模型拆分，而不引入过度通用的 stream abstraction。
