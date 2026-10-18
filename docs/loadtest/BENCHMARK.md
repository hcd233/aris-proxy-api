# 接口性能基准

> 本文档持续更新：所有接口的性能测试结果统一记录在此。
> 测试原始输出（wrk stdout）不入库，关键数据已摘录到下方表格。

## 1. 测试环境

### 1.1 被测服务

| 项 | 值 |
|----|-----|
| 环境 | 生产 `api.lvlvko.top`（真实链路：鉴权 → 限流 → 触发词 → 模型解析 → 审计 → 落库） |
| 部署 | k3s 单节点，2 replicas |
| CPU | requests 50m / limits **2000m**（压测标准配置，2026-10-18 起；2 pod 合计配额 4 核，宿主物理 2 核） |
| 内存 | requests 128Mi / limits 512Mi |
| 服务版本 | git `4985e817`（§3.1~3.3 容量数据）；git `8866e7fd`（§3.6~3.7，2026-10-18） |

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
| 压测工具 | wrk 4.x（Alpine arm64 容器内原生执行） |
| 命令 | `wrk -t4 -c<N> -d120s --timeout 300s --latency -s test/loadtest/<协议>.lua <url>` |
| 鉴权 | `Authorization: <API Key>`（非 `X-API-Key`） |
| 限流 | 100 次/分钟 per API Key；容量测试用 20 Key 轮换（`LT_KEYS`）绕过 |
| 请求标记 | 消息带 `__LOADTEST__` 前缀，保证 checksum 不与真实数据冲突 |

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

> **压测标准（2026-10-18 起）**：环境统一 pod CPU limits **2000m**（§1.1）；SSE 长流（§3.1~3.3）用 wrk 4.x；快接口（§3.6~3.7）用 hey（本机直连），并以 `ab` 交叉验证（`trace/list` c=50：hey 207 / ab 210.5 req/s）。两工具同为 keep-alive 复用连接口径（c=1 串行 p50 20~25ms）。压测流量统一带 `User-Agent: aris-loadtest/1.0` 标记，便于日志侧区分。**2026-10-18 之前的存量数据为 300m 口径，已从本文档移除（见 §4.3）。**

## 3. 接口测试结果

### 3.1 OpenAI Chat Completions

`POST /api/openai/v1/chat/completions`，模型 `mock-chat`。

> 说明：10~30 并发的单 Key 基准档原为 300m 口径，已移除（单 Key 限流 100 次/分钟把有效并发压在 ~30，下表容量已覆盖完整曲线）。

**容量（多 Key，2026-10-17）**

| 并发 | 完成请求 | req/s | p50 | connect 错误 | pod CPU | pod 内存 | 宿主 load |
|------|---------|-------|-----|-------------|---------|---------|----------|
| 50 | 193 | 1.61 | — | 0 | 407m | 175Mi | — |
| 100 | 281 | 2.34 | — | 2 | 554m | 186Mi | — |
| 200 | 296 | **2.47** | — | 5 | 786m | 297Mi | — |
| 400 | 251 | 2.09 | — | 120 | 824m | 453Mi | — |
| 800 | 210 | 1.75 | — | 371 | — | — | — |

**容量复测（多 Key，停 hermes 释放 704MB 内存后，2026-10-17）**

| 并发 | 完成请求 | req/s | p50 | connect 错误 | 宿主 load |
|------|---------|-------|-----|-------------|----------|
| 100 | 290 | 2.41 | 23.87s | 0 | 8.25 |
| 200 | 212 | 1.77 | 42.56s | 2 | 10.09 |
| 400 | 258 | 2.15 | 40.51s | 60 | 11.34 |
| 800 | 658⚠ | 5.48 | 4.44s | 902 | 23.01 |

> ⚠️ 800 档 p50 仅 4.44s（远低于流期望 19s），该批"完成请求"以快速失败为主，不代表真实吞吐。

### 3.2 OpenAI Responses

`POST /api/openai/v1/responses`，模型 `mock-response`。**未做独立容量测试**（旧低并发基准档为 300m 口径，已移除）；与 §3.1 共享同一转发链路与服务端资源，容量特性可参考 §3.1，需要精确数据时按 §2 方法复测（需重建压测专用 API Key）。

### 3.3 Anthropic Messages

`POST /api/anthropic/v1/messages`，模型 `mock-messages`。**未做独立容量测试**（旧低并发基准档为 300m 口径，已移除）；与 §3.1 共享同一转发链路与服务端资源，容量特性可参考 §3.1，需要精确数据时按 §2 方法复测（需重建压测专用 API Key）。

### 3.4 OpenAI Decision

`POST /api/openai/v1/decisions` — 非流式接口，**本轮跳过**。

原因：`internal/infrastructure/transport/mock_proxy.go` 的 `ForwardCreateDecision` 明确不在 mock 范围（`mock Endpoint 不开该能力`，原样透传），mock 不拦截该路径，请求会打到占位上游 `http://mock.invalid` 失败或在能力解析处被拒，**拿不到有意义的压测数据**。要测需：(a) 扩展 mock 支持 `ForwardCreateDecision`（改 Go + 测试 + 部署），(b) 指向真实上游（消耗 token），或 (c) 本接口按用户决策跳过。

### 3.5 Anthropic Count Tokens

`POST /api/anthropic/v1/messages/count_tokens` — **本轮未压**（属 Proxy 分区，不在「Web 后台」批次内）。

备注：mock 装饰器的 `ForwardCountTokens` **是**被拦截的（本地返回假计数），技术上可测；但专用压测 API Key 已全部软删（最近两轮压测各建一个临时 Key，压完即删），重新压测需新建 Key（写操作，需授权）或使用主 Key（非专用 Key 模式）。`GET .../models` 列表已于 2026-10-18 实测（§3.7）。

### 3.6 Web 后台接口（管理后台 / session 查询，2026-10-18 全量实测）

JWT 鉴权（admin）的查询接口，毫秒级快接口——与 §3.1~3.3 的 19s SSE 长流是两类负载。**并发→吞吐语义不同**：快接口的吞吐上限由**服务端处理能力**（pod CPU / PG / 宿主 I/O）决定，而非连接占用。

**方法**

| 项 | 值 |
|----|-----|
| 鉴权 | `Authorization: Bearer <JWT>`（admin `userID=1` 的 HS256 临时 access token，经 env 注入压测工具，不入 git / 不落库） |
| 连接 | hey keep-alive 复用连接，网络地板约 20~25ms |
| 命令 | `hey -z 12s -c 50 -U "aris-loadtest/1.0" -H "Authorization: Bearer <token>" <url>` |
| 冷却 | 接口间 30s（宿主 2 核共享，压测 I/O 需消化，连压会让后测接口偏低） |
| 限流 | session/audit/metrics/trace/cron/trigger 组**无**模块级限流；管理组 20/min，见限流表 |
| 服务配置 | pod CPU limits **2000m** × 2 pod（压测标准配置），宿主物理 2 核 |

**总览：全部接口实测天花板（按吞吐降序）**

> 所有接口 c=50 即饱和（c=50→400 吞吐不再增长、仅延迟线性上升），表中即实测天花板 = min(pod CPU 配额 ÷ 单请求成本, PG/宿主瓶颈)。`ab` 交叉验证：`trace/list` 两工具分别 207 / 210.5 req/s。参数取前端真实调用形态：stats 用 7 天窗 + `granularity=day`、option 类带 `field`、`trace/event/list` 带真实 `id`、`session/*` 用 1060 消息的重会话 56540。

| 接口 | 天花板 req/s | avg | p50 | p90 | 瓶颈 |
|------|------------|-----|-----|-----|------|
| `session/share/list` | **628** | 79ms | 75ms | 106ms | 轻查询 |
| `audit/demo/option/list` | 414 | 120ms | 70ms | 130ms | — |
| `cron/list` | 294 | 168ms | 93ms | 208ms | — |
| `trace/list` | 293 | 169ms | 97ms | 206ms | — |
| `trigger/list` | 278 | 177ms | 100ms | 211ms | — |
| `audit/demo/log/list` | 278 | 177ms | 117ms | 297ms | — |
| `stats/model/cost` | 183 | 271ms | 241ms | 385ms | PG |
| `stats/model/trend` | 182 | 273ms | 255ms | 366ms | PG |
| `stats/model/usage` | 165 | 298ms | 276ms | 400ms | PG |
| `stats/token/latency` | 159 | 310ms | 283ms | 400ms | PG |
| `stats/request/rate` | 150 | 328ms | 311ms | 444ms | PG |
| `stats/token/throughput` | 149 | 330ms | 302ms | 429ms | PG |
| `stats/token/rate` | 149 | 332ms | 304ms | 441ms | PG |
| `audit/cron/log/list` | 136 | 362ms | 318ms | 481ms | — |
| `audit/cron/option/list` | 114 | 432ms | 418ms | 616ms | DISTINCT |
| `session/option/list` | 104 | 473ms | 444ms | 628ms | DISTINCT |
| `audit/model/log/list` | 72 | 679ms | 633ms | 913ms | host/PG 争抢 |
| `metrics/runtime` | 55 | 897ms | 874ms | 1339ms | CPU 配额 |
| `audit/model/option/list` | 55 | 898ms | 855ms | 1284ms | DISTINCT 扫全表 |
| `trace/event/list` | 34 | 940ms | 677ms | 1850ms | 80KB 响应编码 |
| `session/tool/list` | 18 | 1259ms | 648ms | 2954ms | PG（tool_ids 展开） |
| `session/message/list` | 17 | 2620ms | 2099ms | 4581ms | PG（message_ids 展开 + 310KB 响应） |
| `session/list` | 12 | 4023ms | 3916ms | 4874ms | PG |

**限流封顶组（有效上限由限流决定，与性能无关）**

| 接口 | 上限 | 单请求延迟 |
|------|------|-----------|
| `upstream/list`（endpoints+models 分组） | 20/min | 34ms（avg，3 次） |
| `model/list` | 20/min | 75ms |
| `apikey/list` | 20/min | 41ms |
| `user/list` | 20/min | 41ms |
| `user/current` | 20/min | 26ms（15 串行 avg；p50 22ms） |
| `token/refresh` | 10/min | 30ms（8 串行 avg；p50 24ms） |

> 管理组 per-user **20 req/min**（`LimitManageAPIKey`/`LimitManageUser`，`internal/common/constant/`），`token/refresh` 10 req/min per IP——压测直接撞 429，故只测单请求延迟；有效吞吐上限即限流值（0.33 / 0.17 req/s），性能余量充足。`endpoint/list` 已合并进 `upstream/list`（MaaS 重构，404）。

**三条关键结论**

1. **最慢路径是 session 系列**：`session/message/list` 17 req/s（message_ids JSON 展开 + 310KB 响应）、`session/list` 12 req/s（PG-bound）、`session/tool/list` 18 req/s；`trace/event/list`（80KB 响应）34 req/s。要优化这是第一优先级。
2. **瓶颈四分类**（同日 CPU 配额对照实验证实）：pod CPU 配额（轻查询类）、PG 展开/聚合（session 系列 + stats 全系）、宿主 I/O 争抢（audit 日志类）、大响应编码（trace/event）。**CPU-bound 类提高配额可涨 20%~468%，PG/宿主-bound 类完全无效**——优先横向扩容（加节点）而非调大 limits。
3. **真实用量离上限极远**：约 600 次 API 调用/天（≈0.007 req/s）vs 最慢接口 12 req/s——余量 3~4 个数量级，当前无需任何容量优化。

### 3.7 OpenAI Models（Proxy 分区快接口）

`GET /api/openai/v1/models`（2026-10-18 实测）。API Key 鉴权（`APIKeyMiddleware` 每请求查 DB 校验 Key + 所属用户），**无模块级限流**——Proxy 分区首个可完整压吞吐的接口。

**方法**：hey，本机直连（keep-alive），`-z 12s`，每档 c=1/5/10/25/50；临时压测 Key（`loadtest-` 前缀，压完即删）。

| 并发 | req/s | avg | p50 | p90 | p99 | 状态码 |
|------|-------|-----|-----|-----|-----|--------|
| 1 | 45.0 | 22ms | 21ms | 25ms | 73ms | 200×540 |
| 5 | 197.9 | 25ms | 24ms | 29ms | 79ms | 200×2378 |
| 10 | 314.6 | 32ms | 30ms | 39ms | 94ms | 200×3781 |
| 25 | **473.3** | 53ms | 49ms | 72ms | 110ms | 200×5696 |
| 50 | 335.1⚠ | 147ms | 86ms | 157ms | 1410ms | 200×4104 |

> 饱和 ~473 req/s（c=25）；c=50 吞吐回落 29%、p99 涨至 1.4s = 过载坍塌（无 5xx，但延迟已不可用）。压力来自每请求 2 次 DB 校验（API Key + User），在 2000m 配额下与 §3.6 最快的轻查询同水位。

## 4. 附录

### 4.1 容量测试的 CPU 分布

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
- 服务 pod CPU/内存均未饱和（400 并发时 41% / 88%），限制来自宿主
- **Web 后台（§3.6）全部接口已实测天花板（2026-10-18）**：12~628 req/s 分布；最慢是 session 系列（`session/list` 12 / `session/message/list` 17 / `session/tool/list` 18 req/s，PG 展开/聚合 bound）与 `trace/event/list`（34，80KB 响应）；管理组被 20/min 限流封顶（0.33 req/s），真实用量距上限 3~4 个数量级
- **Proxy 快接口 `/models`（§3.7）饱和 ~473 req/s（c=25，全 200）**，c=50 吞吐回落 29% + p99 涨至 1.4s；压力来自每请求 2 次 DB 校验（API Key + User）
- **快接口天花板模型（2026-10-18 实测）**：CPU-bound 类吞吐 ≈ pod CPU 配额 ÷ 单请求成本（2000m 配置下 §3.6~3.7 实测 12~628 req/s）；经配额对照实验验证：**CPU-bound 类提高配额可涨 20%~468%，PG/宿主-bound 类（stats 全系、session/list、audit 日志）完全无效**。宿主 2 核（与 8 个同机容器共享）是最终天花板——扩容优先加节点而非调大 limits；压测本身会把 load 推高（I/O 排队），接口间需留冷却时间

### 4.3 数据附注

原始压测输出体积较大，不入库；各档的完成请求数、req/s、延迟分位、Socket errors、
pod CPU/内存与宿主 load 均已摘录到 §3 与 §4.1 的表格中。

**口径变更**：2026-10-18 起压测环境统一 pod CPU limits 2000m（§1.1）；此前的存量数据为 300m 口径，已从本文档移除（§1.1 版本行标明了现存各节数据的服务版本）。

复现方式见 §2（SSE 长流）与 §3.6（Web 后台）；脚本在 `test/loadtest/`：长流用 `chat_completions.lua`/`responses.lua`/`messages.lua`。快接口示例：

```bash
hey -z 12s -c 50 -U "aris-loadtest/1.0" \
  -H "Authorization: Bearer <token>" "https://api.lvlvko.top/api/web/v1/trace/list?page=1&pageSize=20"
```

## 5. 压测前后必做

### 5.1 压测前

1. **新建专用 API Key**，命名 `loadtest-<date>`（`name` 必须以 `loadtest-` 开头，清理脚本按此前缀做守卫）。**严禁使用主 Key（id=1）或任何真实用户 Key 压测**——清理脚本会按 `api_key_id` 软删 + 硬删该 Key 的全部会话与消息，用错 Key 就是不可逆的误删。
2. 开启 mock 上游开关：`MOCK_UPSTREAM_ENABLED=true`（`api.env` + ConfigMap + 滚动重启三件套，见 `update-prod-config`）。
3. 确认压测脚本参数：`LT_KEYS` 多 Key 轮换、`--timeout 300s` 必加（§2）。

### 5.2 压测后（缺一不可）

1. **关闭 mock 开关**：`MOCK_UPSTREAM_ENABLED=false`（同样三件套）。这是本设计唯一的生产配置改动，长期开启会让误配到 `mock-llm` 的真实模型别名返回假响应且照常计费、照常落库。
2. **软删压测 Key**（写操作，需授权）。
3. **清理压测数据**：

   ```sh
   # 先预览：-v dry_run=1 会打印完整删除量（与真实删除同源），不写任何数据
   psql "$PROD_DATABASE_URL" \
     -v api_key_id=<压测 Key ID> -v confirm=delete-loadtest-data -v dry_run=1 \
     -f scripts/loadtest-cleanup.sql
   # 确认无误后执行（去掉 dry_run）
   ```

   脚本内置守卫：Key 必须存在且未软删、`name` 以 `loadtest-` 开头、必须带 `confirm`，任一不满足即非 0 退出。
4. **复核残留为 0**（脚本 Step 7 输出的 `audits_left` / `sessions_left`）。

> 压测会话以普通 assistant 文本结尾，既不会被 `SoftDeletePurge`（只处理已软删 session）也不会被 `SessionTerminalCleanup`（只处理末条为 assistant + tool_calls 的会话）自动回收——清理必须手工执行，没有 cron 兜底。`__LOADTEST__` 前缀（保证 checksum 不与真实消息冲突）只保护「不误删真实数据」，不提供自动回收。
