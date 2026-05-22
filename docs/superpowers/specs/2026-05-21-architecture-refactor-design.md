# aris-proxy-api 架构修复设计文档

## 1. 概述

根据 CodeGraph 调用图谱分析，本项目（Go 后端 LLM 代理网关）存在 4 个结构性问题，本设计文档规划全面修复方案（方案 C）。

## 2. 当前架构现状

### 2.1 分层结构
```
┌─────────────────────────────────────────┐
│  Router / Handler / Middleware           │  ← 接口层
├─────────────────────────────────────────┤
│  Application (usecase / command / query) │  ← 应用编排层
│  Converter                              │
├─────────────────────────────────────────┤
│  Domain (aggregate / vo / service)       │  ← 领域层
│  Repository Interface                    │
├─────────────────────────────────────────┤
│  Infrastructure                          │  ← 基础设施层
│  - Repository Impl (GORM)
│  - Transport (HTTP Proxy)
│  - DAO / Database Model
│  - Cache / Storage / JWT
├─────────────────────────────────────────┤
│  Bootstrap / API / Config                │  ← 启动层
└─────────────────────────────────────────┘
```

### 2.2 当前 Lint 状态
```bash
$ go run main.go lint conv
[LintConv] All convention checks passed!
```
当前代码零违规。

## 3. 发现的问题

### 3.1 问题 1：Application 层直接依赖 Infrastructure 层接口定义（🔴 高）

**描述：**
`AnthropicProxy` 和 `OpenAIProxy` 接口定义在 `internal/infrastructure/transport/` 中，Application 层的 usecase 直接使用这些接口。

```go
// internal/application/llmproxy/usecase/anthropic.go
type anthropicUseCase struct {
    anthropicProxy transport.AnthropicProxy  // ← 定义在 infrastructure/transport
    openAIProxy    transport.OpenAIProxy     // ← 定义在 infrastructure/transport
}
```

**违反：** Clean Architecture 端口适配器模式。Application 层应定义自己的端口（接口），Infrastructure 层实现。

**影响：** 更换代理实现时需修改 application 层代码。

### 3.2 问题 2：`internal/util` 成为大型工具垃圾堆（🟡 中）

**描述：**
`internal/util` 包包含 10 个文件、1285 行代码，涵盖：
- 协议转换逻辑（`anthropic.go` 253行、`openai.go` 282行、`sse.go` 148行）
- HTTP 响应包装（`http.go` 191行）
- Context 工具（`context.go` 114行）
- 日志头脱敏（`header_log.go` 49行）
- 用户工具（`user.go` 70行）等

**违反：** AGENTS.md 规定"业务包禁止建 `common.go` 工具堆场"，`internal/util` 本身已沦为堆场。

**影响：** 业务边界模糊，可维护性下降。

### 3.3 问题 3：Converter/Usecase 文件规模过大（🟡 中）

**描述：**
单文件承担过多职责：

| 文件 | 行数 | Symbols | 职责 |
|------|------|---------|------|
| `converter/anthropic.go` | ~700 | 45 | Anthropic 全部协议转换 |
| `converter/openai.go` | ~660 | 34 | OpenAI 全部协议转换 |
| `converter/response.go` | ~300 | 28 | Response API 专用转换 |
| `usecase/openai_response.go` | ~300+ | 30 | Response API 全部处理 |
| `usecase/openai_chat.go` | ~300+ | 23 | Chat 全部处理 |
| `usecase/anthropic_message.go` | ~300+ | 22 | Message 全部处理 |

**违反：** 单一职责原则，单文件同时处理协议转换+流式+非流式+审计上报+错误处理。

**影响：** 文件持续膨胀，认知负荷递增。

### 3.4 问题 4：`internal/util` 反向依赖 `internal/dto`（🟢 低）

**描述：**
`internal/util/http.go` 导入了 `internal/dto`：

```go
// internal/util/http.go
import "github.com/hcd233/aris-proxy-api/internal/dto"

func WriteErrorResponse(bodyWriter io.Writer, err *model.Error) error {
    _, writeErr := bodyWriter.Write(lo.Must1(sonic.Marshal(&dto.CommonRsp{Error: err})))
    return writeErr
}
```

**违反：** 底层通用包不应依赖上层 DTO 包。

**影响：** 虽未形成循环依赖，但依赖方向不合理。

## 4. 修复方案（方案 C：全面重构）

### 4.1 修复问题 1：端口迁移（高优先级）

**目标：** 将 `AnthropicProxy` / `OpenAIProxy` 接口从 Infrastructure 迁移到 Application 层。

**操作：**
1. 在 `internal/application/llmproxy/usecase/port.go` 中定义代理端口：
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
2. 修改 `internal/infrastructure/transport/anthropic.go` 和 `openai.go`，让实现类型实现新端口接口。
3. 修改 `internal/bootstrap/container.go`，更新依赖注入类型引用。
4. 修改所有 Application usecase 中的字段类型引用。
5. 删除 `internal/infrastructure/transport` 中的旧接口定义（或保留为内部接口，若 transport 包其他文件需要）。

**预期影响文件：** `container.go`、`anthropic.go`（transport）、`openai.go`（transport）、所有 usecase 文件（field 引用）。

### 4.2 修复问题 2 & 4：Util 包拆分（中优先级）

**目标：** 将协议相关工具从 `internal/util` 迁移到 Application 层专属 util，将通用 HTTP 工具迁移到 API 层。

**操作：**
1. 创建 `internal/application/llmproxy/util/` 目录。
2. 将 LLM 协议相关函数从 `internal/util/anthropic.go`、`openai.go`、`sse.go` 迁移到 `internal/application/llmproxy/util/`。
   - 保留函数名和签名，仅移动文件位置并更新包名/导入路径。
3. 将 HTTP 响应包装相关函数从 `internal/util/http.go` 迁移到 `internal/api/util/` 或 `internal/handler/util/`（需评估哪个更合理，推荐 `internal/api/util/` 因为与 HTTP/Fiber 耦合）。
4. 更新所有引用这些函数的导入路径。
5. 清理 `internal/util` 中遗留的未使用代码。

**预期影响文件：** 原 `internal/util/*.go`、所有引用它们的文件（handler、usecase、middleware、converter 等）。

### 4.3 修复问题 3：Converter/Usecase 文件拆分（中优先级）

**目标：** 将超大 converter/usecase 文件按职责拆分。

**操作：**

**Converter 拆分：**
- `converter/anthropic.go` → `converter/anthropic_chat.go`（ChatCompletion 相关转换） + `converter/anthropic_response.go`（Response API 相关转换） + `converter/anthropic_sse.go`（SSE 转换相关）
- `converter/openai.go` → `converter/openai_chat.go` + `converter/openai_response.go` + `converter/openai_sse.go`
- `converter/response.go` → 保持或根据内容拆分（若已按 chat/response 拆分，response.go 中内容可归入 `openai_response.go` 或 `anthropic_response.go`）

**Usecase 拆分：**
- `usecase/openai_chat.go` → `usecase/openai_chat_stream.go`（流式处理） + `usecase/openai_chat_unary.go`（非流式处理）
- `usecase/openai_response.go` → `usecase/openai_response_stream.go` + `usecase/openai_response_unary.go`
- `usecase/anthropic_message.go` → `usecase/anthropic_message_stream.go` + `usecase/anthropic_message_unary.go`
- 保留 `usecase/openai.go`、`usecase/anthropic.go` 中的公共逻辑和接口定义（作为入口文件）。
- 将 `usecase/common.go` 中的审计辅助函数保留或拆分（若拆分应保留在 usecase 包内，因其依赖 dto 和 TaskSubmitter）。

**预期影响文件：** 所有 converter 和 usecase 文件，container.go（因为构造函数可能需要调整）。

## 5. 依赖关系变更

### 5.1 修复后理想依赖图

```
Router / Handler
    ↓ (依赖)
Application (usecase / command / query / converter)
    ↓ (定义端口)
    Domain (aggregate / vo / service / repo interface)
    ↓ (实现端口)
    Infrastructure (transport / repository impl / dao)
    ↓ (实现)
    Database / Redis / HTTP / External

Application /llmproxy/util (协议工具)
    ↓
    dto (协议 DTO)

api/util (HTTP 包装)
    ↓
    dto (协议 DTO)
    huma / fiber

common/util (通用工具，仅依赖标准库和 common 包)
```

## 6. 验证策略

1. **编译验证：** `go build ./...` 必须成功。
2. **Lint 验证：** `go run main.go lint conv` 必须仍通过 All convention checks passed。
3. **测试验证：** `make test` 或 `go test -count=1 ./...` 必须全绿。
4. **CodeGraph 验证：** 修复后重新索引并确认：
   - `internal/domain` 无 `internal/infrastructure` 导入。
   - `internal/application` 无反向依赖 `internal/infrastructure` 接口（仅通过 container DI 获取实现）。
   - `internal/common/util` 无 `internal/dto` 导入。
   - 单文件 symbols 数量分布趋于均匀（无 >40 symbols 的 converter/usecase 文件）。

## 7. 风险评估

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 拆分过细导致包碎片化 | 开发体验下降 | 严格遵循"按职责拆分"而非"按行数拆分"，控制子包数量 |
| 导入路径变更导致引用遗漏 | 编译失败 | 使用 `codegraph_impact` 追踪所有受影响文件，逐文件更新导入 |
| Go 接口实现隐式约束出错 | 编译失败 | 每次修改后在 container.go 添加变量断言（如 `var _ usecase.AnthropicProxyPort = (*transport.anthropicProxy)(nil)`） |
| 测试数据/夹具引用遗漏 | 测试失败 | 特别关注 `test/unit/` 和 `test/e2e/` 中的导入 |

## 8. 回滚策略

所有变更在一个独立分支中完成。若验证失败（编译/测试不通过），可整体回滚 git 分支，不污染 master。

---

**作者：** centonhuang  
**日期：** 2026-05-21  
**状态：** 待审查
