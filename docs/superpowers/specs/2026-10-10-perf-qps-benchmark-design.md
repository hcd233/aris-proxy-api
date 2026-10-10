# 性能优化 QPS 基准测试设计（7 项优化量化）

> 日期：2026-10-10 ｜ 状态：待评审 ｜ 分支：`docs/perf-qps-benchmark-2026-10-10`

## 1. 背景与目标

项目已落地多项高性能/高并发优化，但缺少量化的性能证据。本任务对以下 7 项优化逐一做基准测试，**每项产出一个独立 md 文档**，记录优化的技术细节与最终 QPS 提升效果：

| # | 优化项 | 核心问题 |
|---|--------|---------|
| 1 | sonic 替换 encoding/json | JSON 编解码库选型对吞吐的影响 |
| 2 | Fiber 替换纯 huma（stdlib 后端） | HTTP 运行时选型对吞吐的影响 |
| 3 | compress 中间件 | 压缩的 CPU 成本 vs 带宽收益 |
| 4 | 落库异步化（Pond 协程池） | 同步写 DB vs 异步提交对请求延迟/QPS 的影响 |
| 5 | 缓存（Redis / 进程内） | 缓存命中 vs 直查 DB 的吞吐差 |
| 6 | 连接池调优 | MaxOpenConns/MaxIdleConns 与吞吐/延迟的关系 |
| 7 | 复合索引 | 索引可用 vs 禁用索引扫描的查询延迟与 QPS |

**成功标准**：7 个文档全部产出，每份包含 ① 优化技术细节（引用代码位置）② 测试方法与对比组 ③ 原始数据 ④ QPS/延迟提升结论（含适用边界）。数字可复现：附复现命令与环境信息。

## 2. 术语澄清（第 2 项）

"huma 与 Fiber 的关系"在需求表述中有歧义，此处钉死测试语义：

- 项目现状：**huma v2.38（路由/DTO 校验层）跑在 humafiber adapter 上，底层是 fiber v3.3（fasthttp）**，二者是叠加关系。
- 本项测试语义（用户确认）：**同一套路由 + DTO 校验逻辑，只替换底层 HTTP 运行时**，对比「huma + stdlib `net/http`」（"纯 huma"，huma 默认后端）vs「huma + Fiber adapter」（项目现状）。
- 为把收益拆干净，压 4 组同口径端点：
  - ① 裸 `net/http` handler（stdlib 极限基线）
  - ② huma + stdlib adapter（huma 层开销 = ②−①）
  - ③ 裸 Fiber handler（fiber 极限基线）
  - ④ huma + Fiber adapter（**fiber 收益 = ④−②**，fiber 上 huma 开销 = ④−③）

## 3. 测试环境拓扑

### 3.1 执行拓扑（推荐：生产机独立被测实例）

**决定性约束**：DB 密集型测试（4/5/6/7）的连接延迟必须真实。若从本机经 SSH 隧道连生产 DB，跨机 RTT（数十 ms）会完全淹没连接池/缓存的差异，测量失真。因此：

```
┌─ api.lvlvko.top（生产服务器）──────────────────────────┐
│                                                        │
│  [线上服务 pod] ← 不压测，不受影响（除共享机器资源）        │
│  [被测实例]     127.0.0.1:18080（独立进程/二进制）         │
│  [perf harness] 127.0.0.1:18081（测试桩，见 §3.2）        │
│  [mock 上游]    127.0.0.1:18090（假 LLM API）            │
│  [wrk] ──────── 同机压测 127.0.0.1:18080/18081          │
│                                                        │
│  [Postgres]  Docker（生产库，真实数据分布）                │
│  [Redis]     Docker（生产缓存实例）                       │
└────────────────────────────────────────────────────────┘
```

- 被测实例与被压端点均为**独立进程、独立端口**，wrk 只打 18080/18081，**绝不压线上服务端口**。
- DB/Redis 经服务器本机 Docker 网络访问，无跨机 RTT 噪声。
- 与线上服务共享 CPU/IO → 执行约束见 §6。

**备用拓扑 B**（若不便在生产机跑压测）：本地起被测实例 + SSH 隧道连生产 DB/Redis。仅用于 1/2/3（不依赖 DB 延迟的项）；4/5/6 的结论必须标注"含隧道 RTT，仅相对值有效"。

### 3.2 perf harness（新增测试代码）

独立 Go 程序，放 `test/perf/`，不进入生产构建。职责：

- **mock 上游**：模拟 LLM API，返回固定 JSON（可配 2KB/20KB/100KB 三档）与固定 SSE 流，杜绝压测打真实 LLM API（成本/限流/慢）。
- **对比端点**：承载 §5 中 1/2/4/7 的对比逻辑（JSON 库切换、huma/fiber 四组、同步/异步落库、并发查询压测）。
- 统一入口 `go run ./test/perf/cmd/...`，配置走 flag。

### 3.3 测试数据与写入隔离

- **测试 API Key**：在生产库创建专用 Key（名称 `perf-bench-2026-10`），所有写入类压测经它落库，测试后按 `api_key_id` 清理。
- **缓存 key**：第 5 项使用真实生产 session（只读），缓存写入为幂等覆盖 + 有限 TTL，不清理（无害）。
- **索引测试**（7）：纯只读，`EXPLAIN ANALYZE` + `SET LOCAL enable_indexscan=off`，不建不删任何生产索引。

## 4. 通用方法学

- **指标**：QPS、延迟分位（p50/p90/p99，`wrk --latency`）、错误数、响应字节数（compress 项）。
- **轮次**：warmup 10s → 正式 30s × 5 轮，取**中位数**；报告同时给出 5 轮极差（噪声幅度）。
- **并发档**：50 / 100 / 200 三档（harness 微端点可加 500），主结论取 100 档。
- **单变量原则**：每组对比只允许一个差异（JSON 库 / HTTP 运行时 / 压缩开关 / 同步异步 / 缓存冷热 / 连接池参数 / 索引开关），其余配置、编译、数据完全一致。
- **噪声控制**：被测实例 `GOMAXPROCS` 固定为 8，wrk `-t 4`；压测期间服务器无其他批量任务（避开 cron 高峰）；每轮之间静默 5s。
- **可复现性**：每份文档记录机器 CPU/内存、go 版本、git commit、wrk 参数、原始 wrk 输出（存 `docs/perf/raw/`）。
- **QPS 折算口径**（第 7 项 DB 查询）：并发压测同一 SQL 的实测 QPS（非 1/延迟 折算），并发数 = 连接池上限，另附单查询 `EXPLAIN ANALYZE` 延迟。

## 5. 逐项设计

### 5.1 sonic 替换 encoding/json

**技术细节**（文档需引用）：`bytedance/sonic v1.15.0`，`fiber.Config.JSONEncoder/JSONDecoder`（`internal/api/fiber.go`）、业务层 `sonic.Marshal/Unmarshal` 散点、`sonic.NoCopyRawMessage` 零拷贝（`internal/client/trace/*`）。

**测试载体**：harness（不改主项目代码）。
- **微基准**（辅助证据）：`go test -bench` 对真实 payload（chat completion 请求/响应、SSE chunk、`MessageStoreTask`）分别 Marshal/Unmarshal，报 ns/op、B/op、allocs/op。
- **端到端**（主证据）：harness 提供 `POST /bench/json`，body 为 10KB 典型 LLM 请求 JSON，handler Unmarshal→改一个字段→Marshal 返回。两组：sonic 实现 vs encoding/json 实现（其余代码逐字节相同）。

**对比组**：`encoding/json`（基线）vs `sonic`（优化后）。
**产出**：`docs/perf/01-sonic-vs-encoding-json.md`。

### 5.2 Fiber 替换纯 huma（HTTP 运行时）

**技术细节**：`huma v2.38 + humafiber adapter`（`internal/api/huma.go`）、`fiber v3.3`（`internal/api/fiber.go`）、humafiber 的 `Context` 包装与 `huma.Context` 转发开销。

**测试载体**：harness 四组端点（§2），路由与 DTO 完全一致：
- DTO：path 参数 + query 参数 + 20 字段 body（含 `required`/`enum`/`format` 校验 tag），handler 返回固定小 JSON。
- ①②③④ 各自注册同一 path/同一 DTO/同一 handler 逻辑，仅 adapter 不同。

**对比组**：①裸 net/http ②huma+stdlib ③裸 fiber ④huma+fiber；**核心结论 = ④ vs ②**，附 ② vs ①（huma 层开销）。
**产出**：`docs/perf/02-fiber-vs-huma-stdlib.md`。

### 5.3 compress 中间件

**技术细节**：`internal/middleware/compress.go`（fiber compress，LevelDefault），位于中间件链 Recover→Metrics→Inflight→Guard→CORS→**Compress**→Trace→Log。

**测试载体**：主服务（本地实例），mock 上游返回三档体积（2KB / 20KB / 100KB JSON）。
**对比组**：compress 开（现状）vs 关（环境开关 `PERF_DISABLE_COMPRESS=1`，一个 if 的测试开关；不改中间件链顺序）。
**指标**：QPS、p99、**响应字节数**（wrk 统计 + 抓 Transfer-Encoding 证据）。压缩比数据（gzip 后/前）单独列表。
**口径说明（必须写进文档）**：本机回环带宽无意义，QPS 差异 = 压缩的净 CPU 成本；广域网收益按"压缩比 × 典型公网带宽/RTT"定性折算，不伪造数字。
**产出**：`docs/perf/03-compress-middleware.md`。

### 5.4 落库异步化（Pond 协程池）

**技术细节**：`internal/infrastructure/pool/pool.go`（storePool/agentPool）、`TaskSubmitter.SubmitMessageStoreTask`（`internal/application/llmproxy/usecase/recorder.go`）、审计聚合批量写。

**测试载体**：harness `POST /bench/store`，handler 写一条真实 audit 记录（真实表结构、真实 DB）。
**对比组**：
- 基线：请求内同步 `INSERT`（等 DB 返回才响应）。
- 优化后：提交 Pond 协程池异步写，立即响应（与现状代码路径一致）。
- 附带观察：异步组在压测结束后的**落库完成时间**（验证不丢数据）与写入总量核对。
**指标**：QPS、p99 延迟（主指标，预计差异最大）、DB 实际写入行数。
**写入清理**：`DELETE FROM model_call_audits WHERE api_key_id = <perf-bench key>`。
**产出**：`docs/perf/04-async-store.md`。

### 5.5 缓存

**技术细节**：`SessionDetailCache`（`internal/infrastructure/cache/session_detail.go`，MGet/MSet 批量）、触发词 AC 自动机（`internal/application/trigger/service.go`，进程内）、`TriggerHitCache` Pipeline 计数。

**测试载体**：主服务真实接口 `GET /api/v1/session/.../messages`（用生产真实 session 数据）。
**对比组**（无需改代码，天然冷热对比）：
- 缓存冷：压测前 DEL 相关缓存 key，每请求直查 DB（等价"无缓存"）。
- 缓存热：warmup 后稳定命中 Redis，DB 零查询。
- 附加：触发词检查路径「AC 自动机（内存）」vs「每请求查 DB」（harness 复现两种实现）。
**指标**：QPS、p99、DB 查询数（pg_stat 或日志佐证）。
**注意**：冷组压测会给 DB 造成真实查询压力，限 30s、并发 ≤100。
**产出**：`docs/perf/05-cache.md`。

### 5.6 连接池调优

**技术细节**：`internal/infrastructure/database/postgresql.go`（`SetMaxIdleConns/SetMaxOpenConns/SetConnMaxLifetime`，现状 10/100/5h）。

**前置小改动**：将 3 个常量改为可被环境变量覆盖（默认值不变），否则无法扫参。这是本任务唯一的生产代码改动，需附回归测试。

**测试载体**：主服务 DB 密集端点（session list 或 audit list）。
**对比组**：`MaxOpenConns ∈ {10, 25, 50, 100, 200}` × `MaxIdleConns ∈ {10, 50}` 扫描，固定并发 100/200。
**指标**：QPS、p99、Postgres 侧活跃连接数、错误率（连接等待超时）。
**结论形态**：QPS-连接数曲线 + 拐点识别（当前 100 是否最优，给出推荐值）。
**产出**：`docs/perf/06-connection-pool.md`。

### 5.7 复合索引

**技术细节**：`model_call_audit` 三组索引（`idx_mca_apikey_created`、`idx_mca_model_created`、`idx_mca_created_at`，见 `internal/infrastructure/database/model/model_call_audit.go` 注释）、`SessionKeywordFilterSQL` 的 SQL 形态优化（`internal/common/constant/sql.go`）。

**测试载体**：生产 DB 只读（走 `query-prod-database` skill）。
**对比组**（不建不删索引）：
- `EXPLAIN (ANALYZE, BUFFERS)` 对同一查询：默认（索引可用）vs `SET LOCAL enable_indexscan=off; enable_bitmapscan=off`（模拟无索引）。
- 代表查询 3 条：audit 按 `api_key_id + created_at` 范围、audit 按 `model_id + created_at`、session keyword 过滤（`SessionKeywordFilterSQL`）。
- 附加：harness 并发执行同一 SQL（有/无索引两组）测真实 QPS。
**指标**：单查询延迟（ms）、执行计划（扫描行数/buffers）、并发 QPS。
**产出**：`docs/perf/07-compound-index.md`。

## 6. 安全与风险

| 风险 | 缓解 |
|------|------|
| 压测影响线上服务 | 只压 18080/18081 独立实例；低峰期执行；单轮 30s、并发 ≤200；异常立即 `kill` 被测进程 |
| 生产 DB/Redis 写入污染 | 专用测试 Key 标记全部写入；测试后 SQL 清理并留存清理语句；缓存写入幂等 + TTL 自然过期 |
| DB 写压力（4/6 项） | 写入量预估（30s × QPS 上限 ≈ 数万行），先小并发验证再放量；监控 DB 负载 |
| 隧道/跨机 RTT 失真 | 默认走 §3.1 同机拓扑；备用拓扑结论标注"仅相对值" |
| 生产凭据 | 遵循 `login-prod-server` skill 安全基线；凭据不进仓库、不进文档 |
| 扫参改动引入回归 | 仅 env 覆盖默认值，附单测钉死默认行为 |

## 7. 交付物与验收

**交付物**（7 份，均中文，放 `docs/perf/`）：

1. `01-sonic-vs-encoding-json.md`
2. `02-fiber-vs-huma-stdlib.md`
3. `03-compress-middleware.md`
4. `04-async-store.md`
5. `05-cache.md`
6. `06-connection-pool.md`
7. `07-compound-index.md`

**每份文档统一结构**：优化背景与技术细节（含代码位置）→ 测试方法（拓扑/对比组/参数）→ 原始数据表 → QPS/延迟提升结论 → 适用边界与口径说明 → 复现命令。

**验收标准**：
- [ ] 7 份文档齐全，结构统一
- [ ] 每份有可复现命令与原始数据存档（`docs/perf/raw/`）
- [ ] 每份给出明确的 QPS 提升数字（或明确说明该项收益不在 QPS 维度，如 compress 的带宽维度、索引的延迟维度）
- [ ] 生产数据清理完成并有证据
- [ ] 唯一的生产代码改动（5.6 env 覆盖）附回归测试，全量测试通过

## 8. 未决问题（评审时确认）

1. **执行时机**：生产机压测需低峰期执行并提前公告，何时执行？若不允许在生产机跑压测，4/5/6 退化为备用拓扑 B（结论弱化，文档标注）。
2. **第 6 项常量改 env 覆盖**是否可接受为唯一生产代码改动？若否，退化为编译期 `-ldflags` 变量方案（同样需要把 const 改 var）。
3. 文档语言确认为中文（遵循 superpowers 文档语言规范）。
