# 订阅账号 OAuth 池化实施计划（Claude Code / Codex）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 Claude（Claude Code）与 ChatGPT（Codex）订阅账号通过 OAuth 授权接入 aris，成为一类新上游 Endpoint，复用现有调度/审计/数据治理/计价，并提供额度展示。

**Architecture:** Endpoint 增加 `auth_type`（`api_key` / `oauth_anthropic` / `oauth_openai`）；OAuth 凭据（access/refresh token）存独立表 `endpoint_oauth_credentials`，AES-256-GCM 加密、repository 层加解密；access token 由 domain 接口 `OAuthTokenProvider` 提供（请求路径按需刷新 + Redis 缓存 + 分布式锁），失败返回可切换错误让现有 fallback 接管；上游 headers/body 特化集中在 transport 的 `sendRequestOnce` / `sendUpstreamRequestOnce` 两个发送函数（`transport/oauth_upstream.go`），`vo.UpstreamEndpoint` 新增 `StableKey` 防止熔断键随 token 轮换漂移。

**Tech Stack:** Go 1.25（Fiber v3 / Huma v2 / GORM / go-redis / sonic）、AES-256-GCM（标准库 crypto）、Next.js 16 前端（React 19 + Tailwind 4）。

**Spec:** `docs/superpowers/specs/2026-10-18-oauth-subscription-pooling-design.md`（下称"规格"；执行者必须先读规格，本计划中的章节号引用均指向它）

## Global Constraints

- **执行前**：创建 worktree 与分支（`feature/oauth-subscription-pooling-2026-10-18`，规范见 `docs/agents/workflow.md`）；每个 Go 文件编辑前按 `docs/agents/workflow.md`「Go 后端 Skill 加载清单」加载 skill 并跑 `use-modern-go list`（禁止截断输出）。
- 单测目录 `test/unit/<topic>/`、e2e 目录 `test/e2e/<topic>/`；`*_test.go` 不得出现在 `internal/` 下；只用标准库 `testing`（禁 testify/gomock）；禁止 `time.Sleep` 做同步。
- 业务错误统一 `internal/common/ierr`（禁 `fmt.Errorf`/`errors.New`）；单测断言错误用 `errors.As`/`errors.Is` 针对 sentinel。
- 统一 `github.com/bytedance/sonic`（禁 `encoding/json`）；DTO/生产代码禁 `any`/`interface{}`（测试内局部临时容器除外）。
- 业务包禁本地 `const` 块：常量进 `internal/common/constant/`（本文新增字面量集中在 `constant/oauth.go`），枚举进 `internal/common/enum/`，Redis key 模板进 `constant/string.go`。
- 路由规范：`/{resource}/{action}`；按 ID 的详情/动作**必须走 `?id=` 查询参数**（不用路径段）。分组前缀：`/api/web/v1/endpoint`。
- 日志 `logger.WithCtx(ctx)`，消息前缀 `[PascalCaseModule]`；凭据一律 `util.MaskSecret()`，**token 不得进日志**。
- HTTP 状态码用 `fiber.StatusXxx`；DTO 时间字段用 `time.Time`。
- 提交：Conventional Commits + 中文描述（如 `feat(oauth): 订阅账号授权流与凭据加密存储`）。
- 验证命令：`make lint`、`go test -v -count=1 -run TestXxx ./test/unit/<topic>/`、`make test`（全量）。
- 上游特化字面量（client_id、URL、UA、beta、system 文案）**只出现于** `internal/common/constant/oauth.go` 与 `internal/infrastructure/transport/oauth_upstream.go`，禁止散落。

## Review Focus

最容易在真实使用中出问题、且规格未显式写明的输入类/失败模式（各自有归属测试，见括号内任务）：

1. **刷新失败后用户可见行为**：凭据 `reauth_required` 时端点应从候选消失（fallback 接管）、管理端出现"需重新授权"提示，而不是所有请求静默 404（Task 8/9 测试 + Task 15 UI）。
2. **熔断键稳定性**：access token 轮换后同一端点的熔断/信号量状态与指标 label 保持稳定（Task 10 测试）。
3. **凭据不泄漏**：`x-api-key`（aris 的 key）在 OAuth 转发中不得到达上游；token 不得出现在 API 响应与日志中（Task 11 测试 + Task 9 demo 掩码断言）。
4. **Codex 端点非法输入**：`stream=false` 的 Responses 请求与 Chat 请求（无转换路径）必须得到明确错误，而不是挂起或 500（Task 12 测试 + Task 9 路由语义）。
5. **授权流超时与重放**：flow 超过 10 分钟过期、state 不匹配、重复 exchange 必须被拒绝（Task 6/7 测试）。

---

## File Structure（总览）

| 区域 | 文件 | 职责 |
|---|---|---|
| 枚举/常量 | `internal/common/enum/endpoint_auth_type.go`、`enum/oauth_provider.go`、`constant/oauth.go`（新） | 认证类型、provider 标识、全部上游字面量 |
| 数据模型 | `internal/infrastructure/database/model/endpoint.go`（改）、`endpoint_oauth_credential.go`（新）、`base.go`（注册） | 表结构 |
| DAO/仓储 | `internal/infrastructure/database/dao/endpoint_oauth_credential.go`、`internal/infrastructure/repository/oauth_credential_repository.go`、`endpoint_repository.go`（改） | 持久化 + 加解密 |
| 领域 | `internal/domain/llmproxy/aggregate/endpoint.go`（改）、`aggregate/oauth_credential.go`（新）、`vo/quota_snapshot.go`（新）、`vo/upstream_endpoint.go`（改）、`service/resolver.go`（改）、`service/oauth_provider.go`（新）、`service/oauth_token_provider.go`（新）、`repository.go`（改） | 聚合/值对象/领域服务接口 |
| OAuth 实现 | `internal/infrastructure/oauthprovider/anthropic.go`、`openai.go`、`token_provider.go`（新） | 两 provider HTTP 客户端 + token 刷新 |
| 应用层 | `internal/application/endpoint/command/oauth_authorize.go`、`oauth_exchange.go`、`oauth_reauth.go`、`oauth_probe.go`、`internal/application/endpoint/query/oauth_quota.go`（新） | 授权/绑定/重授权/探测用例 |
| 传输 | `internal/infrastructure/transport/oauth_upstream.go`（新）、`anthropic.go`/`openai.go`（改）、`guard.go`（改）、`internal/application/llmproxy/usecase/openai.go`（改，`toTransportEndpoint`）、`usecase/common.go`（改，`CanSwitchEndpoint`） | 上游特化 + 稳定键 |
| 路由/处理器 | `internal/router/endpoint.go`（改）、`internal/handler/endpoint.go`（改）、`internal/dto/endpoint.go`（改）、`internal/application/upstream/port/handler.go` + `query/list_upstream.go`（改） | API 契约 |
| Cron | `internal/cron/oauth_maintenance.go`（新）、`cron.go`（注册） | 刷新 + 额度探测 |
| 装配 | `internal/bootstrap/modules/repository.go`、`modules/application.go`、`modules/handler.go`（改） | fx wiring |
| 前端 | `web/src/lib/api-client.ts`、`types.ts`（改）、`web/src/app/(dashboard)/upstream/endpoint-dialog.tsx`、`endpoint-form.ts`、`grouped-view.tsx`、`flat-view.tsx`、`shared.tsx`（改）、`web/src/locales/{zh,en,ja}.json`（改） | UI |
| 文档 | `CONTEXT.md`、`README.md`、`README.zh-CN.md`、`env/api.env.template`（改） | 领域词/能力/配置 |
| 测试 | `test/unit/endpoint-auth/`、`test/unit/oauth-credential/`、`test/unit/oauth-flow/`、`test/unit/oauth-upstream/`、`test/unit/oauth-token/`、`test/e2e/oauth-endpoint/` | 见各任务 |

---

### Task 1: auth_type 贯通（枚举 + 表 + 聚合 + 写命令）

**Files:**
- Create: `internal/common/enum/endpoint_auth_type.go`
- Modify: `internal/infrastructure/database/model/endpoint.go`、`internal/domain/llmproxy/aggregate/endpoint.go`、`internal/infrastructure/repository/endpoint_repository.go`、`internal/application/endpoint/port/handler.go`、`internal/application/endpoint/command/create_endpoint.go`、`internal/application/endpoint/command/update_endpoint.go`、`internal/handler/endpoint.go`
- Test: `test/unit/endpoint-auth/aggregate_test.go`

**Interfaces:**
- Produces:
  - `enum.EndpointAuthType`（string）：`EndpointAuthAPIKey = "api_key"`、`EndpointAuthOAuthAnthropic = "oauth_anthropic"`、`EndpointAuthOAuthOpenAI = "oauth_openai"`；方法 `IsOAuth() bool`。
  - `aggregate.CreateEndpoint(id uint, name, openaiBaseURL, anthropicBaseURL, apiKey string, supportChatCompletion, supportResponse, supportMessage, supportDecision bool, authType enum.EndpointAuthType) (*Endpoint, error)`（新增末参；全部现有调用点传 `enum.EndpointAuthAPIKey`）。
  - `Endpoint.AuthType() enum.EndpointAuthType`；`Endpoint.Update(...)` 新增规则（见测试）。
- Consumes: 现有 `aggregate.RehydrateEndpoint`（签名同样增加末参 `authType`）。

- [ ] **Step 1: Write the failing test**

```go
// test/unit/endpoint-auth/aggregate_test.go
func TestEndpointAuthTypeValidation(t *testing.T) {
    // 1) api_key 类型：OAuth 规则不生效（现有行为不变）
    // 2) oauth_anthropic 类型：Update 改 anthropicBaseURL 或能力标记 → 返回 ierr ErrValidation
    // 3) authType 空串归一为 api_key
    // 4) oauth 类型携带空 apiKey 合法（凭据在 credentials 表）
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v -count=1 -run TestEndpointAuthTypeValidation ./test/unit/endpoint-auth/`
Expected: FAIL（`enum.EndpointAuthAPIKey` / 新签名不存在）

- [ ] **Step 3: Implement**

`endpoint_auth_type.go` 仿 `enum/permission.go` 结构（类型 + 常量 + 注释块）。DB 列：`AuthType string `gorm:"column:auth_type;not null;default:'api_key'"``。聚合 `validate()` 中：当 `authType.IsOAuth()` 时，`apiKey` 允许空、`openaiBaseURL`/`anthropicBaseURL`/能力标记由服务端固定写入（Task 7 负责写值），Update 路径新增拒绝规则：OAuth 类型禁止 `apiKey`/`baseURL`/能力标记参数非 nil（`ierr.ErrValidation`）。`create_endpoint.go` 命令与 handler 传 `enum.EndpointAuthAPIKey`（本任务不改 API 契约：`CreateEndpointReqBody` 不新增字段）。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v -count=1 -run TestEndpointAuthTypeValidation ./test/unit/endpoint-auth/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat(oauth): Endpoint 引入 auth_type（api_key/oauth_anthropic/oauth_openai）"
```

---

### Task 2: 凭据存储与加密（secretbox + 表/DAO/仓储 + 聚合 + QuotaSnapshot）

**Files:**
- Create: `internal/common/util/secretbox.go`、`internal/domain/llmproxy/aggregate/oauth_credential.go`、`internal/domain/llmproxy/vo/quota_snapshot.go`、`internal/infrastructure/database/model/endpoint_oauth_credential.go`、`internal/infrastructure/database/dao/endpoint_oauth_credential.go`、`internal/infrastructure/repository/oauth_credential_repository.go`
- Modify: `internal/infrastructure/database/model/base.go`（`Models` 注册）、`internal/config/config.go`（`OAuthCredentialKey []byte`）、`env/api.env.template`、`internal/domain/llmproxy/repository.go`（仓储接口）
- Test: `test/unit/oauth-credential/secretbox_test.go`、`test/unit/oauth-credential/aggregate_test.go`

**Interfaces:**
- Produces:
  - `util.EncryptSecret(key []byte, plaintext string) (string, error)` / `util.DecryptSecret(key []byte, ciphertext string) (string, error)`（AES-256-GCM，输出 `base64(nonce||ct||tag)`）。
  - `config.OAuthCredentialKey []byte`（viper 键 `oauth.credential_key`，env `OAUTH_CREDENTIAL_KEY`，hex 64 字符；空 = nil）。
  - `aggregate.OAuthCredential`：字段 `endpointID uint`、`provider enum.OAuthProvider`、`accessToken/refreshToken string`（明文，加密是持久化细节）、`expiresAt time.Time`、`accountRef/accountEmail/planType string`、`status enum.OAuthCredentialStatus`（`active`/`reauth_required`）、`lastError string`、`quota vo.QuotaSnapshot`、`quotaUpdatedAt time.Time`；方法：
    - `CreateOAuthCredential(endpointID uint, provider enum.OAuthProvider, access, refresh string, expiresAt time.Time, accountRef, accountEmail, planType string) (*OAuthCredential, error)`
    - `RehydrateOAuthCredential(...) *OAuthCredential`（不校验）
    - `() ApplyTokens(access, refresh string, expiresAt time.Time) `（refresh 为空时保留原值，清 lastError、status=active）、`MarkReauth(errMsg string)`、`SetQuota(q vo.QuotaSnapshot, at time.Time)`、只读访问器。
  - `vo.QuotaSnapshot{Windows []QuotaWindow; PlanType string; CapturedAt time.Time}`、`vo.QuotaWindow{Key string; UsedPercent float64; ResetsAt time.Time; WindowMinutes int}`。
  - `llmproxy.OAuthCredentialRepository`：`FindByEndpointID(ctx, endpointID uint) (*aggregate.OAuthCredential, error)`、`Save(ctx, cred *aggregate.OAuthCredential) error`（upsert by endpoint_id）、`DeleteByEndpointID(ctx, endpointID uint) error`、`ListActive(ctx) ([]*aggregate.OAuthCredential, error)`（供 cron）。
- Consumes: Task 1 的 `enum.EndpointAuthType`。

- [ ] **Step 1: Write the failing tests**

```go
// secretbox_test.go
func TestSecretBoxRoundTrip(t *testing.T)      // 加密→解密得原文；同一明文两次密文不同（nonce 随机）
func TestSecretBoxWrongKeyFails(t *testing.T)  // 错 key 解密返回 error
func TestSecretBoxTamperFails(t *testing.T)    // 篡改密文任一字节 → error（GCM 校验）

// aggregate_test.go
func TestOAuthCredentialLifecycle(t *testing.T) // ApplyTokens 空 refresh 保留旧值/清 lastError；MarkReauth 后 status=reauth_required；SetQuota 写入快照
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -v -count=1 ./test/unit/oauth-credential/`
Expected: FAIL（类型不存在）

- [ ] **Step 3: Implement**

表列（对照规格 §3.2）：`endpoint_id`（uniqueIndex）、`provider`、`access_token_enc`、`refresh_token_enc`、`expires_at`、`account_ref`、`account_email`、`plan_type`、`status`（默认 `active`）、`last_error`、`quota_snapshot`（text + `serializer:json`）、`quota_updated_at`。仓储 `Save` 用 `clause.OnConflict{Columns: endpoint_id, DoUpdates: ...}` upsert；加解密在 `toOAuthCredentialModel`/`toOAuthCredentialAggregate` 两处转换函数中做；**key 为 nil 时创建/读取返回 `ierr.New(ierr.ErrInternal, "OAUTH_CREDENTIAL_KEY 未配置")`**（fail-closed）。`ListActive` 只取 `status=active`。

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v -count=1 ./test/unit/oauth-credential/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat(oauth): 凭据加密存储（AES-256-GCM）与凭据聚合/仓储"
```

---

### Task 3: 读路径投影 oauthStatus + Resolver 过滤

**Files:**
- Modify: `internal/domain/llmproxy/aggregate/endpoint.go`（新增只读投影字段）、`internal/infrastructure/repository/endpoint_repository.go`（`FindByID`/`BatchFindByIDs` 后批量查 credentials.status 填充）、`internal/domain/llmproxy/service/resolver.go`（`collectCandidates` 过滤）
- Test: `test/unit/endpoint-auth/resolver_filter_test.go`

**Interfaces:**
- Produces: `Endpoint.SetOAuthStatus(s enum.OAuthCredentialStatus)`、`Endpoint.OAuthStatus() enum.OAuthCredentialStatus`（零值 = 非 OAuth 或未加载）；`enum.OAuthCredentialStatus`（`active`/`reauth_required`）。
- Consumes: Task 2 的 `OAuthCredentialRepository`（仓储构造注入）。

- [ ] **Step 1: Write the failing test**

```go
func TestResolverSkipsReauthRequired(t *testing.T) {
    // 构造 fake EndpointRepository/OAuthCredentialRepository（interface mock，非 gomock）
    // 1) 端点 A（reauth_required）与 B（active）绑同别名 → ResolveCandidates 只返回 B
    // 2) 仅 A 存在 → 返回 ErrDataNotExists
    // 3) api_key 类型端点不受 status 影响
}
```

- [ ] **Step 2: Run test to verify it fails** → `go test -v -count=1 -run TestResolverSkipsReauthRequired ./test/unit/endpoint-auth/`，Expected: FAIL。

- [ ] **Step 3: Implement**

过滤放在 `collectCandidates` 内（`ep.AuthType().IsOAuth() && ep.OAuthStatus() == enum.OAuthCredentialStatusReauthRequired` → skip）。`FindByID`/`BatchFindByIDs` 在返回前对 OAuth 类型行批量 `SELECT endpoint_id, status` 填充投影（一次 `IN` 查询，空集合不打 SQL）。

- [ ] **Step 4: Run test to verify it passes** → Expected: PASS。

- [ ] **Step 5: Commit** → `git commit -m "feat(oauth): reauth_required 端点从解析候选中剔除"`

---

### Task 4: OAuth 常量 + PKCE/state/code 解析

**Files:**
- Create: `internal/common/constant/oauth.go`、`internal/common/enum/oauth_provider.go`、`internal/common/util/oauth_pkce.go`
- Modify: `internal/common/constant/string.go`（Redis key 模板）
- Test: `test/unit/oauth-flow/pkce_test.go`

**Interfaces:**
- Produces:
  - `enum.OAuthProvider`（`anthropic`/`openai`）+ `ParseOAuthProvider(s string) (enum.OAuthProvider, error)`。
  - `constant.OAuthClaudeClientID`、`OAuthClaudeAuthorizeURL`、`OAuthClaudeTokenURL`、`OAuthClaudeRedirectURI`、`OAuthClaudeScope`、`OAuthOpenAIClientID`、`OAuthOpenAIAuthorizeURL`、`OAuthOpenAITokenURL`、`OAuthOpenAIRedirectURI`、`OAuthOpenAIScope`、`OAuthClaudeBetaHeader = "oauth-2025-04-20"`、`OAuthClaudeUA`、`OAuthCodexOriginator = "codex_cli_rs"`、`OAuthCodexUA`、`OAuthCodexMinCLIVersion = "0.144.0"`、`OAuthClaudeSystemBlock = "You are Claude Code, Anthropic's official CLI for Claude."`、`OAuthFlowTTL = 10 * time.Minute`（值全部对照规格 §4.1 表）。
  - `constant.OAuthFlowKeyTemplate = "oauth:flow:%s"`、`OAuthAccessKeyTemplate = "oauth:access:%d"`、`OAuthRefreshLockKeyTemplate = "oauth:refresh:%d"`（放 `string.go`）。
  - `util.GeneratePKCE(provider enum.OAuthProvider) (verifier, challenge string, err error)`（Claude: 32B base64url；OpenAI: 64B hex；challenge=S256→base64url 无 padding）。
  - `util.GenerateOAuthState() (string, error)`、`util.ParseOAuthCode(input, fallbackState string) (code, state string, err error)`（三形态：URL / `code#state` / 纯 code+fallback，见规格 §4.1）。
- Consumes: 无。

- [ ] **Step 1: Write the failing test**

```go
func TestGeneratePKCE(t *testing.T)        // Claude verifier 43 字符 base64url；OpenAI verifier 128 字符 hex
                                           // challenge = base64url(SHA256(verifier))，用 RFC7636 附录 B 已知向量校验
func TestParseOAuthCode(t *testing.T)      // "http://localhost:1455/auth/callback?code=abc&state=xyz" → (abc, xyz)
                                           // "abc#xyz" → (abc, xyz)；"abc" + fallback "xyz" → (abc, xyz)；"" → error
```

- [ ] **Step 2: Run test to verify it fails** → `go test -v -count=1 -run 'TestGeneratePKCE|TestParseOAuthCode' ./test/unit/oauth-flow/`，Expected: FAIL。

- [ ] **Step 3: Implement**（crypto/rand + crypto/sha256 + encoding/base64、net/url；常量文件带出处注释：`// 参考协议事实，非代码复制`）

- [ ] **Step 4: Run test to verify it passes** → Expected: PASS。

- [ ] **Step 5: Commit** → `git commit -m "feat(oauth): OAuth 常量、PKCE 与授权码解析工具"`

---

### Task 5: OAuth provider 客户端（domain 接口 + 两 provider HTTP 实现）

**Files:**
- Create: `internal/domain/llmproxy/service/oauth_provider.go`（接口）、`internal/infrastructure/oauthprovider/anthropic.go`、`openai.go`
- Test: `test/unit/oauth-flow/provider_test.go`

**Interfaces:**
- Produces:
  - 接口（domain）：`service.OAuthProviderClient`：
    ```go
    Provider() enum.OAuthProvider
    AuthorizeURL(challenge, state string) string
    ExchangeCode(ctx context.Context, code, verifier string) (*service.OAuthTokenSet, error)
    Refresh(ctx context.Context, refreshToken string) (*service.OAuthTokenSet, error)
    ProbeQuota(ctx context.Context, accessToken, accountRef string) (*vo.QuotaSnapshot, error)
    ```
    及 `service.OAuthTokenSet{AccessToken, RefreshToken string; ExpiresAt time.Time; AccountRef, AccountEmail, PlanType string}`。
  - 工厂：`oauthprovider.NewClients() map[enum.OAuthProvider]service.OAuthProviderClient`。
- Consumes: Task 4 常量。

- [ ] **Step 1: Write the failing test**

```go
// 用 httptest.Server 替代真实上游（客户端构造接受可覆盖的 base URL 字段，测试注入）
func TestClaudeExchangeAndRefresh(t *testing.T) // 换 token 请求体含 grant_type/code_verifier；响应映射 expires_in→ExpiresAt、organization.uuid→AccountRef
func TestOpenAIExchangeAndRefresh(t *testing.T) // form 请求体；id_token 解码取 chatgpt_account_id（构造 base64 JWT 片段）
func TestProbeQuotaMapping(t *testing.T)        // Claude five_hour/seven_day、Codex primary/secondary → QuotaSnapshot.Windows
```

- [ ] **Step 2: Run test to verify it fails** → Expected: FAIL。

- [ ] **Step 3: Implement**

按规格 §4.1 表的请求形态（Claude：JSON body；OpenAI：form body + `id_token` 仅 base64 解码不验签）。`AuthorizeURL` 按 provider 拼接 query（Claude 无附加参数；OpenAI 加 `id_token_add_organizations=true&codex_cli_simplified_flow=true`）。额度探测按规格 §4.6（Claude `/api/oauth/usage`、Codex `/backend-api/wham/usage`）。每个客户端持有 `tokenURL/usageURL` 字段（默认取常量，测试可改）。

- [ ] **Step 4: Run test to verify it passes** → Expected: PASS。

- [ ] **Step 5: Commit** → `git commit -m "feat(oauth): Claude/Codex provider 客户端（换 token/刷新/额度探测）"`

---

### Task 6: authorize 命令（授权流 Redis 存储）

**Files:**
- Create: `internal/application/endpoint/command/oauth_authorize.go`、`internal/infrastructure/cache/oauth_flow.go`
- Modify: `internal/domain/llmproxy/service/oauth_provider.go`（如需 flow 值对象）
- Test: `test/unit/oauth-flow/authorize_test.go`

**Interfaces:**
- Produces:
  - `port.AuthorizeOAuthCommand{UserID uint; Provider enum.OAuthProvider}` + `port.AuthorizeOAuthResult{AuthorizeURL string}` + `port.AuthorizeOAuthHandler.Handle(ctx, cmd) (*AuthorizeOAuthResult, error)`。
  - `cache.OAuthFlowStore`：`Put(ctx, state string, flow vo.OAuthFlow) error`、`Take(ctx, state string) (vo.OAuthFlow, bool, error)`（**Take 即取即删**，防重放）、`vo.OAuthFlow{UserID uint; Provider enum.OAuthProvider; Verifier string; CreatedAt time.Time}`（放 `internal/domain/llmproxy/vo/oauth_flow.go`）。
- Consumes: Task 4/5。

- [ ] **Step 1: Write the failing test**

```go
func TestAuthorizeFlowStoredAndSingleUse(t *testing.T) {
    // fake FlowStore（内存实现接口）：Authorize 命令返回 URL 含 code_challenge 与 state
    // Take 一次得 flow，二次 Take 返回 false（防重放）
    // 过期 flow（CreatedAt 早于 TTL）返回 false
}
```

- [ ] **Step 2/4: Run fails / passes**（命令同前，topic `oauth-flow`）

- [ ] **Step 3: Implement**：生成 PKCE + state → `Put`（TTL = `constant.OAuthFlowTTL`）→ 返回 `client.AuthorizeURL(challenge, state)`。

- [ ] **Step 5: Commit** → `git commit -m "feat(oauth): 授权发起命令与一次性 flow 存储"`

---

### Task 7: exchange / reauth 命令（事务创建与更新）

**Files:**
- Create: `internal/application/endpoint/command/oauth_exchange.go`、`oauth_reauth.go`
- Modify: `internal/domain/llmproxy/repository.go`（如需事务内直写接口）、`internal/infrastructure/repository/oauth_credential_repository.go`（事务支持）
- Test: `test/unit/oauth-flow/exchange_test.go`

**Interfaces:**
- Produces:
  - `port.ExchangeOAuthCommand{UserID uint; ScopeUserID *uint; Name string; CodeInput string; State string}` + `port.ExchangeOAuthResult{EndpointID uint}` + `port.ExchangeOAuthHandler`。
  - `port.ReauthOAuthCommand{ScopeUserID *uint; EndpointID uint; CodeInput string; State string}` + `port.ReauthOAuthHandler`。
- Consumes: Task 2（凭据仓储）、Task 4（code 解析）、Task 5（provider）、Task 6（flow Take）。`OwnerUserID/ScopeUserID` 语义与现有 endpoint 命令一致（admin 可代建）。

- [ ] **Step 1: Write the failing test**

```go
func TestExchangeCreatesEndpointAndCredential(t *testing.T) {
    // fake provider（ExchangeCode 返回固定 TokenSet）+ fake 仓储：
    // 1) 成功：创建 endpoint（auth_type=oauth_anthropic；anthropicBaseURL=https://api.anthropic.com/v1；
    //    supportAnthropicMessage=true 其余 false）+ credentials 行；结果 EndpointID > 0
    // 2) state 不属于当前用户 → ErrNoPermission
    // 3) state 已 Take / 过期 → ErrValidation
    // 4) provider 换 token 失败 → ErrorMessage 落 last_error、返回 ierr 错误、不创建行（事务）
}
func TestReauthUpdatesCredentialOnly(t *testing.T) // 只更新凭据、status 回 active、保留端点
```

- [ ] **Step 2/4: Run fails / passes**

- [ ] **Step 3: Implement**：`Take(state)` 校验 `flow.UserID == 命令 UserID`（admin 代建时允许 ScopeUserID 语义）→ `ParseOAuthCode` → `client.ExchangeCode` → 同一事务内创建 Endpoint（OAuth 固定值由本命令写入）+ 凭据。codex 的固定值为 `openaiBaseURL=https://chatgpt.com/backend-api/codex`、`supportOpenAIResponse=true`。reauth 仅更新既有凭据（校验端点类型匹配）。

- [ ] **Step 5: Commit** → `git commit -m "feat(oauth): 授权码兑换与重新授权命令"`

---

### Task 8: token provider（刷新 + 锁 + 缓存 + 可切换错误）

**Files:**
- Create: `internal/domain/llmproxy/service/oauth_token_provider.go`（接口）、`internal/infrastructure/oauthprovider/token_provider.go`（实现）、`internal/common/model/oauth_error.go`（错误类型）
- Modify: `internal/application/llmproxy/usecase/common.go`（`CanSwitchEndpoint` 登记）
- Test: `test/unit/oauth-token/provider_test.go`

**Interfaces:**
- Produces:
  - `service.OAuthTokenProvider`：`GetAccessToken(ctx context.Context, endpointID uint) (service.OAuthAccess, error)`、`RefreshIfNeeded(ctx context.Context, endpointID uint) error`（供 cron）、`service.OAuthAccess{Token, AccountRef string}`。
  - `model.OAuthCredentialError{Reason string}`（实现 `error`；`Retryable() bool` 恒 true——刷新失败视为可切换）。
  - `Locker` 复用 `internal/lock.Locker`；Redis 缓存键取 Task 4 模板。
- Consumes: Task 2/5/4。

- [ ] **Step 1: Write the failing test**

```go
func TestGetAccessTokenFreshCacheHit(t *testing.T)      // 缓存命中（未到提前量）不打 DB 不发 HTTP
func TestGetAccessTokenRefreshOnce(t *testing.T)        // 过期 → 刷新一次 → 写 DB（密文）+ 写缓存；并发两 goroutine 只刷一次（fake locker 串行化断言刷新计数=1）
func TestRefreshInvalidGrantMarksReauth(t *testing.T)   // provider 返回 invalid_grant → status=reauth_required + 返回 *model.OAuthCredentialError
func TestRefreshNetworkErrorKeepsCredential(t *testing.T) // 5xx → 不标 reauth、返回可切换错误
func TestCanSwitchEndpointOAuthError(t *testing.T)      // usecase.CanSwitchEndpoint(&model.OAuthCredentialError{}) == true
```

- [ ] **Step 2/4: Run fails / passes**（topic `oauth-token`）

- [ ] **Step 3: Implement**：按规格 §4.4 流程（缓存 → DB → 3 分钟提前量 → 锁 → 双检 → 刷新 → 落库+缓存；锁竞争且旧 token 未过期则用旧值）。提前量常量放 `constant/oauth.go`（`OAuthTokenRefreshSkew = 3 * time.Minute`）。

- [ ] **Step 5: Commit** → `git commit -m "feat(oauth): access token 按需刷新（锁/缓存）与可切换错误"`

---

### Task 9: 路由/handler/DTO + upstream 列表投影

**Files:**
- Modify: `internal/router/endpoint.go`、`internal/handler/endpoint.go`、`internal/dto/endpoint.go`、`internal/application/upstream/port/handler.go`、`internal/application/upstream/query/list_upstream.go`、`internal/bootstrap/modules/{application,handler,repository}.go`（wiring）
- Test: `test/unit/oauth-flow/handler_masking_test.go`

**Interfaces:**
- Produces（路由，遵循 `/{resource}/{action}` + `?id=` 规范）：
  - `POST /api/web/v1/endpoint/oauth/authorize`（`{provider}` → `{authorize_url, state}`）
  - `POST /api/web/v1/endpoint/oauth/exchange`（`{provider, name, code, state?}` → `{endpoint_id}`）
  - `POST /api/web/v1/endpoint/oauth/reauth?id={id}`（`{code, state?}`）
  - `GET /api/web/v1/endpoint/oauth/quota?id={id}`（读快照）
  - `POST /api/web/v1/endpoint/oauth/probe?id={id}`（同步探测，15s 超时）
  - 全部挂 `endpointGroup` 现有令牌桶中间件 + `LimitUserPermissionMiddleware(..., enum.PermissionUser)`；demo 天然拒绝写路径。
- `UpstreamEndpointView` 增 `AuthType string` 与 `OAuth *UpstreamOAuthView{AccountEmail string; PlanType string; Status string; Quota *vo.QuotaSnapshot; LastError string}`；demo 视图 `AccountEmail` 走 `MaskIdentity`、`LastError` 置空、Quota 保留。

- [ ] **Step 1: Write the failing test**（只测可测的纯函数：`toEndpointView` 的 demo 掩码与 OAuth 字段装配）

```go
func TestUpstreamEndpointViewMasking(t *testing.T) {
    // demo=true：email 掩码（***）、lastError 空、quota 保留、无任何 token 字段（断言结构体无该字段即编译期保证）
    // demo=false：原样
}
```

- [ ] **Step 2/4: Run fails / passes**

- [ ] **Step 3: Implement**：huma 注册仿现有 `initEndpointRouter`；handler 薄封装 `apiutil.WrapHTTPResponse`；探测接口超时用 `context.WithTimeout(ctx, 15*time.Second)`。本任务注册前 4 条路由（authorize / exchange / reauth / quota）；`probe` 路由与其 handler 在 Task 13（依赖探测服务）一并注册实现。

- [ ] **Step 4/5: Commit** → `git commit -m "feat(oauth): 授权/绑定/重授权/额度查询 API 与列表投影"`

---

### Task 10: UpstreamEndpoint 扩展 + EndpointKey + toTransportEndpoint 改造

**Files:**
- Modify: `internal/domain/llmproxy/vo/upstream_endpoint.go`、`internal/infrastructure/transport/guard.go`（`EndpointKey`）、`internal/application/llmproxy/usecase/openai.go`（`toTransportEndpoint` 及 10 处调用）、`internal/application/llmproxy/usecase/anthropic.go`、`query.go`、`openai_chat.go`、`openai_response.go`、`anthropic_message.go`
- Test: `test/unit/endpoint-auth/endpoint_key_test.go`、`test/unit/oauth-token/transport_endpoint_test.go`

**Interfaces:**
- Produces:
  - `vo.UpstreamEndpoint{Model, APIKey, BaseURL string; AuthType enum.EndpointAuthType; OAuthAccountID string; StableKey string}`；构造 `vo.NewUpstreamEndpointFromCredential(model, apiKey, baseURL string, authType enum.EndpointAuthType, oauthAccountID, stableKey string) UpstreamEndpoint`。
  - `toTransportEndpoint(ctx context.Context, u *openAIUseCase|anthropicUseCase, m *aggregate.Model, ep *aggregate.Endpoint, isAnthropic bool) (vo.UpstreamEndpoint, error)`——实际实现为 usecase 方法 `func (u *openAIUseCase) toTransportEndpoint(ctx, m, ep, isAnthropic) (vo.UpstreamEndpoint, error)`（两个 usecase 各一份或抽公共函数；抽公共：`usecase.BuildTransportEndpoint(ctx, ep, m, isAnthropic, tokens service.OAuthTokenProvider) (vo.UpstreamEndpoint, error)`）。
  - `EndpointKey(ep)`：`StableKey != ""` 时 `BaseURL|StableKey`，否则原逻辑。
- Consumes: Task 5 的 `service.OAuthTokenProvider`（usecase 构造新增依赖，wiring 同步）。

- [ ] **Step 1: Write the failing tests**

```go
func TestEndpointKeyStableAcrossTokenRotation(t *testing.T) // 同 StableKey、不同 APIKey（模拟轮换）→ EndpointKey 相等；StableKey 空时行为与旧实现逐字节一致
func TestBuildTransportEndpointOAuth(t *testing.T)          // OAuth 类型：APIKey=token、StableKey=oauth:{id}、OAuthAccountID=AccountRef；
                                                            // token 获取失败 → 返回 *model.OAuthCredentialError
```

- [ ] **Step 2/4: Run fails / passes**

- [ ] **Step 3: Implement**：10 处 `toTransportEndpoint(...)` 调用改为 `u.toTransportEndpoint(ctx, ...)` 并处理新 error 返回（错误直接向上抛，由 `runWithFallback` 判定切换）。api_key 类型路径：`AuthType=api_key`、其余新字段空。

- [ ] **Step 5: Commit** → `git commit -m "feat(oauth): 转发链路注入 OAuth 凭据与稳定熔断键"`

---

### Task 11: Claude 传输特化（headers 透传处理 + body）

**Files:**
- Create: `internal/infrastructure/transport/oauth_upstream.go`
- Modify: `internal/infrastructure/transport/anthropic.go`（`sendRequestOnce`：在 `applyPassthroughRequestHeaders` 后按 `ep.AuthType` 分流）
- Test: `test/unit/oauth-upstream/claude_test.go`

**Interfaces:**
- Produces:
  - `applyAnthropicOAuthUpstream(header http.Header, ep vo.UpstreamEndpoint) error`（headers：`Del(x-api-key)`、覆盖 Authorization、合并 anthropic-beta、覆盖 UA、Set x-app）
  - `transformAnthropicOAuthBody(body []byte, ep vo.UpstreamEndpoint) ([]byte, error)`（system 前置块 + metadata.user_id）
- Consumes: Task 4 常量、Task 10 的 `AuthType` 字段。

- [ ] **Step 1: Write the failing test**

```go
func TestClaudeOAuthHeaders(t *testing.T) {
    // 输入 header 预置：x-api-key=sk-aris、authorization=Bearer sk-aris、
    //   anthropic-beta=context-1m-2025-08-07、user-agent=claude-cli/1.0
    // 断言：x-api-key 为空（安全项）；authorization=Bearer <oauth token>；
    //   anthropic-beta 含两个值（合并去重）；user-agent=constant.OAuthClaudeUA；x-app=cli
}
func TestClaudeOAuthBody(t *testing.T) {
    // 无 system → 生成含块 [0]=constant.OAuthClaudeSystemBlock 的数组
    // 有 system（字符串与数组两种形态）→ 前置插入且原文本保留在块 [1]
    // metadata.user_id 为 JSON 字符串且含 account_uuid=<AccountRef>、session_id 稳定（同输入两次调用一致）
    // 非 OAuth（AuthType=api_key）→ body 与 header 与改造前逐字节一致（回归）
}
```

- [ ] **Step 2/4: Run fails / passes**（topic `oauth-upstream`）

- [ ] **Step 3: Implement**：body 变换用 sonic 反序列化到显式结构体（system 字段用 `sonic.NoCopyRawMessage` 承载 string / 数组 / 缺省三种形态，禁止 `any`）。session_id 派生：`sha256(accountRef + NUL + 首条 user 文本)` 前 16 hex（首条 user 文本提取失败时用随机 UUID 保证不 crash）。

- [ ] **Step 5: Commit** → `git commit -m "feat(oauth): Claude OAuth 上游特化（头删除/合并与 body 注入）"`

---

### Task 12: Codex 传输特化（headers + body + stream 前置校验）

**Files:**
- Modify: `internal/infrastructure/transport/oauth_upstream.go`、`internal/infrastructure/transport/openai.go`、`internal/application/llmproxy/usecase/openai.go`（stream 前置校验，`CreateResponse`/`forwardResponseNative` 入口）
- Test: `test/unit/oauth-upstream/codex_test.go`

**Interfaces:**
- Produces:
  - `applyOpenAIOAuthUpstream(header http.Header, ep vo.UpstreamEndpoint) error`（Authorization、`chatgpt-account-id`、`originator`、UA、`accept: text/event-stream`）
  - `transformCodexOAuthBody(body []byte) ([]byte, error)`（store/stream 强制、字段删除清单、include 补全）
  - usecase 校验：`ep.AuthType() == enum.EndpointAuthOAuthOpenAI && !stream` → 返回明确错误（`port.ProxyError` 400 + 文案 `constant.OAuthCodexStreamRequiredMessage`）。
- Consumes: Task 4/10。

- [ ] **Step 1: 前置核对 Responses DTO 字段覆盖度**（规格 §8 风险项）

读 `internal/dto/openai/response.go`，与规格 §4.3 Codex 字段清单对照：确认 `store` / `stream` / `instructions` / `include` / `reasoning` / `prompt_cache_key` / `client_metadata` 等在 DTO 中已定义且经 `MarshalOpenAIResponseBodyForModel` 序列化后不丢；缺失字段补进 DTO（huma 契约同步 `docs`），跑 `go test -count=1 ./test/unit/...` 确认无回归。

- [ ] **Step 2: Write the failing test**

```go
func TestCodexOAuthHeaders(t *testing.T)  // chatgpt-account-id=<AccountRef>；originator/UA 常量；accept=text/event-stream
func TestCodexOAuthBody(t *testing.T)     // golden：输入含 temperature/top_p/max_output_tokens/store=true/instructions 缺失
                                          // 断言：删除清单字段消失、store=false、stream=true、无 instructions 字段、reasoning 存在时 include 含 encrypted_content
func TestCodexStreamGuard(t *testing.T)   // stream=false 的 Responses 请求命中 oauth_openai 端点 → 400 + 常量文案
```

- [ ] **Step 3: Run tests to verify they fail** → `go test -v -count=1 -run 'TestCodexOAuth|TestCodexStreamGuard' ./test/unit/oauth-upstream/`，Expected: FAIL。

- [ ] **Step 4: Implement**：字段删除清单为 `constant.OAuthCodexStripFields = []string{...}`（13 项，见规格 §4.3）。stream 校验放在 `CreateResponse` 解析后、候选解析前（对不支持的非流式请求快速失败；错误体用 OpenAI 错误格式）。

- [ ] **Step 5: Run tests to verify they pass** → Expected: PASS。

- [ ] **Step 6: Commit** → `git commit -m "feat(oauth): Codex OAuth 上游特化与 stream 前置校验"`

---

### Task 13: 额度维护 cron + 手动探测接口

**Files:**
- Create: `internal/cron/oauth_maintenance.go`
- Modify: `internal/cron/cron.go`（注册 `CronRegistryEntry`）、`internal/common/constant/cron.go`（模块名/spec/描述）、`internal/router/endpoint.go`（注册 probe 路由）、`internal/application/endpoint/command/oauth_probe.go`
- Test: `test/unit/oauth-token/cron_test.go`

**Interfaces:**
- Produces:
  - `cron.NewOAuthMaintenanceCron(credRepo llmproxy.OAuthCredentialRepository, tokens service.OAuthTokenProvider, providers map[enum.OAuthProvider]service.OAuthProviderClient, redisClient *redis.Client) Cron`
  - 行为：遍历 `ListActive` → ①`expires_at < now+30min` → `tokens.RefreshIfNeeded`；②`quota_updated_at` 早于 30 分钟 → `ProbeQuota` 写快照。单账号失败只记日志，不中断循环。
  - `port.ProbeOAuthQuotaCommand{ScopeUserID *uint; EndpointID uint}` + Handler（同步探测一次并写快照）。
- Consumes: Task 2/5/8/9。

- [ ] **Step 1: Write the failing test**

```go
func TestOAuthMaintenanceCronSweepsOnce(t *testing.T) {
    // fake 仓储 2 条凭据：A 临期、B 额度陈旧 → 断言 Refresh 调用 1 次（A）、ProbeQuota 调用 1 次（B）
    // provider 探测抛错 → 不 panic、其余账号继续、last_error 被写
}
```

- [ ] **Step 2/4: Run fails / passes**

- [ ] **Step 3: Implement**：cron 骨架完全仿 `internal/cron/trigger_hit_sync.go`（`wrapCronFunc` + `RedisLocker` + `CronCallAudit`）；spec `*/10 * * * *`；配置开关 `CronOAuthMaintenanceEnabled`（默认 true，`config.go` + env 模板）。探测服务注入 cron 与 probe handler 共用。

- [ ] **Step 5: Commit** → `git commit -m "feat(oauth): 额度维护 cron（临期刷新 + 额度探测）与手动探测接口"`

---

### Task 14: 前端 api/types/form + 创建弹窗授权面板

**Files:**
- Modify: `web/src/lib/api-client.ts`、`web/src/lib/types.ts`、`web/src/app/(dashboard)/upstream/endpoint-form.ts`、`endpoint-dialog.tsx`、`shared.tsx`
- Test: `web/src/app/(dashboard)/upstream/__tests__/oauth-form.test.ts`

**Interfaces:**
- Produces（api-client 方法）：`authorizeOAuth(provider)`、`exchangeOAuth(body)`、`reauthOAuth(id, body)`、`getOAuthQuota(id)`、`probeOAuthQuota(id)`；types：`EndpointAuthType`、`OAuthView`、`QuotaSnapshot`、`QuotaWindow`、扩展 `UpstreamEndpointItem` 与 `CreateEndpointReqBody`（创建表单仍不含 authType——OAuth 走独立流程）。
- `EndpointForm` 增 UI 态字段：`authType: "api_key" | "oauth_anthropic" | "oauth_openai"`（仅驱动弹窗面板，不随 api_key 表单提交）。
- 弹窗交互（仿规格 §6）：三段状态机 `idle → authorized(code 输入) → done`；Claude 提示"复制页面显示的 code#state"；Codex 提示"复制浏览器地址栏整段地址"。

- [ ] **Step 1: Write the failing test**

```go
// vitest（纯函数）：
// normalizeOAuthCodeInput(input) → 与后端 ParseOAuthCode 同规则的前端预校验（URL/#/纯 code 三形态）
// oauthFormDefaults(provider) → 默认名称（如 "Claude 订阅"）与提示文案 key
```

- [ ] **Step 2: Run test to verify it fails** → `cd web && npm run test -- oauth-form`，Expected: FAIL。

- [ ] **Step 3: Implement**（复用现有 Dialog/Input/Button 组件；提交中带 loading 与错误提示）

- [ ] **Step 4: Run test to verify it passes**，随后 `npm run lint && npm run format:check`

- [ ] **Step 5: Commit** → `git commit -m "feat(web): 订阅账号授权面板与前端 API"`

---

### Task 15: 前端列表展示 + 重新授权 + i18n（三语）

**Files:**
- Modify: `web/src/app/(dashboard)/upstream/grouped-view.tsx`、`flat-view.tsx`、`shared.tsx`、`endpoint-dialog.tsx`（编辑态限制）、`web/src/locales/{zh,en,ja}.json`
- Test: `web/src/app/(dashboard)/upstream/__tests__/oauth-display.test.ts`

**Interfaces:**
- Produces：OAuth 端点行显示 provider 徽标 + 账号 email + plan；`reauth_required` 红色提示 + 「重新授权」入口（复用 Task 14 面板，走 reauth API）；额度进度条（`used_percent` + hover `resets_at`）；编辑弹窗对 OAuth 端点禁用 baseURL/apiKey/能力字段（仅名称可改）；i18n 三语 key 齐备（zh/en/ja 同 key 集）。

- [ ] **Step 1: Write the failing test**：`quotaBarText(window)` 格式化（百分比取整、resets_at 相对时间）；`endpointRowBadges(item)` 徽标集合（oauth_anthropic → Claude 徽标 + 状态）。
- [ ] **Step 2/4: Run fails / passes**（`npm run test`）
- [ ] **Step 3: Implement** + 三语文案。
- [ ] **Step 5: Commit** → `git commit -m "feat(web): OAuth 端点列表展示、重新授权与三语文案"`

---

### Task 16: 文档同步（CONTEXT.md / README 双语 / env 模板）

**Files:**
- Modify: `CONTEXT.md`（LLM Proxy 章节：新增 EndpointAuthType、OAuthCredential、QuotaSnapshot 词条；订阅端点协议矩阵写入 Model/Endpoint 词条）、`README.md`、`README.zh-CN.md`（Core Capabilities 增"订阅账号接入"；两份同步改、结构对齐）、`env/api.env.template`（`OAUTH_CREDENTIAL_KEY` 注释与生成示例：`openssl rand -hex 32`）
- Test: 无（文档任务；验证 = `git diff` 自查双语结构对齐）

- [ ] **Step 1: 更新 CONTEXT.md**（词条格式仿现有：概念 + `_Avoid_` 行）
- [ ] **Step 2: 同步 README 双语**（英中对照，同一段落结构）
- [ ] **Step 3: env 模板 + 部署说明**（密钥丢失=全部重新授权的警告）
- [ ] **Step 4: Commit** → `git commit -m "docs(oauth): 订阅账号接入的领域词、README 双语与配置模板"`

---

### Task 17: e2e 用例（mock upstream 链路）

**Files:**
- Create: `test/e2e/oauth-endpoint/oauth_test.go`、`fixtures/requests/*.json`
- 依赖：现有 e2e 基建（`test/e2e/` 其他 topic 的客户端封装与鉴权方式，执行时先读同目录现有用例模仿）

**Interfaces:**
- Consumes: 全部前序任务的公开 API（REST）。

- [ ] **Step 1: Write the failing test**

```go
func TestOAuthEndpointLifecycle(t *testing.T) {
    // 1) POST /endpoint/oauth/authorize → 拿 URL/state（断言 URL 含 code_challenge）
    // 2) 直接向测试库写一条 endpoint_oauth_credentials（模拟已授权，绕过真实 OAuth）
    // 3) 绑定模型别名 → POST /api/anthropic/v1/messages（mock-llm 上游）→ 200 + 审计落库
    // 4) 把凭据置 reauth_required → 同请求 → 404 model not found（候选被剔除）
    // 5) 恢复 active → 请求恢复 200
    // 6) 【Review Focus 4】同一别名绑 oauth_openai 端点时：POST /api/openai/v1/chat/completions → 404
    //    （Chat 无到 Responses 的转换路径；必须是明确的 model not found，而非挂起或 500）
}
```

- [ ] **Step 2/4: Run fails / passes**（`go test -v -count=1 -run TestOAuthEndpointLifecycle ./test/e2e/oauth-endpoint/`）
- [ ] **Step 3: Implement**（fixtures 存请求体；复用现有环境变量与登录封装）
- [ ] **Step 5: Commit** → `git commit -m "test(oauth): 订阅端点 e2e（授权/转发/失效剔除/恢复）"`

---

## 交付与验收

- 全量验证：`make lint && make test`（Go）+ `cd web && npm run lint && npm run test && npm run build`。
- 生产人工验收（用户执行，规格 §9 清单）：真实授权 Claude / Codex 各一 → 客户端走通 → 审计与 Session 核对 → 手动失效观察 fallback 与恢复。
- 部署注意：生产 env 增 `OAUTH_CREDENTIAL_KEY`（`update-prod-config` 三件套）；`database migrate` 由 CI 自动执行。
