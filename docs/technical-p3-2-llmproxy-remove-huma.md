# P3-2 LLM Proxy 去 Huma 技术文档

## 文档状态

- 状态：设计完成，暂不开发
- 范围：OpenAI Chat Completions、OpenAI Responses、Anthropic Messages 的 application/API 分层
- 关联问题：P3-2
- 设计文档：`docs/superpowers/specs/2026-07-23-llmproxy-remove-huma-design.md`

## 1. 当前调用链

```text
Fiber middleware
  -> Huma route
    -> internal/handler/{openai,anthropic}.go
      -> internal/application/llmproxy/port
        -> internal/application/llmproxy/usecase
          -> internal/application/llmproxy/converter
          -> internal/application/llmproxy/util
          -> internal/application/llmproxy/port transport
            -> internal/infrastructure/transport
```

当前 handler 的 Huma 依赖是合理的，因为它是 HTTP 入口；问题在于 usecase port 也返回 `*huma.StreamResponse`，导致 application 需要理解 Huma response callback。`internal/api/util` 同样处于 HTTP/API 层，却被 usecase 直接调用。

## 2. 当前代码责任矩阵

| 能力 | 当前主要位置 | 目标位置 |
| --- | --- | --- |
| 模型/endpoint 查找 | usecase | 保持 usecase |
| 请求 DTO 转换 | converter/usecase | 保持 application |
| 上游 Open/Read | infrastructure transport port | 保持现有两阶段接口 |
| SSE 协议事件转换 | usecase/converter/util | 保持 application，但移除 HTTP writer 类型 |
| HTTP status/header | api util/Huma response | handler adapter |
| JSON 错误 envelope | util + Huma response | application 生成协议数据，adapter 写 HTTP |
| SSE gauge | handler + stream lifecycle | handler adapter 的真实流生命周期 |
| Huma/Fiber context | handler/API | 仅保留在 handler/API |

## 3. 目标接口草案

以下接口用于技术讨论，不是本次提交的可直接实现 API。最终接口必须在阶段一的行为测试后确定。

### 3.1 应用结果

```go
type JSONResult struct {
    Status  int
    Headers map[string]string
    Body    []byte
}

type StreamResult struct {
    Status  int
    Headers map[string]string
    Open    func(context.Context) (Stream, error)
}

type Stream interface {
    Read(context.Context, EventSink) error
    Close() error
}
```

上述草案中的 `Status` 只表示 application 建议的协议状态；HTTP adapter 决定是否在 Huma response 开始前采用它。`Stream` 不得暴露 Huma/Fiber writer，`EventSink` 应表达协议事件而不是字节 writer。

另一种更保守的实现是让 usecase 返回：

```go
type ProxyOutcome struct {
    JSON  *ProtocolJSON
    Stream *ProtocolStream
    Err   *ProxyError
}
```

两者只能择一，不能同时引入。选择标准是避免产生一个无法约束的“万能 response”类型。

### 3.2 错误

```go
type ProxyError struct {
    StatusCode int
    Headers    map[string]string
    Body       []byte
    Cause      error
}
```

`ProxyError` 仅表达 application/API adapter 之间的协议错误，不提供 `Write`、`SendStreamWriter`、`BodyWriter` 等方法。现有 `model.UpstreamError` 可作为上游错误来源，adapter 负责把它映射为 OpenAI 或 Anthropic 客户端格式。

## 4. Open/Read 生命周期

### 打开阶段

```text
handler adapter 调用 usecase
  -> usecase 调用 transport.Open*Stream
     -> 成功：返回未消费的 StreamResult
     -> 失败：返回 ProxyError / UpstreamError
  -> adapter 在 Huma response 尚未开始前写 status、headers、JSON body
```

关键点：不能先返回 200 SSE response，再在 body callback 内才发现上游 429。否则客户端会收到错误的 HTTP 状态码。现有 Open/Read 拆分正是为了保留这一能力。

### 消费阶段

```text
Huma adapter 开始真实写流
  -> 设置 SSE headers/status
  -> 增加 SSE gauge
  -> application Read stream
  -> EventSink 转换协议事件
  -> flush
  -> 正常结束或客户端取消
  -> 关闭 stream、减少 gauge、写审计结果
```

stream 资源所有权必须明确：成功 `Open` 后，最终由 `Read` 或 adapter 的统一 defer 关闭；不得同时由两个层级重复关闭，也不得由任何错误分支遗漏关闭。

## 5. 协议错误转换

### OpenAI

需要继续输出现有 OpenAI error envelope：

```json
{
  "error": {
    "message": "...",
    "type": "...",
    "code": "..."
  }
}
```

模型不存在、内容拦截、内部错误使用当前 status/type/code 常量，不在 P3-2 中重新命名。

### Anthropic

需要继续输出当前 Anthropic envelope：

```json
{
  "type": "error",
  "error": {
    "type": "...",
    "message": "..."
  }
}
```

adapter 根据入口协议选择 envelope。跨协议转发的上游错误不能直接把另一协议的错误 JSON 原样返回，除非现有行为已经明确要求透传；这项行为必须由回归测试锁定。

## 6. 迁移顺序与提交边界

建议每个阶段形成可独立审查的提交：

1. **行为锁定**：增加 application 架构依赖检查和 Open 阶段错误测试，不改生产代码。
2. **结果/错误类型**：只增加 transport-neutral 类型及单元测试。
3. **OpenAI Chat native**：迁移一条完整 native stream，handler adapter 接管 Huma response。
4. **Anthropic Messages native**：迁移一条完整 native stream。
5. **跨协议 Chat**：迁移 OpenAI→Anthropic 与 Anthropic→OpenAI。
6. **Responses API**：迁移 native 和跨协议两条路径。
7. **清理依赖**：删除旧 Huma response factory，执行静态依赖检查。

每个阶段都必须运行对应 package 测试；不允许在多个阶段之间保留同一方法的双返回类型或通过类型别名隐藏 Huma 依赖。

## 7. 测试矩阵

| 场景 | 断言 |
| --- | --- |
| usecase 依赖扫描 | usecase/port 无 Huma、Fiber、api util import |
| native Open 成功 | adapter 返回 SSE，headers 正确，事件完整 |
| Open 返回 429 | HTTP 429，保留上游 headers/body，不启动 SSE |
| Open 返回 401/5xx | 对应 status 和协议错误 envelope |
| model not found | OpenAI/Anthropic 各自 envelope 与 status |
| content blocked | 403 和协议错误 envelope |
| stream read error | 已启动后按既有 SSE 错误/终止语义结束 |
| client cancellation | transport stream 关闭，gauge 恢复，审计完成 |
| cross-protocol | 事件顺序、finish/stop 事件和 usage 保持不变 |
| Responses API | lifecycle 事件顺序保持不变 |
| header passthrough | 允许透传的 header 保持，禁止内部 header 外泄 |

测试实现遵循项目约束：使用标准库 `testing`，文件放在 `test/unit/<topic>/` 或 `test/e2e/<topic>/`，不在 application 包中引入 Fiber/Huma server 作为测试前置条件。

## 8. 完成前检查

实现任务完成前必须运行：

```bash
rg -n 'huma|humafiber|internal/api/util|github.com/gofiber/fiber/v3' \
  internal/application/llmproxy/usecase \
  internal/application/llmproxy/port

go test -count=1 ./cmd/... ./internal/... ./test/...
make lint
make web-build
git diff --check
```

第一条命令应无输出；其余命令必须成功。若设计演进后需要保留某个 application 级协议包，必须在设计评审中说明它只处理业务协议数据，不提供 HTTP writer 或框架 response。

## 9. 暂不开发的原因

P3-2 涉及五条流式路径、两种协议、跨协议转换、上游错误透传和生命周期指标。当前本次提交只记录技术边界和迁移顺序，不改变这些运行时行为。后续开发必须另开实现任务，从阶段一开始，并在每个流式路径迁移后单独验证。
