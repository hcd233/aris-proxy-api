# CODEBUDDY.md

本文件为 CodeBuddy 在此仓库中工作时提供指引。

## 构建与运行命令

```bash
# 构建二进制
make build                # 生产构建（strip 符号）
make build-dev            # 开发构建（保留调试信息）
make build-debug          # 带完整调试信息的构建（用于 dlv 调试）
make build-upx            # 极致压缩构建（strip + UPX）

# 运行本地服务
go run main.go server start --host localhost --port 8080

# 数据库迁移
go run main.go database migrate

# 对象存储创建 Bucket
go run main.go object bucket create

# 安装依赖
go mod download

# 运行测试
make test                 # 全量测试
make test-cover           # 带覆盖率的测试

# 运行单个测试
go test -v -run TestFunctionName ./test/<主题目录>/

# 规范扫描
make lint-conv

# Docker
docker volume create postgresql-data
docker volume create redis-data
docker compose -f docker/docker-compose-full.yml up -d       # 完整栈（PostgreSQL + Redis + MinIO）
docker compose -f docker/docker-compose-single.yml up -d     # 生产单服务
docker compose -f docker/docker-compose-dev-single.yml up -d --build  # 开发单服务
```

## 项目概览

这是一个 Go 后端 API，作为 **LLM 代理网关** 并提供用户管理功能。使用 **Fiber v2** 作为 HTTP 框架，其上层封装 **Huma v2** 实现类型安全的 Handler 和自动 OpenAPI 3.1 规范生成。

### 启动流程

入口：`main.go` → `cmd.Execute()` → `cmd/server.go` 中的 `server start` 命令：

1. `database.InitDatabase()` — PostgreSQL（GORM）
2. `cache.InitCache()` — Redis（go-redis）
3. `pool.InitPoolManager()` — Pond v2 协程池
4. `cron.InitCronJobs()` — 定时任务
5. 注册全局 Fiber 中间件（Recover → fgprof → CORS → Compress → Trace → Log）
6. 非生产环境注册 `/docs` 路由（由 `internal/enum/env.go` 控制）
7. `router.RegisterAPIRouter()` — 注册所有 API 路由
8. `app.Listen()` — 启动服务，主 goroutine 监听 SIGINT/SIGTERM 执行优雅关闭

### 两层配置体系

- **环境变量**（Viper `AutomaticEnv()`）：服务设置、数据库凭证、OAuth2、JWT、存储池配置。从 `env/api.env` 加载，模板在 `env/api.env.template`。Key 使用 `_` 分隔（如 `POSTGRES_HOST`）。协程池配置通过 `PoolConfig` 结构体（`Pool.Store.Workers`/`QueueSize`、`Pool.Agent.Workers`/`QueueSize`）管理
- **数据库配置**（GORM）：LLM 代理模型路由和 API Key 存储在 `ModelEndpoint` 和 `ProxyAPIKey` 表中，通过 `ModelEndpointDAO` 和 `ProxyAPIKeyDAO` 访问

### 请求流

```
Fiber (HTTP) → 全局中间件 → Huma Router → 路由组中间件 → Handler → Service → DAO/Proxy → PostgreSQL/Redis/上游 LLM
```

**两层中间件架构：**
- **Fiber 级**（`fiber.Handler`）：Recover、fgprof、CORS、Compress、Trace、Log — 全局生效
- **Huma 级**（`func(huma.Context, func(huma.Context))`）：JWT、APIKey、RateLimiter、Permission、Lock — 按路由/路由组生效

### 路由结构

```
/health, /ssehealth                    — 健康检查（无需认证）
/api/v1/token/refresh                  — Token 刷新（无需认证）
/api/v1/token/*                        — Token 管理（JWT 认证）
/api/v1/oauth2/{provider}/login        — OAuth2 登录（限流）
/api/v1/oauth2/{provider}/callback     — OAuth2 回调（限流）
/api/v1/user/current                   — 当前用户（JWT 认证）
/api/v1/user/                          — 更新用户（JWT + 权限校验）
/api/v1/session/list                   — Session 列表（API Key 认证）
/api/v1/session/                       — Session 详情（API Key 认证）
/api/openai/v1/models                  — OpenAI 模型列表（API Key 认证）
/api/openai/v1/chat/completions        — OpenAI 聊天补全（API Key 认证）
/api/anthropic/v1/models               — Anthropic 模型列表（API Key 认证）
/api/anthropic/v1/messages             — Anthropic 消息（API Key 认证）
```

### 分层职责

- **`cmd/`** — Cobra CLI 命令（`server start`、`database migrate`、`object bucket create`）
- **`internal/api/`** — Fiber App 和 Huma API 单例，通过 `GetFiberApp()` / `GetHumaAPI()` 获取。`fiber.go` 初始化 Fiber App（含 JSON 编解码、超时、可信代理配置）；`huma.go` 初始化 Huma API（含 OpenAPI 3.1 规范、安全方案定义）。Fiber 使用 Sonic 作为 JSON 编解码器。Huma 注册 `jwtAuth` 和 `apiKeyAuth` 两种安全方案
- **`internal/router/`** — 按领域分组的路由注册。每个文件绑定中间件和 Handler，使用 `huma.Register()` 注册操作
- **`internal/handler/`** — 每个 Handler 为**接口**，私有结构体实现，通过 `NewXxxHandler()` 创建。Handler 持有 Service 引用，用 `util.WrapHTTPResponse()` 包装响应。流式响应（SSE/LLM）返回 `*huma.StreamResponse`。关键 Handler：OpenAIHandler、AnthropicHandler、SessionHandler、TokenHandler、UserHandler、Oauth2Handler
- **`internal/service/`** — 业务编排层（薄）。按领域分组（session、token、openai、anthropic、oauth2、user）。LLM 代理 Service 只负责：端点查找（含 Provider 回退）→ 调用 Converter 转换 → 调用 Proxy 转发 → 消息存储。不含 HTTP 通信和 SSE 读取逻辑。Oauth2Service 有 `NewGithubOauth2Service()` 和 `NewGoogleOauth2Service()` 两个工厂方法
- **`internal/proxy/`** — 上游代理层（纯传输）。负责与上游 LLM 提供者的 HTTP/SSE 通信。`OpenAIProxy` 提供 `ForwardChatCompletion`/`ForwardChatCompletionStream`，`AnthropicProxy` 提供 `ForwardCreateMessage`/`ForwardCreateMessageStream`/`ForwardCountTokens`。流式方法通过**回调函数**将 SSE 事件逐个交给调用方。包含 `UpstreamEndpoint`（端点信息）、`ReplaceModelInBody`/`ReplaceModelInSSEData`（model 替换工具）
- **`internal/converter/`** — 协议转换层（纯 DTO 转换）。`OpenAIProtocolConverter`：Anthropic→OpenAI 转换（`FromAnthropicRequest`/`ToAnthropicResponse`/`ToAnthropicSSEResponse`）。`AnthropicProtocolConverter`：OpenAI→Anthropic 转换（`FromOpenAIRequest`/`ToOpenAIResponse`/`ToOpenAISSEResponse`）。支持文本、工具调用、图片、推理内容的双向转换
- **`internal/infrastructure/database/dao/`** — 泛型 `baseDAO[ModelT]` 提供类型安全的 CRUD、分页和批量操作。具体 DAO 嵌入它。含 `singleton.go` 提供 `GetXxxDAO()` 单例访问。关键 DAO：MessageDAO、SessionDAO、ToolDAO、UserDAO、ModelEndpointDAO、ProxyAPIKeyDAO。软删除使用 `deleted_at`（int64 时间戳，0 = 未删除）
- **`internal/infrastructure/database/model/`** — GORM 模型。`BaseModel` 包含 ID/CreatedAt/UpdatedAt/DeletedAt。关键模型：User、Message（存储 UnifiedMessage JSON + SHA256 CheckSum）、Session（跟踪 APIKeyName + MessageIDs + ToolIDs 为 JSON 数组，含 Summary/Scores 字段）、Tool（存储 UnifiedTool JSON + CheckSum）、ModelEndpoint（含 Provider 字段区分上游协议）、ProxyAPIKey
- **`internal/dto/`** — 请求/响应 DTO。包含完整的 OpenAI 和 Anthropic API 类型定义，以及 `UnifiedMessage` 和 `UnifiedTool` 跨 Provider 格式及双向转换函数。`UnifiedContent` 使用自定义 JSON 序列化/反序列化处理联合类型（string | array | object）。含 `asynctask.go`（`MessageStoreTask`、`SummarizeTask`、`ScoreTask`）、`json_schema.go`（JSON Schema）等
- **`internal/middleware/`** — Recover、Fgprof、CORS、Compress、Fiber 级中间件（全局生效）；JWT、APIKey、RateLimiter、Permission、Lock、Trace、Log Huma 级中间件（按路由生效）。JWT 解码 Token 并注入 userID/userName/permission 到上下文。APIKey 中间件每次请求从数据库查询 API Key 验证，并注入 userName。RateLimiter 使用自定义 Redis Lua 脚本实现令牌桶算法（`redis/go-redis/v9`）。Lock 中间件使用 Redis SETNX + Lua 脚本原子解锁。Trace 中间件生成 UUID 并注入 `X-Trace-Id` 响应头
- **`internal/infrastructure/pool/`** — `PoolManager` 管理 Pond v2 协程池：`storePool` 和 `agentPool`。`storePool` 处理消息存储任务（通过 SHA256 CheckSum 去重，批量 IN 查询后事务创建 messages/tools/sessions）；`agentPool` 处理 Session 总结和评分任务。使用 `util.CopyContextValues()` 安全传递上下文到异步任务
- **`internal/agent/`** — 可复用的 LLM/Agent 能力。通过 `GetSummarizer()` 和 `GetScorer()` 单例访问。Summarizer 使用 Session 总结指令生成 5-10 字中文摘要；Scorer 从连贯性、深度、价值三维度评分
- **`internal/cron/`** — 定时任务调度。包含 `session_dedup`（会话去重）、`session_summarize`（会话总结）、`session_score`（会话评分）、`soft_delete_purge`（软删除清理）
- **`internal/config/`** — Viper 环境变量配置加载
- **`internal/common/constant/`** — 项目常量（上下文 Key、HTTP、Agent、时间、速率、字符串）。含 `agent.go`（Agent 指令和参数）、`rate.go`（速率限制配置）等
- **`internal/common/enum/`** — 公共枚举
- **`internal/common/ierr/`** — 统一错误创建和处理包
- **`internal/common/model/`** — 通用数据模型（`Error` 业务错误、`UpstreamError` 上游通信错误）
- **`internal/enum/`** — 业务枚举（环境、Provider 类型、内容类型、角色、语音、模态、Anthropic SSE 事件/内容块/Delta 类型等）
- **`internal/jwt/`** — JWT 签发和验证单例
- **`internal/lock/`** — Redis 分布式锁
- **`internal/logger/`** — Zap 结构化日志。四路输出：控制台（彩色 Console 编码）+ info.log（JSON）+ error.log（JSON）+ panic.log（JSON），Lumberjack 轮转
- **`internal/oauth2/`** — OAuth2 策略模式实现（GitHub、Google 等）
- **`internal/util/`** — 工具函数。含上下文（`CopyContextValues`）、哈希（`ComputeMessageChecksum`/`ComputeToolChecksum`）、HTTP 响应工具（`WrapHTTPResponse`/`WrapStreamResponse`/`WrapJSONResponse`/`JSONResponseWriter`/`WriteUpstreamError`/`WriteErrorResponse`）、SSE（`WrapErrorSSE`/`ConcatChatCompletionChunks`/`ConcatAnthropicSSEEvents`）、字符串（`MaskSecret`）、错误响应（`SendOpenAIModelNotFoundError`/`SendAnthropicModelNotFoundError`）等

### 安全方案

Huma 注册 `jwtAuth` 和 `apiKeyAuth` 两种安全方案，分别对应 JWT 和 API Key 认证

### 认证机制

两种认证按路由应用：
- **JWT**（`Authorization: Bearer <token>`）— 用户路由。双 Token：AccessToken（短期）+ RefreshToken（长期），不同密钥和过期时间。OAuth2 登录后签发
- **API Key**（`Authorization: Bearer <api-key>`）— LLM 代理路由。Key 存储在 `ProxyAPIKey` 表中

### LLM 代理流（三层架构）

```
Service 层（编排）      → 端点查找 + Converter 调用 + Proxy 调用 + 消息存储
  ↓
Proxy 层（传输）        → HTTP 请求构建 + 发送 + SSE 读取循环 + 事件合并
  ↓
Converter 层（转换）    → 纯 DTO 格式转换（OpenAI ↔ Anthropic）
```

**原生协议流（OpenAI 接口 → OpenAI 上游）：**
```
Client → /api/openai/v1/chat/completions (model=my-alias)
  → APIKeyMiddleware 验证 Key
  → Service 查找 my-alias → ModelEndpoint 表 (provider=openai)
  → 兼容性处理（如 max_tokens → max_completion_tokens）
  → 序列化请求，替换 model 为上游实际名称
  → Proxy 构建 HTTP 请求 → 转发至上游 → SSE 读取 → 回调返回 chunk
  → Service 在回调中替换 model 名 → 写入客户端
  → 异步：转换为 UnifiedMessage → 通过 Pool 存储（CheckSum 去重）
```

**跨协议流（OpenAI 接口 → Anthropic 上游）：**
```
Client → /api/openai/v1/chat/completions (model=my-alias)
  → Service 查找 my-alias → ModelEndpoint 表 (provider=anthropic)
  → Converter 将 OpenAI 请求转为 Anthropic 格式
  → Proxy 通过 Anthropic 协议发送到上游
  → Service 在回调中用 Converter 将 Anthropic 事件转回 OpenAI chunk → 写入客户端
  → 异步：Converter 转回 OpenAI 格式 → 通过 Pool 存储
```

Anthropic 接口同理支持 OpenAI 上游（反向转换）。

**Proxy 层流式回调模式：**
```go
// Proxy 通过回调将 SSE 事件逐个交给 Service
proxy.ForwardChatCompletionStream(ctx, ep, body, func(chunk *dto.ChatCompletionChunk) error {
    chunk.Model = exposedModel
    fmt.Fprintf(w, "data: %s\n\n", lo.Must1(sonic.Marshal(chunk)))
    return w.Flush()
})
```

### 核心设计模式

1. **接口驱动**：Handler、Service、DAO、Proxy、TokenSigner、Locker、ObjDAO 均定义接口
2. **单例模式**：Fiber App、Huma API、DB、Redis、DAO、JWT Signers、PoolManager
3. **泛型 DAO**：`baseDAO[ModelT]` 使用 Go 泛型
4. **策略模式**：OAuth2 平台切换、对象存储平台切换
5. **统一消息格式**：OpenAI/Anthropic DTO → UnifiedMessage/UnifiedTool 跨 Provider 存储
6. **异步协程池**：LLM 请求后的消息存储通过 Pool，SHA256 CheckSum 去重
7. **上下文感知日志**：`logger.WithCtx(ctx)` / `logger.WithFCtx(fctx)` 自动附加 traceID、userID、userName
8. **三层 LLM 代理架构**：Service（编排）→ Proxy（传输）→ Converter（转换），职责分离
9. **回调函数流式处理**：Proxy 通过 `onChunk`/`onEvent` 回调将 SSE 事件逐个交给 Service，Service 在回调中做协议转换和客户端写入
10. **跨协议转换**：注册为 `provider=openai` 的模型可通过 Anthropic 接口调用，反之亦然

### 核心指令

1. **禁止**使用 `json.RawMessage` 或 `any`/`interface{}`
2. **修改 OpenAI 或 Anthropic DTO 前**，必须先查看 `/docs` 中的文档

### 关键依赖

- **Go 1.25.1**
- **Fiber v2** + **Huma v2**：HTTP 框架 + OpenAPI 类型化 Handler
- **GORM** + PostgreSQL：ORM 和数据库
- **Redis**（`redis/go-redis/v9`）：缓存、限流器后端、分布式锁
- **Sonic**：高性能 JSON
- **Cobra / Viper**：CLI 和配置
- **Zap** + Lumberjack：结构化日志
- **MinIO / Tencent COS**：对象存储
- **golang-jwt**：JWT 签发和验证
- **Pond v2**：异步任务协程池
- **robfig/cron/v3**：定时任务调度
- **samber/lo**：Go 函数式工具
- **Eino**（cloudwego/eino）：AI 框架

---

## 测试规范

### 测试目录结构

**所有测试文件（`*_test.go`）必须且只能放在 `test/` 目录下，`internal/` 目录内禁止存放任何测试文件。**

```
test/                              # 所有测试文件的唯一存放位置
├── <主题名>/                       # 按测试主题组织，snake_case 命名
│   ├── fixtures/                  # 测试数据文件（必须放在 fixtures/ 子目录）
│   │   └── cases.json
│   └── xxx_test.go                # 测试代码，package 名与目录名一致
└── ...
```

| 测试类型 | 存放位置 | 说明 |
|---------|---------|------|
| 单元测试 | `test/<主题>/` | 通过导出的公开 API 测试单个函数/方法的行为 |
| 集成测试 / 专项测试 / E2E 测试 | `test/<主题>/` | 跨包跨层、需外部依赖、或 Bug 根因调查 |

### 测试数据管理：数据与代码完全分离

**所有测试数据必须放到 `fixtures/` 目录的 JSON 文件中，测试代码中只做加载和断言，禁止在代码中内联构造测试数据。**

### 用例编写规范

- **命名格式**：`Test<FunctionName>_<场景描述>`，如 `TestComputeToolChecksum_NilParameters`
- 优先使用 **fixture 驱动模式**：通过 fixture 中的 case 名称列表驱动子测试
- 辅助函数**必须**标记 `t.Helper()`
- 每个测试函数只验证一个行为
- 测试代码日志**使用英文**，通过 `t.Logf` 输出关键中间值

### 测试数据加载模式

项目使用 JSON fixtures + helper 函数的数据驱动模式，**禁止使用标准库 `encoding/json`**，统一用 `github.com/bytedance/sonic`：

```go
// 定义测试用例结构体
type testCase struct {
    Name        string `json:"name"`
    Description string `json:"description"`
}

// 加载 fixture 的 helper 函数（必须 t.Helper()）
func loadCases(t *testing.T) []testCase {
    t.Helper()
    data, err := os.ReadFile("./fixtures/cases.json")
    if err != nil {
        t.Fatalf("failed to read fixture: %v", err)
    }
    var cases []testCase
    if err := sonic.Unmarshal(data, &cases); err != nil {
        t.Fatalf("failed to unmarshal fixture: %v", err)
    }
    return cases
}

// 按名称查找用例的 helper
func findCase(t *testing.T, cases []testCase, name string) testCase {
    t.Helper()
    for _, c := range cases {
        if c.Name == name {
            return c
        }
    }
    t.Fatalf("case %q not found", name)
    return testCase{}
}
```

### 断言规范

**不使用** testify/gomock 等第三方断言库，完全依赖标准库 `testing` 包：

```go
// 好：清晰的失败信息，包含 got / want 上下文
if got != want {
    t.Errorf("ComputeChecksum() = %s, want %s", got, want)
}

// 好：使用子测试隔离
t.Run("empty input", func(t *testing.T) {
    result := ComputeChecksum(nil)
    if result != "" {
        t.Errorf("expected empty string, got %q", result)
    }
})

// 差：无上下文的断言
if result != expected {
    t.Fatal("not equal")
}
```

### 开发流程强制要求（MANDATORY）

**每次功能开发/修改/Bug 修复完成后，必须执行以下两步：**

#### Step 1: 沉淀测试用例

| 变更类型 | 必须沉淀的用例 |
|---------|-------------|
| 新增 `util/` 函数 | 对应的单元测试（正常路径 + 边界条件 + 错误路径） |
| 新增/修改 `dto/` 自定义序列化 | 序列化 + 反序列化往返测试 |
| 新增/修改 `service/` 方法 | 单元测试或集成测试 |
| Bug 修复 | **必须**附带回归测试，覆盖触发 Bug 的场景 |
| 新增中间件 | 认证/鉴权/限流等行为测试 |

#### Step 2: 运行全量测试

```bash
# 必须在提交前运行，全部 PASS 才允许提交
go test -count=1 ./...
```

**全部测试 PASS 后方可提交代码。任何 FAIL 必须修复后才能提交。**

### 测试禁止事项

1. **禁止在 `internal/` 任何子目录中创建 `*_test.go` 文件** — 所有测试只能放在 `test/` 目录
2. **禁止提交不通过的测试** — 所有测试必须 PASS 才能提交
3. **禁止删除已有的测试用例** — 除非对应功能已删除
4. **禁止在测试中硬编码环境相关路径** — 使用 `t.TempDir()`、`os.CreateTemp()` 等
5. **禁止测试间相互依赖** — 每个测试必须独立可运行
6. **禁止使用 `time.Sleep()` 做同步** — 使用 channel、WaitGroup 或 deadline
7. **禁止使用 `encoding/json`** — 统一使用 `github.com/bytedance/sonic`
8. **禁止使用第三方断言库（如 testify）** — 使用标准库 `testing` 包的 `t.Errorf` / `t.Fatalf`
9. **禁止在测试代码中内联构造测试数据** — 所有数据必须来自 `fixtures/` 文件

### 常用测试命令

```bash
# 全量测试（提交前必须运行）
go test -count=1 ./...

# 指定目录
go test -v -count=1 ./test/message_checksum/

# 指定函数
go test -v -count=1 -run TestChecksumDifference ./test/message_checksum/

# 带覆盖率
go test -count=1 -cover ./test/...

# 生成覆盖率报告
go test -count=1 -coverprofile=coverage.out ./test/...
go tool cover -html=coverage.out -o coverage.html
```

---

## 代码组织规范

### 公共函数位置规则

**导出的公共函数只能放在 `internal/util/` 或 `internal/common/` 下**，不允许在业务包（如 `service/`、`handler/`）中创建 `common.go` 存放公共函数。

| 类型 | 位置 | 示例 |
|------|------|------|
| HTTP 响应工具 | `util/http.go` | `WrapStreamResponse`、`WrapJSONResponse`、`JSONResponseWriter`、`WriteUpstreamError` |
| SSE 工具 | `util/sse.go` | `WrapErrorSSE`、`ConcatChatCompletionChunks`、`ConcatAnthropicSSEEvents` |
| 哈希/字符串工具 | `util/hash.go`、`util/string.go` | `ComputeMessageChecksum`、`MaskSecret` |
| 公共错误模型 | `common/model/error.go` | `Error`（业务错误）、`UpstreamError`（上游通信错误） |
| 公共枚举 | `common/enum/` | `Permission`、`SSEDataType` 等 |
| 统一错误创建 | `common/ierr/` | `ierr.Wrap`、`ierr.New`、哨兵错误 |

### 包内私有辅助函数

业务包（如 `service/`）内部的辅助函数（未导出）应：
- 直接放在使用它们的文件中（如 `findEndpoint` 放在 `openai.go`）
- 或放在同一包的某个业务文件中（当被多个文件共享时）
- **不建立独立的 `common.go`**

### 字符串模板集中管理

**所有字符串模板（Redis Key、存储路径、ID 格式、Data URL 等）必须定义在 `internal/common/constant/string.go` 中，禁止在业务代码中硬编码。**

| 类型 | 常量命名风格 | 示例 |
|------|------------|------|
| Redis Key 模板 | `XxxKeyTemplate` | `JWTUserCacheKeyTemplate = "jwt:user:%d"` |
| Redis Key 前缀 | `XxxKeyPrefix` | `ScannerBanKeyPrefix = "scanner:ban:"` |
| 存储路径模板 | `XxxDirTemplate` / `XxxPathTemplate` | `ObjectStorageDirTemplate = "user-%d/%s"` |
| ID 生成模板 | `XxxIDTemplate` | `OpenAIChunkIDTemplate = "chatcmpl-%s"` |
| 数据格式模板 | `XxxTemplate` | `DataURLTemplate = "data:%s;base64,%s"` |

**目的**：Redis Key 的一致性对缓存/锁/限流的正确性至关重要，散落在业务代码中容易因拼写错误或格式不一致导致难以排查的 Bug。

### 魔法值禁止规范

**禁止在业务代码中直接使用魔法数字或魔法字符串，所有字面量必须提取为具名常量。**

#### 常量定义位置规则

| 类型 | 定义位置 | 说明 |
|------|----------|------|
| 全局通用字符串（URL、前缀、标识符等） | `internal/common/constant/string.go` | 跨包使用的字符串字面量 |
| 全局通用数字（容量、长度、超时等） | `internal/common/constant/number.go` | 跨包使用的数字字面量 |
| LLM 协议枚举值（stop_reason、source_type 等） | `internal/enum/` | 属于 LLM API 规范的字符串枚举 |
| 通用业务枚举值 | `internal/common/enum/` | 业务通用枚举 |

#### 禁止包内 const

**业务包（`service/`、`handler/`、`proxy/`、`converter/` 等）内禁止定义 `const` 块。**
所有常量必须定义在 `constant/` 或 `enum/` 中，通过 import 引用，确保可在 lint 扫描中统一发现和追踪。

```go
// ❌ 禁止：service 包内定义 const
const APIKeyPrefix = "sk-aris-"

// ✅ 正确：定义在 constant/string.go，业务代码 import 引用
constant.APIKeyPrefix
```

#### 禁止转发封装常量

**禁止在 `constant/`、`enum/` 中定义 `const X = pkg.Y` 形式的转发封装。** 这类定义只是给另一个包的具名常量起别名，毫无意义。

```go
// ❌ 禁止：constant 包中转发封装
const HTTPStatusOK = fiber.StatusOK

// ✅ 正确：业务代码直接引用 fiber 的具名常量
fiberCtx.Status(fiber.StatusOK)
```

#### HTTP 状态码

**禁止使用裸数字 HTTP 状态码，直接使用 `fiber.StatusXxx` 具名常量。**

```go
// ❌ 禁止
writer.WriteError(500, body)
return 200, ""

// ✅ 正确
writer.WriteError(fiber.StatusInternalServerError, body)
return fiber.StatusOK, ""
```

#### 时间字段

**DTO 中的时间字段使用 `time.Time` 类型，禁止在 Service 层格式化为字符串。** 格式化由前端/调用方根据需要处理，或在 JSON 序列化层统一处理。

```go
// ❌ 禁止
type APIKeyItem struct {
    CreatedAt string `json:"createdAt"`
}
// Service 层
CreatedAt: key.CreatedAt.Format("2006-01-02 15:04:05"),

// ✅ 正确
type APIKeyItem struct {
    CreatedAt time.Time `json:"createdAt"`
}
// Service 层
CreatedAt: key.CreatedAt,
```

#### lint 扫描豁免范围

以下路径不参与魔法值扫描（这些包本身就是定义字面量的合法位置）：
- `internal/common/constant/` — 字符串/数字常量定义处
- `internal/common/enum/` / `internal/enum/` — 枚举定义处
- `internal/common/ierr/` — 错误哨兵定义处
- `internal/infrastructure/` — 基础设施层（DB/Redis/对象存储配置）
- `internal/config/` — 环境变量默认值配置
- `internal/logger/` — 日志框架初始化配置

### 循环依赖避免

`util/` 是底层工具包，不允许依赖上层业务包。如果 `util/` 需要引用某个类型，应将该类型下沉到 `common/model/`。

示例：`UpstreamError` 原在 `proxy/` 中定义，但 `util/http.go` 的 `WriteUpstreamError` 需要引用它。由于 `proxy/` → `util/`（已有依赖），反向依赖会产生循环。因此 `UpstreamError` 下沉到 `common/model/`。

### LLM 代理分层规范

```
Handler → Service → Proxy → upstream HTTP
                 ↘ Converter (纯转换)
```

| 层 | 职责 | 规则 |
|---|---|---|
| Service | 端点查找 + 分发 + Converter 调用 + 消息存储 | 不含 HTTP 请求构建/SSE 读取 |
| Proxy | HTTP 请求构建 + 发送 + SSE 读取 + 事件合并 | 不含协议转换逻辑，通过回调交出事件 |
| Converter | 纯 DTO 格式转换 | 无状态、无副作用、不依赖外部服务 |

---

## 开发流程

**每次新增/修改/重构代码时，必须遵循以下流程。**

### Step 1: 编码时自检清单

编写每一段代码时，逐项对照：

#### 错误处理（BLOCKING）

- **禁止** `fmt.Errorf` / `errors.New` — 统一用 `ierr.Wrap` / `ierr.New`
- **禁止** `constant.ErrXxx` — 统一用 `ierr.ErrXxx.BizError()`
- DAO/Util 层：`ierr.Wrap(ierr.ErrXxx, err, "context")`
- Service 层：`rsp.Error = ierr.ErrXxx.BizError()` + `return rsp, nil`（Go error 始终 nil）
- Handler 层：一行 `return util.WrapHTTPResponse(h.svc.Method(ctx, req))`
- Middleware 层：`lo.Must0(util.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrXxx.BizError()))`
- 选择最精确的哨兵错误，不要一律映射为 `ErrInternal`

#### 日志（BLOCKING）

- 格式：`"[PascalCaseModule] English message"`，如 `"[SessionService] Get session detail"`
- 上下文：`logger.WithCtx(ctx)` 或 `logger.WithFCtx(c)`
- 敏感信息（Key/Token/Secret/Password）必须 `util.MaskSecret()`
- 结构化字段：`zap.String()`, `zap.Error()`, `zap.Uint()` 等
- 级别：Error=需人工介入, Warn=可自愈, Info=关键节点, Debug=调试
- 禁止循环内/高频路径打日志

#### 命名（BLOCKING）

- 接口 PascalCase 无 `I` 前缀，实现 camelCase 私有 struct
- 工厂函数 `NewXxx()` 返回接口类型
- Handler 方法 `Handle` 前缀
- DTO：`XxxReq` / `XxxRsp` / `XxxReqBody`
- 禁止 `data1`, `tmp`, `userList`, `userMap` 等无意义/暴露实现的命名

#### 代码结构

- 函数优先 10 行，不超过 20 行
- if 嵌套不超过 2 层，优先 guard clauses
- 参数 0-3 个，超过用参数对象
- 出现 2 次的逻辑必须抽取，禁止复制粘贴
- 禁止死代码（注释掉的旧代码必须删除）
- 能私有就私有，禁止随意导出

#### Import 与依赖

- 三段式分组：标准库 → 第三方 → 项目内部（空行分隔）
- 禁止 `encoding/json`，统一 `github.com/bytedance/sonic`
- 禁止 `json.RawMessage` 和 `any`/`interface{}`

#### 注释

- godoc 格式：第一行中文简述 + `@receiver`/`@param`/`@return`/`@author`/`@update` 标签
- 包注释：`// Package xxx 中文描述`

#### 架构分层

- Handler 只做薄包装，不含业务逻辑
- Service 不直接依赖基础设施实现，不含 HTTP/SSE 通信逻辑
- LLM 代理 Service 只做编排：端点查找 → Converter 转换 → Proxy 转发 → 存储
- Proxy 只做传输：HTTP 请求构建 + SSE 读取 + 回调交出事件
- Converter 只做转换：纯函数，无状态，不依赖外部服务
- 导出的公共函数只放 `util/` 或 `common/`，不在 service 等业务包建 `common.go`
- 所有业务方法第一个参数 `context.Context`
- 单例通过 `GetXxx()` 获取

#### Context 使用规范（BLOCKING）

**核心原则：Context 必须从上层传递，接口逻辑层禁止自行创建。**

##### 分层规则

| 层 | 规则 | 获取方式 |
|---|---|---|
| **Fiber 中间件** | 使用 `c.Context()` 获取 fasthttp context，通过 `c.Locals()` 读写值 | `c.Context()` / `c.Locals(key)` |
| **Huma 中间件** | 使用 `ctx.Context()` 获取底层 `context.Context`，通过 `huma.WithValue()` 注入值 | `ctx.Context()` / `huma.WithValue(ctx, key, val)` |
| **Handler / Service / Proxy / Converter / DTO** | **禁止** `context.Background()` / `context.TODO()`，必须从参数传递 `context.Context` | 函数第一个参数 `ctx context.Context` |
| **Cron 定时任务** | **允许** `context.Background()`，作为独立调度入口创建根 context，必须注入 TraceID | `context.WithValue(context.Background(), constant.CtxKeyTraceID, uuid.New().String())` |
| **基础设施初始化** | **允许** `context.Background()`，用于启动阶段连接验证（Redis Ping、MinIO ListBuckets 等） | `context.Background()` |
| **Agent 单例初始化** | **允许** `context.Background()`，用于启动阶段创建 LLM ChatModel | `context.Background()` |
| **工具函数（util）** | **允许** `context.Background()`，仅限 `CopyContextValues` 创建独立生命周期的 context | `context.Background()` |

##### 上下文 Key 管理

所有 context key 定义在 `internal/common/constant/ctx.go`，类型为 `string` 常量：

| Key | 值类型 | 注入者 | 用途 |
|---|---|---|---|
| `CtxKeyTraceID` | `string` (UUID) | Fiber TraceMiddleware / Cron | 请求追踪、日志关联 |
| `CtxKeyUserID` | `uint` | JWT 中间件 | 用户标识、数据关联 |
| `CtxKeyUserName` | `string` | JWT / APIKey 中间件 | 日志、业务逻辑 |
| `CtxKeyPermission` | `enum.Permission` | JWT 中间件 | Permission 中间件鉴权 |
| `CtxKeyLimiter` | `string` | Rate 中间件 | 限流 key 标识 |
| `CtxKeyClient` | `string` | APIKey 中间件 | 请求客户端 User-Agent |

##### 异步任务 Context 传递

HTTP 请求结束后原始 context 会被取消，异步任务必须使用 `util.CopyContextValues(ctx)` 创建独立 context：

```go
// ✅ 正确：异步任务使用 CopyContextValues 脱离请求生命周期
pool.GetPoolManager().SubmitMessageStoreTask(&dto.MessageStoreTask{
    Ctx: util.CopyContextValues(ctx),  // 仅复制 TraceID + UserID
    ...
})
```

`CopyContextValues` 仅复制 `CtxKeyTraceID` 和 `CtxKeyUserID`，其他值（Permission、Limiter 等）不传递给异步任务。

##### 安全取值

使用 `util.CtxValueString()` / `util.CtxValueUint()` 安全获取 context 值（类型不匹配返回零值，不 panic）：

```go
userName := util.CtxValueString(ctx, constant.CtxKeyUserName)
userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
```

##### 🚫 禁止事项

1. **禁止在接口逻辑层使用 `context.Background()` / `context.TODO()`** — `handler/`、`service/`、`middleware/`、`router/`、`proxy/`、`converter/`、`dto/` 必须从上层传递 context
2. **禁止直接使用 `ctx.Value()` 做类型断言** — 优先使用 `util.CtxValueString()` / `util.CtxValueUint()` 安全取值
3. **禁止在异步任务中使用原始请求 context** — 必须通过 `util.CopyContextValues()` 创建独立 context
4. **禁止新增 context key 时不在 `constant/ctx.go` 注册** — 所有 key 必须集中定义

### Step 2: 运行规范扫描

```bash
make lint-conv
```

修复所有 ERROR，评估所有 WARN。

脚本位于 `script/lint-conventions.sh`，覆盖以下检查项：

| 检查项 | 级别 | 说明 |
|--------|------|------|
| `fmt.Errorf` / `errors.New` 使用 | ERROR | 必须用 ierr 包 |
| `constant.ErrXxx` 使用 | ERROR | 已废弃 |
| `encoding/json` / `json.RawMessage` | ERROR | 用 sonic |
| internal/ 下测试文件 | ERROR | 必须放 test/ |
| test/ 根目录散落测试 | ERROR | 必须放子目录 |
| testify 等第三方断言库 | ERROR | 用标准库 |
| `time.Sleep` 在测试中 | ERROR | 用 channel/WaitGroup |
| Handler 直接操作 DAO/DB | ERROR | 业务逻辑放 Service |
| 接口逻辑层 `context.Background()`/`context.TODO()` | ERROR | 必须从上层传递 context |
| 日志缺少 `[Module]` 前缀 | WARN | 建议修复 |
| 敏感信息未 MaskSecret | WARN | 建议修复 |
| 可能的死代码 | WARN | 人工确认 |
| 暴露实现细节的命名 | WARN | 建议改为复数 |
| Service 返回非 nil error | WARN | 确认是否正确 |
| 核心业务层 `interface{}` | WARN | 优先具体类型/泛型 |

### Step 3: 沉淀测试用例

参照上方「测试规范」章节。

### Step 4: 运行全量测试

```bash
make test
```

全部 PASS 后方可提交代码。

### Step 5: 沉淀规范到指引文档

**如果本次开发过程中用户提出了任何规范性内容（编码规范、架构约定、命名规则、流程要求等），必须在开发完成后将其更新到 `CODEBUDDY.md` 和 `CLAUDE.md` 中对应的章节，确保后续开发遵守。**

| 场景 | 操作 |
|------|------|
| 用户提出新的编码规范/约定 | 追加到对应章节（如「编码时自检清单」「代码组织规范」等） |
| 用户修正/废弃已有规范 | 更新或删除对应条目 |
| 用户提出新的架构设计/分层规则 | 更新「分层职责」「LLM 代理分层规范」等相关章节 |
| 用户提出新的测试要求 | 更新「测试规范」章节 |
| 用户提出新的 Git/CI 流程 | 更新「Git 工作流」章节 |

**注意事项：**
1. `CODEBUDDY.md` 和 `CLAUDE.md` 必须同步更新，保持内容一致
2. 新增规范应放在最相关的已有章节下，避免重复或分散
3. 如果已有章节无法覆盖，可在合适位置新增子章节
4. 更新时保持原有文档风格和格式

---

## Git 工作流

### Pre-commit Hook

项目配置了 pre-commit hook（`.githooks/pre-commit`），提交前自动执行：
1. `gofmt -w` — 自动格式化并重新 stage
2. `go mod tidy` — 验证 go.mod/go.sum 一致性
3. `go vet` — 静态分析
4. `go test -count=1 ./...` — 全量测试

安装 hook：`bash .githooks/setup.sh`

### CI/CD

GitHub Actions（`.github/workflows/docker-publish.yml`）在 push master 和 tag 时自动构建多架构 Docker 镜像（linux/amd64 + linux/arm64），推送到 GHCR。


---

## 开发环境配置

### Worktree 目录偏好

项目使用 `.worktrees/` 作为 Git worktree 的默认目录（项目本地隐藏目录）。

```bash
# 创建 feature worktree
git worktree add .worktrees/feature-name -b feature/feature-name
cd .worktrees/feature-name
```

**配置要求：**
- Worktree 目录 `.worktrees/` 已添加到 `.gitignore`
- 新功能开发应在独立 worktree 中进行，避免污染主工作区
