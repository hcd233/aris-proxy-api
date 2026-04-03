# CODEBUDDY.md This file provides guidance to CodeBuddy when working with code in this repository.

## Build & Run Commands

```bash
# Build binary
go build -o aris-proxy-api main.go

# Run locally
go run main.go server start --host localhost --port 8080

# Database migration
go run main.go database migrate

# Object storage bucket creation
go run main.go object bucket create

# Install dependencies
go mod download

# Run all tests
go test ./...

# Run single test
go test -v -run TestName ./internal/service/

# Docker (full stack with PostgreSQL + Redis + MinIO)
docker compose -f docker/docker-compose-full.yml up -d

# Docker (production single service)
docker compose -f docker/docker-compose-single.yml up -d
```

## Architecture

This is a Go LLM proxy API that unifies upstream LLM providers (OpenAI, Anthropic) behind a single API, with user management via OAuth2/JWT. It uses **Fiber v2** as the HTTP framework with **Huma v2** layered on top for type-safe handlers and automatic OpenAPI 3.1 spec generation.

### Startup Sequence

Entry point is `main.go` → `cmd.Execute()`. The `server start` command in `cmd/server.go` orchestrates initialization:

1. `database.InitDatabase()` — PostgreSQL via GORM
2. `cache.InitCache()` — Redis client
3. `proxy.InitLLMProxyConfig()` — YAML proxy config for model routing
4. `pool.InitPoolManager()` — Pond v2 goroutine pools
5. Register global Fiber middleware (Recover → fgprof → CORS → Compress → Trace → Log)
6. Conditionally register `/docs` route (non-production only, controlled by `internal/enum/env.go`)
7. `router.RegisterAPIRouter()` — All API routes
8. `app.Listen()` — Start serving

### Two-Tier Configuration

- **Environment variables** (Viper `AutomaticEnv()`): Server settings, database credentials, OAuth2, JWT, storage, pool config. Loaded from `env/api.env`. Template at `env/api.env.template`. Keys use `_` separator (e.g., `POSTGRES_HOST`).
- **YAML config** (`config/config.yaml`): LLM proxy model routing and API keys. Uses `::` as key separator to avoid conflicts with model names containing `.` (e.g., `gpt-4.1`). Template at `config/config.yaml.tamplate`.

### Request Flow

```
Fiber (HTTP) → Global Middleware → Huma Router → Route-Group Middleware → Handler → Service → DAO/Proxy → PostgreSQL/Redis/Upstream LLM
```

**Two-level middleware architecture:**
- **Fiber-level** (`fiber.Handler`): Recover, fgprof, CORS, Compress, Trace, Log — applied globally
- **Huma-level** (`func(huma.Context, func(huma.Context))`): JWT, APIKey, RateLimiter, Permission, Lock — applied per route/group

### Route Structure

```
/health, /ssehealth              — Health checks (no auth)
/api/v1/token/refresh            — Token refresh (no auth)
/api/v1/oauth2/{provider}/login  — OAuth2 login (rate limited)
/api/v1/oauth2/{provider}/callback — OAuth2 callback (rate limited)
/api/v1/user/current             — Current user (JWT auth)
/api/v1/user/                    — Update user (JWT + permission)
/api/openai/v1/models            — OpenAI model list (API Key auth)
/api/openai/v1/chat/completions  — OpenAI chat (API Key auth)
/api/anthropic/v1/models         — Anthropic model list (API Key auth)
/api/anthropic/v1/messages       — Anthropic messages (API Key auth)
```

### Layer Responsibilities

- **`cmd/`** — Cobra CLI commands (`server start`, `database migrate`, `object bucket create`)
- **`internal/api/`** — Fiber app and Huma API singletons, initialized via `init()`. Fiber uses Sonic as JSON codec. Huma registers two security schemes: `jwtAuth` and `apiKeyAuth`.
- **`internal/router/`** — Route registration grouped by domain. Each file wires middleware and handlers. Routes use `huma.Register()` with operation configs specifying per-route middleware.
- **`internal/handler/`** — Each handler is an **interface** with a private struct implementation created via `NewXxxHandler()`. Handlers hold service references and wrap responses with `util.WrapHTTPResponse()`. Streaming responses (SSE/LLM) return `*huma.StreamResponse`.
- **`internal/service/`** — Business logic. LLM proxy services handle upstream request construction, SSE streaming relay, model name replacement, and async message storage.
- **`internal/infrastructure/database/dao/`** — Generic `baseDAO[ModelT]` provides type-safe CRUD, pagination, and batch operations. Concrete DAOs embed it. All DAOs are singletons. Soft delete uses `deleted_at` (int64 timestamp, 0 = not deleted).
- **`internal/infrastructure/database/model/`** — GORM models. `BaseModel` has ID/CreatedAt/UpdatedAt/DeletedAt. Key models: User, Message (stores UnifiedMessage JSON + SHA256 CheckSum), Session (tracks APIKeyName + MessageIDs + ToolIDs as JSON arrays), Tool (stores UnifiedTool JSON + CheckSum).
- **`internal/dto/`** — Request/response DTOs. Includes full OpenAI and Anthropic API type definitions, plus `UnifiedMessage` and `UnifiedTool` cross-provider formats with bidirectional conversion functions. `MessageContent` uses custom JSON marshal/unmarshal for union types (string | array).
- **`internal/middleware/`** — JWT decodes token and injects userID/userName/permission into context. APIKey middleware builds a reverse index from proxy config to validate keys and inject userName. RateLimiter uses Redis via `ulule/limiter`. Lock middleware uses Redis SETNX + Lua script atomic unlock.
- **`internal/proxy/`** — `LLMProxyConfig` singleton loaded from YAML. Core structure: `APIKeys` (map[name]key for proxy's own keys) and `Models` (map[alias]ModelConfig). Each `ModelConfig` maps to endpoints per `ProviderType` (openai/anthropic), containing upstream APIKey and BaseURL.
- **`internal/infrastructure/pool/`** — `PoolManager` manages Pond v2 goroutine pools: `pingPool` and `messageStorePool`. Message storage task deduplicates via SHA256 CheckSum (batch IN query), then creates messages/tools/sessions in a transaction. Uses `util.CopyContextValues()` to safely pass context to async tasks.

### LLM Proxy Flow

```
Client → /api/openai/v1/chat/completions (model=my-alias)
  → APIKeyMiddleware validates key from proxy config
  → Service looks up my-alias → finds openai endpoint config
  → Handles compatibility (e.g., max_tokens → max_completion_tokens)
  → Serializes request, replaces model with upstream actual name
  → Builds HTTP request with upstream Authorization header
  → Forwards to upstream LLM provider
  → Streaming: reads SSE line-by-line → replaces model name → relays to client → collects chunks → merges into complete message
  → Non-streaming: reads response → replaces model name → returns
  → Async: converts to UnifiedMessage → stores via Pool (dedup by CheckSum)
```

Anthropic proxy follows the same pattern but uses `x-api-key` header, `/v1/messages` path, `anthropic-version` header, and different SSE event format (event + data dual-line).

### Authentication

Two auth mechanisms applied per-route:
- **JWT** (`Authorization: Bearer <token>`) — User routes. Dual token: AccessToken (short-lived) + RefreshToken (long-lived), different secrets and expiry. Issued after OAuth2 login (GitHub/Google via strategy pattern in `internal/oauth2/`).
- **API Key** (`Authorization: Bearer <api-key>`) — LLM proxy routes. Keys defined in `config.yaml`.

### Key Patterns

1. **Interface-driven**: Handler, Service, DAO, TokenSigner, Locker, ObjDAO, Platform all define interfaces
2. **Singleton pattern**: Fiber App, Huma API, DB, Redis, DAOs, JWT Signers, LLMProxyConfig, PoolManager — all initialized once
3. **Generic DAO**: `baseDAO[ModelT]` with Go generics for type-safe CRUD
4. **Strategy pattern**: OAuth2 platform switching (`Platform` interface), object storage platform switching (COS priority > MinIO)
5. **Unified message format**: OpenAI/Anthropic DTOs → UnifiedMessage/UnifiedTool for cross-provider storage
6. **Async goroutine pool**: Post-LLM-request message storage via Pond, deduplication by SHA256 CheckSum
7. **Context-aware logging**: `logger.WithCtx(ctx)` and `logger.WithFCtx(fctx)` auto-attach traceID, userID, userName

### Key Dependencies

- **Fiber v2** + **Huma v2**: HTTP framework + OpenAPI typed handlers
- **GORM** + PostgreSQL: ORM and database
- **Redis** (go-redis): Caching, rate limiter backend, distributed locks
- **Sonic**: High-performance JSON (replaces encoding/json in Fiber)
- **Cobra/Viper**: CLI and configuration
- **Zap** + Lumberjack: Structured logging with multi-output tee (console + info/error/panic files with rotation)
- **MinIO / Tencent COS**: Object storage (abstracted via ObjDAO interface)
- **golang-jwt**: JWT token generation and validation
- **Pond v2**: Goroutine pool for async tasks
- **ulule/limiter**: Redis-backed rate limiting
- **samber/lo**: Functional utilities for Go

### CORE INSTRUCTION

1. **DO NOT** use json.RAWMessage OR any(interface{})
2. **ALWAYS** check the document in `/docs` before you modify dto of openai or anthropic

---

## Testing（测试规范）

### 测试目录结构

**所有测试文件（`*_test.go`）必须且只能放在 `test/` 目录下，`internal/` 目录内禁止存放任何测试文件。**

```
test/                              # 所有测试文件的唯一存放位置
├── <主题名>/                       # 按测试主题组织，snake_case 命名
│   ├── fixtures/                  # 测试数据文件（必须放在 fixtures/ 子目录）
│   │   └── cases.json
│   └── xxx_test.go                # 测试代码，package 名与目录名一致
└── ...

internal/                          # 禁止存放任何 *_test.go 文件
├── dto/
│   └── session.go                 # 仅源码，无测试
├── service/
│   └── session.go                 # 需要被测试的辅助函数必须导出
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
    // ...其他字段
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
// ✅ 好：清晰的失败信息，包含 got / want 上下文
if got != want {
    t.Errorf("ComputeChecksum() = %s, want %s", got, want)
}

// ✅ 好：使用子测试隔离
t.Run("empty input", func(t *testing.T) {
    result := ComputeChecksum(nil)
    if result != "" {
        t.Errorf("expected empty string, got %q", result)
    }
})

// ❌ 差：无上下文的断言
if result != expected {
    t.Fatal("not equal")
}
```

### ⚠️ 开发流程强制要求（MANDATORY）

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

### 🚫 测试禁止事项

1. **禁止在 `internal/` 任何子目录中创建 `*_test.go` 文件** — 所有测试只能放在 `test/` 目录
2. **禁止提交不通过的测试** — 所有测试必须 PASS 才能提交
3. **禁止删除已有的测试用例** — 除非对应功能已删除
4. **禁止在测试中硬编码环境相关路径** — 使用 `t.TempDir()`、`os.CreateTemp()` 等
5. **禁止测试间相互依赖** — 每个测试必须独立可运行
6. **禁止使用 `time.Sleep()` 做同步** — 使用 channel、WaitGroup 或 deadline
7. **禁止使用 `encoding/json`** — 统一使用 `github.com/bytedance/sonic`
8. **禁止使用第三方断言库（如 testify）** — 使用标准库 `testing` 包的 `t.Errorf` / `t.Fatalf`
9. **禁止在测试代码中内联构造测试数据** — 所有数据必须来自 `fixtures/` 文件

### 常用命令

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

## Development Workflow（开发流程）

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
- Service 不直接依赖基础设施实现
- 所有业务方法第一个参数 `context.Context`
- 单例通过 `GetXxx()` 获取

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