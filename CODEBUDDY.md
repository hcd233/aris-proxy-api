# CODEBUDDY.md

本文件为 CodeBuddy 在此仓库中工作时提供指引。

## 构建与运行命令

```bash
# 构建二进制
make build                # 生产构建（strip 符号）
make build-dev            # 开发构建（保留调试信息）
make build-debug          # 带完整调试信息的构建（用于 dlv 调试）

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

- **环境变量**（Viper `AutomaticEnv()`）：服务设置、数据库凭证、OAuth2、JWT、存储池配置。从 `env/api.env` 加载，模板在 `env/api.env.template`。Key 使用 `_` 分隔（如 `POSTGRES_HOST`）。
- **YAML 配置**（`config/config.yaml`）：LLM 代理模型路由和 API Key。使用 `::` 作为 Key 分隔符以兼容包含 `.` 的模型名（如 `gpt-4.1`）。模板在 `config/config.yaml.tamplate`。

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
/api/v1/oauth2/{provider}/login        — OAuth2 登录（限流）
/api/v1/oauth2/{provider}/callback     — OAuth2 回调（限流）
/api/v1/user/current                   — 当前用户（JWT 认证）
/api/v1/user/                          — 更新用户（JWT + 权限校验）
/api/openai/v1/models                  — OpenAI 模型列表（API Key 认证）
/api/openai/v1/chat/completions        — OpenAI 聊天补全（API Key 认证）
/api/anthropic/v1/models               — Anthropic 模型列表（API Key 认证）
/api/anthropic/v1/messages             — Anthropic 消息（API Key 认证）
```

### 分层职责

- **`cmd/`** — Cobra CLI 命令（`server start`、`database migrate`、`object bucket create`、`cron`）
- **`internal/api/`** — Fiber App 和 Huma API 单例。Fiber 使用 Sonic 作为 JSON 编解码器。Huma 注册 `jwtAuth` 和 `apiKeyAuth` 两种安全方案
- **`internal/router/`** — 按领域分组的路由注册。每个文件绑定中间件和 Handler，使用 `huma.Register()` 注册操作
- **`internal/handler/`** — 每个 Handler 为**接口**，私有结构体实现，通过 `NewXxxHandler()` 创建。Handler 持有 Service 引用，用 `util.WrapHTTPResponse()` 包装响应
- **`internal/service/`** — 业务逻辑层。LLM 代理 Service 处理上游请求构建、SSE 流式转发、模型名替换和异步消息存储
- **`internal/infrastructure/database/dao/`** — 泛型 `baseDAO[ModelT]` 提供类型安全的 CRUD、分页和批量操作。所有 DAO 为单例。软删除使用 `deleted_at`（int64 时间戳，0 = 未删除）
- **`internal/infrastructure/database/model/`** — GORM 模型。`BaseModel` 包含 ID/CreatedAt/UpdatedAt/DeletedAt。关键模型：User、Message（存储 UnifiedMessage JSON + SHA256 CheckSum）、Session（跟踪 APIKeyName + MessageIDs + ToolIDs）、Tool（存储 UnifiedTool JSON + CheckSum）、ModelEndpoint、ProxyAPIKey、Param
- **`internal/dto/`** — 请求/响应 DTO。包含完整的 OpenAI 和 Anthropic API 类型定义，以及 `UnifiedMessage` 和 `UnifiedTool` 跨 Provider 格式及双向转换函数
- **`internal/middleware/`** — JWT 解码 Token 并注入 userID/userName/permission。APIKey 中间件从代理配置构建反向索引验证 Key。RateLimiter 使用 Redis + `ulule/limiter`。Lock 中间件使用 Redis SETNX + Lua 脚本原子解锁
- **`internal/infrastructure/pool/`** — `PoolManager` 管理 Pond v2 协程池。消息存储通过 SHA256 CheckSum 去重（批量 IN 查询），事务中创建 records
- **`internal/agent/`** — 可复用的 LLM/Agent 能力（Session 总结器、评分器）
- **`internal/cron/`** — 定时任务调度（会话去重、总结、评分、软删除清理）
- **`internal/config/`** — Viper 环境变量配置加载
- **`internal/common/constant/`** — 项目常量
- **`internal/common/enum/`** — 公共枚举
- **`internal/common/ierr/`** — 统一错误创建和处理包
- **`internal/common/model/`** — 通用数据模型
- **`internal/enum/`** — 业务枚举
- **`internal/jwt/`** — JWT 签发和验证单例
- **`internal/lock/`** — Redis 分布式锁
- **`internal/logger/`** — Zap 结构化日志（多输出 tee + Lumberjack 轮转）
- **`internal/oauth2/`** — OAuth2 策略模式实现（GitHub、Google）
- **`internal/util/`** — 工具函数（上下文、哈希、HTTP、SSE、字符串、用户、OpenAI/Anthropic 适配）

### 认证机制

两种认证按路由应用：
- **JWT**（`Authorization: Bearer <token>`）— 用户路由。双 Token：AccessToken（短期）+ RefreshToken（长期）。OAuth2 登录后签发
- **API Key**（`Authorization: Bearer <api-key>`）— LLM 代理路由。Key 定义在 `config.yaml`

### LLM 代理流

```
Client → /api/openai/v1/chat/completions (model=my-alias)
  → APIKeyMiddleware 验证 Key
  → Service 查找 my-alias → 找到上游 endpoint 配置
  → 兼容性处理（如 max_tokens → max_completion_tokens）
  → 序列化请求，替换 model 为上游实际名称
  → 构建 HTTP 请求，附加上游 Authorization 头
  → 转发至上游 LLM 提供者
  → 流式：逐行读取 SSE → 替换 model 名 → 转发客户端 → 收集 chunks → 合并完整消息
  → 非流式：读取响应 → 替换 model 名 → 返回
  → 异步：转换为 UnifiedMessage → 通过 Pool 存储（CheckSum 去重）
```

### 核心设计模式

1. **接口驱动**：Handler、Service、DAO、TokenSigner、Locker、ObjDAO 均定义接口
2. **单例模式**：Fiber App、Huma API、DB、Redis、DAO、JWT Signers、PoolManager
3. **泛型 DAO**：`baseDAO[ModelT]` 使用 Go 泛型
4. **策略模式**：OAuth2 平台切换、对象存储平台切换
5. **统一消息格式**：OpenAI/Anthropic DTO → UnifiedMessage/UnifiedTool 跨 Provider 存储
6. **异步协程池**：LLM 请求后的消息存储通过 Pool，SHA256 CheckSum 去重
7. **上下文感知日志**：`logger.WithCtx(ctx)` / `logger.WithFCtx(fctx)` 自动附加 traceID、userID、userName

### 核心指令

1. **禁止**使用 `json.RawMessage` 或 `any`/`interface{}`
2. **修改 OpenAI 或 Anthropic DTO 前**，必须先查看 `/docs` 中的文档

### 关键依赖

- **Go 1.25.1**
- **Fiber v2** + **Huma v2**：HTTP 框架 + OpenAPI 类型化 Handler
- **GORM** + PostgreSQL：ORM 和数据库
- **Redis**（go-redis）：缓存、限流器后端、分布式锁
- **Sonic**：高性能 JSON
- **Cobra / Viper**：CLI 和配置
- **Zap** + Lumberjack：结构化日志
- **MinIO / Tencent COS**：对象存储
- **golang-jwt**：JWT 签发和验证
- **Pond v2**：异步任务协程池
- **robfig/cron/v3**：定时任务调度
- **ulule/limiter**：Redis 限流
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
