# aris-proxy-api

LLM 代理网关 + 配套管理后台。本文件是项目的领域词汇表（glossary），只定义术语，不含实现细节。

## Identity & Access（身份与访问）

**User（用户）**:
一个使用平台的自然人。通过 OAuth2（GitHub / Google）注册和登录，档案包含 Name、Email、Avatar，绑定平台第三方 ID。初始权限为 Pending，经审核升为 User 或 Admin。
_Avoid_: account, member, operator

**Permission（权限）**:
用户的三级权限体系：`pending`（待审，功能受限）→ `user`（普通用户，可管理自身 API Key 和会话）→ `admin`（管理员，可管理所有资源和配置）。通过 `Permission.Level()` 比较等级。
_Avoid_: role, role group

**ProxyAPIKey（代理密钥）**:
用户签发的 API Key，用于通过网关转发 LLM 请求。每个用户有配额上限（`APIKeyQuota`）。密钥值（`APIKeySecret`）仅在创建时明文返回一次，后续只展示脱敏后字符串。属于某个 User，鉴权时从请求头提取并与数据库比对。
_Avoid_: api key, credential, token

**OAuthProvider（OAuth 平台）**:
支持的第三方 OAuth2 登录平台，当前为 GitHub 和 Google。用户通过任一平台提供的 OAuthUserInfo（id、name、email、avatar）完成注册或登录绑定。
_Avoid_: social login, sso provider

**TokenPair（令牌对）**:
OAuth2 完成后下发的 JWT 访问令牌对，含 AccessToken（短时效，用于 API 鉴权）和 RefreshToken（长时效，用于静默续期）。通过 `jwtAuth` Security Scheme 注入中间件鉴权链路。
_Avoid_: jwt token, session token

## LLM Proxy（LLM 代理）

**Endpoint（上游端点）**:
一个上游 LLM 服务连接配置，包含名称、OpenAI 和 Anthropic 两个协议的 Base URL、共享 API Key，以及各接口（OpenAI Chat Completion / OpenAI Response / Anthropic Message）的支持标记。通过 `EndpointResolver` 按模型别名解析出目标端点。
_Avoid_: upstream, provider, backend

**Model（模型别名）**:
一条将对外别名（`alias`）映射到上游真实模型名的记录，归属于某个 Endpoint。同一别名可通过不同 endpoint_id 关联多个端点以支持负载均衡。客户端请求时传入别名，网关解析后转发到对应的上游模型。
_Avoid_: model mapping, model route, alias record

**EndpointAlias（端点别名）**:
客户端请求中暴露的模型名。与上游实际模型名分离：`alias` 是客户端看到的（如 "gpt-4"），`model` 是向上游真正发送的名称（如 "gpt-4-turbo-2024-04-09"）。
_Avoid_: model name, exposed name

**UpstreamCreds（上游凭证）**:
调用上游 LLM 所需的 BaseURL 与 APIKey。由 EndpointResolver 根据请求协议（OpenAI / Anthropic）从 `Endpoint` 表取对应协议的 Base URL + 共享 API Key 组合而成，与 `Model`（上游真实模型名）一同返回给 UseCase。
_Avoid_: connection info, auth config

**ProtocolType（协议类型）**:
网关支持的三种上游 LLM 协议：`openai-chat-completion`（OpenAI Chat Completions）、`openai-response`（OpenAI Response API）、`anthropic-message`（Anthropic Messages）。决定请求的序列化/反序列化方式和传输通道。网关支持跨协议转换（如 OpenAI 接口调用 Anthropic 上游）。
_Avoid_: provider type, api type

**EndpointResolver（端点解析器）**:
按模型别名解析出目标 Endpoint 和 Model 的领域服务。输入 `alias`，查 `model` 表收集所有关联的 `endpoint_id`，随机选一个，再查 `endpoint` 表组装 `UpstreamCreds`。调用方根据请求协议取对应 Base URL 并检查接口支持标记。
_Avoid_: model router, endpoint lookup

**Cross-Protocol Conversion（跨协议转换）**:
网关的核心能力：客户端使用 OpenAI 协议，网关可将其转换为 Anthropic 协议再转发，反之亦然。覆盖 7 条转发路径（OpenAI Chat native、Chat→Anthropic、Response native、Response→Chat、Response→Anthropic、Anthropic Message native、Message→Chat）。
_Avoid_: protocol translation, api bridge

**ClientConfigExport（客户端配置导出）**:
管理后台从模型列表一键生成「让外部 Agentic 客户端接入本网关」的安装脚本的纯前端能力（无后端接口）。当前支持两种目标：OpenCode（在 provider 字典里注册多个模型，patch `~/.config/opencode/opencode.json`）与 Claude Code（按 opus/sonnet/haiku 三档别名映射 `ANTHROPIC_DEFAULT_*_MODEL` 环境变量、用 `ANTHROPIC_AUTH_TOKEN` 认证、指向 `/api/anthropic/v1`，patch `~/.claude/settings.json` 的 env 块）。生成的 bash 脚本内嵌 Python 做幂等 patch 并备份 `.bak`。
_Avoid_: config generator, setup wizard, integration script

**ClaudeCodeModelTier（Claude Code 模型档位）**:
Claude Code 通过别名解析模型的三个档位：`opus`（最强推理，主任务与计划模式，对应 `ANTHROPIC_DEFAULT_OPUS_MODEL`）、`sonnet`（均衡，日常编码与 opusplan 执行，对应 `ANTHROPIC_DEFAULT_SONNET_MODEL`）、`haiku`（快速廉价，后台与子任务，对应 `ANTHROPIC_DEFAULT_HAIKU_MODEL`，已取代废弃的 `ANTHROPIC_SMALL_FAST_MODEL`）。导出时每档独立从模型别名中选取、可留空，留空档位不写入对应环境变量。上下文窗口不是 per-model 字典（与 OpenCode 不同），Claude Code 内置各模型的窗口大小；当某档选中模型的 `contextLength` 达到 1M 时，导出会在别名后追加 `[1m]` 后缀以启用 1M 上下文（Claude Code 转发上游前自动剥离该后缀）。
_Avoid_: model level, model size, model class

## Session & Conversation（会话与对话）

**Session（会话）**:
一次完整的 LLM 交互聚合：由 Proxy Capture 或 Transcript Ingestion 沉淀，包含多个 Message（消息）和 Tool（工具调用）。归属于某个 API Key Owner。支持人工评分（`SessionScore`，范围 1-5）。可被分享（`SessionShare`）给未登录用户查看。
_Avoid_: conversation（"Conversation" 用于其他语境不可混用）, chat, thread

**Message（消息）**:
一次 LLM 请求/响应中的单条消息。含 Role（user / assistant / tool / system）、Content（UnifiedMessage）、上游模型名、校验和。基于内容寻址去重（`checksum`），不可变（无 Update 接口）。存储在 `messages` 表，内容为 JSON 列。
_Avoid_: chat message, prompt, response

**UnifiedMessage（统一消息体）**:
网关自定义的协议无关消息结构。Content 为 UnifiedContent（支持 text 或多模态 parts），额外包含 ReasoningContent（推理内容）、ToolCalls、ToolCallID、Refusal，以及压缩相关字段（RawContent / CompressionStrategy）。是 Message 表 JSON 列的持久化格式。
_Avoid_: message dto, generic message

**Tool（工具调用）**:
LLM 发起的工具（函数）调用记录。含 ToolCallID、调用参数、执行结果。内容寻址去重（checksum），不可变。通过 `Message.ToolCalls` 关联到对应消息，执行结果以 `role=tool` 或 `role=user + tool_call_id` 的消息形式回传。
_Avoid_: function call, tool use, tool result

**SessionScore（会话评分）**:
用户对会话的人工评分（1-5 整数），nil 表示未评分。带评分时间。幂等：重复评分覆盖之前的值。由 Session 聚合的 `UpdateScore` 方法管理。
_Avoid_: quality score, rating, feedback score

**SessionShare（分享链接）**:
用户将会话内容分享给未登录用户查看的机制。基于 UUID 分享标识，存储在 Redis（默认 24h 过期，支持 1d/7d/30d/never/custom），无鉴权但基于 IP 限流。通过反向索引 `session_shares:{sessionID}` Set 做重复分享校验。
_Avoid_: share link, public link, shared session

**APIKeyOwner（会话所有者）**:
Session 所属的 API Key 名称值对象，来自鉴权中间件注入的 context。用于权限校验：用户只能访问其 API Key 名下的会话。
_Avoid_: owner name, api key identifier

## Model Call Audit（模型调用审计）

**ModelCallAudit（模型调用审计）**:
每次 LLM 模型调用的完整审计记录。由审计聚合根 `RecordCall` 工厂方法构造，包含：API Key ID、Model ID、模型名、协议（入口/上游）、Endpoint、token 四维统计、延迟两段（首 token / 流式持续）、调用状态、User-Agent、Trace ID。通过异步任务 `ModelCallAuditTask` 经由协程池写入，不阻塞响应。
_Avoid_: model call log, usage record, api audit

**TokenBreakdown（Token 统计）**:
模型调用的 token 用量四维统计：Input（输入）、Output（输出）、CacheCreation（缓存创建，仅 Anthropic）、CacheRead（缓存读取，两边均可能有）。覆盖 OpenAI / Anthropic / Response API 三种上游的 token 字段。
_Avoid_: token usage, token count, usage stats

**CallLatency（调用延迟）**:
模型调用的两段延迟：FirstToken（首 token 延迟，非流式 = 总延迟）、Stream（流式传输持续时间，从首 token 到流结束；非流式 = 0）。
_Avoid_: response time, latency, duration

**CallStatus（调用状态）**:
模型调用的结果状态，含 UpstreamStatusCode（200=成功，>0=HTTP 状态码，-1=连接错误，0=未知错误）和 ErrorMessage（错误信息）。
_Avoid_: response status, call result

**Granularity（聚合粒度）**:
审计数据按时间窗口聚合的粒度：`minute` / `hour` / `day` / `week`。用于仪表盘的时序图和时间范围选择。
_Avoid_: interval, time unit, bucket size

## Blocked Words（敏感词）

**Blocked（敏感词）**:
管理员配置的敏感词黑名单条目。每条记录一个 `word`（敏感词内容）和 `hitCount`（命中次数）。通过 Aho-Corasick 自动机做 O(n) 子串匹配。LLM 代理请求内容命中时返回 403 Forbidden 并记录审计。
_Avoid_: blocked word, ban word, forbidden term

**BlockedService（敏感词服务）**:
管理 AC 自动机生命周期的领域服务。启动时从 DB 加载所有活跃敏感词构建自动机；增删敏感词后重建（`sync.RWMutex` 保护）；提供 `Check(text) []uint` 方法，返回所有命中词 ID。命中计数先递增 Redis（`blocked:hit:{id}`），再由 cron 定时同步回 DB。
_Avoid_: word filter, content moderation, block service

## Authentication Middleware（认证中间件）

**APIKeyMiddleware（密钥鉴权中间件）**:
从 HTTP Header `X-API-Key` 提取 API Key，查数据库校验有效性，将 UserID、APIKeyID、APIKeyName、Permission 注入请求 context。未通过、不存在、已禁用时返回 401。是 LLM 转发链路的主鉴权方式。
_Avoid_: key auth, api key check, credential middleware

**JwtMiddleware（JWT 鉴权中间件）**:
从 `Authorization: Bearer <token>` 提取 Token，通过 JWT 公钥 + Redis 缓存校验。注入 UserID、Permission 到 context。用于管理后台 API（非 LLM 转发路径）。支持 refresh token 自动续期。
_Avoid_: jwt auth middleware, token validation

## Rate Limiting（限流）

**Request Rate Limiter（请求维度限流）**:
基于 Redis 令牌桶的请求频率限流中间件，每个 LLM 转发请求固定消耗 1 个令牌。可按 IP 或 API Key ID 为维度。桶耗尽时返回 429 + `Retry-After`。
_Avoid_: rate limit, throttling, qos

**Token Rate Limiter（Token 维度限流）**:
基于 Redis 令牌桶的 Token 用量限流中间件，按实际 token 用量（input + output）扣减。请求前 peek 桶（只读令牌数），请求后由 UseCase 通过 `TokenUsageReporter` 上报实际用量并扣减（deduct Lua），桶可 transient 为负。默认 1,000,000 TPM。
_Avoid_: token limit, usage cap, quota control

## Compression（消息压缩）

**Compression（请求体压缩）**:
在转发上游 LLM 前，对请求体中的 tool output 内容做确定性压缩以减少传输 token 用量。直接在 typed DTO 上 in-place 修改（非序列化 bytes 方案），每个 Locator（OpenAIChat / AnthropicMessages / OpenAIResponses）按协议特异性定位 tool output。
_Avoid_: content compression, body compression, payload shrinking

**ItemCompressionResult（压缩项结果）**:
单条 tool output 的压缩记录。含 ToolCallID（关联到存储消息）、Input（压缩前内容）、Output（压缩后内容或跳过/失败时的原始内容）、Strategy（策略名，如 smart_crusher / log_compressor / search_compressor / passthrough）、Applied（是否实际压缩）。
_Avoid_: compression record, tool output compression

## Cron & Maintenance（定时维护）

**Distributed Cron Lock（分布式 Cron 锁）**:
基于 Redis SETNX 的分布式互斥锁，确保每个 cron 任务在集群中同一时刻只有一个实例执行。锁 Key 模板 `cron:lock:{module}`，带 TTL 自动过期。
_Avoid_: cron mutex, scheduled lock, scheduler lock

**SoftDeletePurge（软删除清理）**:
定时 cron 任务，遍历被软删除的 Session，提取引用的 Message/Tool IDs，与活跃 Session 引用的 IDs 计算差集后硬删除孤儿记录。避免多 Session 共享 Message/Tool 时误删。使用 `lo.Difference` 做集合运算。

**SessionDedup（会话去重）**:
定时 cron 任务，检查 Session 间是否存在内容完全相同的 Message/Tool 引用（通过 checksum 比对），合并冗余引用，软删除空 Session。

**CronModule（定时任务模块）**:
平台所有定时任务遵循 `CronRegistryEntry` 模式注册：`SessionScore`（已废弃，原 LLM 自动评分）、`SessionSummarize`（自动摘要）、`SessionDedup`（去重合并）、`ThinkExtract`（推理内容提取）、`SoftDeletePurge`（清理孤儿数据）。

**Transcript Ingestion（会话摄取）**:
对订阅制 Agentic 工具（Claude Code、Codex）流量的离线捕获方式。读取工具写在本地的会话文件后摄取进平台。沉淀为 Trace（沉淀会话）。与 Proxy Capture 是两条互斥的数据入口。
_Avoid_: log scraping, import, sync, ingestion

## Metrics & Monitoring（指标与监控）

（继续使用现有 CONTEXT.md 中的 Metrics、Data Capture 部分，不做改动。）

## Infrastructure（基础设施）

**Dig Container（DI 容器）**:
使用 `go.uber.org/dig` 管理全部依赖注入。所有 Provider（Repository、UseCase、Handler、Cache、Cron）在 `internal/bootstrap/container.go` 中统一注册，按模块分组。使用 `fx.Module` / `fx.Annotate` 模式管理命名实例、接口绑定和分组。
_Avoid_: di container, dependency graph, ioc container

**Pond Pool（协程池）**:
基于 `github.com/alitto/pond` 的协程池，管理异步任务提交。提供 `SubmitModelCallAuditTask`（审计写入）、`SubmitMessageStoreTask`（消息持久化）等池方法。池大小、队列容量可配置，优雅关闭时 draining 等待进行中任务。
_Avoid_: goroutine pool, task queue, worker pool

**SessionDetailCache（会话详情缓存）**:
三层 Redis 读缓存（SessionMeta + Message + Tool）以加速 session 详情加载。缓存 TTL 统一 1h，不做主动失效。cache miss 降级到 DB，不阻断请求。适用于 session 详情 metadata 接口和 message/tool 分页列表接口。
_Avoid_: detail cache, session cache, performance cache

**Graceful Shutdown（优雅关闭）**:
接收 SIGINT/SIGTERM 后按顺序执行 8 步关闭：停止 cron → 停止协程池 → draining 拒绝新请求 → 等待在途请求完成 → 关闭 HTTP Server → 同步日志 → 关闭 DB → 关闭 Redis。K8s 部署配合 `terminationGracePeriodSeconds: 660` + `preStop: sleep 10` 实现无损下线。
_Avoid_: shutdown sequence, graceful stop, pod termination
