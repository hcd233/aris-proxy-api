# Aris

[English](README.md) | **简体中文**

面向个人大模型使用场景的 **MaaS 模型服务聚合网关 + 使用数据沉淀平台 + Agent Harness 调用链路分析平台**。

Aris 将多个 MaaS 平台、多种模型和多种 API 协议统一接入，通过模型别名、上游端点管理、协议转换、鉴权、限流、审计和可视化管理，客户端只需面向统一网关地址和模型别名发起请求，无需感知供应商、真实模型名和上游接口差异。

## 核心能力

- **模型聚合与统一入口**：Endpoint/Model 两级配置（用户级多租户，普通用户管理自己的端点与模型，admin 可按用户过滤与代建），模型别名屏蔽上游差异，一个别名可关联多个 Endpoint；同时提供 OpenAI Chat Completions、OpenAI Responses、Anthropic Messages 兼容入口，并支持 OpenAI 与 Anthropic 之间的跨协议转换（含 SSE 流式事件）。
- **数据沉淀与治理**：模型交互聚合为 Session，Message/Tool 以协议无关结构存储并按 checksum 内容寻址去重；支持评分、分享链接、会话筛选与 ShareGPT JSONL 流式导出，为私有模型 SFT 沉淀数据资产。
- **审计与运行监控**：每次调用记录 Token 四类用量、首 Token 延迟、流式时长、上游状态码和 Trace ID；提供趋势/成功率/吞吐/用量统计、Cron 审计，以及跨 Pod 的 QPS、P95、SSE 连接、goroutines/heap/CPU 运行时监控。
- **Agent Harness Trace 分析**：独立编译的 `aris` CLI 采集 Codex Hook 事件与 rollout 记录，本地 spool 批量上报；服务端将事件重建为可阅读的 TraceConversation，用于观察 Agent 的模型请求、推理过程与工具调用链路。
- **订阅账号 OAuth 接入（Claude Code / Codex）**：订阅账号以独立的端点类型（`oauth_anthropic` / `oauth_openai`）经官方 OAuth 授权接入。access token 自动刷新（请求路径 3 分钟提前量 + 维护 Cron），凭据以 AES-256-GCM 密文存储；熔断键使用端点级稳定身份，token 轮换不会重置熔断状态。模型别名像普通端点一样绑定——调度、fallback、审计、计价与数据治理零改动复用；订阅窗口用量（Claude 5h/7d、Codex primary/secondary）由 Cron 探测并在管理端展示。
- **客户端配置导出**：从管理后台一键生成 OpenCode、Claude Code、Codex、Pi 的接入配置或安装脚本，幂等 patch、`.bak` 备份、原子替换、凭证文件 `0600`。
- **安全与治理**：GitHub/Google OAuth2 登录 + JWT 管理鉴权，Proxy API Key 调用鉴权，`pending`/`demo`/`user`/`admin` 四级权限与用户审核（demo 演示账户：模块白名单 + 会话白名单/全量脱敏双视角 + 一键登录 + 访问审计），Aho-Corasick 触发词拦截，Redis 令牌桶请求限流与 Token 用量限流。

## 同类项目横向对比

对比四个主流 MaaS 开源项目（快照 **2026-10-19**）：[new-api](https://github.com/QuantumNous/new-api)（Go/Gin + React；AGPLv3 + 第 7 条附加条款）、[LiteLLM](https://github.com/BerriAI/litellm)（Python + React；MIT + 商业企业授权）、[OmniRoute](https://github.com/diegosouzapw/OmniRoute)（Node/TypeScript + SQLite；MIT）、[sub2api](https://github.com/Wei-Shaw/sub2api)（Go/Gin + Vue；LGPL-3.0）。只对比技术侧能力，不含产品与运营侧功能。

三点结构性差异：

- **对话正文是数据模型选择，不是路线图排期。** Aris 落库，因此内容寻址去重、评分与 ShareGPT/SFT 导出是结构性能力；其余四家没有会话聚合——LiteLLM 可显式开启，但数据仍是 spend-log 形态，其数据集导出面向评测。
- **「配额强控」与「成本估算」是两种设计。** new-api 与 sub2api 走配额/余额扣减（预扣 → 结算、窗口软上限）；LiteLLM 的预算族依赖数据库，**无 DB 时 fail open**；Aris 只做成本估算，没有预算闸门。
- **Agent 可观测性是同一个问题的三种答案。** Aris：客户端 Hook + 本地 spool + 服务端重建，不改动 Agent 运行时；LiteLLM：SDK harness + CLI 包装 + 服务端 agentic loop，且 trace 分析落在独立的 Lens 服务；OmniRoute：在流量层捕获，可视化对象是路由与压缩而非 Agent 步骤。

标记：✅ 有 · ◐ 部分 · — 无。评分依据各项目在快照时点的自身 README/官方文档；带编号的单元格见下方[参考说明](#参考说明)。

| 维度 | Aris | new-api | LiteLLM | OmniRoute | sub2api |
| --- | --- | --- | --- | --- | --- |
| **协议入口覆盖** | ◐[1] | ✅[1] | ✅[1] | ✅ | ◐[1] |
| **跨协议转换** | ✅ | ✅[2] | ✅ | ✅ | ◐[2] |
| **上游渠道类型（API Key / 订阅 OAuth）** | ✅[3] | ◐[3] | ✅[3] | ✅[3] | ✅[3] |
| **模型别名与多端点绑定** | ✅ | ✅ | ✅ | ✅ | ◐[4] |
| **路由与调度** | ✅[5] | ✅[5] | ✅[5] | ✅[5] | ✅[5] |
| **熔断 / 隔离 / 降级** | ✅[6] | ◐[6] | ◐[6] | ✅[6] | ✅[6] |
| **多租户与权限** | ✅ | ✅ | ✅ | —[7] | ✅ |
| **认证与登录** | ✅[8] | ✅ | ✅[8] | ◐[8] | ✅ |
| **限流与并发控制** | ✅[9] | ✅ | ✅[9] | ✅ | ✅ |
| **用量计量与配额强控** | ◐[10] | ✅[10] | ✅[10] | ◐[10] | ✅[10] |
| **调用审计与用量统计** | ✅[11] | ✅[11] | ✅ | ◐ | ◐[11] |
| **对话正文落库** | ✅[12] | —[12] | ◐[12] | — | — |
| **对话数据集导出（ShareGPT/SFT）** | ✅[13] | — | ◐[13] | — | — |
| **Agent Harness 调用链采集** | ✅[14] | — | ✅[14] | ◐[14] | — |
| **Trace 重建与可视化** | ✅[15] | — | ✅[15] | ◐[15] | — |
| **内容安全与脱敏** | ✅[16] | ◐[16] | ✅[16] | ✅ | ◐ |
| **可观测性与运行时监控** | ✅ | ◐[17] | ✅[17] | ◐[17] | ◐[17] |
| **Token 压缩** | ✅[18] | — | ◐[18] | ✅[18] | — |
| **MCP / A2A** | — | ◐[19] | ✅[19] | ✅[19] | ◐[19] |
| **部署与水平扩展** | ✅[20] | ✅[20] | ✅[20] | ◐[20] | ✅ |
| **扩展机制** | ◐[21] | ✅[21] | ✅[21] | ✅[21] | ✅[21] |

### 说明

**1 · 协议入口覆盖** — Aris ◐：仅文本协议（Chat/Responses/Messages + Decision 原生透传）。new-api：四种文本协议 + Realtime/Responses WebSocket + 媒体 + 任务插件路由，约 60 种渠道类型。LiteLLM：面最广——batches/files/fine-tuning/evals/realtime/videos/ocr/search/memory、Bedrock/Vertex 原生、MCP + A2A。sub2api ◐：面向订阅客户端（Responses 含 `/backend-api/codex/responses`、Messages、Chat、`/v1/systemone`、Antigravity）。

**2 · 跨协议转换** — Aris：OpenAI ↔ Anthropic 共 7 条路径，含 SSE 流式事件。new-api：RelayKit 转换四种文本协议，但提示协议特有工具/字段可能无法精确映射。LiteLLM：`/v1/messages` 覆盖全部支持的提供商，Rust 核心仅少量路由。sub2api ◐：按提供商定向转换（Anthropic → xAI Responses、Codex Responses ← Chat、Antigravity）。

**3 · 上游渠道类型** — Aris ✅：`api_key` + 订阅 OAuth 端点（`oauth_anthropic` / `oauth_openai`，AES-256-GCM，3 分钟提前量刷新），共用同一转发链路。new-api ◐：约 60 种 API-Key 渠道 + `ChannelTypeCodex = 57 "ChatGPT Subscription (Codex)"`（OAuth key JSON，仅 `/v1/responses`）；Claude Code OAuth 渠道（原 type 53）**已被回滚**。LiteLLM ✅：API Key + ChatGPT 订阅 device flow + Copilot 逐用户 OAuth，前者无 cost map 价（spend 恒为 $0），Anthropic Pro/Max 未文档化。OmniRoute ✅：四层 fallback（Subscription → API Key → Cheap → Free）+ PKCE 登录。sub2api ✅：OAuth 订阅账号池（Claude/Codex/Grok/Antigravity）+ 粘性会话。

**4 · 模型别名与多端点绑定** — sub2api ◐：Composite Groups 把模型解析到具体提供商；其余四家为一等公民的「别名 → N 端点」绑定（带优先级与权重）。

**5 · 路由与调度** — Aris：优先级分档 + 档内 A-Res 加权洗牌 + 会话亲和（TTL 5 分钟，fail-open）。new-api：加权随机 + 可拖拽优先级 + 渠道亲和 + `auto` 跨分组重试。LiteLLM：5 种策略（默认 `simple-shuffle`）+ 四类专用路由器 + Auto Router。OmniRoute：19 种 combo 策略 + 16 因子打分。sub2api：粘性会话 + 分组内选号。

**6 · 熔断 / 隔离 / 降级** — Aris ✅：重试循环外 `Guard`——`BaseURL\|APIKey` 三态熔断（60s × 6 桶，≥10 请求且 ≥50% 错误打开，30s → 半开 1 次探测）+ 按 Key 信号量（≤32、1s）→ 503 + `Retry-After`，429 不计入。new-api ◐：定时健康探测 + 自动禁用/恢复，无阈值熔断。LiteLLM ◐：仅 router cooldown。OmniRoute ✅：提供商熔断（仅 408/5xx，8×/12×/2× → 半开 60/30/15s）+ 连接冷却（×2 退避，遵循 `Retry-After`）+ 单模型锁死 + 过载排队。sub2api ✅：账号错误策略 + 临时不可调度 + failover。

**7 · 多租户与权限** — OmniRoute —：单运营者，仅 API-Key 作用域、IP 过滤与 33 个 MCP scope。Aris：用户级多租户。new-api：用户/分组/令牌 + 细粒度管理权限。LiteLLM：Organizations → Teams → Users → Virtual Keys（企业版组织/团队管理员）。sub2api：用户/分组 + 用户级与账号级并发。

**8 · 认证与登录** — Aris ✅：OAuth2 + JWT + API Key，**无 2FA 与 passkey**。LiteLLM ✅：SSO（OIDC/SAML，≤5 用户免费）+ JWT-OIDC 懒注册 + SCIM（企业版）+ 登录失败限流。OmniRoute ◐：密码登录 + 可选 OIDC 网关。

**9 · 限流与并发控制** — Aris ✅：按 IP/API Key 100 次/分钟；Token 维度中间件存在但 **TPM 刻意等效不限**（`LimitCallProxyLLMToken = 1e8`，源码注释为设计决定）。LiteLLM ✅：四级 RPM/TPM，`enforce_model_rate_limits` 升级为硬限制（429 + `retry-after: 60`）；多实例必须共享 Redis，否则按进程各算。

**10 · 用量计量与配额强控** — Aris ◐：按模型定价（微单位、时段窗口 + 上下文分档 + 缓存创建 5m/1h 分档、调用时刻快照），但**只估算不拦截**，配额是每 Key 请求数上限。new-api ✅：配额点数（1 USD = 500,000）预扣 → 结算 → 差额调整 + 分组/令牌配额 + 订阅额度与重置周期。LiteLLM ✅：预算族（Key/用户/团队/团队成员/Tag/全局），**依赖数据库——无 DB 时 `max_budget` fail open**。OmniRoute ◐：按 Key 的 USD 花费配额 + 成本分析（订阅类提供商显示 $0）。sub2api ✅：Token 级成本 + 余额/订阅扣减 + 5h/1d/7d 窗口（**请求后软上限**，在途可超出）。

**11 · 调用审计与用量统计** — Aris ✅：逐调用 `ModelCallAudit`（Token 四维、首 Token 与流式两段延迟、上游状态码、UA、Trace ID、估算费用），协程池异步落库。new-api ✅：使用/绘图/任务日志含 Token、费用、耗时与首字时间、请求 ID/上游请求 ID，管理员另有重试链、请求转换与计费路径。sub2api ◐：仅看板级请求/Token/费用与账号状态。

**12 · 对话正文落库** — Aris ✅：每次调用的完整历史落 Session 聚合，Message/Tool 按 checksum 去重、协议无关存储，同前缀快照实时合并（例外：触发词 `omit` 与 Playground）。new-api —：日志字段只有 Token/费用/延迟/ID，请求记录 prompt 的 issue #924/#2721/#2982/#4852 全部 closed。LiteLLM ◐：默认关闭，需 `store_prompts_in_spend_logs` 开启，另有 `turn_off_message_logging`、`no-log` 脱敏开关。

**13 · 对话数据集导出** — Aris ✅：ShareGPT JSONL 流式导出（SSE 带进度），保留多轮对话、`<think>` 思维链与工具调用，支持评分/模型/时间窗筛选与导出前统计预览。LiteLLM ◐：Lens datasets（Export JSONL）面向**评测**，需独立 Lens 服务 + ClickHouse，无 ShareGPT/SFT 格式。

**14 · Agent Harness 调用链采集** — Aris ✅：独立编译的 `aris` CLI 注册 Codex/Claude Code Hook，fail-open，本地 spool（`0600`，256 MiB 上限）批量上报。LiteLLM ✅：三套并存——`litellm.agent()` SDK harness（8 类 typed event）、`lite claude\|codex\|opencode` 包装器（打 `harness,<name>` 标签）、服务端 `agentic_loop_hook`（默认 3 次重跑，流式 Chat 不触发）；均 beta，沙箱需预装 CLI。OmniRoute ◐：一键 CLI 接入 + MITM/TPROXY 捕获，属流量层而非 Hook 事件级。

**15 · Trace 重建与可视化** — Aris ✅：服务端重建为 `TraceConversation`，按消息/工具/`session_meta` 时间线渲染并可展开原始 payload。LiteLLM ✅：Lens（独立服务 + ClickHouse）提供对话视图、subagent 分支、Investigations 与 OTLP API——early access，`/lens` 仅 proxy admin 可用，Claude Code 依赖其自带 OTel exporter（有覆盖缺口），Codex 依赖第三方集成（自动安装仅 macOS）。OmniRoute ◐：Studios 可视化路由级联与压缩瀑布，非 Agent 调用链。

**16 · 内容安全与脱敏** — Aris ✅：Aho-Corasick 触发词三动作（`deny`/`omit`/`capture`），Redis pub/sub + 版本轮询热更新。LiteLLM ✅：约 50 个 guardrail 集成、四种模式，但**流式输出上改写不生效**，且 `llmguard`/`llamaguard`/`hide_secrets`/`lakera` 需企业授权。new-api ◐：仅有渠道额外设置与上游 Header 覆盖。

**17 · 可观测性与运行时监控** — LiteLLM ✅：指标族最完整（TTFT、overhead、预算余量、准入/拒绝、数据库队列）、2 个官方 Grafana 面板，专用指标端口绕过虚拟 Key 鉴权（勿暴露公网）。new-api ◐：看板 + 健康探测 + Webhook 告警 + pprof/Pyroscope，仅自定义 `/api/perf-metrics`，**无 Prometheus 依赖**。sub2api ◐：管理台 + 渠道监控，`go.mod` 无 Prometheus 依赖。OmniRoute ◐：看板 + `X-OmniRoute-*` 头 + SQLite 审计轨迹，无 `/metrics`。

**18 · Token 压缩** — Aris ✅：转发前对 tool output 做确定性压缩，typed DTO 就地修改，策略 `smart_crusher`/`log_compressor`/`search_compressor`/`passthrough` 并记录压缩前后。OmniRoute ✅：12 引擎流水线（RTK、Caveman、LLMLingua-2、GCF v3.2、OmniGlyph）+ 默认开启膨胀保护。LiteLLM ◐：`compress()`（beta）。

**19 · MCP / A2A** — LiteLLM ✅：MCP Gateway（按 Key 的 server/tool 权限、MCP 成本追踪、`/mcp` 可被 Cursor 接入）+ A2A Agent Gateway。OmniRoute ✅：内置 MCP 服务端 110 工具（stdio/HTTP/SSE，33 scope）+ A2A v0.3。new-api ◐：仅 JS 任务插件 + 市场。sub2api ◐：仅 `hashicorp/go-plugin` 插件体系。

**20 · 部署与水平扩展** — Aris ✅：Compose/K8s（`preStop` + `terminationGracePeriodSeconds: 660`）+ 八步优雅关闭排空 SSE，跨 Pod 状态走 Redis。new-api ✅：多节点需共享主库 + 统一 `SESSION_SECRET`/`CRYPTO_SECRET`，共享限流需 Redis。LiteLLM ✅：Helm（HPA/KEDA）+ AWS/GCP 官方 Terraform（无 Azure）+ cosign 签名 + Prisma 迁移，多实例必须 Redis。OmniRoute ◐：单节点 SQLite，远程模式用作用域 token 驱动另一实例。

**21 · 扩展机制** — Aris ◐：无插件体系，扩展点是独立编译的 `aris` 客户端（新增 agent = 1 adapter 文件 + 2 行注册）、models.dev 导入与 Fx 装配。new-api ✅：JS 任务插件（文档化 API + 市场）+ 自定义 OAuth 提供商。LiteLLM ✅：8 个 `CustomLogger` 挂载点 + 任意语言 guardrail API + 自定义认证/SSO/密钥管理 + 改一个 JSON 注册提供商 + `s3://`/`gcs://` 加载回调。OmniRoute ✅：插件框架、市场与 skills 框架。sub2api ✅：`hashicorp/go-plugin` 与模块化 skills。

## 系统架构

后端按 DDD 分层组织：接口层（router / 十级中间件链 / handler / dto）→ 应用层（17 个用例模块的 `command`/`query`/`port`）→ 领域层（聚合、值对象、仓储接口）；基础设施层反向实现仓储接口完成依赖倒置，Cron 定时治理与 `bootstrap`/Fx 装配作为横切关注点。三类调用方分别是 Web 管理后台（JWT）、`aris` Trace CLI 与 Agentic 客户端（`X-API-Key`）。

![系统总览 · DDD 分层](docs/diagrams/archify/aris-arch-01.gif)

交互版（节点搜索 / 影响追踪 / 深浅主题 / 引导导览，节点附源码锚点）：[docs/diagrams/archify/aris-arch-01.html](docs/diagrams/archify/aris-arch-01.html)

**LLM 代理链路**：鉴权/限流/触发词 → 端点解析（用户级隔离）→ 跨协议转换 → 上游；响应同步返回，消息与审计经 Pond 协程池异步落库，不阻塞请求。

![LLM 代理链路](docs/diagrams/archify/aris-arch-02.gif)

**上游容错（熔断/隔离/降级）**：proxy 转发链路在重试外层接入容错守卫 `Guard`——按 `BaseURL|APIKey` 维度的三态熔断器（滑动窗口 60s × 6 桶、≥10 请求且错误率 ≥50% 打开、30s 后半开限量 1 探测恢复）与 per-key 信号量隔离（并发 ≤32、等待 1s 超时）组合判定；熔断打开或满载时快速失败为 503 + `Retry-After` + 按入口协议格式的错误体。429 仍可重试但不计熔断（限流不算故障），已建立的 SSE 连接不受熔断影响；熔断状态与拒绝计数注册 Prometheus 指标（label 为 endpoint key）。

![上游容错：熔断 / 隔离 / 降级](docs/diagrams/archify/aris-arch-14.gif)

**会话数据模型与生命周期**：LLM 转发与 Trace 摄取两路写入 Session 聚合根，Message/Tool 按 checksum 内容寻址去重，审计、缓存、导出与后台消费围绕其展开。

![会话数据模型与生命周期](docs/diagrams/archify/aris-arch-03.gif)

**Agent Harness Trace 摄取**：Codex / Claude Code Hook 与 Rollout 事件由 `aris` CLI（fail-open）本地 spool 批量上报，鉴权去重后落库，Web 端投影为可阅读的调用链路。

![Agent Harness Trace 摄取](docs/diagrams/archify/aris-arch-04.gif)

**Web 管理后台前端架构**：浏览器加载静态导出产物 → 根布局 Provider 链（I18n → Theme → Auth）→ 路由组（后台 14 页 / 公开 3 页，管理员页由 `PermissionGuard` 守卫）→ 组件与 6 个通用 hooks → `src/lib` 作为唯一后端出入口（Bearer 注入、401 单飞刷新、错误码 toast、数据集 SSE 导出）。

![Web 管理后台前端架构](docs/diagrams/archify/aris-arch-05.gif)

**前端构建与内嵌交付**：`next build` 静态导出（`basePath: /web`）→ `make web-build` 拷贝并逐文件 `gzip -9` 预压缩（10MB → 3.5MB）→ `embed.FS` 随服务端二进制发布 → `/web/*` 显式解析 `Accept-Encoding` 直发预压缩内容，非静态路径回落 `index.html`。

![前端构建与内嵌交付](docs/diagrams/archify/aris-arch-06.gif)

**运行时指标采集**：进程内 `HTTPCollector`（请求时延 histogram + 成功/失败 counter）、`SSEGauge`（SSE 并发连接）、`TokenUsageCounter`（输入/输出 token 吞吐）与 Go runtime 采集器（`go_goroutines` / `go_memstats_alloc_bytes` / `process_cpu_seconds_total`）注册进 Prometheus Registry；每 Pod 一个 `MetricsFlusher` 周期（5s）`BuildSnapshot` 把快照写入 Redis（ZSET `metrics:runtime:data:{pod}` + 实例注册表，retention 24h 清理）；运行时大盘经聚合层对相邻快照做 gauge 桶内均值 / counter 正向 delta÷桶宽 / histogram 桶合并求 P95，产出 QPS / P95 / 成功率 / CPU% / tokens·s；`/metrics` 端点经 `promhttp.Handler` 直读本 Pod Registry。

![运行时指标采集](docs/diagrams/archify/aris-arch-09.gif)

**优雅退出**：`SIGINT` / `SIGTERM` 触发 `fx.Lifecycle.OnStop` 逆序执行（注册顺序 Redis → DB → TriggerService → Logger → HTTP → Flusher → Inflight → Pool → Cron，故 OnStop 反序）；依次停 cron（`CronManager.StopAll`，3min）→ 停 `pond` 协程池（`StopWithContext`，3min）→ `inflight` drain 等在途（5min）→ 关 HTTP（`ShutdownWithContext` 30s，泄流 SSE 长连接，停 Flusher）→ 同步日志 + 停 TriggerService pub/sub → 关 DB → 关 Redis；K8s 配合 `preStop sleep 10` + `terminationGracePeriodSeconds 660` 无损下线。

![优雅退出序列](docs/diagrams/archify/aris-arch-10.gif)

**触发词热更新**：管理后台 `/api/web/v1/trigger` 写入 DB 后 `NotifyChanged` 经 Redis `Publish(trigger:changed)` 即时信号 + `INCR(trigger:version)` 版本广播；各 Pod `TriggerService` 通过 pub/sub 订阅、版本轮询（2s）与低频兜底（5min）三重同步触发全量 `Rebuild`（`ListAll` 重建内存 Aho-Corasick matcher）；LLM 请求经 `Check` 命中后按 action 拦截（deny）/ 打码（omit），命中计数经 `TriggerHitSync` cron（每 5min `PopAll` → `BatchIncrementHitCount`）批量回写 DB。

![触发词热更新](docs/diagrams/archify/aris-arch-11.gif)

**定时任务触发与执行**：管理后台 `/api/web/v1/cron` 更新 spec / enabled（`update_cron_job` 校验 spec、核心任务禁关）或手动触发 `CronManager.Trigger`；`CronManager` 热重载（`Restart` / `Enable` / `Disable`）并经 `cron:reload` Redis pub/sub 跨 Pod 广播（其他 Pod `handleMessage` 同步）；任务以 Redis 分布式锁（`cron:lock:{name}`，TTL 5min + ticker 续期）保证单实例执行，`wrapCronFunc` 注入 traceID / enabled 检查 / panic 恢复，4 个任务（terminal-cleanup 每小时扫描最近 24h 终态会话 / purge 周日 04:00 / think 每日 00:00 / hit-sync 每 5min）每次执行落 `CronCallAudit`（status + duration + trigger source）。Session 前缀去重不再走 cron——插入事务提交后以独立短事务 + `FOR UPDATE` 行锁实时合并同前缀会话。

![定时任务触发与执行](docs/diagrams/archify/aris-arch-12.gif)

**Demo 演示账户架构**：四级权限 `pending` < `demo` < `user` < `admin`，全局单例 demo 账户一键只读体验。登录页「Continue as Demo」经无鉴权 IP 限流端点（令牌桶 5s/8）调用 `POST /api/web/v1/demo/login`，DemoLogin 命令校验 `login_enabled` 并按 permission 定位 demo 用户签发 JWT，登录与模块访问事件（path/IP/UA）落 `demo_access_audits` 供 admin 审计；前端以 `isDemo()` + `demoModules` 驱动 PermissionGuard 三态守卫与 Nav/删除按钮置灰锁定。demo 只读请求先过访问防线：权限中间件按模块白名单放行（fail-closed），`TokenBucketRateLimiterMiddleware` 以 `WithPermissionFilter(demo)` 仅对 demo 权限启用 `demoAccess` IP 令牌桶（5s/30，8 个接口组全局共享一桶，超限 429 + `Retry-After`；非 demo 用户零开销放行）。数据分两路视角：**会话白名单**——admin 在独立 `/demo` tab 勾选批量增删 `demo_sessions`（`session_id` 唯一索引），demo 列表走 `ListSessionsByIDs`、详情经 `IsAllowed` 成员校验，不在白名单一律返回"不存在"（防遍历），白名单内容明文（admin 选取即授权）；**全量脱敏**——audit/models/endpoints 展示全量数据但关键字段脱敏（身份类 `MaskIdentity` → `***`，连接类 `MaskSecret` → 保留前 4 后 4），统计数字与别名有意保留。全部写接口与存量 API Key 的 LLM 转发因 demo 权限低于 user 被天然拒绝，零额外改动。

![Demo 演示账户架构](docs/diagrams/archify/aris-arch-13.gif)

## 技术栈

| 层 | 技术 |
| --- | --- |
| 后端 | Go 1.25 · Fiber v3 · Huma v2 · Cobra · Viper · Uber Fx |
| 数据 | PostgreSQL + GORM · Redis（缓存/令牌桶/Cron 锁）· SQLite（单测）· Pond 协程池 |
| 可观测 | Zap + Lumberjack · 腾讯云 CLS · Prometheus · fgprof · `X-Trace-Id` 链路追踪 |
| 前端 | Next.js 16 (App Router) · React 19 · TypeScript · Tailwind CSS 4 · Base UI/shadcn · Recharts · Mermaid |
| 交付 | 前端静态产物 gzip 预压缩 + `embed.FS` 内嵌 · Docker 多阶段构建（Distroless nonroot）· Docker Compose · Kubernetes · GitHub Actions |

## 快速开始

### 环境要求

- Go 1.25+、Node.js 22+（仅前端开发/构建需要）
- PostgreSQL、Redis（可用 Docker Compose 启动）

### 启动依赖与配置

```bash
git clone https://github.com/hcd233/aris-proxy-api.git
cd aris-proxy-api
go mod download

# 准备配置（按需修改数据库、Redis、JWT Secret、OAuth2 等）
cp env/api.env.template env/api.env
cp env/postgresql.env.template env/postgresql.env
cp env/redis.env.template env/redis.env

# 启动 PostgreSQL 和 Redis
docker volume create postgresql-data && docker volume create redis-data
docker compose -f docker/docker-compose-full.yml up -d
```

### 迁移并启动服务

```bash
# 数据库迁移
go run ./cmd/server database migrate

# 构建前端并嵌入（生产模式必需；纯后端开发可跳过）
make web-build

# 启动服务
go run ./cmd/server server start --host localhost --port 8080
```

启动后访问：

- 管理后台 `http://localhost:8080/`
- 健康检查 `/health` · 就绪检查 `/ready` · SSE 健康检查 `/ssehealth`
- 开发环境 API 文档 `/docs`、OpenAPI `/openapi.json`（生产环境自动关闭）

首次使用：通过 OAuth2 登录管理后台 → 管理员审核用户 → 创建 Proxy API Key → 配置 Endpoint 和 Model → 即可用 OpenAI/Anthropic 兼容接口调用。

### 前端独立开发

```bash
cd web && npm ci && npm run dev   # http://localhost:3000
```

## 常用命令

| 命令 | 说明 |
| --- | --- |
| `make build` | 生产构建服务端 + 四平台 Trace 客户端 |
| `make build-server` / `make build-client[-all]` | 分别构建服务端 / `aris` 客户端 |
| `make web-build` | 构建前端到 `internal/web/dist` 并 gzip 预压缩 |
| `make test` / `make test-cover` | 全量测试 / 覆盖率报告 |
| `make lint` | 自定义规范 lint（`lint conv`）+ 静态检查（vet/staticcheck/golangci-lint） |
| `make web-lint` / `make web-format` | 前端 ESLint / Prettier |
| `make fgprof` | 拉取远程 fgprof profile 并打开火焰图 |

服务端 CLI：`server start`、`database migrate`、`lint conv/static`（`cmd/server`）；Trace 客户端 CLI：`aris init`、`aris model export`、`aris trace ingest`、`aris trace install`、`aris status`、`aris update`（`cmd/client`）。

## API 概览

所有 API 由 Huma 注册并生成 OpenAPI，开发环境可在 `/docs` 交互查看。

| 前缀 | 认证 | 说明 |
| --- | --- | --- |
| `/api/openai/v1` · `/api/anthropic/v1` | `X-API-Key` | LLM 兼容入口：chat/completions、responses、messages、count_tokens、models |
| `/api/cli/v1/model/list` · `/trace/event` · `/aris/client/check` | `X-API-Key` | `aris` 客户端模型分发、Trace 事件上报与客户端 Key 检查 |
| `/api/web/v1/oauth2` · `/token` | 公开（限流） | OAuth2 登录/回调、Token 刷新 |
| `/api/web/v1/user` | JWT | 个人资料；管理员可列表、审核（approve/demote）、删除用户 |
| `/api/web/v1/apikey` · `/session` · `/dataset` · `/trace` | JWT + Owner 隔离 | API Key、会话/分享、数据集导出、Trace 查询 |
| `/api/web/v1/endpoint` · `/model` · `/upstream` | JWT（用户级多租户） | 端点、模型（用户隔离，admin 可按用户过滤与代建）、上游聚合视图 |
| `/api/web/v1/trigger` · `/cron` · `/audit/cron` · `/metrics` | JWT + admin | 触发词、Cron 管理/触发、Cron 审计、运行时指标 |
| `/api/web/v1/audit` | JWT | 模型调用审计与统计（管理员可见全局） |
| `/health` `/ready` `/ssehealth` `/metrics` `/install.sh` | 公开 | 探针、Prometheus 指标、Trace 客户端安装脚本 |

## 部署

- **单机 Compose**：`docker compose -f docker/docker-compose-single.yml up -d`（需 `env/api.env`、`IMAGE_TAG` 和外部网络 `1panel-network`，迁移容器成功后 API 才启动，宿主机 `7070` → 容器 `8080`）；开发环境用 `docker-compose-dev-single.yml`（`7060` 端口）。
- **Kubernetes**：Deployment + Service + ConfigMap/Secret，滚动更新；`preStop` + draining + `terminationGracePeriodSeconds` 支持 SSE 长连接无损下线。
- **CI**：GitHub Actions 构建 GHCR 镜像并发布 `aris` 客户端 Release（tar.gz + sha256），服务端 `/install.sh` 提供自包含安装脚本。

![部署与发布拓扑](docs/diagrams/archify/aris-arch-07.gif)

## 项目结构

```text
cmd/server        服务端入口（start / database migrate / lint）
cmd/client        aris Trace 客户端入口
internal/
  router/         路由注册（Huma Operation + 中间件绑定）
  handler/        HTTP 适配层
  application/    用例编排（identity/session/llmproxy/trace/...）
  domain/         领域模型与规则
  infrastructure/ GORM Repository、Redis、JWT、LLM Transport、协程池
  cron/           定时任务（终态清理、软删数据清理、Think 提取、触发词命中同步）
  bootstrap/      Fx 依赖注入与生命周期
  web/dist        前端构建产物（embed.FS）
web/              Next.js 管理后台源码
  src/app/        App Router 路由组（(dashboard) 14 页 · login / callback / share）
  src/components/ 基础与业务组件（ui / charts / chat / session-detail / trace-detail）
  src/lib/        api-client · auth-context · i18n · theme · types（后端 DTO 镜像）
docker/           Dockerfile 与 Compose 配置
env/              环境变量模板
docs/             设计文档与架构图
test/             单元测试与 E2E 测试
```

## 文档

- 架构图集：[docs/diagrams/](docs/diagrams/)
- 设计与排障文档：[docs/](docs/)
- 开发规范（面向贡献者与 AI Agent）：[AGENTS.md](AGENTS.md) 与 [docs/agents/](docs/agents/)

## License

[Apache License 2.0](LICENSE)
