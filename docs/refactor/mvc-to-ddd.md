# MVC 到 DDD 架构重构说明

> 面向开发者的架构变更文档，描述从传统 MVC 分层到领域驱动设计（DDD）的改造方案与设计决策。
>
> 分支：`refactor/mvc_to_ddd_20260424`
> 对比基线：`master`

---

## 1. 重构动机

### 1.1 原有 MVC 架构的问题

重构前的架构遵循典型的 MVC 分层：

```
Handler → Service → DAO / Proxy / Converter
```

核心问题集中在 `internal/service/` 层：

| 问题 | 表现 |
|------|------|
| **Service 膨胀** | 每个 Service 文件 250-450 行，`openai.go` 超 800 行，包含端点查找、协议转换、HTTP 通信、流式读写、消息存储、审计等所有逻辑 |
| **业务规则散落** | 配额校验（"一个用户最多 N 个 Key"）、所有权判断（"Key 只能由所有者或 admin 吊销"）、摘要文本校验等规则嵌入在 Service 方法中，难以发现和复用 |
| **基础设施耦合** | Service 直接调用 `dao.GetProxyAPIKeyDAO()`、`database.GetDBInstance(ctx)`，无法对业务逻辑进行单元测试 |
| **概念缺失** | 没有"聚合根""值对象""仓储"等建模概念，所有数据都是 DTO 或 GORM 模型，业务含义由函数名暗示 |

### 1.2 目标架构

本次重构引入 DDD 战术模式，建立清晰的四层架构：

```
┌─────────────────────────────────────────────────────┐
│  Handler（入口层）                                  │
│  提取上下文 → 构造 Command/Query → 映射 View 到 DTO │
├─────────────────────────────────────────────────────┤
│  Application（用例层）                              │
│  Command（写）/ Query（读）/ UseCase（复杂流程）    │
│  编排域对象 + 仓储 + 领域服务 → 完成业务用例        │
├─────────────────────────────────────────────────────┤
│  Domain（领域层，无外部依赖）                       │
│  Aggregate + ValueObject + Repository Interface     │
│  + Domain Service                                   │
├─────────────────────────────────────────────────────┤
│  Infrastructure（基础设施层）                       │
│  Repository Impl / Transport / Agent / JWT / OAuth2 │
│  DAO / GORM / Redis / HTTP                          │
└─────────────────────────────────────────────────────┘
```

### 1.3 请求数据流全景

以 `POST /api/v1/apikey/`（创建 API Key）为例，跟踪一个请求穿越各层时数据的形态变化。其他 CRUD 接口遵循相同的分层模式，LLM 代理接口的区别见 [1.4 节](#14-llm-代理流对比)。

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ 请求进入                                                                    │
│ POST /api/v1/apikey/                                                        │
│ Authorization: Bearer <jwt>       Body: {"name": "my-key"}                  │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 1. Fiber 全局中间件                                                         │
│    Recover → fgprof → CORS → Compress → TraceMiddleware → LogMiddleware     │
│                                                                             │
│    TraceMiddleware 注入 X-Trace-Id 到 ctx，后续所有日志自动携带             │
│    ctx 中此时有: {CtxKeyTraceID: "550e8400-..."}                            │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 2. Huma 路由匹配 → 路由组中间件                                             │
│    router/apikey.go → initAPIKeyRouter()                                    │
│                                                                             │
│    apikeyGroup.UseMiddleware(middleware.JwtMiddleware())                    │
│      → 解码 JWT，注入到 ctx:                                                │
│        {CtxKeyUserID: 42, CtxKeyUserName: "alice", CtxKeyPermission: "user"}│
│                                                                             │
│    apikeyGroup.UseMiddleware(middleware.TokenBucketRateLimiterMiddleware()) │
│      → Redis Lua 令牌桶，超限返回 429                                       │
│                                                                             │
│    单路由中间件:                                                            │
│    middleware.LimitUserPermissionMiddleware("createAPIKey", PermissionUser) │
│      → 校验 ctx 中 permission >= user，不满足返回 403                       │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 3. Handler 层 (入口 + 出口适配)                                             │
│    handler/apikey.go → HandleCreateAPIKey(ctx, req)                         │
│                                                                             │
│    入参:                                                                    │
│      ctx  → context.Context (含 TraceID, UserID, UserName, Permission)      │
│      req  → *dto.CreateAPIKeyReq  (Huma 自动将 JSON Body 反序列化到此结构)  │
│              req.Body.Name = "my-key"                                       │
│                                                                             │
│    Handler 的职责:                                                          │
│      a. 从 ctx 提取调用者身份:                                              │
│           userID := util.CtxValueUint(ctx, CtxKeyUserID)  // → 42           │
│                                                                             │
│      b. 构造应用层命令 (原始类型 → 有类型的 Command):                       │
│           cmd := command.IssueAPIKeyCommand{UserID: 42, Name: "my-key"}     │
│                                                                             │
│      c. 委托给应用层:                                                       │
│           result, err := h.issue.Handle(ctx, cmd)                           │
│                                                                             │
│      d. 将应用层结果映射为 HTTP DTO:                                        │
│           rsp.Key = &dto.APIKeyDetail{                                      │
│               ID: result.KeyID, Name: result.Name,                          │
│               Key: result.Secret, CreatedAt: result.CreatedAt}              │
│                                                                             │
│      e. 返回统一响应:                                                       │
│           return util.WrapHTTPResponse(rsp, nil)                            │
│                                                                             │
│    Handler 不包含:                                                          │
│      ✗ 业务规则 (配额校验 → 聚合根)                                         │
│      ✗ 数据库操作 (CRUD → 仓储)                                             │
│      ✗ 密钥生成算法 (crypto/rand → 领域服务)                                │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 4. Application 层 (用例编排)                                                │
│    application/apikey/command/issue_api_key.go → IssueAPIKeyHandler.Handle  │
│                                                                             │
│    入参:                                                                    │
│      cmd := IssueAPIKeyCommand{UserID: 42, Name: "my-key"}                  │
│                                                                             │
│    编排流程 (Command Handler 是流程导演，不演具体角色):                     │
│                                                                             │
│    ┌─ Step 4a: 跨域校验 (通过适配器接口)                                    │
│    │   userExistsCh.Exists(ctx, 42)  →  true                                │
│    │   若 false → 返回 ierr.ErrDataNotExists                                │
│    │                                                                        │
│    ├─ Step 4b: 统计现有数量 (通过仓储)                                      │
│    │   repo.CountByUser(ctx, 42)  →  3 (int64)                              │
│    │                                                                        │
│    ├─ Step 4c: 生成密钥 (通过领域服务)                                      │
│    │   generator.Generate()  →  APIKeySecret{value: "sk-aris-AbC12..."}     │
│    │                                                                        │
│    ├─ Step 4d: 创建聚合根 (工厂方法校验不变量)                              │
│    │   aggregate.IssueProxyAPIKey(                                          │
│    │       42,                              // userID                       │
│    │       APIKeyName("my-key"),            // name → 值对象包装            │
│    │       secret,                          // APIKeySecret 值对象          │
│    │       DefaultAPIKeyQuota(),            // {Max: 10}                    │
│    │       3,                               // existing count               │
│    │   )                                                                    │
│    │   → 聚合内部: 3 < 10 → 允许创建                                        │
│    │   → 返回 *ProxyAPIKey{userID:42, name:"my-key", secret:{...}, ...}     │
│    │                                                                        │
│    └─ Step 4e: 持久化 (通过仓储)                                            │
│        repo.Save(ctx, key)                                                  │
│        → 仓储将聚合映射为 dbmodel.ProxyAPIKey → dao.Create() → PostgreSQL   │
│        → 回填 key.SetID(record.ID) → key.AggregateID() = 15                 │
│                                                                             │
│    返回:                                                                    │
│      *IssueAPIKeyResult{KeyID:15, Name:"my-key", Secret:"sk-aris-AbC12...", │
│                          CreatedAt: time.Time}                              │
│                                                                             │
│    Command Handler 的所有依赖都是接口:                                      │
│      - apikey.APIKeyRepository    (领域层定义, 基础设施层实现)              │
│      - service.APIKeyGenerator    (领域服务接口)                            │
│      - UserExistenceChecker       (跨域适配器接口)                          │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 5. 数据在各层间的类型映射                                                   │
│                                                                             │
│   HTTP Body (JSON bytes)                                                    │
│     │  Huma 反序列化                                                        │
│     ▼                                                                       │
│   dto.CreateAPIKeyReq  (DTO 层)                                             │
│     │  Handler 提取字段                                                     │
│     ▼                                                                       │
│   command.IssueAPIKeyCommand{UserID: uint, Name: string}  (应用层命令)      │
│     │  Command Handler 调用值对象构造函数                                   │
│     ▼                                                                       │
│   vo.APIKeyName("my-key"), vo.APIKeySecret{...}  (领域层值对象)             │
│     │  聚合工厂校验 + 组装                                                  │
│     ▼                                                                       │
│   *aggregate.ProxyAPIKey  (领域层聚合根)                                    │
│     │  Repository.toAPIKeyAggregate() 映射                                  │
│     ▼                                                                       │
│   *dbmodel.ProxyAPIKey  (基础设施层 GORM 模型)                              │
│     │  DAO.Create()                                                         │
│     ▼                                                                       │
│   PostgreSQL 行                                                             │
│                                                                             │
│   返回路径 (逆过程):                                                        │
│   PostgreSQL 行 → dbmodel → aggregate → IssueAPIKeyResult →                 │
│   dto.APIKeyDetail → dto.HTTPResponse (JSON) → 客户端                       │
└─────────────────────────────────────────────────────────────────────────────┘
```

**关键要点：**

| 层 | 数据类型 | 转换方向 | 谁做转换 |
|----|---------|---------|---------|
| Handler | `dto.XxxReq` → `Command/Query` | 原始类型 → 有类型命令 | Handler |
| Application | `Command/Query` → `VO` + `Aggregate` | 命令字段 → 值对象包装 | Command Handler |
| Domain | `Aggregate` 内部 | 不变量校验 + 行为执行 | 聚合根自身 |
| Infrastructure (写) | `Aggregate` ↔ `dbmodel` | 聚合字段 → GORM 字段 | Repository 私有函数 |
| Infrastructure (读) | `dbmodel` → `View` | DAO 结果 → 只读投影 | Query Handler |
| Handler (返回) | `View/Result` → `dto.XxxRsp` | 应用层视图 → HTTP DTO | Handler |

### 1.4 LLM 代理流对比

LLM 代理接口（`/api/openai/v1/chat/completions` 等）的流程更为复杂，因为涉及协议转换和流式传输：

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ LLM 代理请求流 (以 OpenAI ChatCompletion 为例)                              │
└─────────────────────────────────────────────────────────────────────────────┘

Fiber 中间件 → Huma 路由 → APIKeyMiddleware (ctx 注入 apiKeyName)
                                    │
                                    ▼
┌─ Handler ───────────────────────────────────────────────────────────────────┐
│ handler/openai.go → HandleChatCompletion(ctx, req)                          │
│                                                                             │
│   入参: ctx (含 apiKeyName), req (*dto.OpenAIChatCompletionRequest)         │
│         req.Body.Model = "my-gpt-4"                                         │
│         req.Body.Messages = [...]                                           │
│                                                                             │
│   一行委托:                                                                 │
│     return h.useCase.CreateChatCompletion(ctx, req)                         │
│                                                                             │
│   注意: LLM 代理的 Handler 比 CRUD Handler 更薄——连 Command 都不构造，      │
│   直接将整个请求体交给 UseCase。因为流式响应需要直接操作 bufio.Writer。     │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─ Application UseCase ───────────────────────────────────────────────────────┐
│ application/llmproxy/usecase/openai.go → CreateChatCompletion               │
│                                                                             │
│   Step 1: 端点解析 (通过领域服务)                                           │
│     ep, err := resolver.Resolve(ctx,                                        │
│         vo.EndpointAlias("my-gpt-4"),   // 值对象包装                       │
│         enum.ProviderOpenAI,            // 主 Provider                      │
│         enum.ProviderAnthropic)         // 回退 Provider                    │
│     → 查询 endpoint 表: alias="my-gpt-4" + provider="openai" → 命中         │
│     → 返回 *aggregate.Endpoint{alias:"my-gpt-4", provider:openai,           │
│                                creds:{model:"gpt-4o", key:"...", url:"..."}}│
│                                                                             │
│   Step 2: 映射为 transport 层结构                                           │
│     upstream := transport.UpstreamEndpoint{                                 │
│         Model: "gpt-4o", APIKey: "...", BaseURL: "https://api.openai.com"}  │
│                                                                             │
│   Step 3: 请求兼容性处理                                                    │
│     req.Body.max_tokens → max_completion_tokens (OpenAI 特定端点要求)       │
│                                                                             │
│   Step 4: 分流处理                                                          │
│     if ep.Provider() == ProviderAnthropic:                                  │
│         → forwardChatViaAnthropic  (跨协议: Converter 转换 + AnthropicProxy)│
│     else:                                                                   │
│         → forwardChatNative       (同协议: ReplaceModelInBody + OpenAIProxy)│
│                                                                             │
│   Step 5: 流式场景 — 构造 *huma.StreamResponse                              │
│     return util.WrapStreamResponse(func(w *bufio.Writer) {                  │
│         // 5a. 调用 Transport 层发送 HTTP 请求到上游                        │
│         openAIProxy.ForwardChatCompletionStream(ctx, upstream, body,        │
│             func(chunk *ChatCompletionChunk) error {                        │
│                 // 5b. 回调中替换 model 名 (上游真实名 → 客户端别名)        │
│                 chunk.Model = "my-gpt-4"                                    │
│                 // 5c. 写入 SSE 到客户端                                    │
│                 fmt.Fprintf(w, "data: %s\n\n", sonic.Marshal(chunk))        │
│                 return w.Flush()                                            │
│             })                                                              │
│         // 5d. 流结束后写 [DONE]                                            │
│         fmt.Fprintf(w, "data: [DONE]\n\n")                                  │
│         // 5e. 异步提交审计 + 消息存储任务 (通过 Pool)                      │
│         pool.GetPoolManager().SubmitModelCallAuditTask(...)                 │
│         pool.GetPoolManager().SubmitMessageStoreTask(...)                   │
│     })                                                                      │
│                                                                             │
│   对比旧架构: 以上全部逻辑原来在 service/openai.go 的一个 200+ 行方法中，   │
│   现在 UseCase 仍然是编排者，但端点解析 → 领域服务，Transport → 基础设施层，│
│   Converter → 应用层协议转换，职责边界清晰。                                │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
         ┌──────────────────────────┴──────────────────────────┐
         ▼                                                     ▼
┌─ Transport 层 ───────────────┐    ┌─ Converter 层 ────────────────┐
│ infrastructure/transport/    │    │ application/llmproxy/         │
│   /openai.go                 │    │   converter/anthropic.go      │
│                              │    │                               │
│ 构建 HTTP 请求               │    │ OpenAI Request ↔ Anthropic    │
│ 发送到上游 URL               │    │    Request                    │
│ 读取 SSE 字节流              │    │ Anthropic SSE Event           │
│ 解析为 ChatCompletionChunk   │    │    → OpenAI ChatCompletion    │
│ 通过回调交给 UseCase         │    │    Chunk (流式)               │
│                              │    │ Anthropic Message             │
│ 纯传输，不含协议转换         │    │    → OpenAI ChatCompletion    │
│                              │    │    (非流式)                   │
│                              │    │                               │
│                              │    │ 纯 DTO 转换，无状态、无副作用 │
└──────────────────────────────┘    └───────────────────────────────┘
```

**CRUD 接口 vs LLM 代理接口的流程差异：**

| 对比维度 | CRUD 接口 (apikey/session/user) | LLM 代理接口 |
|---------|-----------------------------------|-------------|
| Handler 厚度 | 提取 ctx → 构造 Command/Query → 映射 View → 写响应 | 提取 ctx → 一行委托给 UseCase |
| Application 模式 | Command Handler / Query Handler | UseCase (多对象协作 + 流式回调) |
| 响应方式 | `util.WrapHTTPResponse(rsp, nil)` → JSON | `*huma.StreamResponse` → SSE / 直写 BodyWriter |
| 读路径 | Query Handler 直接走 DAO | Query Handler 直接走 DAO (ListModels/CountTokens 一致) |
| 是否有异步任务 | 无 | 有 (消息存储 + 审计记录投递到协程池) |

重构将原有的单一 `service/` 包拆分为 **7 个限界上下文**：

### 2.1 领域总览

| 限界上下文 | 目录 | 聚合根 | 核心职责 |
|-----------|------|--------|---------|
| **APIKey** | `domain/apikey/` | `ProxyAPIKey` | API Key 签发、吊销、配额管理 |
| **Identity** | `domain/identity/` | `User` | 用户注册、档案更新、权限变更 |
| **Session** | `domain/session/` | `Session` | 会话创建、摘要、评分 |
| **LLMProxy** | `domain/llmproxy/` | `Endpoint` | 模型端点查找（含 Provider 回退） |
| **Conversation** | `domain/conversation/` | `Message`, `Tool` | 消息与工具的去重存储 |
| **ModelCall** | `domain/modelcall/` | `ModelCallAudit` | 模型调用审计记录 |
| **OAuth2** | `domain/oauth2/` | （无聚合根） | OAuth2 平台抽象接口 |

### 2.2 聚合根（Aggregate Root）

所有聚合根实现 `aggregate.Root` 接口并嵌入 `aggregate.Base`：

```go
// domain/common/aggregate/root.go
type Root interface {
    AggregateID() uint
    AggregateType() string
}

type Base struct {
    id uint
}
```

**关键设计原则：**

- **自封装**：所有字段为私有，通过 getter 方法对外暴露（如 `key.UserID()`、`key.Name()`）
- **工厂方法**：创建用 `IssueProxyAPIKey(...)`/`RegisterUser(...)`/`CreateSession(...)` → 在构造函数中校验不变量
- **重建方法**：从仓储加载用 `RestoreProxyAPIKey(...)`/`RestoreUser(...)`/`RestoreSession(...)` → 跳过来回校验
- **行为封装**：权限判断落聚合内（`key.IsOwnedBy(userID)`、`user.UpdateProfile(...)`），不泄漏到 Command Handler

### 2.3 值对象（Value Object）

值对象是不可变的类型包装，携带领域校验和行为：

| 域 | 值对象 | 类型 | 提供的行为 |
|----|--------|------|-----------|
| APIKey | `APIKeyName` | `string` | `IsEmpty()` |
| APIKey | `APIKeySecret` | `struct{value}` | `Raw()` / `Masked()` / `IsEmpty()` — 包裹脱敏逻辑 |
| APIKey | `APIKeyQuota` | `struct{Max}` | `Allows(existing int64) bool` — 配额判断 |
| Identity | `UserName` / `Email` / `Avatar` | `string` | `IsEmpty()` |
| Identity | `TokenPair` | `struct{AccessToken, RefreshToken}` | 纯数据载体 |
| Session | `APIKeyOwner` | `string` | `IsEmpty()` / `String()` |
| Session | `SessionSummary` | `struct{text, error}` | `Text()` / `Error()` / `IsEmpty()` / `Failed()` |
| Session | `SessionScore` | `struct{coherence, depth, value, total...}` | `IsEmpty()` / `Failed()` / 构造时自动计算 `total` |
| LLMProxy | `EndpointAlias` | `string` | `IsEmpty()` / `String()` |
| LLMProxy | `UpstreamCreds` | `struct{Model, APIKey, BaseURL}` | 纯数据载体 |
| Conversation | `Checksum` | `string` | SHA256 消息去重 |
| ModelCall | `TokenBreakdown` | `struct{Input, Output, Cache...}` | Token 统计 |
| ModelCall | `CallLatency` | `struct{firstTokenMs, streamMs}` | 延迟统计 |
| ModelCall | `CallStatus` | `struct{UpstreamStatusCode, ErrorMessage}` | 调用结果 |

**设计决策：** 简单值类型（如 `APIKeyName`）用 `type Xxx string`，含多字段的值对象用私有 struct + 构造函数（如 `APIKeySecret`、`SessionScore`）。前者零分配，后者保证不可变。

### 2.4 仓储接口（Repository Interface）

仓储接口定义在领域层包根文件（如 `domain/apikey/repository.go`），与 GORM 完全解耦：

```go
type APIKeyRepository interface {
    Save(ctx context.Context, key *aggregate.ProxyAPIKey) error
    FindByID(ctx context.Context, id uint) (*aggregate.ProxyAPIKey, error)
    ListByUser(ctx context.Context, userID uint) ([]*aggregate.ProxyAPIKey, error)
    ListAll(ctx context.Context) ([]*aggregate.ProxyAPIKey, error)
    CountByUser(ctx context.Context, userID uint) (int64, error)
    Delete(ctx context.Context, id uint) error
}
```

**约定：**
- `FindByXxx` 未找到时返回 `(nil, nil)`，而非 `gorm.ErrRecordNotFound`
- `Save` 首次持久化后回填聚合根 ID（`key.SetID(record.ID)`）
- 接口只暴露聚合根类型，不泄漏 `dbmodel` 或 DAO

### 2.5 领域服务（Domain Service）

当逻辑不属于任何聚合根时，提取为无状态的领域服务：

| 领域服务 | 接口 | 职责 |
|---------|------|------|
| `APIKeyGenerator` | `Generate() (APIKeySecret, error)` | 使用 rejection sampling 生成密码学安全的 API Key 密钥 |
| `EndpointResolver` | `Resolve(ctx, alias, primary, fallback) (*Endpoint, error)` | 按 primary→fallback Provider 顺序查找模型端点 |
| `TokenSigner` | `EncodeToken(userID) / DecodeToken(token)` | JWT 签发/验证（实现在 `infrastructure/jwt/`） |
| `Platform` (OAuth2) | `GetRedirectURL / GetToken / GetUserInfo` | OAuth2 平台策略接口（实现在 `infrastructure/oauth2/`） |

---

## 3. 应用层设计（CQRS）

### 3.1 命令 / 查询分离

重构后，原来 `service/` 中的每个业务方法被拆分为一个或多个 **Command Handler**（写操作）或 **Query Handler**（读操作）：

**目录结构：**
```
application/
├── apikey/
│   ├── command/
│   │   ├── issue_api_key.go      # IssueAPIKeyHandler
│   │   └── revoke_api_key.go     # RevokeAPIKeyHandler
│   └── query/
│       └── list_api_keys.go      # ListAPIKeysHandler
├── identity/
│   ├── command/
│   │   ├── refresh_tokens.go     # RefreshTokensHandler
│   │   └── update_profile.go     # UpdateProfileHandler
│   └── query/
│       ├── check_user_exists.go  # UserExistenceChecker (跨域适配)
│       └── get_current_user.go
├── session/
│   └── query/
│       ├── session_queries.go    # ListSessionsHandler + GetSessionHandler
│       └── view_builder.go       # BuildOrderedMessages / BuildOrderedTools
├── oauth2/
│   └── command/
│       └── handle_callback.go    # HandleOAuth2CallbackHandler
└── llmproxy/
    ├── usecase/                  # 复杂流程用 UseCase 而非简单 Command/Query
    │   ├── openai.go             # OpenAIUseCase (ChatCompletion + Response API)
    │   ├── anthropic.go          # AnthropicUseCase
    │   └── query.go              # ListModels / CountTokens
    └── converter/                # 协议转换下沉到应用层
        ├── openai.go
        └── anthropic.go
```

### 3.2 Command Handler 模式

以 `IssueAPIKeyHandler` 为例：

```go
type IssueAPIKeyCommand struct {
    UserID uint
    Name   string
}

type IssueAPIKeyHandler interface {
    Handle(ctx context.Context, cmd IssueAPIKeyCommand) (*IssueAPIKeyResult, error)
}
```

**Handler 内部编排流程：**
1. 调用领域服务 `generator.Generate()` 生成密钥
2. 调用仓储 `repo.CountByUser()` 统计已有 Key 数
3. 调用聚合工厂 `aggregate.IssueProxyAPIKey(...)` 校验配额并创建聚合
4. 调用仓储 `repo.Save()` 持久化

**关键：** Command Handler 是编排者，不包含业务规则。业务规则在聚合根（配额校验）和领域服务（密钥生成）中。

### 3.3 Query Handler 模式

查询处理器直接走 DAO 投影，**不重建聚合根**：

```go
type ListSessionsHandler interface {
    Handle(ctx context.Context, q ListSessionsQuery) ([]*SessionSummaryView, *model.PageInfo, error)
}
```

内部直接调用 `dao.SessionDAO.Paginate()`，返回 application 层定义的 View 类型（`SessionSummaryView`），不暴露 `dbmodel` 到 Handler 层。

### 3.4 UseCase 模式（LLM Proxy 专用）

LLM 代理的转发路径涉及多对象协作（Resolver + Transport + Converter + Pool），不适合简单的 Command/Query 模式。引入 `UseCase` 作为薄编排层：

```
OpenAIUseCase.CreateChatCompletion(ctx, req)
  → resolver.Resolve(alias, primary, fallback) → 找到 Endpoint
  → 根据 ep.Provider() 分流：
      - provider=openai  → forwardChatNative（同协议透传）
      - provider=anthropic → forwardChatViaAnthropic（跨协议转换）
  → 回调中写入客户端（SSE/JSON）
  → 异步提交审计任务 + 消息存储任务
```

### 3.5 跨域适配器

`UserExistenceChecker` 接口定义在 `application/apikey/command/`，实现在 `application/identity/query/`：

```go
// apikey 域命令层定义
type UserExistenceChecker interface {
    Exists(ctx context.Context, userID uint) (bool, error)
}
```

这是 DDD 的 **开放主机服务（OHS）** 模式 —— 防止 apikey 域强依赖 identity 域的仓储，通过一个最小接口完成解耦。

---

## 4. 基础设施层重组

### 4.1 新增：仓储实现

`internal/infrastructure/repository/` 包含全部 8 个仓储实现：

| 仓储实现 | 实现接口 | 文件 |
|---------|---------|------|
| `apiKeyRepository` | `apikey.APIKeyRepository` | `api_key_repository.go` |
| `userRepository` | `identity.UserRepository` | `user_repository.go` |
| `sessionRepository` | `session.SessionRepository` | `session_repository.go` |
| `endpointRepository` | `llmproxy.EndpointRepository` | `endpoint_repository.go` |
| `messageRepository` | `conversation.MessageRepository` | `message_repository.go` |
| `toolRepository` | `conversation.ToolRepository` | `tool_repository.go` |
| `auditRepository` | `modelcall.AuditRepository` | `audit_repository.go` |
| `audioDirCreator` | 基础设施内部接口 | `audio_dir_creator.go` |

**每个仓储实现的职责：**
- 封装 `dbmodel` ↔ `aggregate` 的双向映射（通过 `toXxxAggregate()` 私有函数）
- GORM 错误（如 `ErrRecordNotFound`）转换为约定的 `(nil, nil)`
- 所有数据库错误通过 `ierr.Wrap(ierr.ErrDBXxx, err, "context")` 统一包装

### 4.2 迁移：Transport 层

```
internal/proxy/  →  internal/infrastructure/transport/
```

语义上，"Proxy" 是传输层的正确描述（HTTP + SSE 通信），迁移到 infrastructure 目录下。`transport.UpstreamEndpoint` 保留原 `proxy.UpstreamEndpoint` 的结构，仅做包路径调整。

### 4.3 迁移：Agent / JWT / OAuth2

```
internal/agent/   →  internal/infrastructure/agent/
internal/jwt/     →  internal/infrastructure/jwt/
internal/oauth2/  →  internal/domain/oauth2/service/  (接口)
                  +  internal/infrastructure/oauth2/   (实现)
```

OAuth2 的拆分尤为关键：领域层的 `Platform` 接口定义在 `domain/oauth2/service/platform.go`，GitHub/Google 的实现留在 `infrastructure/oauth2/`，实现了依赖倒置。

---

## 5. 包迁移汇总

| 原路径 | 新路径 | 变更类型 |
|--------|--------|---------|
| `internal/service/` | **删除** | 逻辑分散到 domain + application |
| `internal/proxy/` | `internal/infrastructure/transport/` | 移动（纯传输层） |
| `internal/converter/` | `internal/application/llmproxy/converter/` | 移动（应用层协议转换） |
| `internal/agent/` | `internal/infrastructure/agent/` | 移动 |
| `internal/jwt/` | `internal/infrastructure/jwt/` | 移动 |
| `internal/oauth2/` | `internal/domain/oauth2/service/` + `internal/infrastructure/oauth2/` | 拆分（接口下沉领域层） |

---

## 6. 调用链对比

### 6.1 重构前（MVC）

```
Handler
  → handler.NewAPIKeyHandler()
  → 直接调用 service.NewAPIKeyService()
  → service 内部调用 dao.GetProxyAPIKeyDAO()
  → 一行代码里混合校验 + 生成 + 存储 + 日志
```

### 6.2 重构后（DDD + CQRS）

```
Handler (handler.NewAPIKeyHandler)
  → 获取 ctx values (userID, permission)
  → 构造 IssueAPIKeyCommand / RevokeAPIKeyCommand / ListAPIKeysQuery
  → 调用 application.{command|query}.Handle(ctx, cmd)
    → Command:
        → domain service: generator.Generate()     # 生成密钥
        → repository: repo.CountByUser(...)         # 查已有数量
        → aggregate: IssueProxyAPIKey(...)           # 校验配额、创建聚合
        → repository: repo.Save(...)                # 持久化
    → Query:
        → repository: repo.ListByUser/ListAll(...)  # 直接查询
        → 映射为 View → 返回
  → 映射 View 到 DTO → 写响应
```

---

## 7. 核心设计原则

### 7.1 依赖倒置

```
领域层定义接口 ────────────────┐
                              │ 实现
                              ▼
                    基础设施层提供 GORM 实现
```

`domain/apikey/repository.go` 定义 `APIKeyRepository` → `infrastructure/repository/api_key_repository.go` 实现。领域层完全不引用 GORM、DAO 或数据库相关包。

### 7.2 聚合根的自封装

所有字段为私有，仅暴露只读访问器。可变操作通过具名方法（`UpdateProfile`、`RecordLogin`、`ChangePermission`），保证一次修改就是一个完整的业务动作。

### 7.3 工厂与重建分离

- **工厂方法**（如 `IssueProxyAPIKey`）：校验所有不变量，返回新聚合
- **重建方法**（如 `RestoreProxyAPIKey`）：直接赋值，不加校验（数据来自数据库，已校验过）

### 7.4 CQRS 读/写分离

- **Command**：重建聚合 → 调用聚合方法 → 仓储 Save
- **Query**：绕过聚合，直接通过 DAO 投影到 View（读模型不需重建完整的聚合根）

---

## 8. 测试影响

新增的测试目录：

| 测试 | 验证内容 |
|------|---------|
| `test/domain_apikey/` | `IssueProxyAPIKey` 的配额和空名校验 |
| `test/domain_conversation_vo/` | 值对象序列化/反序列化 |
| `test/endpoint_resolver/` | `EndpointResolver.Resolve` 的主/回退查询 |
| `test/oauth2_initiate/` | OAuth2 平台接口 |

现有 `test/session_service/` 的函数测试已适配新的 Handler 接口。

---

## 9. 后续计划

本次重构（Step 1）的核心目标是建立领域模型和分层结构，以下内容在后续迭代中推进：

- **事件驱动**：`aggregate.Base` 预留了事件记录 API，但当前不生产领域事件
- **Conversation 域 Command Handler**：Message/Tool 聚合的命令侧（创建、去重）仍通过 Pool 异步任务实现，Will be extracted to application commands in后续步骤
- **更细粒度的域服务**：部分 UseCase 中的辅助函数可以进一步下潜为领域服务

---

## 10. 快速参考：如何新增一个功能

以"新增一个用户功能"为例：

1. **领域层**（如果需要新的业务规则）：
   - `domain/xxx/aggregate/` — 定义聚合根及其行为方法
   - `domain/xxx/vo/` — 定义值对象
   - `domain/xxx/repository.go` — 定义仓储接口
   - `domain/xxx/service/` — 定义领域服务接口

2. **应用层**：
   - `application/xxx/command/` — Command + Handler（写操作）
   - `application/xxx/query/` — Query + View + Handler（读操作）

3. **基础设施层**：
   - `infrastructure/repository/` — 实现仓储接口

4. **Handler 层**：
   - `handler/xxx.go` — 提取上下文 → 构造 Command/Query → 调用 Application Handler → 映射 DTO

5. **路由**：
   - `router/xxx.go` — 注册路由和中间件
