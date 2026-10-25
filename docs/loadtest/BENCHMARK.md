# 接口性能基准

> 本文档持续更新：所有接口的性能测试结果统一记录在此。
> 测试原始输出（wrk stdout）不入库，关键数据已摘录到下方表格。
>
> **2026-10-26 注记**：agent trace 功能已移除，文中 `trace/list`、`trace/event/list` 相关数据仅作为历史压测快照保留，不再对应现行接口。

## 1. 测试环境

### 1.1 被测服务

| 项 | 值 |
|----|-----|
| 环境 | 生产 `api.lvlvko.top`（真实链路：鉴权 → 限流 → 触发词 → 模型解析 → 审计 → 落库） |
| 部署 | k3s 单节点，2 replicas |
| CPU | requests 50m / limits **1000m** |
| 内存 | requests 128Mi / limits 512Mi |
| 服务版本 | git `a152bf50`（2026-10-22 全部数据；镜像构建 2026-10-22 22:30 CST） |
| 测试时间 | 2026-10-22 22:53 ~ 2026-10-22 23:45（低峰，压测前宿主 load 0.22） |

> **本轮服务版本包含 2026-10-22 落地的 3 批 QPS 优化 + CR 修复**：`11772bb9`（日志 body 策略 + 压缩降档）、`862a3381`（APIKey 鉴权 Redis 缓存 + Guard 1s 负缓存）、`dc434f20`（stats 聚合与 DISTINCT 选项 60s 读缓存）、`a152bf50`（缓存 singleflight/抖动、keyword 不缓存、负缓存逐条淘汰）。复测方法与 10-21 相同，**新增 `ms CPU/req` 口径**（见 §2）。

### 1.2 宿主机

| 项 | 值 |
|----|-----|
| CPU | **2 核** |
| 内存 | 3913 MB（无 swap） |
| 系统 | Ubuntu 24.04 / kernel 6.8.0-31 / k3s v1.35.5 |
| 同机容器 | postgresql、redis、openresty(1Panel)、xray、pgadmin4、portainer、1Panel-agent 等 |
| 网络参数 | `somaxconn=4096`、`tcp_max_syn_backlog=256` |

### 1.3 Mock 上游（被测模型的行为）

| 参数 | 取值 |
|------|------|
| 输出长度 | `min(max_tokens, U(1, 16384))` 字符 |
| TTFT（首 token 延迟） | `U(0, 1s)` |
| 输出速率（TPS） | `U(20, 200) tok/s` |
| 流时长 | `TTFT + (N-1)/TPS`，期望约 19s，最长约 205s |
| 字符集 | 纯可打印 ASCII |

三个 mock 模型（`upstream_model=mock-llm`）：`mock-chat` / `mock-response` / `mock-messages`。

## 2. 测试方法

| 项 | 值 |
|----|-----|
| 压测工具 | wrk 4.x（宿主 `/usr/bin/wrk` 原生执行） |
| 命令 | `wrk -t4 -c<N> -d120s --timeout 300s --latency -s test/loadtest/<协议>.lua <url>` |
| 鉴权 | `Authorization: <API Key>`（非 `X-API-Key`） |
| 限流 | 100 次/分钟 per API Key；容量测试用 20 个 Key 轮换（`LT_KEYS`）绕过 |
| 请求标记 | 消息带 `__LOADTEST__` 前缀，保证 checksum 不与真实数据冲突 |
| 冷却 | SSE 各档之间 60s；Web 后台接口之间 30s（宿主 2 核共享，压测 I/O 需消化） |

**指标口径**

| 指标 | 来源 | 说明 |
|------|------|------|
| 完成请求数 / req/s | wrk / hey | 长流场景下完成速率低属正常（单请求平均占用约 19s） |
| 延迟分位 | wrk `--latency` | 含整个 SSE 流时长 |
| TTFT / 流持续 | 审计表 `model_call_audits` | `first_token_latency_ms` / `stream_duration_ms` |
| 状态码 | 审计表 | `upstream_status_code` |
| 资源占用 | `kubectl top pods` / 宿主 `top`/`free` | — |
| ms CPU/req | cAdvisor `container_cpu_usage_seconds_total{container="app"}` 前后差 ÷ 完成请求数 | 2026-10-22 新增；对宿主负载不敏感，是跨轮次最可比的口径 |

> `--timeout 300s` 必须显式指定：wrk 默认 2s，会把平均 19s 的长流全判为 timeout。

> **两类负载口径不同**：§3.1~3.3 是 SSE 长流（秒级，核心口径是并发连接数）；§3.6 Web 后台是毫秒级快接口（核心口径是服务端处理能力），鉴权与压测方法差异见 §3.6。

> **压测标准（2026-10-22 复测沿用）**：pod CPU limits **1000m** × 2 pod（§1.1）；SSE 长流（§3.1~3.3）用 wrk 4.x（宿主内原生执行）；快接口（§3.5~3.7）用 hey（本机直连，keep-alive），并以 `ab` 交叉验证（`trace/list` c=50：hey 267 / ab 286 req/s）。压测流量统一带 `User-Agent: aris-loadtest/1.0` 标记，便于日志侧区分。**2026-10-18 之前的存量数据为 300m 口径，已从本文档移除（见 §4.3）。**

> **2026-10-21 复测口径**：20 个专用 Key（`loadtest-20261021-01..20`）轮换；临时启用 3 个 mock 模型（mock-chat / mock-response / mock-messages）；压测结束后 mock 模型已恢复 `enabled=false`、`MOCK_UPSTREAM_ENABLED` 已关闭、Key 与压测数据（0 残留）已清理。

> **2026-10-22 复测口径**：20 个专用 Key（`loadtest-20261022-01..20`，id 52~71）同样直插；mock 模型 49/50/51 临时 `enabled=true`、`MOCK_UPSTREAM_ENABLED=true`；SSE 档间冷却 60s（c=400 后对 responses 复测额外冷却到 120s）、Web 接口间 30s。收尾按 `scripts/loadtest-cleanup.sql` 逐 Key 清理（`audits_left`/`sessions_left` 全 0）、软删 20 个 Key、恢复 `enabled=false` 与关闭 mock 开关并滚动重启；临时文件（Key 列表/驱动日志）已删除。

## 3. 接口测试结果

### 3.1 OpenAI Chat Completions

`POST /api/openai/v1/chat/completions`，模型 `mock-chat`。

> 说明：10~30 并发的单 Key 基准档原为 300m 口径，已移除（单 Key 限流 100 次/分钟把有效并发压在 ~30，下表容量已覆盖完整曲线）。

**容量（20 Key 轮换，2026-10-22）**

| 并发 | 完成请求 | 成功 | req/s | p50 | p90 | p99 | 非 2xx | pod CPU（均值） | pod 内存 | 宿主 load |
|------|---------|------|-------|-----|-----|-----|--------|----------------|---------|----------|
| 50 | 198 | 198 | 1.65 | 17.90s | 45.55s | 1.35m | 0 | 386m | 33/36Mi | 2.33 |
| 100 | 402 | 402 | 3.35 | 20.82s | 50.64s | 1.38m | 0 | 555m | 35/37Mi | 8.48 |
| 200 | 608 | 608 | **5.06** | 27.78s | 0.96m | 1.51m | 0 | 641m | 36/43Mi | 13.53 |
| 400 | 516 | 516 | 4.30 | 44.77s | 1.60m | 1.92m | **0** | 661m | 47/60Mi | 15.15 |

> pod CPU 为 120s 窗口的均值（cAdvisor 容器 CPU 秒差 ÷ 120s），内存是压测进行中的 `kubectl top` 快照，宿主 load 为 1 分钟 load average 中途采样。
>
> **c=400 本轮零失败**（0 非 2xx、0 socket error）：10-21 同档有 336 条（41%）秒级快速失败。c=200 仍是吞吐拐点（5.06 req/s），c=400 吞吐回落 15%、p50 上涨 61%——但这是**排队**而非失败，服务仍可用。
>
> ⚠️ **mock 流长每轮随机**（单请求 `min(max_tokens, U(1,16384))` 字符、TTFT `U(0,1s)`、TPS `U(20,200)`），档间 req/s 与分位天然含随机波动（本轮 c=50/c=100 抽到的流比 10-21 更长，req/s 因此偏低）。跨轮次比较请看同档位的相对变化与 `ms CPU/req`，不要用单次 req/s 判定回归。

**审计侧时序（`model_call_audits`，按档位时间窗聚合）**

| 并发 | 审计条数 | TTFT p50 | TTFT p95 | 流时长 p50 | 流时长 p95 |
|------|---------|----------|----------|-----------|-----------|
| 50 | 246 | 508ms | 968ms | 18.0s | 62.4s |
| 100 | 502 | 486ms | 947ms | 19.1s | 62.5s |
| 200 | 808 | 504ms | 965ms | 25.9s | 70.8s |
| 400 | 916 | 522ms | 982ms | 41.1s | 117.4s |

> TTFT 基本不随并发劣化（≈TTFT 分布本身 U(0,1s)），**流时长 p50 从 18.0s 涨到 41.1s**——排队发生在流式转发阶段而非首包。审计条数 = wrk 完成数 + 结束瞬间的在途数（c=400：516 + 400 = 916）；审计中 `upstream_status_code=0` 的记录是客户端中断（wrk 退出），已完成的上游调用全部 200。

**c=400 失败归因（本轮实测）**：c=400 稳态**零失败**，但**排空期**仍触发 PG 连接上限——c=400 结束约 97s 后（15:05:59~15:06:10 UTC，约 12s），PostgreSQL 报 **45 次 `FATAL: sorry, too many clients already`**，导致紧随其后的 responses c=100 档在 13 秒内出现 **31 条非 2xx**（`401`：APIKey 查询失败，应用日志 `[APIKeyMiddleware] API key not found`；同窗口伴随 `404 model_not_found`：模型解析查询失败）。根因仍是 `PostgresMaxOpenConns=100`/pod × 2 pod > PG `max_connections=100`（还被同机其他容器占用），只是触发点从「并发峰值」后移到「异步写排空」：c=400 的约 400 条在途流在客户端断开后集中落库，两个 pod 的连接池同时扩张到上限，挤掉随后的鉴权/模型解析查询。对比 10-21：**604 次/60s → 45 次/12s（-93%），且不再发生在 c=400 稳态**。

**与 2026-10-21 对比**：c=200 完成数 476 → 608，但 p50 32.34s → 27.78s——本轮抽到的随机流更短，吞吐提升主要来自流长而非服务端；同档 CPU 均值 641m（10-21 为 541m 点采样，口径不同不可直接比）。**真正的变化在 c=400：336 条失败 → 0，PG `too many clients` 604 → 45 次，且只发生在排空期。**

### 3.2 OpenAI Responses

`POST /api/openai/v1/responses`，模型 `mock-response`。与 §3.1 共享同一转发链路与服务端资源；2026-10-22 复测 c=100 单档。

| 并发 | 完成请求 | req/s | p50 | p90 | p99 | 非 2xx | pod CPU（均值） | pod 内存 | 宿主 load |
|------|---------|-------|-----|-----|-----|--------|----------------|---------|----------|
| 100 | 386 | 3.21 | 19.61s | 47.69s | 1.40m | 0 | 617m | 39/41Mi | 9.19 |

> 首轮（15:05:23）紧接 c=400 排空期，381 完成中 31 条 401，已作废；**上表是冷却 3 分钟后复测的干净数据**。审计侧首轮 443 条（TTFT p50 563ms / p95 968ms，流时长 p50 21.2s / p95 76.2s；98 条 code 0 为在途中断）；复测档审计行随收尾清理删除，故只有 wrk 侧数据。吞吐为 chat 同档（3.35）的 96%，三协议容量一致。

### 3.3 Anthropic Messages

`POST /api/anthropic/v1/messages`，模型 `mock-messages`。与 §3.1 共享同一转发链路与服务端资源；2026-10-22 复测 c=100 单档。

| 并发 | 完成请求 | req/s | p50 | p90 | p99 | 非 2xx | pod CPU（均值） | pod 内存 | 宿主 load |
|------|---------|-------|-----|-----|-----|--------|----------------|---------|----------|
| 100 | 387 | 3.22 | 21.49s | 44.61s | 1.51m | 0 | 583m | 34/37Mi | 9.32 |

> 审计侧（首轮，同为零失败）：审计 467 条，TTFT p50 564ms / p95 983ms，流时长 p50 19.4s / p95 77.5s。与 chat（3.35）/responses（3.21）基本持平，三协议转发链路容量一致。

### 3.4 OpenAI Decision

`POST /api/openai/v1/decisions` — 非流式接口，**本轮跳过**。

原因：`internal/infrastructure/transport/mock_proxy.go` 的 `ForwardCreateDecision` 明确不在 mock 范围（`mock Endpoint 不开该能力`，原样透传），mock 不拦截该路径，请求会打到占位上游 `http://mock.invalid` 失败或在能力解析处被拒，**拿不到有意义的压测数据**。要测需：(a) 扩展 mock 支持 `ForwardCreateDecision`（改 Go + 测试 + 部署），(b) 指向真实上游（消耗 token），或 (c) 本接口按用户决策跳过。

### 3.5 Anthropic Count Tokens

`POST /api/anthropic/v1/messages/count_tokens`，模型 `mock-messages`。**无模块级限流**（仅 API Key 鉴权），mock 装饰器本地返回假计数（20B 响应）。2026-10-22 实测：

| 并发 | req/s | avg | p50 | p90 | p99 | 状态码 |
|------|-------|-----|-----|-----|-----|--------|
| 25 | **409.7** | 61ms | 56ms | 84ms | 138ms | 200×4960 |
| 50 | 245~300 | 105~110ms | 93~105ms | 199~230ms | 1.38~1.79s | 200×3004~3674 |

> 饱和点仍在 c=25，但吞吐 296 → **410 req/s（+38%）**、p99 320ms → 138ms；c=50 两次实测 300 / 245，均低于 c=25 且 p99 涨到 1.4~1.8s = **过载坍塌**。纯 CPU 路径（不落库、不转发），但 c=50 已先于 `/models`（c=100 仍 834）坍塌——瓶颈在请求体 JSON 解析 / huma 绑定，宿主 2 核先饱和。

### 3.6 Web 后台接口（管理后台 / session 查询，2026-10-22 全量实测）

JWT 鉴权（admin）的查询接口，毫秒级快接口——与 §3.1~3.3 的 19s SSE 长流是两类负载。**并发→吞吐语义不同**：快接口的吞吐上限由**服务端处理能力**（pod CPU / PG / 宿主 I/O）决定，而非连接占用。

**方法**

| 项 | 值 |
|----|-----|
| 鉴权 | `Authorization: Bearer <JWT>`（admin `userID=1` 的 HS256 临时 access token，经 env 注入压测工具，不入 git / 不落库） |
| 连接 | hey keep-alive 复用连接，网络地板约 20~25ms（单发 `curl` 每次新连接含 TLS，约 75~90ms，不可用于单请求延迟口径） |
| 命令 | `hey -z 12s -c 50 -U "aris-loadtest/1.0" -H "Authorization: Bearer <token>" <url>` |
| 冷却 | 接口间 30s（宿主 2 核共享，压测 I/O 需消化，连压会让后测接口偏低） |
| 限流 | session/audit/metrics/trace/cron/trigger 组**无**模块级限流；管理组 20/min，见限流表 |
| 服务配置 | pod CPU limits **1000m** × 2 pod（压测标准配置），宿主物理 2 核 |
| 缓存语义 | `stats/*` 7 接口、`session/option/list`、`audit/*/option/list` 已接 60s±10s 读缓存（2026-10-22 版本）；表内为**命中态**吞吐，首请求冷、后续命中，冷单发见下方注 |
| 客户端上限 | 缓存类实测 809~964 req/s；对 `stats/model/trend` 加压 c=50/100/200 = 873/882/920（p50 53/109/209ms，吞吐在 ~900 附近封顶而延迟线性涨），同期 pod CPU 仅 0.3~0.6ms/req、宿主 `vmstat` id=0 / sy=57% —— 瓶颈在宿主 2 核的 TLS/网关，不是应用或 PG |

**总览：全部接口实测天花板（按吞吐降序）**

> 口径统一 `-z 12s -c 50`。参数取前端真实调用形态：stats 用 7 天固定窗（`2026-10-15T00:00:00Z` ~ `2026-10-22T00:00:00Z`）+ `granularity=day`、option 类带 `field`、`trace/event/list` 带真实 `id=376`（80KB 响应）、`session/*` 用 1208 消息 / 126 工具的会话 80938、`session/list` 为 pageSize=20（79KB 响应）、`session/message/list` pageSize=50（290KB 响应）。`ab` 交叉验证为 10-21 数据（`trace/list` hey 267 / ab 286 req/s），本轮未重跑。

| 接口 | 天花板 req/s | avg | p50 | p90 | p99 | ms CPU/req | vs 10-21 | 瓶颈 |
|------|------------|-----|-----|-----|-----|-----------|----------|------|
| `audit/cron/option/list` | 964 | 52ms | 48ms | 72ms | 109ms | 0.27 | **8.5x** | 缓存命中/宿主 |
| `audit/model/option/list` | 928 | 54ms | 51ms | 76ms | 110ms | 0.22 | **21x** | 缓存命中/宿主 |
| `stats/model/trend` | 881 | 57ms | 53ms | 83ms | 124ms | 0.57 | **9.2x** | 缓存命中/宿主 |
| `session/option/list` | 876 | 57ms | 54ms | 82ms | 129ms | 0.06 | **12.7x** | 缓存命中/宿主 |
| `stats/model/cost` | 873 | 57ms | 53ms | 80ms | 123ms | 0.28 | **8.6x** | 缓存命中/宿主 |
| `stats/token/rate` | 864 | 58ms | 55ms | 85ms | 127ms | 0.96 | **10.8x** | 缓存命中/宿主 |
| `stats/request/rate` | 863 | 58ms | 54ms | 86ms | 122ms | 0.38 | **10.9x** | 缓存命中/宿主 |
| `stats/token/throughput` | 826 | 60ms | 56ms | 90ms | 135ms | 0.27 | **9.2x** | 缓存命中/宿主 |
| `stats/model/usage` | 813 | 61ms | 56ms | 91ms | 144ms | 0.40 | **9.3x** | 缓存命中/宿主 |
| `stats/token/latency` | 809 | 62ms | 57ms | 93ms | 141ms | 0.12 | **9.5x** | 缓存命中/宿主 |
| `session/share/list` | 648 | 77ms | 74ms | 106ms | 149ms | 0.91 | +12% | 轻查询/宿主 |
| `audit/demo/option/list` | 428 | 116ms | 68ms | 136ms | 1069ms | 1.29 | +9% | 轻查询/宿主 |
| `trigger/list` | 333 | 147ms | 83ms | 166ms | 1151ms | 0.41 | +25% | 轻查询/宿主 |
| `trace/list` | 321 | 154ms | 86ms | 168ms | 1312ms | 0.32 | +20% | 轻查询/宿主 |
| `cron/list` | 319 | 155ms | 87ms | 166ms | 1400ms | 1.14 | +6% | 轻查询/宿主 |
| `session/tool/list` | 308 | 155ms | 119ms | 262ms | 709ms | 1.71 | +43%⚠ | PG（tool_ids 展开） |
| `audit/demo/log/list` | 295 | 168ms | 97ms | 188ms | 1404ms | 0.20 | +36% | 轻查询/宿主 |
| `audit/cron/log/list` | 130 | 378ms | 348ms | 545ms | 1248ms | 2.13 | 0% | PG 分页 |
| `trace/event/list` | 101 | 482ms | 336ms | 1049ms | 1886ms | 2.53 | **2.25x** | 80KB 响应编码 |
| `audit/model/log/list` | 58 | 842ms | 758ms | 1190ms | 2654ms | 1.18 | -3% | host/PG 争抢 |
| `metrics/runtime` | 49 | 992ms | 950ms | 1620ms | 1998ms | 13.86 | +23% | CPU 配额（无缓存） |
| `session/message/list` | 29 | 1659ms | 1399ms | 3248ms | 5102ms | 8.70 | +24% | PG + 290KB 响应 |
| `session/list` | 13 | 3697ms | 3702ms | 4538ms | 5462ms | 5.11 | +6% | PG（COUNT OVER + jsonb） |

> ⚠️ `session/tool/list` 本轮连扫 308 req/s，未单独复测；10-21 为连扫 144 / 单测 216——换算到单测口径约 +43%~+114%。
>
> **冷缓存单发**（`hey -n 1`，含 TLS ~25ms）：`stats/model/cost` 207ms（命中态 avg 57ms）、`session/option/list` 96ms（57ms）、`audit/model/option/list` 111ms（54ms）。缓存 TTL 60s±10s，看板刷新越频繁命中率越高；写路径不主动失效，最迟 TTL 后可见（`keyword` 非空时按设计不走缓存）。
>
> **接口列已按 2026-10-22 的路径简写**：`stats/*` 实际路径为 `/api/web/v1/audit/stats/*`，`session/*` 为 `/api/web/v1/session/*`。

**限流封顶组（有效上限由限流决定，与性能无关）**

| 接口 | 上限 | 单请求延迟（hey `-c 1` keep-alive） |
|------|------|-----------|
| `upstream/list`（endpoints+models 分组） | 20/min | 34ms avg / 25ms p50（6 次） |
| `model/list` | 20/min | 31ms avg / 25ms p50（6 次） |
| `apikey/list` | 20/min | 53ms avg / 39ms p50（6 次） |
| `user/list` | 20/min | 90ms avg / 25ms p50（6 次；含 1 次 420ms 冷查询） |
| `user/current` | 20/min | 49ms avg / 43ms p50（15 串行；p90 128ms） |
| `token/refresh` | 10/min | 51ms avg / 28ms p50（8 串行） |

> 管理组 per-user **20 req/min**（`LimitManageAPIKey`/`LimitManageUser`，`internal/common/constant/`），`token/refresh` 10 req/min per IP——压测直接撞 429，故只测单请求延迟；有效吞吐上限即限流值（0.33 / 0.17 req/s），性能余量充足。`endpoint/list` 已合并进 `upstream/list`（MaaS 重构，404）。

**五条关键结论**

1. **最慢路径只剩「没有接缓存」的四个接口**：`session/list` 13 req/s（PG，p50 3.7s）、`session/message/list` 29（message_ids 展开 + 290KB 响应）、`metrics/runtime` 49（CPU 配额，未做缓存）、`audit/model/log/list` 58（PG 分页）。对应 QPS-OPTIMIZATION 的第 4/5 批与 §4.8，本轮未落地，数字与 10-21 基本持平（-3%~+24%）。
2. **已接缓存的三组出现数量级提升**：stats 7 接口 79~102 → **809~881 req/s**（8.6~10.9 倍）；选项类 44~113 → **876~964 req/s**（8.5~21 倍）。命中态完全不吃 PG，单请求 CPU 0.06~0.96ms。
3. **瓶颈从 PG 转移到宿主/网关**：缓存类吞吐 ~900 即封顶、延迟随并发线性涨，而 pod CPU 只有 0.3~0.6ms/req、宿主 `vmstat` id=0 / sy=57%、openresty 37%——2 核宿主的 TLS/网关/软中断先饱和。`session/*` 与 `audit/model/log` 仍是 PG-bound，加 pod CPU 或缓存都无效，要改 SQL（拆 `COUNT(*) OVER ()`、冗余 summary、大响应直出 JSON）或加节点。
4. **PG 连接数仍是硬天花板，但触发点后移到排空期**：c=400 稳态零失败，排空期仍报 45 次 `too many clients`，并打挂紧随其后档位的 31 条鉴权（§3.1）。修法不变：按 PG 容量下调 per-pod 池（如 2 pod × 40），或提高 `max_connections` 并同步扩容宿主。
5. **真实用量离上限更远**：约 600 次 API 调用/天（≈0.007 req/s）vs 最慢接口 13 req/s——约 1900 倍（3 个数量级），当前无需任何容量优化；本轮优化的价值是让基准从「环境噪声」变成「代码属性」。

**与 2026-10-21 对比（同口径 c=50）**：轻查询类 +6%~+36%（日志中间件不再解析成功响应体 + gzip best-speed + Guard 负缓存）；`stats/*` **8.6~10.9 倍**、选项类 **8.5~21 倍**（60s 读缓存）；`trace/event/list` 45 → 101（2.25 倍，80KB 响应不再被日志解析）、`session/message/list` 23 → 29（+24%，290KB 响应）、`metrics/runtime` 40 → 49（+23%）、`session/list` 12 → 13（+6%，PG 结构问题未动）、`audit/model/log/list` 60 → 58（-3%，PG 分页且无缓存）。

### 3.7 OpenAI Models（Proxy 分区快接口）

`GET /api/openai/v1/models`（2026-10-22 实测）。API Key 鉴权已改为 **Redis 缓存优先**（`auth:apikey:<sha256>`，30s TTL + 抖动 + singleflight；命中时每请求 0 次 DB 查询），**无模块级限流**——Proxy 分区可完整压吞吐的接口。响应 434B / 5 个模型（含临时启用的 3 个 mock 模型）。

**方法**：hey，本机直连（keep-alive），`-z 12s`，每档 c=1/5/10/25/50（本轮补 c=100）；临时压测 Key（`loadtest-` 前缀，压完即删）。

| 并发 | req/s | avg | p50 | p90 | p99 | 状态码 |
|------|-------|-----|-----|-----|-----|--------|
| 1 | 48.5 | 21ms | 19ms | 23ms | 65ms | 200×582 |
| 5 | 220.5 | 23ms | 21ms | 25ms | 61ms | 200×2648 |
| 10 | 389.6 | 26ms | 24ms | 31ms | 78ms | 200×4678 |
| 25 | **688.1** | 36ms | 33ms | 49ms | 99ms | 200×8273 |
| 50 | 761.0 | 66ms | 62ms | 92ms | 126ms | 200×9169 |
| 100 | 834.2 | 105ms | 113ms | 158ms | 231ms | 200×10310 |

> **c=50：350 → 761 req/s（+117%）**，且不再坍塌（10-21 同档 p99 1.07s）；c=25：467 → 688（+47%）。饱和点从 c=25 后移到 ≥100，c=100 仍有 834 req/s、p99 231ms，与 Web 缓存类（~900 封顶）同受宿主/网关约束。涨幅来自鉴权缓存：每请求 DB 查询 2 次 → 0 次，`c=1` 单请求 p50 也从 24ms 降到 19ms。

## 4. 附录

### 4.1 容量测试的 CPU 分布

> 下表为 **2026-10-17** 的 400 并发采样（宿主 `id=0%`，完全饱和）；2026-10-22 复测只采集 pod 级 CPU/内存（§3.1），另对缓存接口做了一次容器级采样（见下）。

400 并发压测进行中采样（宿主 `id=0%`，完全饱和）：

```
%Cpu(s): 37.0 us, 33.3 sy, 0.0 id, 29.6 si
```

| 容器 | CPU% |
|------|------|
| 1Panel-openresty | 38.8% |
| postgresql | 35.9% |
| xray | 26.0% |
| aris-proxy-api pod×2 | ~40%（合计） |
| redis / 其他 | <1% |

**2026-10-22 补充采样**（`stats/model/trend` c=100、891 req/s 进行中）：`vmstat` **id=0 / sy=57%**，openresty **37.0%**、postgresql **0.2%**、redis **7.6%**；pod CPU 折算 0.3~0.6ms/req。PG 已不在缓存接口的路径上，宿主 2 核被 TLS/网关/软中断吃满——与 10-17 的「PG 35.9%」形成对照，也是 stats/选项类缓存生效的直接证据。

### 4.2 关键约束

- 限流：100 次/分钟 per API Key → 单 Key 并发上限约 30（流均 19s）
- 宿主 2 核被 openresty / postgresql / xray / 软中断瓜分，服务 pod 实际可用约 0.4~0.8 核
- **SSE 有效吞吐拐点在 c=200（5.06 req/s）**；c=400 零失败但吞吐回落 15%、p50 44.8s，限制是宿主/网关排队，不再由 PG 连接数决定（c=400 稳态未触发 `too many clients`）
- **PG 连接数天花板（2026-10-22 实测）**：`PostgresMaxOpenConns=100`/pod × 2 pod，而 PG `max_connections=100` 且被同机容器占用。触发点从 c=400 稳态后移到 **c=400 排空期**（45 次/12s，10-21 为 604 次/60s），表现为紧随其后档位的 401/404（鉴权查询与模型解析失败）。修法不变：按 PG 容量下调 per-pod 池（如 2 pod × 40），或提高 `max_connections` 并同步扩容宿主
- **Web 后台（§3.6）2026-10-22 全量实测**：13~964 req/s；已接缓存的三组（stats 7 + session/option + audit/*/option）809~964，受宿主 ~900 上限约束；未接缓存的 `session/list` 13、`session/message/list` 29、`metrics/runtime` 49、`audit/model/log/list` 58 仍是慢路径。管理组被 20/min 限流封顶（0.33 req/s），真实用量距上限 3~4 个数量级
- **Proxy 快接口 `/models`（§3.7）饱和 ~834 req/s（c=100，全 200）**，c=50 为 761 且不再坍塌（10-21 同档 350 + p99 1.07s）；压力已从「每请求 2 次 DB 校验」转移——鉴权缓存命中时 0 次 DB。`count_tokens` c=25 为 410 req/s、c=50 过载回落（245~300），同样无模块级限流
- **快接口天花板模型更新（2026-10-22）**：缓存类（stats/选项）pod CPU 仅 0.06~0.96ms/req，封顶在**宿主 2 核的 TLS/网关/软中断**（id=0、sy=57%）；PG-bound 类（session/list、audit/model/log）加 pod CPU 无效，需改 SQL 或加节点；只有 `metrics/runtime` 这类纯 CPU 路径仍可能受配额影响。压测本身会把 load 推高（I/O 排队），接口间需留冷却时间

### 4.3 数据附注

原始压测输出体积较大，不入库；各档的完成请求数、req/s、延迟分位、Socket errors、
pod CPU/内存与宿主 load 均已摘录到 §3 与 §4.1 的表格中。

**口径变更**：2026-10-18 起压测环境统一 pod CPU limits 1000m（§1.1）；此前的存量数据为 300m 口径，已从本文档移除（§1.1 版本行标明了现存各节数据的服务版本）。

**2026-10-21 复测**：§3.1~3.7 全部数据在 git `8f5944a5` 上重测；20 Key 轮换、SSE 档间冷却 60s、Web 接口间冷却 30s；单请求延迟统一改用 hey `-c 1` keep-alive（单发 `curl` 每次新建连接含 TLS 握手，会比真实服务耗时虚高约 50ms）。

**2026-10-22 复测**：§3.1~3.7 全部数据在 git `a152bf50` 上重测；20 个 Key（`loadtest-20261022-01..20`，id 52~71）直插、mock 模型 49/50/51 临时启用；SSE 档间冷却 60s（c=400 之后 responses 额外冷却到 120s）、Web 接口间 30s；新增 `ms CPU/req`（cAdvisor 容器 CPU 秒差 ÷ 完成请求数）。首轮 responses c=100 受 c=400 排空期污染（31 条 401），已在冷却后复测替换；messages c=100 首轮即干净。收尾逐 Key 执行 `scripts/loadtest-cleanup.sql`（`audits_left`/`sessions_left` 全 0）、软删 20 个 Key、恢复 mock 关闭并滚动重启。

复现方式见 §2（SSE 长流）与 §3.6（Web 后台）；脚本在 `test/loadtest/`：长流用 `chat_completions.lua`/`responses.lua`/`messages.lua`。快接口示例：

```bash
hey -z 12s -c 50 -U "aris-loadtest/1.0" \
  -H "Authorization: Bearer <token>" "https://api.lvlvko.top/api/web/v1/trace/list?page=1&pageSize=20"
```

## 5. 压测前后必做

### 5.1 压测前

1. **新建专用 API Key**，命名 `loadtest-<date>-NN`（`name` 必须以 `loadtest-` 开头，清理脚本按此前缀做守卫）。**严禁使用主 Key（id=1）或任何真实用户 Key 压测**——清理脚本会按 `api_key_id` 软删 + 硬删该 Key 的全部会话与消息，用错 Key 就是不可逆的误删。
   - 容量压测需要 **20 个 Key**（c=400 / 20 Key = 每 Key 20 并发 ≈ 63 req/min，仍在 100/min 限流内），但 API 侧配额 `APIKeyMaxCount=5`，因此 20 个 Key 由直接 `INSERT proxy_api_keys`（`user_id=1`、`sk-aris-` + 24 位随机）创建，需单独授权；Key 值只在服务器本地生成与使用，不回显、不入库。
2. 开启 mock 上游开关：`MOCK_UPSTREAM_ENABLED=true`（`api.env` + ConfigMap + 滚动重启三件套，见 `update-prod-config`），并把 3 个 mock 模型 `enabled=true`（`models.id=49/50/51`）。
3. 确认压测脚本参数：`LT_KEYS` 多 Key 轮换、`--timeout 300s` 必加（§2）。

### 5.2 压测后（缺一不可）

1. **关闭 mock 开关**：`MOCK_UPSTREAM_ENABLED=false`（同样三件套），并把 3 个 mock 模型恢复 `enabled=false`。长期开启会让误配到 `mock-llm` 的真实模型别名返回假响应且照常计费、照常落库。
2. **清理压测数据**（先清数据，再软删 Key——脚本守卫要求 Key 仍处于未软删状态）：

   ```sh
   # 先预览：-v dry_run=1 会打印完整删除量（与真实删除同源），不写任何数据
   psql "$PROD_DATABASE_URL" \
     -v api_key_id=<压测 Key ID> -v confirm=delete-loadtest-data -v dry_run=1 \
     -f scripts/loadtest-cleanup.sql
   # 确认无误后执行（去掉 dry_run）
   ```

   脚本内置守卫：Key 必须存在且未软删、`name` 以 `loadtest-` 开头、`confirm` 取值必须等于 `delete-loadtest-data`，任一不满足即非 0 退出。
3. **软删压测 Key**（`UPDATE proxy_api_keys SET deleted_at=extract(epoch from now())::bigint ...`，写操作，需授权）。
4. **复核残留为 0**（脚本 Step 7 输出的 `audits_left` / `sessions_left`）。

> 压测会话以普通 assistant 文本结尾，既不会被 `SoftDeletePurge`（只处理已软删 session）也不会被 `SessionTerminalCleanup`（只处理末条为 assistant + tool_calls 的会话）自动回收——清理必须手工执行，没有 cron 兜底。`__LOADTEST__` 前缀（保证 checksum 不与真实消息冲突）只保护「不误删真实数据」，不提供自动回收。
