# Dashboard 一屏指挥台 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把首页 dashboard 重做成 1440×900 一屏的指挥台：6 个带环比的 KPI 卡片加 6 块面板，并补齐对应的后端聚合接口；旧的 8 个统计接口和 8 个图表组件随之删除。

**Architecture:**
- 后端在 `model_call_audit` 上新增独立的 `modelcall.DashboardRepository`，提供 7 个聚合查询：PG `FILTER` / `percentile_cont`，结果走读缓存。
- 应用层 `port.DashboardService` 统一做一次权限范围解析，然后负责上一周期计算、并发查询、补桶、Top 聚合和 demo 脱敏。
- handler 扩展现有的 `AuditHandler`，挂在 `/api/web/v1/audit/stats/dashboard/*` 下。
- 前端新建 `components/dashboard/`：页面持有唯一的时间范围，每个面板各自请求、各自处理失败。

**Tech Stack:** Go（fiber v3 + huma v2 + GORM/PostgreSQL + uber-fx + samber/lo + x/sync/errgroup），Next.js 16 static export + React 19 + recharts + Tailwind v4 + vitest。

**Spec:** `docs/superpowers/specs/2026-10-28-dashboard-redesign-design.md`（§3 布局、§4 口径、§5 权限、§6 接口是本计划的依据）。可视化稿：`.superpowers/brainstorm/59206-1793178348/content/layout-a-dense.html`。

## Global Constraints

- 工作目录：`/Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/dashboard-redesign`，分支 `feature/dashboard-redesign-2026-10-28`。所有路径都相对这个 worktree，**不要改主工作区**。
- 写 Go 代码前先加载这些 skill：`use-modern-go`（先跑 `list`）、`golang-naming`、`golang-code-style`、`golang-samber-lo`；改 DTO 时加 `huma-dto-conventions`；改 fx 装配时加 `golang-uber-fx`。`docs/agents/go-backend.md` 的硬约束优先级高于 skill。
- 业务包禁止本地 `const`：SQL 片段放 `internal/common/constant/sql.go`，读缓存 purpose 放 `internal/common/constant/readcache.go`，枚举放 `internal/common/enum/`。
- 错误统一用 `ierr`（`ierr.Wrap(ierr.ErrDBQuery, err, "...")`、`ierr.ErrValidation`、`ierr.ErrNoPermission`、`ierr.ErrUnauthorized`），禁止 `errors.New` / `fmt.Errorf`；JSON 统一用 sonic。
- 测试只能放在 `test/unit/<pkg>/` 或 `test/e2e/<pkg>/`（conv lint 禁止 `internal/` 下出现 `_test.go`）；test 目录禁止 `time.Sleep`。
- 范围语义：`apiKeyIDs == nil` 表示 admin/demo 全量；非 nil 但为空时，仓储方法第一行必须短路返回空或零值（`if apiKeyIDs != nil && len(apiKeyIDs) == 0`），不能触达 SQL。
- 统计口径（spec §4，逐字遵守）：
  - 成功 = `upstream_status_code = 200`。
  - 失败分类：429 → rateLimit，500–599 → server，-1 → connection，其余非 200 → other（= requests − success − 前三类）。
  - Token、费用、延迟、输出速度只统计成功请求；请求数、热力图、客户端分布统计全部请求。
  - 费用只汇总 `cost_micro IS NOT NULL` 的记录，币种固定为 `enum.CurrencyUSD`。
  - 输出速度 = `SUM(output_tokens)*1000.0/NULLIF(SUM(stream_duration_ms),0)`，只统计 `stream_duration_ms > 0` 的成功行。
  - 缓存命中率 = cacheRead / (input + cacheWrite + cacheRead)，分母为 0 时为 nil。
- 上一周期 = `[start − (end − start), start)`，实现为 `prevStart = start.Add(-(end.Sub(start)))`、`prevEnd = start.Add(-time.Microsecond)`。
- `limit` ∈ [1, 10]，默认 5（huma `minimum/maximum/default` 标签）。`tz` 必须能被 `time.LoadLocation` 解析，否则返回 `ierr.ErrValidation`。
- 补桶后的时间桶数超过 `constant.DashboardMaxBuckets = 1500` 时返回 `ierr.ErrValidation`。
- 前端：
  - 新增文案在 `web/src/locales/{zh,en,ja}.json` 三份里都补齐。
  - 含翻译文本的 `useMemo` 依赖 `locale`，不依赖 `t`。
  - 截断文本用 Tooltip 包裹（lint 规则 `truncate-requires-tooltip`）。
  - 颜色只用主题 token 和 `lib/theme` 的 hook，不写死 hex。
  - 改动的 ts/tsx 文件提交前都要跑 `npm run format`（CI 会跑 `format:check`）。
- 每个 commit 由仓库 pre-commit hook 跑全仓 gofmt/vet/test/lint。接口签名变更必须和它的全部实现者、测试 fake 放在同一个 commit；untracked 文件要手动 `git add`。

## Review Focus

1. **区间内一条记录都没有**（新用户有 Key 但没流量）：KPI 显示 0 或「—」，延迟分位数为 nil 而不是 0，环比为 nil，页面不能出现 NaN 或 Infinity。测试：Task 3 `TestDashboardOverview_EmptyRangeHasNilLatency`，Task 5 `deltaPct(0, 0) === null`。
2. **上一周期为 0、本期非 0**：环比必须是 nil（显示「—」），不能是 Infinity%。测试：Task 5 `deltaPct(5, 0) === null`。
3. **调用方手写 `granularity=minute` 查 30 天**：桶数爆炸会拖垮 percentile 查询。测试：Task 3 `TestDashboardOverview_RejectsTooManyBuckets` 期望返回 `ierr.ErrValidation`，并断言 repo 一次都没被调用。
4. **`tz` 传入非法值或注入串**（如 `UTC'; drop table`）：必须在触达 SQL 前被拒绝。测试：Task 3 `TestDashboardHeatmap_RejectsInvalidTimezone`。
5. **demo 账户看排行和 Endpoint 表**：用户名、Key 名、Endpoint 名必须脱敏；普通用户请求 `dimension=user` 必须被拒。测试：Task 3 `TestDashboardTop_DemoMasksNames`、`TestDashboardEndpoints_DemoMasksEndpoint`、`TestDashboardTop_UserDimensionForbiddenForUser`。

---

### Task 1: Dashboard 仓储——总览聚合（stats + series）

**Files:**
- Create: `internal/domain/modelcall/dashboard.go`
- Create: `internal/common/enum/dashboard_dimension.go`
- Create: `internal/infrastructure/repository/dashboard_repository.go`
- Modify: `internal/common/constant/sql.go`（追加 dashboard SQL 常量块）
- Modify: `internal/common/constant/readcache.go`（追加 7 个 purpose）
- Modify: `internal/common/constant/`（任选合适文件追加 `DashboardMaxBuckets = 1500`、`DashboardErrorMessageMaxLen = 200`）
- Test: `test/unit/dashboard_repo/stats_test.go`

**Interfaces:**
- Produces（`modelcall` 包）：
```go
type DashboardStats struct {
    Requests, Success, RateLimitFailures, ServerFailures, ConnectionFailures int64
    InputTokens, OutputTokens, CacheReadTokens, CacheWriteTokens          int64
    CostMicro, PricedSuccess                                              int64
    LatencyP50Ms, LatencyP95Ms, LatencyP99Ms                              *float64
    OutputTokensPerSecond, AvgStreamDurationMs                            *float64
}   // 每个字段带 gorm:"column:<snake_case>"
type DashboardStatsPoint struct { Time time.Time `gorm:"column:time"`; DashboardStats `gorm:"embedded"` }
type DashboardModelPoint struct { ModelID string; Time time.Time; Requests, Tokens, CostMicro int64 }
type DashboardErrorRow struct { StatusCode int; Message string; Count int64 }
type DashboardEndpointRow struct { Endpoint string; Requests, Failures int64; LatencyP95Ms, OutputTokensPerSecond *float64; CostMicro int64 }
type DashboardGroupRow struct { Key string; Requests, InputTokens, OutputTokens, CacheReadTokens, CacheWriteTokens, CostMicro int64 }
type DashboardHeatmapCell struct { Weekday, Hour int; Count int64 } // Weekday = PG ISODOW 1..7
type DashboardRepository interface {
    QueryStats(ctx context.Context, apiKeyIDs []uint, start, end time.Time) (*DashboardStats, error)
    QueryStatsSeries(ctx context.Context, apiKeyIDs []uint, start, end time.Time, granularity enum.Granularity) ([]*DashboardStatsPoint, error)
    QueryModelSeries(ctx context.Context, apiKeyIDs []uint, start, end time.Time, granularity enum.Granularity) ([]*DashboardModelPoint, error)
    QueryTopErrors(ctx context.Context, apiKeyIDs []uint, start, end time.Time, limit int) ([]*DashboardErrorRow, error)
    QueryEndpointHealth(ctx context.Context, apiKeyIDs []uint, start, end time.Time, limit int) ([]*DashboardEndpointRow, error)
    QueryGroupUsage(ctx context.Context, apiKeyIDs []uint, start, end time.Time, dimension enum.DashboardDimension, rulesFingerprint string) ([]*DashboardGroupRow, error)
    QueryHeatmap(ctx context.Context, apiKeyIDs []uint, start, end time.Time, location string) ([]*DashboardHeatmapCell, error)
}
```
- Produces（`enum` 包）：`type DashboardDimension = string`，常量 `DashboardDimensionModel="model"`、`DashboardDimensionUser="user"`、`DashboardDimensionAPIKey="apikey"`、`DashboardDimensionHarness="harness"`。
- Produces（`repository` 包）：`func NewDashboardRepository(db *gorm.DB, rc *cache.ReadCache) modelcall.DashboardRepository`。本任务实现 `QueryStats` 和 `QueryStatsSeries`，其余 5 个方法先放 `return nil, ierr.New(ierr.ErrInternal, "not implemented")` 占位，由 Task 2 填实。

- [ ] **Step 1: Write the failing tests** — 新建 `test/unit/dashboard_repo/stats_test.go`。PG DryRun 的建库方式照抄 `test/unit/agent_harness/audit_trend_test.go` 的 `newAuditDryRunDB` 和 SQL 捕获 callback。
  - `TestQueryStatsSQL`：nil scope 产生 1 条 SQL，SQL 包含以下片段（小写比较）：`count(*) filter (where upstream_status_code = 200)`、`upstream_status_code = 429`、`between 500 and 599`、`upstream_status_code = -1`、`percentile_cont(0.95) within group (order by first_token_latency_ms)`、`cost_micro is not null`、`stream_duration_ms > 0`、`deleted_at = 0`；**不包含** `api_key_id in`。
  - `TestQueryStatsSeriesSQL`：granularity=hour 时 SQL 包含 `date_trunc('hour'` 和 `group by`。传 `[]uint{7}` 时包含 `api_key_id in`。
  - `TestDashboardStats_EmptyScopeShortCircuits`：sqlite 内存库（`sqlitetest.OpenPrivateMemoryDB`）+ AutoMigrate `ModelCallAudit` + 两行种子数据。`QueryStats(ctx, []uint{}, …)` 返回 `&DashboardStats{}` 零值且 err 为 nil；`QueryStatsSeries(ctx, []uint{}, …)` 返回长度为 0 且 err 为 nil。
  - `TestDashboardStats_CacheHit`：沿用 `test/unit/read_cache_window/read_cache_window_test.go` 的 `newAuditRepo` 计数做法（miniredis + 查询计数 callback），在新文件里改为构造 `NewDashboardRepository`。同参数连续调两次 `QueryStats`，DB 只查 1 次；终点挪 2 分钟后回源，DB 计 2 次。

- [ ] **Step 2: Run tests to verify they fail**
  Run: `rtk go test ./test/unit/dashboard_repo/...`
  Expected: 编译失败，报 `undefined: repository.NewDashboardRepository` / `modelcall.DashboardStats`。

- [ ] **Step 3: Implement**
  - SELECT 列表写成一个常量 `constant.DashboardStatsSelect`，被 `QueryStats`（无分组）和 `QueryStatsSeries`（前缀 `dateTruncSQL(g) + " AS time, "`，`GROUP BY time ORDER BY time`）共用。
  - 计数和求和一律包 `COALESCE(...,0)`，分位数和速度保留 NULL（对应 Go 端的 `*float64` nil）。
  - 成功条件复用 `constant.SQLConditionUpstreamSuccess`。
  - 缓存：
    - `QueryStats` 用 `cachedSlice` 包成单元素切片：key = `readCacheKey(ReadCachePurposeDashboardStats, cacheScopeIDs(ids), cacheTimeBucket(start), cacheTimeBucket(end))`，loader 返回 `[]*DashboardStats{row}`，取 `[0]`。
    - `QueryStatsSeries` 的 key 额外带 granularity。
  - `dateTruncSQL` 已在 `audit_repository.go` 里，同包直接复用。

- [ ] **Step 4: Run tests to verify they pass**
  Run: `rtk go test ./test/unit/dashboard_repo/...`
  Expected: PASS。

- [ ] **Step 5: Commit**
```bash
rtk git add internal/domain/modelcall/dashboard.go internal/common/enum/dashboard_dimension.go internal/infrastructure/repository/dashboard_repository.go internal/common/constant test/unit/dashboard_repo
rtk git commit -m "feat(dashboard): 新增总览聚合仓储（stats/series）"
```

### Task 2: Dashboard 仓储——模型序列、错误、Endpoint、分组、热力图

**Files:**
- Modify: `internal/infrastructure/repository/dashboard_repository.go`
- Modify: `internal/common/constant/sql.go`
- Test: `test/unit/dashboard_repo/breakdown_test.go`

**Interfaces:**
- Consumes：Task 1 的 `DashboardRepository` 接口和类型。
- Produces：5 个方法的真实实现（签名见 Task 1）。

- [ ] **Step 1: Write the failing tests**（PG DryRun 捕获 SQL，断言小写片段）
  - `TestQueryModelSeriesSQL`：`group by model_id, time`；tokens 和 cost 求和带 `filter (where upstream_status_code = 200`。
  - `TestQueryTopErrorsSQL`：`upstream_status_code <> 200`、`left(error_message, 200)`、`order by count desc`，并且 limit 走绑定参数（`tx.Statement.Vars` 里含 5）。
  - `TestQueryEndpointHealthSQL`：`group by endpoint`、`order by requests desc`、`percentile_cont(0.95)`。
  - `TestQueryGroupUsageSQL`：表驱动三种 dimension：`model` → `group by model_id`；`apikey` → `cast(api_key_id as text)`；`harness` → 包含 `agent_harness_rules` 和 `'Other'`。传入 `user` 时返回 `ierr.ErrValidation` 且不产生 SQL（user 维度由应用层把 apikey 维度的行按用户聚合）。
  - `TestQueryHeatmapSQL`：包含 `extract(isodow from created_at at time zone ?)` 和 `extract(hour from created_at at time zone ?)`；`Vars` 里出现两次 `"Asia/Shanghai"`；SQL 文本里**不出现**字面量 `Asia/Shanghai`（证明走的是参数绑定）。
  - `TestQueryGroupUsageCacheKeyIncludesFingerprint`：harness 维度下，指纹 `"1:1"` 与 `"2:2"` 各回源一次（仿照 `TestQueryHarnessTrendCacheKeyIncludesRulesFingerprint`）。
  - `TestDashboardBreakdown_EmptyScopeShortCircuits`：5 个方法传 `[]uint{}` 都返回空结果且 err 为 nil（sqlite 下没有短路就会因 PG 语法报错，这本身就是判据）。

- [ ] **Step 2: Run tests to verify they fail**
  Run: `rtk go test ./test/unit/dashboard_repo/...`
  Expected: FAIL，报 `not implemented`。

- [ ] **Step 3: Implement**
  - 分组 key 表达式按 dimension 白名单 `switch`：model → `constant.FieldModelID`；apikey → `CAST(api_key_id AS TEXT)`；harness → `constant.AuditHarnessGroupSQL`。default 返回 `ierr.ErrValidation`。
  - heatmap 用 `db.Select(constant.DashboardHeatmapSelect, location, location).Group("1, 2")`。
  - 每个方法都用 `cachedSlice` 包裹，key 包含 scope、时间桶，以及 limit / dimension / granularity / location / fingerprint 中各自用到的参数。

- [ ] **Step 4: Run tests to verify they pass**
  Run: `rtk go test ./test/unit/dashboard_repo/...`
  Expected: PASS。

- [ ] **Step 5: Commit** — `feat(dashboard): 补齐模型序列/错误/Endpoint/分组/热力图聚合`

### Task 3: DashboardService（权限范围、上一周期、补桶、Top 聚合、脱敏）

**Files:**
- Create: `internal/application/audit/port/dashboard.go`
- Create: `internal/application/audit/query/dashboard_service.go`
- Modify: `internal/application/audit/query/harness_trend.go`：把 `harnessRulesFingerprint` 挪到 `dashboard_service.go`。旧文件在 Task 8 删除，挪动后旧文件仍需能编译。
- Test: `test/unit/dashboard_service/dashboard_service_test.go`

**Interfaces:**
- Consumes：`modelcall.DashboardRepository`；`port.APIKeyIDLookup`；`agentharness.RuleRepository`；`port.BuildBuckets(start, end, g, fallback)`；`commonutil.MaskIdentity/MaskSecret`。
- Produces（`port` 包）：
```go
type DashboardScope struct { Permission enum.Permission; UserID uint }
type DashboardPeriod struct { Summary *modelcall.DashboardStats; Series []*modelcall.DashboardStatsPoint }
type DashboardOverview struct { Current, Previous DashboardPeriod }
type DashboardTopItem struct { Key, Name string; Requests, Tokens, CacheReadTokens, PromptTokens, CostMicro int64 }
type DashboardTop struct { Items []*DashboardTopItem; OtherRequests, OtherCostMicro, TotalCostMicro int64 }
type DashboardRelationLookup interface { BatchGetRelations(ctx context.Context, apiKeyIDs []uint) (map[uint]*modelcall.AuditRelation, error) }
type DashboardService interface {
    Overview(ctx context.Context, s DashboardScope, start, end time.Time, g enum.Granularity) (*DashboardOverview, error)
    ModelSeries(ctx context.Context, s DashboardScope, start, end time.Time, g enum.Granularity) ([]*modelcall.DashboardModelPoint, error)
    TopErrors(ctx context.Context, s DashboardScope, start, end time.Time, limit int) ([]*modelcall.DashboardErrorRow, error)
    EndpointHealth(ctx context.Context, s DashboardScope, start, end time.Time, limit int) ([]*modelcall.DashboardEndpointRow, error)
    TopUsage(ctx context.Context, s DashboardScope, start, end time.Time, dim enum.DashboardDimension, limit int) (*DashboardTop, error)
    Heatmap(ctx context.Context, s DashboardScope, start, end time.Time, location string) ([]*modelcall.DashboardHeatmapCell, error)
}
```
  - Tokens = input + output + cacheRead + cacheWrite；PromptTokens = input + cacheRead + cacheWrite（handler 用它算命中率）。
- Produces（`query` 包）：`func NewDashboardService(repo modelcall.DashboardRepository, apiKeyIDs port.APIKeyIDLookup, relations port.DashboardRelationLookup, rules agentharness.RuleRepository) port.DashboardService`。

- [ ] **Step 1: Write the failing tests** — fake repo 记录每次调用的 `apiKeyIDs` 和时间参数。
  - `TestDashboardScope_Dispatch`：Admin 和 Demo 的 repo 收到 nil；User 收到 lookup 返回的 `[]uint{3,4}`；未知权限返回 `ierr.ErrUnauthorized`。
  - `TestDashboardScope_UserWithoutKeysShortCircuits`：lookup 返回空时，6 个方法都返回零值或空结果，fake repo 调用次数为 0；Overview 的 `Current.Series` 仍是补齐后的零值桶（长度 = BuildBuckets 的长度）。
  - `TestDashboardOverview_PreviousPeriod`：start=10-21T00:00Z、end=10-28T00:00Z 时，repo 收到的上一周期为 `[10-14T00:00Z, 10-20T23:59:59.999999Z]`。
  - `TestDashboardOverview_FillsMissingBuckets`：repo 只返回 1 个桶，granularity=day、区间 7 天时，Series 长度为 8，缺失桶的 Requests 为 0、Latency 为 nil。
  - `TestDashboardOverview_EmptyRangeHasNilLatency`：repo 返回 `&DashboardStats{}` 时，Summary 的 LatencyP95Ms 为 nil。
  - `TestDashboardOverview_RejectsTooManyBuckets`：granularity=minute、区间 30 天时返回 `ierr.ErrValidation`，repo 调用次数为 0。
  - `TestDashboardTop_UserDimensionAggregatesByOwner`：admin 下 repo 的 apikey 维度返回 key "1"、"2"（都属于 user 9）和 "3"（属于 user 8），relations 返回对应 UserID 和 UserName。结果 Items[0].Key=="9"，requests 为前两行之和，Name 为用户名；结果按 CostMicro 降序、Requests 次序；limit=1 时 OtherRequests 等于第二名的 requests；TotalCostMicro 为全部之和。
  - `TestDashboardTop_UserDimensionForbiddenForUser`：User 权限 + dimension=user 返回 `ierr.ErrNoPermission`。
  - `TestDashboardTop_APIKeyNamesFromRelations`：apikey 维度的 Name 取 relation.APIKeyName，找不到 relation 时 Name 用 Key 原值。
  - `TestDashboardTop_DemoMasksNames`：Demo 下 user 维度的 Name 等于 `commonutil.MaskIdentity(原名)`，apikey 维度的 Name 等于 `commonutil.MaskSecret(原名)`。
  - `TestDashboardEndpoints_DemoMasksEndpoint`：Demo 下 Endpoint 等于 `commonutil.MaskSecret(原值)`；Admin 下保持原值。
  - `TestDashboardHeatmap_RejectsInvalidTimezone`：`"UTC'; drop table x"` 和 `"Mars/Base"` 都返回 `ierr.ErrValidation`，repo 调用次数为 0；`"Asia/Shanghai"` 原样透传给 repo。

- [ ] **Step 2: Run tests to verify they fail**
  Run: `rtk go test ./test/unit/dashboard_service/...`
  Expected: 编译失败，报 `undefined: query.NewDashboardService`。

- [ ] **Step 3: Implement**
  - `resolveDashboardScope(ctx, s) (ids []uint, empty bool, err error)` 是唯一的分流点：Admin/Demo → `(nil, false, nil)`；User → lookup，结果为空时 `(nil, true, nil)`；其他 → `ierr.ErrUnauthorized`。
  - Overview：
    - 先校验桶数（`len(port.BuildBuckets(start,end,g,nil)) > constant.DashboardMaxBuckets` 时拒绝）。
    - 用 `errgroup.WithContext` 并发跑 4 条查询。
    - series 按 `BuildBuckets` 补零值点；DB 返回的桶按 `time.Equal` 匹配，可借助 `port.IndexSeries` 或用 `map[int64]` 以 UnixNano 为 key 自写。
  - TopUsage 的 user 维度：用 apikey 维度的 rows，经 `BatchGetRelations` 按 UserID 合并，Key 为 `strconv.FormatUint(userID)`；relation 缺失的行按 Key 归为 `"0"`，Name 为空。
  - Demo 判定统一用 `s.Permission == enum.PermissionDemo`。

- [ ] **Step 4: Run tests to verify they pass**
  Run: `rtk go test ./test/unit/dashboard_service/...`
  Expected: PASS。

- [ ] **Step 5: Commit** — `feat(dashboard): 新增 DashboardService 统一权限分流与聚合`

### Task 4: DTO、Handler、路由、fx 装配

**Files:**
- Create: `internal/dto/dashboard.go`
- Modify: `internal/handler/audit.go`（`AuditHandler` 接口新增 6 个方法；`AuditDependencies` 新增 `Dashboard auditport.DashboardService`）
- Create: `internal/handler/dashboard.go`（6 个 Handle 方法，以及 `toDashboardStatsItem` 等映射函数）
- Modify: `internal/router/audit.go`（注册 6 个路由）
- Modify: `internal/bootstrap/modules/application.go`（Provide `NewDashboardService` wrapper）、`internal/bootstrap/modules/repository.go`（Provide `NewDashboardRepository` wrapper）、`internal/bootstrap/modules/handler.go`（`NewAuditDependencies` 增加参数）
- Modify: 所有实现 `handler.AuditHandler` 的测试 stub（先用 `rtk grep "handler.AuditHandler" test` 找出来，嵌入接口的 stub 不需要改）
- Test: `test/unit/dashboard_handler/dashboard_handler_test.go`

**Interfaces:**
- Consumes：Task 3 的 `port.DashboardService` 和 `port.*` 类型。
- Produces（HTTP 契约，前端 Task 5 依赖，字段名逐字使用）：
  - 路由（`auditGroup` 下）：`GET /stats/dashboard/overview`、`/stats/dashboard/model/series`、`/stats/dashboard/error/list`、`/stats/dashboard/endpoint/list`、`/stats/dashboard/top/list`、`/stats/dashboard/heatmap`。OperationID 依次为 `queryDashboardOverview`、`queryDashboardModelSeries`、`listDashboardErrors`、`listDashboardEndpoints`、`listDashboardTop`、`queryDashboardHeatmap`。中间件用 `LimitUserPermissionWithDemoMiddleware(<opID>, enum.PermissionUser, enum.DemoModuleAudit, …)`。
  - Req：
    - `DashboardRangeReq{StartTime, EndTime, Granularity}`：tag 照抄 `ModelTrendReq`。
    - `DashboardWindowReq{StartTime, EndTime, Limit int \`query:"limit" minimum:"1" maximum:"10" default:"5"\`}`。
    - `DashboardTopReq`：在 `DashboardWindowReq` 字段基础上加 `Dimension enum.DashboardDimension \`query:"dimension" required:"true" enum:"model,user,apikey,harness"\``。
    - `DashboardHeatmapReq{StartTime, EndTime, TZ string \`query:"tz" required:"true" maxLength:"64"\`}`。
  - JSON 字段：
    - `DashboardStatsItem`：`requests, success, failures{rateLimit, server, connection, other}, tokens{input, output, cacheRead, cacheWrite}, cost, pricedSuccess, latency{p50, p95, p99}, outputTokensPerSecond, avgStreamDurationMs`。cost 用 `dto.PriceDisplayFromMicro`；p50/p95/p99、outputTokensPerSecond、avgStreamDurationMs 为 `*float64`，带 omitempty 时 nil 被省略，前端按 undefined 处理。
    - `DashboardStatsPointItem` = `time` + `DashboardStatsItem` 的全部字段（嵌入）。
    - `DashboardOverviewRsp.data`：`{currency, current:{summary, series[]}, previous:{summary, series[]}}`。
    - `DashboardModelSeriesRsp.data[]`：`{modelId, time, requests, tokens, cost}`。
    - `DashboardErrorListRsp.data[]`：`{statusCode, message, count}`。
    - `DashboardEndpointListRsp.data[]`：`{endpoint, requests, errorRate, p95LatencyMs?, outputTokensPerSecond?, cost}`，其中 errorRate = failures / requests，requests 为 0 时为 0。
    - `DashboardTopListRsp.data`：`{items[{key, name, requests, tokens, cacheHitRate?, cost, costShare}], otherRequests, otherCost}`，costShare = cost / totalCost，total 为 0 时为 0。
    - `DashboardHeatmapRsp.data[]`：`{weekday, hour, count}`，其中 weekday 由 ISODOW 1..7 转为 0..6（0 = 周一）。

- [ ] **Step 1: Write the failing tests** — 用 fake `DashboardService` 直接调用 handler 方法（ctx 里注入 permission 和 userID 的方式参照 `test/unit/audit_query/` 现有做法）：
  - `TestDashboardHandler_OverviewMapsCostAndFailures`：CostMicro=1_500_000 映射为 cost 1.5；Requests=10、Success=6、RateLimit=2、Server=1、Connection=0 时 `failures.other == 1`；currency 为 `"USD"`。
  - `TestDashboardHandler_TopComputesShareAndHitRate`：TotalCostMicro=0 时 costShare 为 0；PromptTokens=0 时 cacheHitRate 为 nil；Prompt=100、CacheRead=40 时 cacheHitRate 为 0.4。
  - `TestDashboardHandler_HeatmapWeekdayZeroBased`：ISODOW 1 映射为 0，7 映射为 6。
  - `TestDashboardFxGraphValid`：`fx.ValidateApp(bootstrap.BuildFxAppOptions()...)` 返回 nil（若已有同类冒烟测试，就在原测试里确认它覆盖到了新依赖）。

- [ ] **Step 2: Run tests to verify they fail**
  Run: `rtk go test ./test/unit/dashboard_handler/...`
  Expected: 编译失败。

- [ ] **Step 3: Implement** — handler 只负责从 ctx 取 permission/userID、映射 DTO、按 `HandleModelCost` 的写法记录日志和包装错误。`ierr.ErrValidation` / `ierr.ErrNoPermission` 透传给 `apiutil.NewHumaBizError`，保持原有错误码，不要降级成 ErrInternal。

- [ ] **Step 4: Run tests and full backend check**
  Run: `rtk go test ./test/unit/dashboard_handler/... ./test/unit/dashboard_service/... ./test/unit/dashboard_repo/... && rtk go vet ./... && go run ./cmd/lint conv ./... && go run ./cmd/lint static ./...`
  Expected: 全部通过（`cmd/lint static` 在 `test/e2e/model_id` 有 6 条既有 SA5011，忽略即可，不能新增）。

- [ ] **Step 5: Commit** — `feat(dashboard): 注册 dashboard 统计接口并完成装配`

### Task 5: 前端数据层（类型、API、范围 hook、纯函数）

**Files:**
- Modify: `web/src/lib/types.ts`（新增 `DashboardStats`、`DashboardStatsPoint`、`DashboardOverview`、`DashboardModelPoint`、`DashboardErrorItem`、`DashboardEndpointItem`、`DashboardTopItem`、`DashboardTop`、`DashboardHeatmapCell`、`DashboardDimension`，字段与 Task 4 的 JSON 一致）
- Modify: `web/src/lib/api-client.ts`（新增 `fetchDashboardOverview`、`fetchDashboardModelSeries`、`fetchDashboardErrors`、`fetchDashboardEndpoints`、`fetchDashboardTop`、`fetchDashboardHeatmap`，写法照抄 `fetchModelCost`）
- Create: `web/src/lib/dashboard.ts`
- Create: `web/src/components/dashboard/use-dashboard-range.ts`
- Test: `web/src/lib/dashboard.test.ts`

**Interfaces:**
- Produces（`lib/dashboard.ts`）：
```ts
export type DeltaTone = "good" | "bad" | "neutral";
export type KpiKind = "requests" | "tokens" | "cost" | "successRate" | "cacheHitRate" | "latency";
export function deltaPct(cur: number, prev: number): number | null;          // prev<=0 → null
export function deltaDiff(cur: number | null | undefined, prev: number | null | undefined): number | null;
export function deltaTone(kind: KpiKind, delta: number | null): DeltaTone;   // cost↑/latency↑/successRate↓ → bad；requests/tokens/cacheHitRate↑ → good；null 或 0 → neutral
export function successRate(s: DashboardStats): number | null;               // requests 0 → null
export function cacheHitRate(t: DashboardStats["tokens"]): number | null;
export function totalTokens(t: DashboardStats["tokens"]): number;
export function rangeDays(startTime: string, endTime: string): number;      // 最少 1/24（用于日均）
export function peakBucket(series: DashboardStatsPoint[]): number;
export type ModelMetric = "cost" | "requests" | "tokens";
export function foldTopModels(rows: DashboardModelPoint[], metric: ModelMetric, n?: number): { keys: string[]; data: Array<{ time: string } & Record<string, number>> };  // 前 n 名按总量排序，其余合并为 "__other__"
export function heatmapMatrix(cells: DashboardHeatmapCell[]): number[][];    // 7×24，缺失补 0
```
- Produces（`use-dashboard-range.ts`）：`useDashboardRange(): { key, customStart, customEnd, setRange(key, cs, ce), startTime, endTime, granularity, prevLabelDays, refreshToken, refresh() }`。用 `usePersistentState("dashboard.range", "7d")` 持久化，内部调用 `computeRange`；`refresh()` 让 `refreshToken` 自增，面板以它作为 effect 依赖。

- [ ] **Step 1: Write the failing tests** — `web/src/lib/dashboard.test.ts`：
  - `deltaPct(0,0)===null`；`deltaPct(5,0)===null`；`deltaPct(112,100)` ≈ 0.12。
  - `deltaTone("cost",0.2)==="bad"`；`deltaTone("successRate",-0.003)==="bad"`；`deltaTone("latency",-210)==="good"`；`deltaTone("requests",0.1)==="good"`；`deltaTone("requests",null)==="neutral"`。
  - `cacheHitRate({input:60,output:5,cacheRead:40,cacheWrite:0})===0.4`；全为 0 时返回 null。
  - `foldTopModels`：7 个模型、n=5 时，`keys.length===6` 且最后一个为 `"__other__"`；每个时间点各 key 的值之和等于该时间点原始总量；metric 换成 requests 时排名随之改变。
  - `heatmapMatrix([{weekday:6,hour:23,count:4}])[6][23]===4`，其余格子为 0，矩阵尺寸 7×24。

- [ ] **Step 2: Run tests to verify they fail**
  Run: `cd web && rtk vitest run src/lib/dashboard.test.ts`
  Expected: FAIL，报模块不存在。

- [ ] **Step 3: Implement** — 只写纯函数和 hook，不涉及 UI。

- [ ] **Step 4: Run tests to verify they pass**
  Run: `cd web && rtk vitest run src/lib/dashboard.test.ts && npx tsc --noEmit`
  Expected: PASS。

- [ ] **Step 5: Commit** — `feat(web): dashboard 数据层与纯函数`

### Task 6: 前端 KPI 行 + 中间行面板（用量趋势 / 可靠性 / 延迟）

**Files:**
- Create: `web/src/components/dashboard/panel.tsx`（通用面板壳：标题、右侧 slot、loading skeleton、error 加重试、empty 态）
- Create: `web/src/components/dashboard/kpi-card.tsx`
- Create: `web/src/components/dashboard/kpi-row.tsx`（调用 `fetchDashboardOverview`，通过 render prop 或 context 把 overview 共享给可靠性和延迟面板，避免重复请求）
- Create: `web/src/components/dashboard/usage-trend-panel.tsx`
- Create: `web/src/components/dashboard/reliability-panel.tsx`
- Create: `web/src/components/dashboard/latency-panel.tsx`
- Create: `web/src/components/dashboard/overview-context.tsx`（`OverviewProvider` 加 `useOverview(): { data?: DashboardOverview; loading: boolean; error: boolean; retry(): void }`）
- Modify: `web/src/locales/{zh,en,ja}.json`（新增 `dashboard.kpi.*`、`dashboard.panel.*` 文案）

**Interfaces:**
- Consumes：Task 5 的 `useDashboardRange`、`lib/dashboard.ts` 的全部函数、`api.fetchDashboard*`。
- Produces：`<OverviewProvider range={…}>`、`<KpiRow/>`、`<UsageTrendPanel range/>`、`<ReliabilityPanel range/>`、`<LatencyPanel/>`。Task 7 的页面直接拼装这些组件。

- [ ] **Step 1: Implement KPI 卡片**，6 张卡的内容严格按 spec §3 表格：
  - 费用卡的辅助值为「日均 · 每千次请求成本 · 已计价调用占比」，没有月预估。
  - 迷你趋势线用 recharts `LineChart`（无坐标轴），本期为实线，上一周期为 `opacity .35` 的虚线，取值都来自 overview.series。
  - 环比徽标颜色按 `deltaTone` 映射：good → 青绿 chart token，bad → `--primary`/coral，neutral → `muted-foreground`。
  - 主值用 `MetricNumber`。
- [ ] **Step 2: Implement 三块中间面板**，规格见 spec §3「中间行」：
  - 用量趋势用 `foldTopModels`，三档指标切换使用 `toggle-group`，切换时不重新请求；上一周期总量虚线来自 overview.previous.series（按索引对齐本期桶）。
  - 可靠性：堆叠柱为 rateLimit / server / connection / other，叠加成功率线；底部一行调用 `fetchDashboardErrors({limit:1})`，文案超长时截断并配 Tooltip。
  - 延迟：P50、P95、P99 三条线，P50–P95 之间填充；底部显示输出 tok/s 和平均流式时长。
  - 动效统一走 `useChartMotion()`。
- [ ] **Step 3: Verify**
  Run: `cd web && npm run lint && npx tsc --noEmit && npm run format`
  Expected: 0 error，`truncate-requires-tooltip` 0 违规。
- [ ] **Step 4: Commit** — `feat(web): dashboard KPI 行与趋势/可靠性/延迟面板`

### Task 7: 前端底部行 + 页面拼装

**Files:**
- Create: `web/src/components/dashboard/endpoint-health-panel.tsx`
- Create: `web/src/components/dashboard/top-ranking-panel.tsx`
- Create: `web/src/components/dashboard/activity-panel.tsx`（热力图 + 客户端分布条，后者调用 `fetchDashboardTop({dimension:"harness", limit:3})`）
- Modify: `web/src/app/(dashboard)/page.tsx`（整体重写）
- Modify: `web/src/locales/{zh,en,ja}.json`

**Interfaces:**
- Consumes：Task 5 和 Task 6 的全部 Produces。

- [ ] **Step 1: Implement 三块底部面板**
  - Endpoint 表的圆点按 errorRate 着色：< 0.01 正常，[0.01, 0.05) 警示，≥ 0.05 异常。
  - Top 表的维度页签：`isAdmin()` 显示 4 个；demo 显示 4 个（服务端已脱敏）；普通 user 不显示「用户」。
  - 热力图 tz 取 `Intl.DateTimeFormat().resolvedOptions().timeZone`，格子透明度按 `count/max` 线性映射到 `--primary`。
- [ ] **Step 2: 重写 page.tsx**
  - 页头：标题、区间与对比文案、`TimeRangePicker`、刷新按钮。
  - 下面依次是 `OverviewProvider` 包裹的 KPI 行（`grid-cols-2 md:grid-cols-3 xl:grid-cols-6`）、中间行（`xl:grid-cols-[5fr_3fr_3fr]`）、底部行（`xl:grid-cols-[4fr_4fr_3fr]`）。
  - xl 及以上用 `xl:h-[calc(100dvh-<layout 顶部与内边距>)]` 和 `grid-rows-[auto_minmax(0,1fr)_minmax(0,1fr)]` 锁一屏；具体偏移量去 `app/(dashboard)/layout.tsx` 的 main 内边距实测。面板内部用 `min-h-0` 加 `ResponsiveContainer height="100%"` 撑满。
  - 删除旧的 StatCard 和 8 个图表的 import。
  - 保留 `PermissionGuard module="dashboard"`。
- [ ] **Step 3: Verify**
  Run: `cd web && npm run lint && npx tsc --noEmit && npm run test && npm run format`
  Expected: 全部通过。
- [ ] **Step 4: Commit** — `feat(web): dashboard 底部面板与一屏页面拼装`

### Task 8: 删除旧统计接口与旧图表，迁移测试和文档

**Files:**
- Delete（后端）：
  - `internal/application/audit/query/{model_trend,harness_trend,request_rate,token_throughput,token_rate,token_usage,first_token_latency,model_cost}.go`
  - `port/fill_series.go` 里的 `Fill*` 系列函数（`BuildBuckets` / `bucketStep` / `truncateBucket` 被 Task 3 使用，保留；`IndexSeries` 没人用了就删）
  - `AuditRepository` 的 6 个 `Query*` 方法及其 loader 和专属常量
  - `dto/audit_stats.go` 里的旧 Req/Rsp
  - `AuditService` / `AuditHandler` 的 8 个方法
  - `router/audit.go` 里的 8 个注册
  - `bootstrap/modules/application.go` 里相关的 wrapper 和 Provide
- Delete（前端）：
  - `web/src/components/charts/{model-cost-bar-chart,model-token-bar-chart,model-trend-chart,harness-trend-chart,request-rate-chart,token-volume-chart,token-rate-chart,first-token-latency-chart}.tsx`
  - 删完再 grep `line-chart-card`、`stacked-ratio-bar`、`lib/token-volume.ts`，没有引用的一并删除
  - `api-client.ts` 的 8 个 fetch 方法和 `types.ts` 中对应类型
- Modify（测试）：
  - `test/unit/audit_query/audit_query_test.go`：删掉 Fill*、Dispatch、AggregateModelUsage、ByUserMetrics 相关用例，保留 List/Option 用例
  - `test/unit/audit_query/demo_dispatch_test.go`：只删统计相关断言
  - 删除 `test/unit/model_cost/`、`test/unit/agent_harness/audit_trend_test.go`、`test/unit/audit_repo/audit_metrics_scope_test.go`、`test/unit/audit_repo/audit_admin_nil_scope_test.go`
  - `test/unit/read_cache_window/read_cache_window_test.go`：删掉 `TestAuditStatsCacheHits`（等价覆盖已在 Task 1 的 `TestDashboardStats_CacheHit`）
  - `web/src/components/shadcn-native.test.ts`：删掉指向已删文件的断言（L103-104、L156、L276 附近）
- Modify（e2e）：
  - `test/e2e/empty_key_scope/empty_key_scope_test.go`：路径列表换成 6 个新路由（top/list 带 `dimension=model`，heatmap 带 `tz=UTC`）；断言改为 overview 的 `current.summary.requests==0`，其余接口 data 为空
  - `test/e2e/demo/demo_account_test.go:345`：改为请求 overview
  - `test/e2e/model_pricing/pricing_test.go:140`：改为 `top/list?dimension=model`，断言对应模型的 cost
  - `test/e2e/agent_harness/agent_harness_test.go:205`：改为 `top/list?dimension=harness`，断言 harness 名出现在 items 中
- Modify（文档）：`docs/agents/go-backend.md` L105 的 Audit 路由一览改成新的 6 条。

- [ ] **Step 1: 确认旧接口没有其他调用方** — 用 CodeGraph `codegraph_callers` 逐个确认 `HandleModelTrend` 等 8 个 handler 方法和 6 个 repo 方法；再执行 `rtk grep "stats/(model|harness|request|token)/" web/src internal test docs`，确认只剩本任务要改的位置。
- [ ] **Step 2: 删除并迁移**（按上面的 Files 列表）。
- [ ] **Step 3: Verify**
  Run: `rtk go build ./... && rtk go vet ./... && rtk go test ./... && go run ./cmd/lint conv ./... && cd web && npm run lint && npm run test && npx tsc --noEmit && npm run format:check`
  Expected: 全部通过；e2e 包能编译（`go vet ./test/e2e/...` 通过即可，运行需要 BASE_URL）。
- [ ] **Step 4: Commit** — `refactor(audit): 移除旧统计接口与旧 dashboard 图表`

### Task 9: 端到端验证（一屏、主题、SQL 真实执行）

**Files:**
- 无产品代码改动。临时 harness 放 `/tmp/dashboard-verify/`，不进仓库。

- [ ] **Step 1: 生产库只读验证 SQL** — 加载 `query-prod-database` skill。从 Task 1–2 的 DryRun 测试里打印出 7 条 SQL（nil scope、30d、day 粒度、limit 5、tz `Asia/Shanghai`），逐条执行 `EXPLAIN ANALYZE`。要求全部能执行，每条 Execution Time < 300ms。结果记入 PR 描述。超标就停下来报告，不要自行加索引（建索引需要用户授权）。
- [ ] **Step 2: 一屏与主题验证**
  - 构建：`cd web && npx next build --webpack`（worktree 软链 node_modules 时必须用 webpack）。
  - 起本地后端，连接本地或测试库，数据可以用 sqlite 以外的 PG（找不到时退而用 mock API：把 6 个接口的固定 JSON 用 python http.server 托管，并改 `out/` 的 API 前缀）。
  - 用 chrome-headless-shell，在 1440×900 和 1920×1080 下分别注入 `localStorage.theme = anthropic|moonshot`，共 4 组。用 `--dump-dom` 配合注入脚本输出 `scrollingElement.scrollHeight <= innerHeight` 的结果，以及所有 `[data-slot=card]` 内文本的 `scrollWidth > clientWidth` 计数。
  - 期望：4 组都是 `fits=true`、`overflow=0`。不满足就回到 Task 7 调整间距或字号。
- [ ] **Step 3: 口径对账** — 同一个 7d 区间，overview 的 `current.summary.requests` 必须等于 `GET /audit/model/log/list` 同时间范围的 `pageInfo.total`，success 必须等于 `filter=status:200` 时的 total。
- [ ] **Step 4: 清理** — 删除 `web/.next`、`web/out` 和 `.worktrees/dashboard-redesign/internal/web/dist` 的占位文件（不要删目录）。按项目规范用 `mcp__serena__write_memory` 沉淀本次的工程经验。
- [ ] **Step 5: Commit**（如果 Step 2 有调整）— `fix(web): dashboard 一屏布局微调`
