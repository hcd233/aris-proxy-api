# 接口性能基准

> 本文档持续更新：所有接口的性能测试结果统一记录在此。
> 测试原始输出（wrk stdout）不入库，关键数据已摘录到下方表格。

## 1. 测试环境

### 1.1 被测服务

| 项 | 值 |
|----|-----|
| 环境 | 生产 `api.lvlvko.top`（真实链路：鉴权 → 限流 → 触发词 → 模型解析 → 审计 → 落库） |
| 部署 | k3s 单节点，2 replicas |
| CPU | requests 50m / limits **1000m** |
| 内存 | requests 128Mi / limits 512Mi |
| 服务版本 | git `8f5944a5`（2026-10-21 全部数据；镜像构建 2026-10-21 16:30 CST） |
| 测试时间 | 2026-10-21 23:24 ~ 2026-10-22 00:10（低峰，压测前宿主 load 0.11） |

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
| 完成请求数 / req/s | wrk | 长流场景下完成速率低属正常（单请求平均占用约 19s） |
| 延迟分位 | wrk `--latency` | 含整个 SSE 流时长 |
| TTFT / 流持续 | 审计表 `model_call_audits` | `first_token_latency_ms` / `stream_duration_ms` |
| 状态码 | 审计表 | `upstream_status_code` |
| 资源占用 | `kubectl top pods` / 宿主 `top`/`free` | — |

> `--timeout 300s` 必须显式指定：wrk 默认 2s，会把平均 19s 的长流全判为 timeout。

> **两类负载口径不同**：§3.1~3.3 是 SSE 长流（秒级，核心口径是并发连接数）；§3.6 Web 后台是毫秒级快接口（核心口径是服务端处理能力），鉴权与压测方法差异见 §3.6。

> **压测标准（2026-10-21 复测沿用）**：pod CPU limits **1000m** × 2 pod（§1.1）；SSE 长流（§3.1~3.3）用 wrk 4.x（宿主内原生执行）；快接口（§3.5~3.7）用 hey（本机直连，keep-alive），并以 `ab` 交叉验证（`trace/list` c=50：hey 267 / ab 286 req/s）。压测流量统一带 `User-Agent: aris-loadtest/1.0` 标记，便于日志侧区分。**2026-10-18 之前的存量数据为 300m 口径，已从本文档移除（见 §4.3）。**

> **2026-10-21 复测口径**：20 个专用 Key（`loadtest-20261021-01..20`）轮换；临时启用 3 个 mock 模型（mock-chat / mock-response / mock-messages）；压测结束后 mock 模型已恢复 `enabled=false`、`MOCK_UPSTREAM_ENABLED` 已关闭、Key 与压测数据（0 残留）已清理。

## 3. 接口测试结果

### 3.1 OpenAI Chat Completions

`POST /api/openai/v1/chat/completions`，模型 `mock-chat`。

> 说明：10~30 并发的单 Key 基准档原为 300m 口径，已移除（单 Key 限流 100 次/分钟把有效并发压在 ~30，下表容量已覆盖完整曲线）。

**容量（20 Key 轮换，2026-10-21）**

| 并发 | 完成请求 | 成功 | req/s | p50 | p90 | p99 | 非 2xx | pod CPU（2 pod 合计） | pod 内存 | 宿主 load |
|------|---------|------|-------|-----|-----|-----|--------|---------------------|---------|----------|
| 50 | 231 | 231 | 1.92 | 17.41s | 43.59s | 1.27m | 0 | 352m | 114Mi | 4.47 |
| 100 | 408 | 408 | 3.40 | 20.25s | 47.59s | 1.32m | 0 | 455m | 127Mi | 7.64 |
| 200 | 476 | 476 | **3.96** | 32.34s | 1.07m | 1.62m | 0 | 547m | 116Mi | 12.07 |
| 400 | 814 | 478 | 6.78⚠ | 19.01s⚠ | 1.48m | 1.90m | **336** | 541m | 116Mi | 14.54 |

> ⚠️ c=400 的 req/s 与 p50 均失真：336 条（41%）是秒级快速失败，把中位数拉低；按成功请求算实际吞吐 **3.98 req/s**，与 c=200（3.96）持平——**c=200 已是稳定拐点，c=400 只增加失败与尾延迟，不增加有效吞吐**。

**审计侧时序（`model_call_audits`，按档位时间窗聚合）**

| 并发 | 审计条数 | TTFT p50 | TTFT p95 | 流时长 p50 | 流时长 p95 |
|------|---------|----------|----------|-----------|-----------|
| 50 | 279 | 509ms | 952ms | 15.9s | 56.4s |
| 100 | 508 | 462ms | 937ms | 19.5s | 63.8s |
| 200 | 676 | 508ms | 958ms | 30.6s | 82.4s |
| 400 | 663 | 559ms | 984ms | 45.7s | 113.3s |

> TTFT 基本不随并发劣化（≈TTFT 分布本身 U(0,1s)），**流时长 p50 从 15.9s 涨到 45.7s**——排队发生在流式转发阶段而非首包。审计条数 ≈ wrk 完成数 + 结束瞬间的在途数（c=400 因失败请求不产生审计而低于完成数）；`upstream_status_code=0` 的记录为客户端中断（wrk 退出），已完成的上游调用全部 200。

**c=400 失败归因（实测）**：15:35:23 起 PostgreSQL 在 1 分钟内报 **604 次 `FATAL: sorry, too many clients already`**。应用侧 177 条非 200（`401`×134、`404`×29、`500`×14，按应用日志逐条统计），其余非 2xx 未落到应用日志（网关/连接层快速失败）。根因是大并发下每个请求都要查 API Key + 解析模型：`PostgresMaxOpenConns=100` × 2 pod > PG `max_connections=100`（还被同机其他容器占用），连接池互相挤兑后鉴权/模型解析查询失败。

**与 2026-10-17 对比**：拐点吞吐 2.47 → **3.96 req/s**（+60%），同档 CPU 786m → 547m；800 并发档未复测（10-17 该档 p50 仅 4.44s、connect 错误 902，已判定为快速失败噪声）。

### 3.2 OpenAI Responses

`POST /api/openai/v1/responses`，模型 `mock-response`。与 §3.1 共享同一转发链路与服务端资源；2026-10-21 补测 c=100 单档。

| 并发 | 完成请求 | req/s | p50 | p90 | p99 | 非 2xx | pod CPU | pod 内存 | 宿主 load |
|------|---------|-------|-----|-----|-----|--------|---------|---------|----------|
| 100 | 365 | 3.04 | 20.55s | 0.95m | 1.73m | 0 | 496m | 81Mi | 9.90 |

> 审计侧：审计 465 条，TTFT p50 507ms / p95 996ms，流时长 p50 18.9s / p95 71.2s。吞吐约为 chat 同档（3.40）的 89%，主因是该批随机流偏长（p90 0.95m）。

### 3.3 Anthropic Messages

`POST /api/anthropic/v1/messages`，模型 `mock-messages`。与 §3.1 共享同一转发链路与服务端资源；2026-10-21 补测 c=100 单档。

| 并发 | 完成请求 | req/s | p50 | p90 | p99 | 非 2xx | pod CPU | pod 内存 | 宿主 load |
|------|---------|-------|-----|-----|-----|--------|---------|---------|----------|
| 100 | 396 | 3.30 | 20.95s | 46.90s | 1.45m | 0 | 549m | 111Mi | 9.73 |

> 审计侧：审计 496 条，TTFT p50 529ms / p95 980ms，流时长 p50 19.8s / p95 64.6s。与 chat 同档（3.40）基本持平，三协议转发链路容量一致。

### 3.4 OpenAI Decision

`POST /api/openai/v1/decisions` — 非流式接口，**本轮跳过**。

原因：`internal/infrastructure/transport/mock_proxy.go` 的 `ForwardCreateDecision` 明确不在 mock 范围（`mock Endpoint 不开该能力`，原样透传），mock 不拦截该路径，请求会打到占位上游 `http://mock.invalid` 失败或在能力解析处被拒，**拿不到有意义的压测数据**。要测需：(a) 扩展 mock 支持 `ForwardCreateDecision`（改 Go + 测试 + 部署），(b) 指向真实上游（消耗 token），或 (c) 本接口按用户决策跳过。

### 3.5 Anthropic Count Tokens

`POST /api/anthropic/v1/messages/count_tokens`，模型 `mock-messages`。**无模块级限流**（仅 API Key 鉴权），mock 装饰器本地返回假计数（20B 响应）。2026-10-21 实测 c=25：

| 并发 | req/s | avg | p50 | p90 | p99 | 状态码 |
|------|-------|-----|-----|-----|-----|--------|
| 25 | 295.6 | 84ms | 76ms | 114ms | 320ms | 200×3571 |

> 纯 CPU 路径（不落库、不转发），单请求成本低于 §3.7 `/models`（后者每请求 2 次 DB 校验），但吞吐反而更低（296 vs 467 req/s）——瓶颈更可能在请求体 JSON 解析 / huma 绑定一侧（未做剖析）。属轻负载接口，真实用量下无瓶颈。

### 3.6 Web 后台接口（管理后台 / session 查询，2026-10-21 全量实测）

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

**总览：全部接口实测天花板（按吞吐降序）**

> 口径统一 `-z 12s -c 50`（10-18 已验证快接口在 c=50 即饱和；慢接口受 4s 级延迟限制并发利用率，表内数字为 c=50 实测值）。`ab` 交叉验证：`trace/list` 两工具分别 267 / 286 req/s。参数取前端真实调用形态：stats 用 7 天窗 + `granularity=day`、option 类带 `field`、`trace/event/list` 带真实 `id=376`（80KB 响应）、`session/*` 用 1208 消息 / 126 工具的会话 80938、`session/list` 为 pageSize=20（78KB 响应）。

| 接口 | 天花板 req/s | avg | p50 | p90 | p99 | 瓶颈 |
|------|------------|-----|-----|-----|-----|------|
| `session/share/list` | **581** | 86ms | 79ms | 125ms | 192ms | 轻查询 |
| `audit/demo/option/list` | 391 | 126ms | 72ms | 134ms | 940ms | — |
| `cron/list` | 300 | 165ms | 98ms | 243ms | 1138ms | — |
| `trace/list` | 267 | 185ms | 104ms | 242ms | 1310ms | — |
| `trigger/list` | 266 | 186ms | 103ms | 259ms | 1332ms | — |
| `audit/demo/log/list` | 217 | 227ms | 139ms | 436ms | 1539ms | — |
| `session/tool/list` | 144⚠ | 300ms | 190ms | 507ms | 2118ms | PG（tool_ids 展开） |
| `audit/cron/log/list` | 130 | 377ms | 345ms | 541ms | 1247ms | — |
| `audit/cron/option/list` | 113 | 439ms | 433ms | 635ms | 1170ms | DISTINCT |
| `stats/model/cost` | 102 | 482ms | 454ms | 732ms | 1235ms | PG |
| `stats/model/trend` | 96 | 514ms | 509ms | 684ms | 1366ms | PG |
| `stats/token/throughput` | 90 | 552ms | 521ms | 747ms | 1480ms | PG |
| `stats/model/usage` | 87 | 561ms | 534ms | 767ms | 1823ms | PG |
| `stats/token/latency` | 85 | 580ms | 553ms | 754ms | 1806ms | PG |
| `stats/token/rate` | 80 | 619ms | 566ms | 790ms | 2636ms | PG |
| `stats/request/rate` | 79 | 625ms | 596ms | 875ms | 1581ms | PG |
| `session/option/list` | 69 | 715ms | 709ms | 952ms | 1624ms | DISTINCT |
| `audit/model/log/list` | 60 | 818ms | 767ms | 1158ms | 2593ms | host/PG 争抢 |
| `trace/event/list` | 45 | 1015ms | 806ms | 2112ms | 4258ms | 80KB 响应编码 |
| `audit/model/option/list` | 44 | 1116ms | 1080ms | 1391ms | 3005ms | DISTINCT 扫全表 |
| `metrics/runtime` | 40 | 1196ms | 1180ms | 1954ms | 2491ms | CPU 配额 |
| `session/message/list` | 23 | 1945ms | 1815ms | 3686ms | 5828ms | PG（message_ids 展开 + 142KB 响应） |
| `session/list` | 12 | 3950ms | 4008ms | 4778ms | 5319ms | PG |

> ⚠️ `session/tool/list` 在连续扫测中为 144 req/s，但**单独复测为 216 req/s**（p50 197ms）——连扫时受前序接口残留负载影响，144 是偏保守的下界。

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

**四条关键结论**

1. **最慢路径仍是 session 系列 + stats 全系**：`session/list` 12 req/s（PG-bound，p50 4.0s）、`session/message/list` 23 req/s（message_ids JSON 展开 + 142KB 响应）、`stats/*` 79~102 req/s（全系 PG 聚合）；`audit/model/option/list` 44、`trace/event/list` 45。`session/tool/list` 已从 10-18 的 18 req/s 升至 144（单测 216）req/s，不再是最慢路径。
2. **瓶颈四分类**（10-18 CPU 配额对照实验结论，本轮未重做对照）：pod CPU 配额（轻查询类）、PG 展开/聚合（session 系列 + stats 全系）、宿主 I/O 争抢（audit 日志类）、大响应编码（trace/event）。**CPU-bound 类提高配额可涨 20%~468%，PG/宿主-bound 类完全无效**——优先横向扩容（加节点）而非调大 limits。
3. **PG 连接数是当前最硬的并发天花板**：`PostgresMaxOpenConns=100`/pod × 2 pod > PG `max_connections=100`（且被同机其他容器占用）。SSE 400 并发即可打爆，表现为 401/404/500 批量失败（§3.1）。修法二选一：按 PG 容量下调 per-pod 池（如 2 pod × 40），或提高 `max_connections` 并同步扩容宿主。
4. **真实用量离上限极远**：约 600 次 API 调用/天（≈0.007 req/s）vs 最慢接口 12 req/s——余量 3~4 个数量级，当前无需任何容量优化。

**与 2026-10-18 对比（同口径 c=50）**：`stats/*` 全面下滑 40%~47%（audit 表已增至 8.4 万行，7 天窗聚合变重）；`session/tool/list` 提升约 8 倍（18 → 144，单测 216）；轻查询类（share/cron/trace/trigger）基本持平（-9%~+2%），`audit/demo/log/list` 下滑 22%；`session/list` 11.6 vs 12 持平。

### 3.7 OpenAI Models（Proxy 分区快接口）

`GET /api/openai/v1/models`（2026-10-21 实测）。API Key 鉴权（`APIKeyMiddleware` 每请求查 DB 校验 Key + 所属用户），**无模块级限流**——Proxy 分区可完整压吞吐的接口。响应 434B / 5 个模型（含临时启用的 3 个 mock 模型）。

**方法**：hey，本机直连（keep-alive），`-z 12s`，每档 c=1/5/10/25/50；临时压测 Key（`loadtest-` 前缀，压完即删）。

| 并发 | req/s | avg | p50 | p90 | p99 | 状态码 |
|------|-------|-----|-----|-----|-----|--------|
| 1 | 38.7 | 26ms | 24ms | 30ms | 82ms | 200×465 |
| 5 | 179.0 | 28ms | 26ms | 34ms | 74ms | 200×2152 |
| 10 | 292.4 | 34ms | 31ms | 43ms | 96ms | 200×3516 |
| 25 | **466.7** | 53ms | 50ms | 76ms | 127ms | 200×5626 |
| 50 | 349.6⚠ | 142ms | 93ms | 166ms | 1067ms | 200×4265 |

> 饱和 ~467 req/s（c=25）；c=50 吞吐回落 25%、p99 涨至 1.07s = 过载坍塌（无 5xx，但延迟已不可用）。压力来自每请求 2 次 DB 校验（API Key + User），与 10-18 的 473 / 335 同水位，仍是 Proxy 分区最快的可压接口。

## 4. 附录

### 4.1 容量测试的 CPU 分布

> 下表为 **2026-10-17** 的 400 并发采样，本轮未重新采集容器级 CPU（仅采集 pod 级 CPU/内存，见 §3.1，压测期间两 pod 合计 352~547m、内存 81~127Mi）。

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

### 4.2 关键约束

- 限流：100 次/分钟 per API Key → 单 Key 并发上限约 30（流均 19s）
- 宿主 2 核被 openresty / postgresql / xray / 软中断瓜分，服务 pod 实际可用约 0.4~0.8 核
- **SSE 有效吞吐拐点在 c=200（3.96 req/s）**；c=400 时 pod CPU（541m）与内存（116Mi）都未饱和，限制来自 **PG 连接数**（见下一条），不是 CPU
- **PG 连接数天花板（2026-10-21 实测）**：`PostgresMaxOpenConns=100`/pod × 2 pod，而 PG `max_connections=100` 且被同机容器占用；c=400 时 1 分钟内 604 次 `too many clients`，应用批量返回 401/404/500（401 = API Key 查询失败、404 = 模型解析失败）。当前最硬的并发上限
- **Web 后台（§3.6）全部接口已实测天花板（2026-10-21）**：12~581 req/s 分布；最慢是 `session/list` 12、`session/message/list` 23、`audit/model/option/list` 44、`trace/event/list` 45，以及 stats 全系 79~102 req/s；管理组被 20/min 限流封顶（0.33 req/s），真实用量距上限 3~4 个数量级
- **Proxy 快接口 `/models`（§3.7）饱和 ~467 req/s（c=25，全 200）**，c=50 吞吐回落 25% + p99 涨至 1.07s；压力来自每请求 2 次 DB 校验（API Key + User）。`count_tokens` c=25 为 296 req/s（§3.5），同样无模块级限流
- **快接口天花板模型（2026-10-18 配额对照实验）**：CPU-bound 类吞吐 ≈ pod CPU 配额 ÷ 单请求成本；**CPU-bound 类提高配额可涨 20%~468%，PG/宿主-bound 类（stats 全系、session/list、audit 日志）完全无效**。宿主 2 核（与 8 个同机容器共享）是最终天花板——扩容优先加节点而非调大 limits；压测本身会把 load 推高（I/O 排队），接口间需留冷却时间

### 4.3 数据附注

原始压测输出体积较大，不入库；各档的完成请求数、req/s、延迟分位、Socket errors、
pod CPU/内存与宿主 load 均已摘录到 §3 与 §4.1 的表格中。

**口径变更**：2026-10-18 起压测环境统一 pod CPU limits 1000m（§1.1）；此前的存量数据为 300m 口径，已从本文档移除（§1.1 版本行标明了现存各节数据的服务版本）。

**2026-10-21 复测**：§3.1~3.7 全部数据在 git `8f5944a5` 上重测；20 Key 轮换、SSE 档间冷却 60s、Web 接口间冷却 30s；单请求延迟统一改用 hey `-c 1` keep-alive（单发 `curl` 每次新建连接含 TLS 握手，会比真实服务耗时虚高约 50ms）。

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
