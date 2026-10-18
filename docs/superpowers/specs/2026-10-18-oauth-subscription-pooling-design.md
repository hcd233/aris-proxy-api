# 订阅账号 OAuth 池化设计（Claude Code / Codex）

> 日期：2026-10-18
> 状态：待批准（brainstorming 产出）
> 参考实现：Wei-Shaw/sub2api（LGPL-3.0）。本文只引用其**协议事实**（OAuth 端点 URL、client_id、header 名、请求体字段名与字面量），不复制其代码；aris 为 Apache-2.0，任何实现均需自研。

## 1. 背景与目标

### 1.1 为什么做

用户持有 Claude（Claude Code）与 ChatGPT（Codex）订阅账号，其额度目前只能通过官方客户端直连使用，流量完全绕过 aris：不进审计、不进 Session 数据治理、不能与 API Key 端点统一调度。本次把订阅账号的 OAuth 凭据接入 aris，作为**新的一类上游 Endpoint**，使订阅流量获得与 API Key 端点完全相同的待遇。

用户确认的四项诉求（全部纳入范围）：

1. **省钱**：订阅额度优先、API Key 端点兜底（复用现有 priority/weight 调度即可达成）。
2. **纳入统一审计与数据治理**：订阅流量同样落 ModelCallAudit 与 Session/Message/Tool。
3. **客户端统一入口**：Claude Code / Codex 只连 aris，模型别名统一。
4. **配额感知与监控**：展示订阅窗口（Claude 5h/7d、Codex primary/secondary）用量与重置时间。

### 1.2 目标

- 两类订阅账号（Anthropic / OpenAI）可通过 Web 后台完成 OAuth 授权、绑定为 Endpoint、被模型别名引用并正常转发。
- Access token 自动刷新（请求路径按需 + 后台定时），失效可感知、可恢复（重新授权）。
- 凭据加密落库，任何 API 响应与日志不出现明文 token。
- 订阅窗口用量可在管理端查看（前端展示 + 定时探测 + 手动刷新）。

### 1.3 非目标（明确排除，二期或独立议题）

- **不做账号级 HTTP 代理**（sub2api 有；aris 的 Transport 连环境变量代理都不生效，独立议题）。
- **不做额度阈值调度联动**（用尽自动降级/跳过；首版靠 429 + 现有 fallback 兜底，额度仅展示）。
- **不做完整客户端拟真**（Claude 的 billing header、工具名混淆、cache_control 归一等；首版为"最小必要集"，被上游拒绝时再增量补）。
- **不做 Claude setup-token 之外的自动化授权**（Cookie/sessionKey 换 code 之类，不做）。
- **不新增 Chat→Response 转换路径**（Codex OAuth 端点因此只接受 Responses 协议入口，见 §4.2）。
- **不做响应头被动采集额度**（`anthropic-ratelimit-unified-*`、`x-codex-*`；二期）。

### 1.4 反目标（做出来即为失败）

- 引入"账号池 ↔ 分组"第二套调度体系，与现有 Model↔Endpoint 语义并存冲突。
- OAuth 凭据（尤其 refresh token）明文落库、进日志或出现在任何 API 响应里。
- 让 demo/其他租户可见或可用他人的 OAuth 凭据。

## 2. 关键决策与理由

| # | 决策 | 理由 |
|---|------|------|
| D1 | **OAuth 账号 = 一类新 Endpoint**（用户已确认），复用模型绑定、priority/weight 调度、会话亲和、跨端点 fallback、熔断、审计、定价、数据治理 | aris 是单用户自用；现有 Model↔Endpoint 多绑定本身就是"池"（多账号=多 Endpoint 绑同一别名）。不引入第二套调度 |
| D2 | **凭据独立表 + AES-256-GCM 加密**（本文拍板） | token 字段多且易变，与配置实体 Endpoint 分表；refresh token 生命周期长于 API Key 且无法一键轮换，加密成本低（~50 行 + 一个 env），纵深防御备份泄露场景。sub2api（4.4 万 star）选择明文，我们做得更好一点 |
| D3 | **上游特化集中在 transport 层**（`sendRequestOnce` / `sendUpstreamRequestOnce`），按 endpoint 认证类型分流：改 headers + 改 body | 认证注入点已在此（`anthropic.go:198` / `openai.go:175`）；body 是 `[]byte`，JSON 层改写与 DTO 覆盖度解耦；2 个函数集中改动，`toTransportEndpoint` 只需携带认证上下文。不采用"usecase 10 个调用点各改"或"新增转换路径"的方案 |
| D4 | **授权交互 = Web 后台发起 + 回贴 code**（本文拍板） | 两个 provider 的 redirect_uri 都是上游固定值（无法改为 aris 域名），且是自用单实例。生成授权链接 → 用户在浏览器授权 → 回贴 code（Claude 为 `code#state`；Codex 复制 localhost 回调页整段 URL）→ 服务端换 token。不做本地 CLI 辅助（可作为后续体验优化） |
| D5 | **Token 刷新 = 请求路径按需（提前 3 分钟）+ 后台 cron 主动（提前 30 分钟）+ Redis 缓存 + 分布式锁** | 请求路径保证正确性；cron 降低请求路径的刷新延迟与失败率；锁防并发刷新（refresh token 轮换竞争） |
| D6 | **额度 = 主动探测（cron 30 分钟节流 + 手动按钮）+ 展示**；调度不联动 | 用户诉求是"感知与监控"；探测 API 非公开，fail-soft（失败只影响展示） |
| D7 | **OAuth 端点创建/重授权走独立路由**（`/oauth/*`），不挤进现有 createEndpoint | 现有 Create 校验要求 apiKey 非空、baseURL 手填，与 OAuth 形态冲突；独立路由零破坏存量 API 契约 |
| D8 | **凭据状态机**：`active` / `reauth_required`；`reauth_required` 的端点被 Resolver 过滤（fail-closed） | 失效 token 每请求必失败；过滤让 fallback 立即切备选端点，行为与"端点被禁用"一致 |

## 3. 领域模型

### 3.1 EndpointAuthType（新枚举）

`internal/common/enum`：`api_key`（默认，全部存量）、`oauth_anthropic`、`oauth_openai`。

DB 列：`endpoints.auth_type varchar not null default 'api_key'`（AutoMigrate 加列，存量行默认即兼容）。

### 3.2 OAuthCredential（新聚合 / 新表）

表 `endpoint_oauth_credentials`（1:1 关联 endpoint，随 endpoint 删除级联删除）：

| 列 | 说明 |
|---|---|
| id / endpoint_id | endpoint_id 唯一索引 |
| provider | `anthropic` / `openai`（冗余便于直查） |
| access_token_enc / refresh_token_enc | AES-256-GCM 密文（base64(nonce+ct+tag)），加解密只在 repository 层做 |
| expires_at | access token 到期时刻 |
| account_ref | Claude: organization uuid；Codex: chatgpt_account_id |
| account_email | 展示用 |
| plan_type | 展示用（如 max / pro / plus / team），可空 |
| status | `active` / `reauth_required` |
| last_error | 最近一次错误（脱敏，展示用） |
| quota_snapshot | text 列 + `serializer:json`，结构见 3.3；空 = 未探测 |
| quota_updated_at | 额度快照时间 |
| created_at / updated_at | — |

领域聚合 `OAuthCredential` 挂在 `internal/domain/llmproxy/aggregate`（与 Endpoint 同域），领域层只见明文 token（加密是持久化细节）。

**凭据绝不出现在 API 响应中**：列表/详情只回 email、plan_type、status、quota。

### 3.3 QuotaSnapshot（值对象，JSON 存储）

```json
{
  "windows": [
    {"key": "five_hour", "used_percent": 12.5, "resets_at": "2026-10-18T20:00:00Z", "window_minutes": 300},
    {"key": "seven_day",  "used_percent": 40.0, "resets_at": "2026-10-21T00:00:00Z", "window_minutes": 10080}
  ],
  "plan_type": "max",
  "captured_at": "2026-10-18T15:00:00Z"
}
```

映射：Claude `five_hour` / `seven_day` / `seven_day_sonnet` / `seven_day_overage_included`；Codex `primary_window`（5h）/ `secondary_window`（7d）。统一为 windows 数组，前端按序渲染进度条。

### 3.4 UpstreamEndpoint 扩展（vo）

```go
type UpstreamEndpoint struct {
    Model   string
    APIKey  string          // OAuth 时 = access token（复用现有字段注入 Bearer）
    BaseURL string
    AuthType enum.EndpointAuthType  // 新增；api_key 为默认值，存量路径语义不变
    OAuthAccountID string           // 新增；Codex 需 chatgpt-account-id；Claude 留空
    StableKey string                // 新增；端点稳定身份，用于熔断/并发隔离键与指标 label
}
```

**StableKey（修正 builder check 发现的缺陷）**：现有 `EndpointKey(ep) = BaseURL|sha256(APIKey)[:8]`（`transport/guard.go`）同时是熔断/信号量隔离键与 Prometheus label。OAuth 场景 APIKey 字段承载 access token，会随刷新轮换 → 熔断状态每次重置、指标序列无限增长。改造：`EndpointKey()` 在 `StableKey != ""` 时以 `StableKey` 取代 APIKey 摘要；构造时 API Key 端点 StableKey 留空（行为完全不变），OAuth 端点填 `oauth:{endpointID}`。

`toTransportEndpoint` 改造：签名加 `ctx` 与 token 提供者，OAuth 类型时经 `OAuthTokenProvider.GetAccessToken(ctx, endpointID)` 取（可能触发刷新），失败返回**可切换错误**：新增错误类型（如 `OAuthCredentialError`）并登记进 `usecase.CanSwitchEndpoint` 白名单，由现有 `runWithFallback` 切到下一候选（详见 §4.4）。

### 3.5 Endpoint 聚合新增

- `authType`（构造与 Update 校验：OAuth 类型禁止改 baseURL/apiKey/协议能力标记；API Key 类型禁止传 authType=oauth_*）。
- `oauthStatus`（从 credentials 表联结恢复，只读投影）：`reauth_required` 时 `collectCandidates` 跳过该端点。
- OAuth 端点的能力标记由服务端在创建时强制写入，前端不可改：
  - `oauth_anthropic` → `supportAnthropicMessage=true`，`anthropicBaseURL` 固定 `https://api.anthropic.com/v1`，其余 false/空。
  - `oauth_openai` → `supportOpenAIResponse=true`，`openaiBaseURL` 固定 `https://chatgpt.com/backend-api/codex`，其余 false/空。
- **BaseURL 取值依据**（已核对代码）：transport 拼 URL 为 `baseURL + path`，path 常量为 `UpstreamPathAnthropicMessages = "/messages"`、`UpstreamPathOpenAIResponses = "/responses"`（`constant/upstream.go`），故 baseURL 必须带上完整前缀（Claude 含 `/v1`，Codex 含 `backend-api/codex`）。

## 4. 流程设计

### 4.1 授权绑定流程

```
Web 后台                      aris 服务端                     上游
  │ POST /oauth/authorize      │
  │ {provider}                 │ 生成 PKCE(verifier,challenge) + state
  │ ──────────────────────────►│ 存 Redis flow 记录（TTL 10min，含 userID/verifier/provider）
  │ {authorize_url} ◄──────────│
  │ 打开授权链接（新窗口）──────────────────────────────────────►│ 用户登录并授权
  │◄────────────────────────────────────────────────────────────│ Claude: 页面显示 code#state
  │                                                              │ Codex: 跳 localhost:1455（页面打不开，
  │                                                              │        从地址栏复制整段 URL）
  │ POST /oauth/exchange        │
  │ {provider,name,code}        │ 解析 code（见下）→ 校验 state → 取 flow
  │ ──────────────────────────►│ → 换 token（上游 token 端点）
  │                             │ → 拉账号信息（email/plan，best-effort）
  │                             │ → 加密落库 credentials + 创建 Endpoint（同事务）
  │ {endpoint_id} ◄─────────────│
```

**code 解析规则**（exchange 入参 `code` 字段兼容三种形态）：
- 以 `http` 开头 → 解析 URL query 的 `code` 与 `state`（Codex 粘贴回调 URL）。
- 含 `#` → 按 `#` 切分 `code#state`（Claude 页面格式）。
- 纯 code → 无 state，用请求中的可选 `state` 字段，二者皆无则拒（`ErrValidation`）。

**OAuth 常量**（集中在 `internal/common/constant/oauth.go` 一处定义，上游变更时单点修改）：

| 项 | Anthropic | OpenAI |
|---|---|---|
| client_id | `9d1c250a-e61b-44d9-88ed-5944d1962f5e` | `app_EMoamEEZ73f0CkXaXp7hrann` |
| authorize | `https://claude.com/cai/oauth/authorize` | `https://auth.openai.com/oauth/authorize` |
| token | `https://platform.claude.com/v1/oauth/token` | `https://auth.openai.com/oauth/token` |
| redirect_uri | `https://platform.claude.com/oauth/code/callback` | `http://localhost:1455/auth/callback` |
| scope | `user:profile user:inference user:sessions:claude_code user:mcp_servers user:file_upload`（空格连接） | `openid profile email offline_access` |
| PKCE | S256；verifier=32B base64url 无 padding | S256；verifier=64B hex |
| 附加参数 | — | `id_token_add_organizations=true`、`codex_cli_simplified_flow=true` |
| 换 token 请求 | POST JSON `{code, grant_type:authorization_code, client_id, redirect_uri, code_verifier, state}` | POST form `grant_type=authorization_code&client_id&code&redirect_uri&code_verifier` |
| 刷新请求 | POST JSON `{grant_type:refresh_token, refresh_token, client_id}` | POST form 同结构 + `scope=openid profile email` |
| 账号信息来源 | 响应 `organization.uuid` / `account.email_address` | id_token 解码 `https://api.openai.com/auth` → `chatgpt_account_id`（**只解码不验签**，自用可接受） |

### 4.2 转发流程（协议入口矩阵）

| 客户端入口 | oauth_anthropic 端点 | oauth_openai 端点 |
|---|---|---|
| Anthropic `/messages` | ✅ native | ❌（无转换路径） |
| OpenAI `/chat/completions` | ✅ 走现有 Chat→Anthropic 转换 | ❌（无 Chat→Response 路径，见非目标） |
| OpenAI `/responses` | ✅ 走现有 Response→Anthropic 转换 | ✅ native |
| OpenAI `/decisions` | ❌ | ❌ |

> Codex CLI/IDE 均使用 Responses 协议，✅ 路径覆盖其全部场景。Chat 客户端命中 Codex OAuth 端点会得到 404 model not found（与现有"端点不支持该 API"语义一致）。此限制写入 CONTEXT.md 与前端提示。

**oauth_openai 端点仅支持流式**（修正 builder check 发现的设计矛盾）：Codex 后端要求 `stream=true`，而 aris 的非流式路径（`forwardResponseNativeUnary`）会把上游 SSE 原文当 JSON 回给客户端。因此：客户端 `stream=false`（或缺省，Responses 缺省为 false）打到 oauth_openai 端点时，在 usecase 转发前返回明确错误（400 + 文案"该订阅端点的 Responses 接口仅支持 stream=true"），不透传给上游。首版不做 aris 侧 SSE 聚合（列为二期；Codex 客户端 100% 流式，实际影响≈0）。`previous_response_id` 等续链字段保留透传（Codex 后端原生支持）。

转发链路（以 Claude OAuth + Anthropic native 为例）：

```
usecase 组装 UpstreamEndpoint（AuthType=oauth_anthropic, APIKey=access_token, OAuthAccountID=""）
  → transport.sendRequestOnce：
      1) headers 特化（§4.3）
      2) body 特化（§4.3）
      3) 发往固定 host（endpoint.baseURL 已是 api.anthropic.com）
```

### 4.3 上游特化清单（最小必要集）

**Claude（oauth_anthropic）**

- **透传头处理**（已核对 `middleware/header_passthrough.go`：黑名单之外**全部客户端头透传**，`transport` 层再覆盖；OAuth 下必须显式处理，否则 aris 的 key 会漏给上游——安全项，须进测试断言）：
  - `Del("x-api-key")`：删除透传进来的客户端 x-api-key（aris 不用它给 OAuth 上游），同时 OAuth 分支跳过现有代码的 `Set(x-api-key)`。
  - `Authorization`：`Set("Bearer <access_token>")` 覆盖透传值。
  - `anthropic-beta`：**合并**（不覆盖）——客户端透传值 ∪ `oauth-2025-04-20`，逗号连接去重（避免丢掉客户端自身的 beta，如 1M context）。
  - `User-Agent`：`Set("claude-cli/<ver> (external, cli)")` 覆盖（ver 用常量，如 `2.1.7`）。
  - `x-app: cli`（Set）。
  - 其余（`anthropic-version`、content-type、其它透传头）照旧。
- Body（JSON 层轻改）：
  - `system`：前置块 `"You are Claude Code, Anthropic's official CLI for Claude."`（无 system 则创建数组；有则插入首位）
  - `metadata.user_id`：补 `{"device_id":"<稳定派生>","account_uuid":"<account_ref>","session_id":"<稳定派生>"}` JSON 字符串（session_id 由账号 + 首条 user 文本派生，保证同会话稳定）
  - 其余不动。
- **首版不做**（sub2api 有、被拒时再补）：billing header system 块、原 system 迁入 messages、工具名混淆/还原、cache_control 归一、TLS 指纹以外的替身 UA 细节。

**Codex（oauth_openai）**

- Headers（同上透传原则：Set 覆盖，无删除项——`Authorization` 被覆盖即可）：
  - `Authorization: Bearer <access_token>`
  - `chatgpt-account-id: <OAuthAccountID>`
  - `originator: codex_cli_rs`
  - `User-Agent: codex_cli_rs/<ver> (Mac OS 15.0; arm64) terminal`（ver ≥ `0.144.0`，低于此上游返回 404；常量 `0.146.0`）
  - `accept: text/event-stream`
- Body（JSON 层改写）：
  - 强制 `store=false`、`stream=true`（`stream` 已由 §4.2 前置校验保证为 true）
  - 删除上游不接受的字段：`max_output_tokens`、`max_completion_tokens`、`temperature`、`top_p`、`frequency_penalty`、`presence_penalty`、`user`、`metadata`、`prompt_cache_retention`、`safety_identifier`、`stream_options`、`truncation`、`stop_sequences`
  - `instructions`：**客户端带则保留；未带则不填**（首版不注入默认文本——避免引入来源不明的字面量；若实测上游拒绝空 instructions，再从 Apache-2.0 许可的 openai/codex 仓库取真实 base instructions，并在代码注释标明出处）
  - `include`：含 `reasoning` 时补 `["reasoning.encrypted_content"]`
- **首版不做**：`item_reference` 过滤、工具名别名化、`x-codex-turn-state` 回放/跨账号剥离、`session_id`/`conversation_id` 覆盖（客户端透传值原样保留）。

> **实现策略**：特化函数返回"headers 补丁 + body 变换"，集中在一个文件（`transport/oauth_upstream.go`），全部字面量（UA 版本、beta 值、系统提示文本）用命名常量，便于实测后单点调整。**不引入运行时开关**（YAGNI）。

### 4.4 Token 刷新流程

```
GetAccessToken(ctx, endpointID):
  cache = Redis GET oauth:access:{id}           // 命中且 now < expires_at - 60s → 直接返回
  若无缓存 → 查 DB 读凭据
  if DB.expires_at > now + 3min → 写缓存(剩余-1min) 并返回
  // 需要刷新
  got = Locker.Lock(oauth:refresh:{id}, ttl=60s)
  if !got → 短暂等待后重试一次；仍锁住且旧 token 未过期 → 用旧 token；否则报可切换错误
  if got:
    re-read DB（双检，别的实例可能已刷）
    if 已新鲜 → 写缓存返回
    resp = 调用对应 provider 刷新端点（§4.1）
    成功 → 更新 DB（密文）+ 清 last_error + status=active + 写缓存
    失败 invalid_grant/401 → status=reauth_required + 返回可切换错误
    其它失败（网络/5xx）→ 保留旧凭据 + 返回可切换错误（不标 reauth，靠 cron 重试）
    Unlock
```

- refresh token 轮换：响应含新 refresh_token 时覆盖（两家皆然；Claude 无则保留原值）。
- Redis 键：`oauth:access:{endpointID}`、`oauth:refresh:{endpointID}`、授权流 `oauth:flow:{state}`。

### 4.5 失效与恢复

- `reauth_required` 后：Resolver 过滤该端点（fail-closed）→ 备用端点接管；前端在上游页醒目展示"需重新授权" + 重新授权入口。
- 恢复：`POST /oauth/reauth`（同 exchange 流程，但更新既有 endpoint 的凭据，成功后 status=active、清 last_error、清 Redis 缓存）。
- 上游 401 的自动重试（内部刷新一次再重试）**首版不做**：靠 3 分钟提前量 + cron 30 分钟提前刷新覆盖；漏网场景由 fallback + 人工重授权兜底。列入二期。

### 4.6 额度探测

- Claude：`GET https://api.anthropic.com/api/oauth/usage`，头 `Authorization: Bearer` + `anthropic-beta: oauth-2025-04-20`；响应 `five_hour{utilization,resets_at}`、`seven_day`、`seven_day_sonnet`。
- Codex：`GET https://chatgpt.com/backend-api/wham/usage`，头 `Authorization: Bearer` + `chatgpt-account-id`；响应 `rate_limit.primary_window{used_percent,limit_window_seconds,reset_at}`、`secondary_window`。
- 写 `quota_snapshot` + `quota_updated_at`；探测失败 fail-soft（写 last_error，不改变 status）。

## 5. 接口设计（Web API，JWT + user 权限）

| 操作 | 方法与路径 | 请求 | 响应 |
|---|---|---|---|
| 发起授权 | `POST /api/web/v1/endpoint/oauth/authorize` | `{provider}` | `{authorize_url, state}` |
| 完成绑定 | `POST /api/web/v1/endpoint/oauth/exchange` | `{provider, name, code, state?}` | `{endpoint_id}` |
| 重新授权 | `POST /api/web/v1/endpoint/oauth/reauth` | `{endpoint_id, code, state?}` | `{endpoint_id}` |
| 读取额度 | `GET /api/web/v1/endpoint/oauth/quota/{endpoint_id}` | — | `{quota: QuotaSnapshot?, status, last_error?}` |
| 刷新额度 | `POST /api/web/v1/endpoint/oauth/quota/{endpoint_id}/probe` | — | 同上（同步探测一次，超时 15s） |

- 现有 `GET /upstream/list` 等列表响应：endpoint 视图增加 `auth_type`、`oauth{account_email,plan_type,status,quota}`（对 demo 走现有掩码策略：email 掩码、无凭据字段）。
- 事务性：exchange 中"建 endpoint + 写 credentials"同事务；reauth 只更新 credentials。
- 权限：`LimitUserPermissionMiddleware(…, enum.PermissionUser)`；demo 天然拒绝（写路径）。
- 速率：挂现有 `endpointManage` 令牌桶组。

## 6. 前端设计（web/）

- **`endpoint-dialog.tsx`**：创建弹窗顶部加"账号类型"三选（API Key / Claude 订阅 / Codex 订阅），默认 API Key（表单不变）。选订阅后切换为授权面板：点「开始授权」→ 打开授权链接 + 显示粘贴框（提示 Claude 复制页面的 `code#state`；Codex 复制浏览器地址栏整段 URL）→ 提交。
- **列表行**：OAuth 端点显示 provider 徽标 + 账号 email + plan + 状态徽标（有效 / 需重新授权）；操作菜单加「重新授权」「刷新额度」。
- **额度展示**：行内进度条（hover 显示 resets_at）或详情抽屉；无快照显示"—"。
- **错误**：`reauth_required` 行内联红色提示 + 一键重新授权。
- i18n：zh/en 双语文案（复用现有 locales 结构）。demo 视图：OAuth 凭据不可见，email 掩码。

## 7. 配置、迁移与运维

- **新增 env**：`OAUTH_CREDENTIAL_KEY`（hex 64 字符 = 32B，AES-256-GCM 密钥）。
  - 为空时：OAuth 创建/授权/转发路径 fail-closed 并给出明确提示；API Key 端点不受影响。
  - 生产更新走 `update-prod-config` skill（api.env + ConfigMap + restart 三件套），并写入 `env/api.env.template`。
- **迁移**：`endpoints` 加 `auth_type` 列、新增 `endpoint_oauth_credentials` 表 → 注册进 `model.Models` 由 AutoMigrate 建（`database migrate` 命令，CI 部署已带）。
- **cron**：新增 `OAuthMaintenanceCron`（`*/10 * * * *`，Title 级 functional）：
  1. 扫 `status=active` 且 `expires_at < now+30min` 的凭据 → 主动刷新；
  2. 扫 `quota_updated_at` 超过 30 分钟的凭据 → 探测额度。
  - 走现有分布式锁与 `CronCallAudit`；配置开关 `cron.oauth_maintenance_enabled` 默认 true。
- **文档**：CONTEXT.md 增补领域词条（EndpointAuthType / OAuthCredential / QuotaSnapshot / 订阅端点协议矩阵）；README 双语同步"订阅账号"能力说明；密钥丢失=全部重新授权的运维警告写入部署文档。

## 8. 风险与缓解

| 风险 | 缓解 |
|---|---|
| **ToS / 账号封禁**：以官方客户端形态调用订阅额度可能违反上游条款（用户已知情并明确要求实现） | 最小特化集 + 文档显著警告；不使用账号做任何非转发用途 |
| **最小特化集可能暂不被接受**（400/403） | 特化常量与逻辑集中单文件，实测迭代；被拒时按 sub2api 清单增量补（§4.3 已列出差异项） |
| **上游 client_id / 端点变更** | 全部常量集中在 `constant/oauth.go` 单点修改 |
| **额度探测 API 非公开、可能变更** | fail-soft，只影响展示 |
| **加密密钥丢失** | 部署文档警告；重新授权即恢复（无需重建数据） |
| **Responses 请求 DTO 覆盖度**：body 由 DTO 重序列化，Codex 客户端字段（如 `prompt_cache_key`、`client_metadata`）若 DTO 未覆盖会丢失 | 实施第一步先核对 `dto/openai/response.go` 覆盖度，必要时补齐字段（独立小改动） |

## 9. 测试与验收

- **单测**（仓库测试契约：只测导出符号，`go test`）：
  - PKCE/state 生成与校验；code 三形态解析（`code#state` / URL / 纯 code）。
  - 换 token / 刷新：HTTP mock + 失败分支（invalid_grant → reauth_required；网络错 → 保留凭据）。
  - 加密 round-trip；密钥缺失时 fail-closed。
  - body 特化：Claude system 前置与 metadata；Codex store/stream 强制、字段删除清单、instructions 缺省填充（golden 断言）。
  - headers 注入：断言 `x-api-key` 在 OAuth 下缺席、`anthropic-beta`/`chatgpt-account-id` 存在。
  - 刷新并发：锁竞争下仅一次上游刷新。
  - Endpoint 聚合校验：OAuth 类型禁止改 baseURL/能力标记；reauth_required 端点被 Resolver 过滤。
- **e2e**（`test/e2e/oauth-endpoint/`）：mock upstream（复用 mock-llm 机制）验证 OAuth 端点的转发、审计落库、reauth 过滤与 fallback 切换；**真实 OAuth 授权无法自动化**，作为部署后人工验收。
- **生产人工验收清单**（部署后由用户执行）：
  1. 后台完成 Claude 授权 → 端点出现、额度可见 → Claude Code（或 curl /messages）走通 → 审计与 Session 落库。
  2. 后台完成 Codex 授权 → Codex CLI 走通 → 上述核对。
  3. 手动把凭据标记失效（或改错 token）→ 观察 fallback 与"需重新授权"提示 → 重新授权恢复。

## 10. 实施顺序（供 writing-plans 细化）

1. **基础层**：enum + 迁移 + 聚合/表/仓储（含加密）+ Endpoint 扩展与校验 + Resolver 过滤（reauth_required）+ `vo.UpstreamEndpoint.StableKey` 与 `EndpointKey()` 改造。
2. **OAuth 核心**：常量 + PKCE/state + flow（Redis）+ 换 token/刷新 + token provider（锁/缓存/可切换错误）+ 路由/handler/dto + 审计日志。
3. **传输特化**：`toTransportEndpoint` 携带 AuthType/AccountID/StableKey + transport 两个发送函数的 headers/body 特化（含透传头删除/合并）+ oauth_openai 的 stream=true 前置校验（Claude 先、Codex 后，各自附单测）。
4. **额度**：探测客户端（两 provider）+ cron + 手动探测接口。
5. **前端**：创建弹窗授权面板 + 列表展示 + 重新授权 + 额度 UI + i18n。
6. **文档与验收**：CONTEXT.md / README(双语) / 部署文档；e2e 用例；生产人工验收。

> 实施提示：第 3 步前先核对 `dto/openai/response.go` 对 Codex 客户端请求字段的覆盖度（风险表最后一行的验证项）；第 3 步的"最小必要集"被上游拒绝时，按 §4.3 "首版不做"清单增量补（仅调整特化文件，不动架构）。
