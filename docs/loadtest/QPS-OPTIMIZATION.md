# 接口 QPS 优化分析（基于 [BENCHMARK.md](BENCHMARK.md)）

> 本文是分析报告，不含代码改动。所有"预期收益"都是在**明确假设**下给出的估算，落地前须用 §6 的方法实测验证。
>
> **实施状态**：第 1 批（§3.1 日志 body 策略 + §3.4 压缩级别）与第 2 批（§3.2 APIKey 鉴权缓存
> + §3.3 Guard 负缓存）已落地，见 [`internal/middleware/`](../../internal/middleware/)；
> 第 3 批中 §4.6（stats 聚合）与 §4.7（DISTINCT 选项）已落地，见
> [`internal/infrastructure/repository/read_cache.go`](../../internal/infrastructure/repository/read_cache.go)，
> §4.8（`metrics/runtime`）尚未做。其余批次未开始，收益待实测。
>
> **2026-10-22 CR 复核后的口径修订**（代码已同步；收益估算不变，但边界比初版严）：
>
> | 条目 | 修订 |
> |------|------|
> | §3.1 | **请求体不再随失败过滤**：LLM 代理的 prompt 只存在于请求体，审计表不存内容，成功请求丢请求体等于丢掉「这句话是谁问的」。现行策略是「请求体始终记录（>2KB 截断、不解析）+ 响应体只在失败时记录」，省下的 CPU 主要来自响应体一侧 |
> | §3.2 | TTL 加 ±20% 抖动 + 同密钥并发回源的 singleflight（固定 TTL 会让同批条目在同一秒集体过期，把省下的 DB 查询换成周期尖峰）；归属用户已被删除时返回 401 而非 500 |
> | §3.3 | 负缓存改 RWMutex（命中只读）；超限逐条淘汰而非整体清空（源 IP 被打满时整体清空会让所有结论周期性失效）；Redis 检查失败不再写入负缓存 |
> | §4.6 / §4.7 | `keyword` 非空时不走缓存（联想输入逐字符变化，键基数不可控，缓存反而放大 Redis 写入） |
> | §3.4 | **压缩档位收益仍待实测**：需同时记录响应体积与 QPS 两个指标，只测 QPS 无法排除「省 CPU 换来带宽上涨」 |
>
> **2026-10-22 实测验证**（git `a152bf50`，数据见 [BENCHMARK §3.6/§3.7](BENCHMARK.md)）：第 1~3 批收益已兑现——
> `stats/*` 79~102 → 809~881 req/s（8.6~10.9 倍）、DISTINCT 选项类 44~113 → 876~964（8.5~21 倍）、
> `/models` c=50 350 → 761（+117%，c=100 834）、`count_tokens` c=25 296 → 410（+38%）；轻查询类 +6%~+36%；
> SSE c=400 从 336 条失败降到 0（PG `too many clients` 只残留排空期 45 次/12s）。
> 未接缓存的 `session/list` 13、`session/message/list` 29、`audit/model/log` 58、`metrics/runtime` 49 仍是慢路径。

## 0. 结论先行

按「收益 ÷ 改动成本」排序，真正值得做的是 **2 个横切改动 + 5 个定点改动**：

| # | 改动 | 影响面 | 已实测基线 | 预期 | 成本 |
|---|------|--------|-----------|------|------|
| 1 | **请求/响应体日志改为采样或关闭** | 全部 JSON 接口 | 大响应类 17~34 req/s | 1.5~2.5x | 小（1 个中间件 + 配置） |
| 2 | **APIKey 鉴权加 Redis 缓存**（对齐 JWT） | 全部 Proxy 接口 | `/models` 473 req/s | 1.5~2x | 小（1 个中间件，复用 JWT 模式） |
| 3 | **`session/list` 去掉 `COUNT(*) OVER ()`** | 3 个列表接口 | 12 req/s | 5~20x | 小（1 处 SQL 拆分） |
| 4 | **stats 7 接口加 30~60s 结果缓存** | 7 个接口 | 149~183 req/s | 3~10x（命中时） | 小（仓储层 `GetOrLoad`） |
| 5 | **DISTINCT 选项类加缓存** | 3 个接口 | 55~114 req/s | 4~10x（命中时） | 小 |
| 6 | **`metrics/runtime` 加 2~5s 结果缓存** | 1 个接口 | 55 req/s | 3~5x | 小 |
| 7 | **大响应链路瘦身**（gzip 级别 + 去双重 JSON） | message/tool/trace event | 17~34 req/s | 1.5~2x | 中（涉及缓存载荷格式） |

**但必须先说清楚**：真实用量约 600 次调用/天（≈0.007 req/s），距最慢接口仍有 3~4 个数量级余量。除 `/models` 这类对外协议接口外，**这些优化在当前业务规模下没有收益**，其价值在于「基准本身可解释、可复现」——即把 bench 数字从"环境噪声"变成"代码属性"。

---

## 1. 口径校正：当前 benchmark 的 4 个方法学问题

在看具体接口之前，必须先修正几个会让结论偏掉的测量口径。否则优化做完也证明不了收益。

### 1.1 压测工具与被测服务抢同一份 CPU

`hey`/`wrk` 在**同一台 2 核宿主机**上跑。BENCHMARK §2 明确写了「快接口（§3.6~3.7）用 hey（本机直连）」——`hey -c 50` 在高并发下自身就要吃掉 0.3~1 核，与被测 pod、postgres、openresty 抢同一份 2 核。

后果：
- 所有"天花板"都是**低估**，且低估幅度随并发线性变化（c 越高，工具吃得越多）；
- c=50 之后"吞吐不再增长"有一部分是**工具自己先饱和**，不一定是服务端饱和；
- BENCHMARK §3.1 复测里 800 档的 `connect 错误 902`，很可能有开压端 `ulimit -n`/本地端口耗尽的贡献。

**改法**：压测机搬到另一台机器（哪怕是同一局域网的另一台 2 核）；或用 cgroup/`taskset` 把 hey 限制在 1 个核、服务留 1 个核。否则跨版本对比没有意义。

### 1.2 `c=50 即饱和` 的结论对 DB-bound 接口不成立

连接池是 `MaxOpenConns=100`/pod（`internal/common/constant/database.go:9`），2 pod 共 200 条到 PG。c=50 远没到池上限，「c=50 饱和」对 `stats/*`、`session/list` 这类 PG-bound 接口说明的是 **PG 本身处理不过来**，而不是连接数打满。两者的优化方向完全不同（前者该加索引/改 SQL，后者该调池）。

### 1.3 压测数据自己进了被查的表

容量测试会写 `model_call_audits`、`sessions`、`messages`。§3.6 压 `audit/*`、`stats/*` 时，表里的行数是**上一轮压测刚写进去的**。同一轮里先跑 BENCHMARK §3.1 再做 §3.6，后者的 PG 成本天然比干净数据高。

### 1.4 没有"单请求成本"口径

现在只有 req/s，没有「每请求消耗多少 CPU 时间」。而 req/s 的天花板 = `可用 CPU ÷ 单请求 CPU 成本`（CPU-bound 时）。缺少这个口径，就无法判断"优化后涨了 30%"是因为代码变快，还是因为那台机器当时恰好空闲。

**建议新增指标**：`Δ(container_cpu_usage_seconds_total) ÷ 完成请求数 = ms CPU/req`。这个数字对宿主机负载不敏感，是唯一可跨轮次比较的口径。本文后续所有预估都基于它。

---

## 2. 成本模型：一个请求到底花在哪

以 Web 后台（JWT 鉴权）为例，一次 `GET /api/web/v1/...` 的固定开销链路：

```
Fiber
 ├─ Metrics（Prometheus 计数器）
 ├─ Inflight（内存计数）                          ~0
 ├─ Guard      → Redis EXISTS（每请求 1 次 RTT）   ← internal/middleware/guard.go:107
 ├─ CORS / Compress（gzip level 6）
 ├─ Trace
 ├─ Locale
 └─ Log        → 解析 request body + response body，序列化后写文件 + 上报 CLS
                                                              ← internal/middleware/log.go:174-177
Huma 路由
 ├─ JWT        → Redis GET（用户缓存）             ← internal/middleware/jwt.go:98
 └─ Handler → Usecase → Repository
      └─ DB 查询 N 次（每次 GORM + pgx + 反序列化）
```

Proxy（API Key 鉴权）路径的差异：

```
 ├─ Guard      → Redis EXISTS
 ├─ APIKey     → DB 查 key + DB 查 user（2 次 DB，无缓存）
                                                              ← internal/middleware/apikey.go:49,57
 ├─ 限流        → Redis Lua（仅 chat/responses/messages 三条路由）
 └─ Handler → Redis 读缓存（可选）→ DB
```

**结论：所有接口都在为两件事付固定税**——(a) 每请求 2 次 Redis RTT（Guard + JWT/鉴权）；(b) 每请求把响应体解析成 `map[string]any` 再序列化一遍写日志。后者对**大响应接口是主成本**，对**小响应接口是固定税**。

---

## 3. 横切优化（影响全部接口）

### 3.1 【最高优先级】日志中间件解析并记录完整响应体

**现状**（`internal/middleware/log.go:103-117, 174-177`）：对每个非采样路径的请求，`buildResponseBodyFields` 把**整个响应体** `sonic.Unmarshal` 成 `map[string]any`，再经 `zap.Any` 序列化进日志。采样规则只覆盖 `/health`、`/ready`、`/sse/health` 三条路径（`internal/bootstrap/container.go:90-96`），业务接口**全量记录**。

对 `session/message/list`（310KB）、`trace/event/list`（80KB）、`metrics/runtime`（10~100KB）意味着每个请求多付：

| 动作 | 310KB 响应的成本量级 |
|------|---------------------|
| 解析成 `map[string]any`（分配 + GC 压力） | 3~8ms |
| zap JSON 编码该 map | 3~6ms |
| 写 lumberjack 文件（同步 fs.Write） | 0.3~1ms |
| CLS producer 再编码 + 入队（开启时） | 2~5ms |
| **合计** | **8~20ms/req** |

> 这些是量级估算，不是实测值。落地前用 §6.2 的 `ms CPU/req` 做前后对比，或直接对 310KB 响应跑一次 `sonic.Unmarshal(&map[string]any)` 的 benchmark。

在"有效 0.5 核"的宿主上，8~20ms/req 正好对应 25~60 req/s 的天花板——这与 §3.6 中大响应接口实测的 17~34 req/s 同量级。

**改法**（三档，按风险从低到高）：

1. **只记元信息**：默认不解析 body，只留 status / latency / 字节数 / content-type。排障需要时按 header 或 query 打开（如 `X-Debug-Body: 1`）。
2. **采样 + 截断**：按路径采样（如 1%），且 `body` 超过 2KB 截断。保留可观测性，去掉 99% 的成本。
3. **保留全量但异步**：把解析挪到一个有界队列的 worker 里（现有 Pond 池可直接用），请求线程只投递 `[]byte` 引用——需要拷贝，成本仍在，不推荐。

推荐 **2**。顺带收益：日志体积下降 1~2 个数量级（现在一条 `session/message/list` 日志就是 300KB，`emptyDir` 512Mi 会持续滚动），CLS 费用同步下降。

**注意**：`buildRequestBodyFields` 同样会解析 LLM 请求体（完整对话上下文）。对长流接口（BENCHMARK §3.1），这增加了每连接的 CPU 与磁盘写入，虽然相对 19s 的流时长可忽略，但 400 并发下是可观的聚合负载。

### 3.2 APIKey 鉴权加缓存（对齐已有的 JWT 缓存）

**现状**：`JwtMiddleware` 已经把 `userID → {name, permission}` 缓存在 Redis（`internal/middleware/jwt.go:98-118`），而 `APIKeyMiddleware` **每请求查两次库**（key 一次、user 一次，`internal/middleware/apikey.go:49,57`）。BENCHMARK §3.7 的结论也指向这里："压力来自每请求 2 次 DB 校验"。

**改法**：缓存 `key → {apiKeyID, name, userID, userName, permission}`，TTL 30~60s，key 值做 hash 后作为缓存键（不要把明文 Key 写进 Redis key，避免 `SCAN` 泄露）。吊销/改权限走写路径主动 `DEL`（`apikey` 仓储已有删除方法，挂一个 hook 即可）。

**预期**：`/models` 473 → 700~1000 req/s。按 2 pod 分摊，473 req/s ≈ 单 pod 4ms/req，其中 2 次 DB 往返在饱和宿主上是 0.5~2ms；更重要的是这两次查询直接压在已经是瓶颈的 PG 上。

**风险**：吊销延迟 ≤ TTL。对"删 Key 后立即拒绝"的语义要求，靠写路径 DEL 保证，fail-open 到 DB 即可。

### 3.3 Guard 中间件的每请求 Redis EXISTS

`internal/middleware/guard.go:107` 每个请求都 `EXISTS scanner:ban:{ip}`。这是 1 次 RTT + 1 次 Redis 命令，占轻查询接口固定成本的 10~20%。

**改法**：进程内做一层 1~2s 的 LRU 负缓存（"该 IP 未被封禁"），或把 ban 状态同步到进程内 bloom/set。封禁生效延迟 1~2s 可接受（本来就是风控，不是鉴权）。

**预期**：轻查询类（628 req/s 那一档）可能有 10~20% 提升；对 PG-bound 类无感。

### 3.4 压缩级别

`internal/middleware/compress.go:15` 用 `compress.LevelDefault`，在 fasthttp 下映射为 **gzip level 6 + brotli 默认档**（`middleware/compress/compress.go:65-72`）。注意：

- `hey`/`wrk` 默认只发 `Accept-Encoding: gzip` → 压测走 **gzip 6**；
- 浏览器（Web 后台的真实流量）会要 **br** → 走 **brotli**，level 4 左右，CPU 约为 gzip 6 的 2~3 倍。

BENCHMARK 的压测数字只反映 gzip 路径，**线上浏览器流量比压测更吃 CPU**——这也是"压测好看、实际卡顿"的常见来源。

**改法**：`LevelBestSpeed`（gzip 1 / brotli 最快档）或对大响应单独降级。
**风险**：带宽 +10~15%。对大 JSON 来说压缩率损失很小而 CPU 省 2~3 倍，JSON API 基本稳赚。

### 3.5 DB 连接池：200 条连接对 2 核 PG 明显过量

`MaxOpenConns=100`/pod × 2 pod = 200，而 PG 只有 2 核。对 CPU-bound 的查询，连接数超过核数的 2~4 倍只会增加上下文切换与锁竞争。

**改法**：降到 20~30/pod（2 pod 共 40~60），并确认 PG 侧 `max_connections ≥ 60`（还要给 admin 工具留余量）。
**预期**：PG-bound 接口（stats、session/list、audit 列表）吞吐 +10~30%，尾延迟更稳。
**验证**：这是典型的"反直觉"调参，必须用 §6 的方法扫 10/20/40/100 四档。

---

## 4. 按接口类别

### 4.1 Proxy SSE 长流（§3.1~3.3，1.6~2.5 req/s）

**这类"QPS"不是吞吐指标，是 `并发数 ÷ 流时长`。** 流时长（期望 19s）由上游决定，服务端改不动。所以唯一可优化的是**单机可持续并发数**。

| 约束 | 现状 | 优化 |
|------|------|------|
| 宿主 CPU | 400 并发时宿主 `id=0%` 全饱和 | 加机器（不是加 limits） |
| 连接建立 | `tcp_max_syn_backlog=256`，800 并发时 371~902 个 connect 错误 | 调到 4096；同时确认 openresty `worker_connections` 与 `ulimit -n` |
| 内存 | 400 并发 453Mi / 512Mi limit，而 `GOMEMLIMIT=440MiB` | **453Mi > 440MiB → Go 运行时在整个压测期间持续 GC**，这是 CPU 饱和的隐藏来源；把 memory limit 抬到 768Mi 或把 GOMEMLIMIT 调到 480MiB（`k8s/deployment.yaml:38,79`） |
| 写入 | audit/session/message 落库走 Pond 池（异步） | 已合理；确认 `POOL_QUEUE_SIZE=100` 在高压下不丢任务 |

**结论**：BENCHMARK §3.1 的 req/s 曲线没有优化空间（它是上游延迟的函数），只有"能撑多少并发"有空间，而那是**部署问题**（宿主 2 核被 8 个容器瓜分），不是代码问题。

### 4.2 Proxy 快接口（§3.7 `/models`，473 req/s）

已确认 `/models` 的模型别名走 Redis 读缓存（`internal/infrastructure/repository/endpoint_repository.go:344-357`，`cache:rd:alias:{userID}`），所以剩余成本 = **鉴权 2 次 DB + Guard Redis + 日志**。

- 主改动：§3.2（APIKey 缓存）→ 预期 700~1000 req/s。
- 次改动：§3.3（Guard 负缓存）→ 再 +5~10%。
- 结论：这个接口的 QPS 几乎完全由**中间件链**决定，与业务逻辑无关。

### 4.3 Web 轻查询（`session/share/list` 628、`audit/demo/option/list` 414、`cron/list` 294、`trace/list` 293、`trigger/list` 278）

这些已经接近"固定税"天花板（1.5~2ms/req）。剩余优化只有：§3.1 + §3.3 + §3.4。再往上要减 DB 往返（如把 `COUNT(*)` 缓存 30s），但收益递减。

**不建议动**。它们的单请求成本已经在个位数毫秒，真正的瓶颈是宿主 CPU。

### 4.4 Web PG-bound 列表（`session/list` 12 req/s ← 本文最大单点收益）

这是全表最慢的接口，代码里有两个叠加的问题。

**问题 A：`COUNT(*) OVER ()` 让分页变成全表扫描**

`internal/common/constant/sql.go:280`：

```sql
id, created_at, updated_at, score,
COALESCE(jsonb_array_length(message_ids::jsonb), 0) AS message_count,
COALESCE(jsonb_array_length(tool_ids::jsonb), 0) AS tool_count,
questions, model_ids, COUNT(*) OVER () AS total_count
```

窗口函数在 `LIMIT` **之前**求值。PG 必须对 `WHERE deleted_at = 0` 命中的**全部 164K 行**计算 `jsonb_array_length(...)`——这是每行一次 TOAST 解压。设计注释写的是"省掉一次独立 COUNT 的 roundtrip"，实际换来的是把一次便宜的 COUNT 变成了 164K 次 JSONB 解压。

`ListAllSessions` / `ListSessionsByOwnerIDs` / `ListSessionsByIDs` 三处都受影响（`internal/infrastructure/repository/session_repository.go:324, 400, 448`）。

**改法**：拆成两条 SQL——
1. `SELECT COUNT(*) FROM sessions WHERE <同一组条件>`（不带 jsonb 表达式，可走 index-only scan；进一步可缓存 30~60s）；
2. 原查询去掉 `COUNT(*) OVER ()` 与两个 `jsonb_array_length`，只 `LIMIT 20`；`message_ids` / `tool_ids` 原样选出，在应用层对这 20 行 `len()` 即可。

**问题 B：为拿 summary 再查 20 条完整 message**

`internal/application/session/query/jwt_session_queries.go:103-119`：列表返回后，用 `questions[0]` 再发一次 `FindMessagesByIDs`，把 20 条**完整 message JSONB**（agent 会话单条可达数十 KB）加载并反序列化成 `vo.UnifiedMessage`，只为了 `ExtractMessageText` 取开头一段。

**改法**（按成本从低到高）：
1. 在 `sessions` 表冗余一列 `summary`（VARCHAR(500)），创建会话时写入首条用户消息的截断文本；存量数据一次性回填。这是最彻底的方案，列表接口从此只查一张表。
2. 过渡方案：`SELECT id, left(message::jsonb->>'content', 500)`，只取字符串不反序列化结构体；对 content 为 parts 数组的多模态消息退化为 `questions` 的兜底文案。

**预期**：12 → 60~150 req/s（A 单独 5~20x，B 再 2~4x）。
**验证**：`EXPLAIN (ANALYZE, BUFFERS)` 对比改前改后，重点看 `Planning Time`、`Buffers`、是否有 `WindowAgg`。

### 4.5 Web 大响应（`session/message/list` 17、`session/tool/list` 18、`trace/event/list` 34）

三者结构相同：Redis 缓存命中（`internal/infrastructure/cache/session_detail.go`）→ 反序列化成结构体 → Huma 再序列化 → gzip → **日志中间件再解析一遍**。

单请求 CPU 分解（估算，310KB 响应）：

| 环节 | 成本 | 可优化性 |
|------|------|---------|
| Redis MGET 310KB | 0.5~1ms | 低 |
| sonic 反序列化到 `[]*MessageCacheRecord`（结构化） | 3~6ms | **中：改成"缓存直接存已渲染 JSON"，跳过 struct 往返** |
| huma 序列化 310KB | 1~3ms | 低 |
| gzip level 6（浏览器为 brotli） | 5~8ms | **高：best speed → 2~3ms** |
| 日志解析 + 编码 + 写盘（§3.1） | 8~20ms | **最高** |

**预期**：三件一起做，17 → 35~50 req/s（2~3x）。若只做日志采样，约 1.3~1.6x（日志只是 CPU 成本的一部分，不是全部）。
**风险**：改缓存载荷格式要 bump key 版本号（`MessageKeyTemplate`），并处理新旧格式共存——一次性切换、双写一个 TTL 窗口最省事。

**另一个方向（产品侧，收益最大但需确认）**：`session/message/list` 的 `pageSize` 上限是 500（`internal/common/constant/sql.go:209`）。前端如果默认拉 100 条，响应会是 1.5MB 级。确认前端实际取值，必要时把默认页大小压到 20、大页走单独的"导出"接口。

### 4.6 统计接口（`stats/*`，149~183 req/s，BENCHMARK 标注 PG）

7 个接口共用同一张 `model_call_audits`、同一组过滤条件（7 天窗口 + day 粒度），各自 `GROUP BY model_id, date_trunc(...)` 扫一遍。

**改法（两步）**：
1. **短期**：在 service 层加 30~60s 结果缓存，key = `(endpoint, permission, userID, startTime, endTime, granularity)`。管理看板 30s 陈旧完全可接受，命中时 ~2ms。
2. **长期**（有真实需求时）：按天预聚合到 `model_daily_stats` 表（cron 增量写入），查询只扫几十~几百行。

**验证**：先 `EXPLAIN (ANALYZE, BUFFERS)` 确认是否真的走了 `idx_mca_created_at`；如果 `api_key_id IN (...)` + 时间窗导致 planner 选错索引，需要 `(api_key_id, created_at)` 复合索引（已有 `idx_mca_apikey_created`，应能命中）。

**预期**：命中缓存时 149~183 → 500+ req/s；未命中不变。

### 4.7 DISTINCT 选项类（`session/option/list` 104、`audit/cron/option/list` 114、`audit/model/option/list` 55）

**现状**：
- `audit/model/option/list`：`SELECT DISTINCT model_id FROM model_call_audits WHERE deleted_at = 0 LIMIT 50`（`internal/infrastructure/repository/audit_repository.go:153`）——没有时间窗时是全表去重。
- `session/option/list`：`DISTINCT jsonb_array_elements_text(model_ids::jsonb)`（`sql.go:390`）——需要展开每一行的 JSONB 数组，无法走索引。
- `audit/cron/option/list`：同类 DISTINCT。

**特点**：这些数据**变化极慢**（模型就几十个、用户名就几个、状态码就十来种），是缓存的最佳对象。

**改法**：仓储层加 `cache.GetOrLoad`（项目已有统一封装，见 `internal/infrastructure/cache/read_cache.go`），TTL 60s ± 抖动，key 带 scope（admin/user/demo）+ keyword。空结果写空值标记防穿透。

**预期**：55~114 → 400~800 req/s（命中时）。**性价比最高的一组**。

### 4.8 `metrics/runtime`（55 req/s）

**现状**（`internal/application/metrics/query/runtime_metrics.go:94-108`）：对每个实例执行一次 `ZRANGEBYSCORE` 取回窗口内**全部快照原始 JSON**，逐条 `sonic.Unmarshal` 后聚合。快照按 5s 落盘（`internal/common/constant/metrics.go:101-102` 的 bucket 与 flush 间隔对齐），所以 `range=1h` 是 **720 条/实例**、`range=15m` 是 180 条/实例——每个请求都要做几百次 JSON 解析 + 1MB 级 Redis 读取。

**改法**：
1. 加 2~5s 结果缓存（key = range + since 对齐到桶），配合 singleflight。前端本来就是 5~10s 轮询，感知不到。
2. 中期：把聚合结果写回 Redis（由采集侧或首个查询者计算），后续查询直接读聚合结果。

**预期**：55 → 200~300 req/s。
**注意**：`since` 增量语义要和缓存 key 对齐，否则会出现"缓存把增量结果当成全量"的错乱。

### 4.9 审计日志列表（`audit/model/log/list` 72、`audit/cron/log/list` 136）

每次请求 4 次 DB 往返：`COUNT(*)` + 分页 SELECT + `BatchGetRelations`（key 一次、user 一次，`audit_repository.go:278-310`）。

- `COUNT(*)` 对百万级审计表是主成本 → 缓存 30s，或改用 `pg_class.reltuples` 估算（admin 列表的 total 精度要求低）。
- `BatchGetRelations` 是 per-page（20 行）的两次小查询，可复用 §3.2 的 key/user 缓存。
- SELECT 的列很多（`AuditRepoFields` 有 20 列），可考虑列表页只取展示必需列。

**预期**：72 → 120~200 req/s。

---

## 5. 不建议做的

| 想法 | 为什么不做 |
|------|-----------|
| 给 stats 建物化视图 / 日汇总表 | 真实用量 0.007 req/s，还有 3~4 个数量级余量。30~60s 结果缓存已足够，预聚合是纯成本 |
| 引入 Redis 之外的进程内全量缓存 | 多副本一致性成本远高于收益；单 key 级别的 `singleflight` 已经解决击穿 |
| 把 Huma 换成手写 Fiber 路由 | 「fiber 替换 huma」在本项目是误表述：huma 本来就跑在 fiber 上。`2026-10-10-perf-qps-benchmark-design.md` 的四组基线已拆过 huma 层开销，不是瓶颈 |
| 为 SSE 长流优化单请求 CPU | 19s 的流里省 5ms 毫无意义；瓶颈是宿主核数 |
| 扩大 pod CPU limits | 已到 1000m = 整机核数；再加只会让单个 pod 吃满整机（2026-09 的 504 就是这么来的，`k8s/deployment.yaml:75-77` 有注释） |
| 对 `session/list` 的 keyword 路径继续调优 | 已做过两轮生产 RCA（Serena memory：`perf/session-list-keyword-trigram-2026-08-02`、`perf/session-keyword-correlated-subplan-rca-2026-10-15`），冷查询 3~4s 是 TOAST recheck 的物理下限，只能靠 60s ids 缓存摊薄 |

---

## 6. 验证方法（否则数字会骗人）

### 6.1 基础设施先就位

1. **压测机与被测机分离**（最重要）。实在只有一台，用 `taskset -c 0` 把 hey 绑到 1 个核，服务用另外的核；并在报告里注明。
2. 每轮固定：`GOMAXPROCS`（当前默认 = 容器 CPU limit，2 pod 各 1）、Pod 副本数、`hey -z 12s`、接口间冷却 30s。
3. 同一轮内固定顺序：先轻后重，且每轮开始前记录 PG 表行数。

### 6.2 每项优化必须报三个数

| 指标 | 采集方式 | 作用 |
|------|---------|------|
| req/s | hey/wrk | 结果 |
| **ms CPU / req** | `Δcontainer_cpu_usage_seconds_total ÷ 完成请求数` | **判断是否真的变快** |
| p99 | hey | 判断是否过载坍塌 |

`ms CPU/req` 降但 req/s 没变 → 瓶颈不在 CPU（DB/网络），继续往下查；req/s 升且 `ms CPU/req` 降 → 优化真实有效；req/s 升但 `ms CPU/req` 反而升 → 只是吃到了更多 CPU（并发变高），不算优化。

### 6.3 SQL 类改动必须 EXPLAIN

```sql
EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)
SELECT ... FROM sessions WHERE deleted_at = 0 ORDER BY id LIMIT 20;
```

关注：
- `WindowAgg` 是否消失；
- `Rows Removed by Index Recheck`、`Heap Fetches`；
- `Buffers: shared hit/read`（读数说明在做 IO）。

### 6.4 回归红线

- `session/list` 的 `total` 在 page>1 时仍必须准确（现在空结果时 total 会退化为 0，拆分 COUNT 后这条 fallback 注释可以删掉）；
- 缓存类改动必须验证**写路径失效**：改模型/删 Key 后，选项列表与鉴权在 TTL 内是否可接受地陈旧；
- 日志改动必须验证：错误请求仍能拿到足够上下文（保留 status、error、traceID，body 可截断）。

---

## 7. 建议实施顺序

```
第 1 批（横切，1 个 PR）：§3.1 日志采样 + §3.4 压缩级别
   → 影响全部接口，风险低，是后续所有对比的基线
第 2 批（Proxy）：§3.2 APIKey 缓存 + §3.3 Guard 负缓存
   → 直接抬 /models 与所有 Proxy 快接口
第 3 批（Web 定点）：§4.7 DISTINCT 缓存 + §4.6 stats 缓存 + §4.8 runtime 缓存
   → 三处同构，可复用同一套 GetOrLoad 封装
第 4 批（Web 结构）：§4.4 session/list 拆 COUNT + summary 冗余
   → 收益最大、改动最重，单独一个 PR + 数据回填脚本
第 5 批（大响应）：§4.5 缓存载荷直出 JSON
   → 依赖第 1 批的效果评估，如果第 1 批已把 CPU 打下来，这步可以不做
```

每批完成后**重跑 §3.6/§3.7 的同参数档位**并更新 BENCHMARK.md，同时补上 `ms CPU/req` 列。
