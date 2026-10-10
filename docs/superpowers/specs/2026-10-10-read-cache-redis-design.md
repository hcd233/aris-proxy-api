# 高频读接口 Redis 读缓存设计（穿透 / 雪崩防护）

> 日期：2026-10-10
> 分支：`feature/read-cache-redis-2026-10-10`
> 目标：为高频读接口增加 Redis cache-aside 缓存，防护缓存穿透与缓存雪崩，并用端到端压测给出加缓存前后的 QPS。

## 背景

模型目录类读接口（`GET /api/openai/v1/models`、`GET /api/anthropic/v1/models`、
`GET /api/cli/v1/model/list`、`GET /api/web/v1/model/list`）每个请求都同步查 PostgreSQL：

- `EndpointReadRepository.ListAliases` / `ListEnabledModelDetails`：每次 LLM 客户端启动、
  重连、导出配置都会调用，数据低频变更、读高频。
- `ModelRepository.PaginateWithFilter`：Web 管理页轮询，带筛选/排序/分页参数。

读多写少的目录数据每请求回源，既浪费数据库往返，也让读接口 QPS 上限被数据库锁死。

## 范围

### 本次缓存的接口

| 缓存点 | 服务接口 | 载荷 |
|---|---|---|
| `EndpointReadRepository.ListAliases` | `GET /api/openai/v1/models`、`GET /api/anthropic/v1/models` | `[]*ModelAliasProjection` |
| `EndpointReadRepository.ListEnabledModelDetails` | `GET /api/cli/v1/model/list` | `[]*ModelDetailProjection` |
| `ModelRepository.PaginateWithFilter` | `GET /api/web/v1/model/list` | DB 行 `[]*dbmodel.Model` + `*model.PageInfo` |

### 明确不做（及理由）

- `EndpointRepository.BatchFindByIDs`、`EndpointResolver.Resolve` / `FindEndpointByAlias`：
  载荷含上游 API Key 凭据，按 `2026-09-27-apikey-auth-cache-design` 的结论**不把凭据写进 Redis**；
  端点解析缓存属于架构 review 子项 C（端点 failover + 解析结果缓存），与 failover 必须同周期改造。
- `GET /api/web/v1/upstream/list`：组装载荷（endpoint 聚合）含上游凭据，同上。
- session 列表 / 详情：已有 `SessionDetailCache` 与专门性能优化（`2026-05-29-session-detail-perf-design`），
  且用户数据读的一致性敏感度高，不在本次范围。

选择依据：生产 CLS 流量画像（近 2 小时）显示当前部署流量极低，无法按线上 QPS 排序，
故按「被客户端高频轮询 + 读多写少 + 载荷无凭据」的架构判据选择缓存点。

## 方案选型

| 方案 | 说明 | 结论 |
|---|---|---|
| A. 应用层查询处理器缓存 | 在 query handler 外包缓存，需新增跨模块应用层 port（lint 禁止 application 导入 `infrastructure/cache`） | 样板多、跨模块 port 无处安放，放弃 |
| B. 仓储层缓存（选定） | 在 `internal/infrastructure/repository` 的读方法内部缓存可序列化载荷（投影 / DB 行），写方法失效 | 零业务层改动、失效与数据同层、载荷天然可序列化且无凭据 |
| C. HTTP 响应缓存 | handler 层缓存序列化响应 | 违反「Handler 保持薄封装」契约，放弃 |

## 缓存核心

新增 `internal/infrastructure/cache/read_cache.go`：

```go
type ReadCache struct { client *redis.Client; group singleflight.Group }

func NewReadCache(client *redis.Client) *ReadCache
func GetOrLoad[T any](ctx context.Context, c *ReadCache, key string,
    loader func(context.Context) (mo.Option[T], error)) (mo.Option[T], error)
func (c *ReadCache) InvalidateAll(ctx context.Context)
```

三类风险的对应防护：

1. **缓存穿透**（反复查询不存在的数据，如 `model/list?query=随机词`、无模型用户拉 `/models`）：
   `loader` 返回 `mo.None` 视为「查无此物」，写入**空值标记**并使用短 TTL（30s + 抖动），
   后续请求命中空值标记直接返回，不再回源。
2. **缓存雪崩**（大量 key 同时过期 / 失效后同时重建）：
   写入 TTL = 基准 TTL + `rand.Int64N(抖动)`（5m ± 1m，空值 30s ± 10s），过期时刻被打散；
   主动失效（`InvalidateAll`）后重建同样受抖动 + singleflight 保护。
3. **缓存击穿**（热点 key 过期瞬间并发回源打穿数据库）：
   `golang.org/x/sync/singleflight` 合并同 key 并发回源，只有一个 goroutine 查库。

降级语义：**Redis 故障 fail-open**——读失败直接回源、写失败忽略，缓存永不影响正确性，
因此不需要额外的配置开关做逃生门（压测的「加缓存前」= 不装配缓存的仓储构造函数）。

## 失效策略

写路径成功后调用 `InvalidateAll()`（`SCAN cache:rd:*` + `DEL`）：

- `modelRepository`：`Create` / `Update` / `Delete` / `DeleteByEndpointID` / `UpdateWithHistorySync`
- `endpointRepository`：`Create` / `Update` / `Delete` / `DeleteCascade`（级联删模型）

选择全量失效而非按 owner 记账：写操作低频（管理动作），`SCAN` 成本可接受；
按 userID 记账需要在删除路径上反查归属，存在「admin 删他人模型漏失效」的记账漏洞，
全量失效最简单且无正确性风险。失效后的新鲜数据由 TTL 抖动 + singleflight 平滑回源。

## 构造与装配

仓储新增带缓存构造函数，原构造函数保持不变（无缓存、供测试与「加缓存前」对照）：

- `NewEndpointReadRepository(db)` / `NewCachedEndpointReadRepository(db, c)`
- `NewModelRepository(db)` / `NewCachedModelRepository(db, c)`
- `NewEndpointRepository(db)` / `NewCachedEndpointRepository(db, c)`

`internal/bootstrap/modules/`：`NewReadCache(*redis.Client)` 提供 `*cache.ReadCache`，
`RepositoryModule` 改用 Cached 构造函数。

## 常量

全部进 `internal/common/constant/`（业务包禁止本地 const 块）：

- `rediskey.go`：`ReadCacheAliasKeyTemplate`、`ReadCacheDetailKeyTemplate`、
  `ReadCacheModelListKeyTemplate`、`ReadCacheKeyScanPattern`
- `readcache.go`：TTL、抖动上限、空值标记、SCAN 批次

## 测试策略

### 单元测试 `test/unit/cache/read_cache_test.go`（miniredis）

1. 未命中 → 回源 1 次并写缓存；再次读 → 回源 0 次
2. `loader` 返回 `mo.None` → 写空值标记（短 TTL），重复读不回源（防穿透）
3. 写入 TTL 存在随机抖动（多个 key TTL 互不相同，防雪崩）
4. 并发同 key 只回源一次（singleflight，防击穿）
5. Redis 读写失败 → fail-open 直接回源、不失效正确性
6. `InvalidateAll` → 命名空间内 key 全部清除

### 端到端测试 `test/e2e/read_cache/`（RegisterAPIRouter + sqlite + miniredis）

1. 缓存命中：`model/list` 重复请求的 SQL 查询数（GORM callback）严格下降；
   目录接口（openai/anthropic/cli）缓存后只剩 API Key 鉴权查询
2. 空结果缓存防穿透：`model/list?query=不存在的词` 重复请求 SQL 查询数为 0；
   无缓存装配对照组每次都查库
3. 写后失效：`POST /api/web/v1/model` 后立即读列表/别名/详情接口均可见新模型（不脏读）
4. TTL 抖动：多条缓存 key 的 TTL 互不相同（雪崩防护证据）
5. **QPS 对比压测**（`qps_test.go`）：同一条 HTTP 链路（API Key / JWT 中间件 + 真实路由 +
   sqlite + miniredis）分别以「无缓存装配（= 加缓存前）」与「有缓存装配（= 加缓存后）」压测：
   - 网关/客户端目录接口（无限流）测吞吐：workers=8、2s/项
   - Web 管理接口受 TokenBucket 限流（`constant.LimitManageAPIKey` = 20 次/分钟/用户），
     吞吐上限由限流器决定，改测顺序请求平均延迟

### 回归验证方法论

每条用例写完后临时注入对应缺陷（去掉空值标记 / 去掉抖动 / 去掉失效钩子）确认用例 FAIL，再恢复。

## 优化效果（端到端压测结果）

压测口径：`test/e2e/read_cache/qps_test.go`，同一条端到端链路（`RegisterAPIRouter` + APIKey/JWT
中间件 + 真实仓储/用例 + sqlite + miniredis）同一二进制下「无缓存装配（= 加缓存前）」与
「有缓存装配（= 加缓存后）」对比；数据集 200 个模型，workers=8，每项 2s。
DB 连接池固定 1 条（模拟生产 DB 连接池瓶颈），缓存预热后测稳态。

| 接口 | 加缓存前 | 加缓存后 | 提升 |
|---|---|---|---|
| `GET /api/openai/v1/models` | 6,111 QPS | 18,332 QPS | **3.00x** |
| `GET /api/anthropic/v1/models` | 6,203 QPS | 18,060 QPS | **2.91x** |
| `GET /api/cli/v1/model/list` | 1,162 QPS | 14,654 QPS | **12.61x** |
| `GET /api/web/v1/model/list`* | 0.686ms | 0.463ms | 1.48x（延迟） |
\* Web manage API 受 TokenBucket 限流（`constant.LimitManageAPIKey` = 20 次/分钟/用户），
吞吐上限由限流器决定，改测顺序请求平均延迟。

数据取自合并 master（模型调度/Decision/Playground）后的复测；独立重复运行的波动在
同一量级（如 openai `/models` 另一次测得 6189 → 18848 QPS，3.05x）。

口径说明：sqlite + miniredis 均在进程内，测得的是受控环境下的相对值；生产 PostgreSQL
往返（网络 + 解析）远高于内存 sqlite，真实收益不低于该比值。`model/list` 提升幅度较小的
原因见「已知限制 2」（仅缓存分页行，端点/用户回填仍逐请求查库）。

## 已知限制

1. **miniredis + sqlite 内存库不等于生产网络拓扑**：压测 QPS 是受控环境下的相对值，
   生产 PostgreSQL 往返（网络 + 解析）远高于内存 sqlite，真实收益应不低于压测比值。
2. **`model/list` 缓存只覆盖分页行**：端点/用户回填（`BatchFindByIDs`）仍逐请求查库，
   其中端点聚合含上游 API Key（不入 Redis），故未缓存。升级路径：在应用层缓存
   `ListModelView` 结果（需应用层 port，见方案 A）。
3. **绕过仓储层的写操作（运维直改库）不失效缓存**，由 TTL 兜底（最长 5m + 1m）。
4. **单实例 singleflight**：多副本部署下跨 pod 回源不合并，靠 TTL 抖动分散。

## 实现期项目硬约束

- 常量入 `internal/common/constant/`，业务包禁止本地 `const` 块
- 错误创建与包装统一 `internal/common/ierr`，禁止 `errors.New` / `fmt.Errorf`
- `internal/` 下不得放 `_test.go`，测试放 `test/unit/`、`test/e2e/`
- JSON 统一 `github.com/bytedance/sonic`；生产代码禁止 `any` / `interface{}`（用泛型或具体结构体）
- 日志 `logger.WithCtx(ctx)`，前缀 `[PascalCaseModule]`
- 编辑 Go 文件前跑 `use-modern-go` 的 `list`；改 `internal/bootstrap/modules/**` 前加载 `golang-uber-fx`
