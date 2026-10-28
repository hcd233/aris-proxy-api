# Dashboard 重设计：一屏指挥台

日期：2026-10-28
状态：待用户审阅
可视化稿：`.superpowers/brainstorm/59206-1793178348/content/layout-a-dense.html`（「A 加密版」，用户选定）

## 1. 为什么做

现在的 dashboard（`web/src/app/(dashboard)/page.tsx`）有三个问题，原话是「很丑、信息量很低、图表还要上下滑动才能看完」：

- 顶部 4 张卡片只是资源计数（API Key 数、会话数、端点数、模型数），对日常判断几乎没有帮助。
- 下面 8 张图两列平铺，没有主次，必须滚动；每张图各自带一个时间范围选择器，彼此不联动。
- `model_call_audit` 里已有的状态码、错误信息、缓存 token、首字延迟、流式时长、endpoint、User-Agent 等字段基本没被利用。

用户打开 dashboard 时，三个问题都想一眼得到答案：**花了多少钱、服务稳不稳、谁在用/用了什么**。

## 2. 目标 / 非目标 / 反目标

目标

- 1440×900 视口下一屏看完全部面板（侧栏、顶栏占位按现有 layout 计算），无纵向滚动。
- 信息密度对齐可视化稿：6 个 KPI 卡片，每张卡有主值、环比、2-3 个辅助值和迷你趋势线；表格类面板每行有 5-6 列指标。
- 全页只有一个时间范围选择器，所有面板联动，环比固定对比「上一个等长周期」。
- anthropic 与 moonshot 两套主题下都成立，只用现有主题 token（`--primary`、chart 色板、卡片样式），不新造主题。
- 后端补齐新面板所需的聚合接口，沿用现有的权限分流和数据隔离语义。

非目标

- 不做「缓存帮你省了多少钱」（用户明确不需要）。
- 不做「月预估」或任何外推/预测值（用户不按月盘算成本）。
- 不做告警、阈值配置或自定义面板拖拽。
- 不改 `/monitor` 页（运行时指标）。
- 不做多币种换算。

反目标（做成这样就算失败，哪怕功能都在）

- 1440×900 下还要滚动。
- 环比、占比、成功率口径和审计列表页对不上，用户看到两个互相矛盾的数。
- 普通用户能看到别人的数据，demo 用户能看到未脱敏的用户名或 Key 名。
- 某个面板加载失败导致整页白屏或整页报错。

## 3. 页面布局（已定）

从上到下共四段（对应可视化稿）：

**页头**：标题「概览」，右侧显示当前区间及对比区间（如 `10-21 → 10-28 · 对比上一 7 天`）、时间范围选择器（1h / 24h / 7d / 30d / 自定义），以及手动刷新按钮。时间范围默认 7d，选择结果存 localStorage（沿用 `usePersistentState`）。

**KPI 行（6 等分）**，每张卡：标题、环比徽标、主值、辅助值行、迷你趋势线（实线为本期，淡虚线为上一周期）。

| 卡片 | 主值 | 环比 | 辅助值 |
|---|---|---|---|
| 请求 | 总请求数 | % | 日均请求数 · 峰值（单个时间桶内的最大请求数，按当前粒度换算成 rpm/rph 展示） |
| Token | 总 token | % | 输入 · 输出 · 平均每请求 token |
| 费用 | 已计价总费用 | % | 日均 · 每千次请求成本 · 已计价调用占比 |
| 成功率 | 成功率 | 百分点差 | 失败数 · 429 占比 · 5xx 占比 |
| 缓存命中率 | 命中率 | 百分点差 | 缓存读 · 缓存写 · 未命中 token |
| 首字延迟 | P95 | 毫秒差 | P50 · 输出速度 tok/s |

环比颜色按「好坏」而不是按「涨跌」：费用上涨、成功率下降、延迟上升显示为警示色（coral），其余显示为正向色（青绿）。请求数与 Token 的涨跌视为中性偏正向。

**中间行（5 : 3 : 3）**

1. 用量趋势：按模型堆叠的面积图，可切换 费用 / 请求 / Token，叠加上一周期总量虚线；只展示 Top 5 模型，其余合并为「其他」；悬停显示该时间桶的总量和各模型拆分。
2. 可靠性：失败请求按原因堆叠的柱状图（429 / 5xx / 连接失败 / 其他），叠加成功率折线；底部一行显示出现次数最多的错误信息及次数。
3. 延迟：首字延迟 P50 / P95 / P99 三条线（P50–P95 之间填充），底部显示输出速度和平均流式时长。

**底部行（4 : 4 : 3）**

1. Endpoint 健康表：Endpoint · 请求 · 错误率 · P95 首字 · 输出 tok/s · 费用。行首圆点按错误率着色（< 1% 正常，1–5% 警示，≥ 5% 异常），按请求数降序，最多显示 5 行。
2. Top 排行表：维度可切换 模型 / 用户 / API Key / 客户端，列为 名称 · 请求 · Token · 缓存命中率 · 费用 · 费用占比（含内联比例条），最多 5 行。
3. 活跃时段热力图（星期 × 小时，7×24，按浏览器本地时区）+ 客户端分布条（Top 3 客户端 + 其他）。

响应式：小于 `lg` 时按段落纵向堆叠，允许滚动；「一屏」约束只对 ≥ 1440 宽的桌面视口生效。

## 4. 统计口径（前后端统一）

- 统计对象：`model_call_audit` 中 `deleted_at = 0` 且 `created_at ∈ [startTime, endTime]` 的记录。
- 成功：`upstream_status_code = 200`。成功率 = 成功数 / 总数。
- 失败原因分类：429 →「限流」；500–599 →「5xx」；-1 →「连接失败」；其余非 200（含 0、4xx 非 429）→「其他」。
- Token：
  - `input_tokens` 已归一化为净输入（见 `recorder.go`）。
  - 总输入 = input + cache_creation + cache_read；总 token = 总输入 + output。
  - 缓存命中率 = cache_read / 总输入（分母为 0 时返回 null，前端显示「—」）。
  - Token 类指标只统计成功请求，与现有 `QueryTokenThroughput` 口径一致。
- 费用：只汇总 `cost_micro IS NOT NULL` 的记录，按 `pricing_currency` 分组返回；KPI 主值显示费用最高的币种，其他币种放在 tooltip 里。已计价调用占比 = 已计价成功调用数 / 成功调用数。
- 延迟：只统计成功请求的 `first_token_latency_ms`，用 PostgreSQL `percentile_cont` 计算 P50 / P95 / P99。
- 输出速度：sum(output_tokens) / sum(stream_duration_ms) × 1000，只统计 `stream_duration_ms > 0` 的成功请求。注意这和现有 `loadTokenThroughput` 不同：现有 SQL 没过滤非流式行，非流式请求的输出 token 进了分子但时长为 0，速度被高估。新接口采用更准确的口径；旧接口随本次删除，不存在两套口径并存。
- 客户端：沿用 `constant.AuditHarnessGroupSQL`（UA → 最长前缀规则名，未命中归 Other），缓存键带规则指纹。
- 上一周期：`[startTime - (endTime - startTime), startTime)`，时间桶粒度和本期相同。
- 时间桶粒度由前端按区间长度决定，和现有图表一致：1h → minute，24h → hour，7d / 30d → day（自定义区间沿用 `lib/time-range` 现有规则）。

## 5. 权限与数据隔离

沿用审计统计现有的三路分流（`auditService` 里的 `switch permission`），不新增权限模型：

- Admin：全量数据（`apiKeyIDs = nil`）。Top 排行四个维度都可用。
- Demo：全量聚合数据（与现有统计接口一致）。Top 排行的「用户」和「API Key」维度返回脱敏名称（`commonutil.MaskIdentity` / `MaskSecret`，与审计列表一致）；热门错误信息原样返回（审计列表对 demo 也不脱敏 error_message，实现时核对，若列表有脱敏则保持一致）。
- User：只统计自己名下 API Key 的数据。名下没有 Key 时，handler 层短路返回空结果，仓储层保留 `apiKeyIDs != nil && len == 0` 守卫（「宁漏勿越」，见既有安全修复）。不支持「用户」维度：请求该维度时返回 422，前端对 user 角色隐藏该页签。
- 路由挂在现有 `auditGroup` 下，使用 `LimitUserPermissionWithDemoMiddleware(..., enum.PermissionUser, enum.DemoModuleAudit, ...)`，与旧统计接口一致。
- 前端 `PermissionGuard module="dashboard"` 保持不变。

## 6. 后端接口

前缀 `/api/web/v1/audit/stats/dashboard`。所有接口都接收 `startTime`、`endTime`（RFC3339，必填），需要时间桶的再加 `granularity`（minute / hour / day / week）。每个面板一个接口，好处是面板可以各自加载、各自失败重试，也能各自缓存。

| 接口 | 用途 | 关键返回字段 |
|---|---|---|
| `GET /summary` | KPI 主值、辅助值与环比 | `current`、`previous` 两个同构对象：requests、success、failures{rateLimit, server, connection, other}、tokens{input, output, cacheRead, cacheWrite}、cost[{currency, amount}]、pricedSuccess、latency{p50, p95, p99}、outputTokensPerSecond、avgStreamDurationMs、peakBucketRequests |
| `GET /series` | KPI 迷你趋势线、可靠性柱图、延迟线 | `current`、`previous` 两组时间桶数组，每桶：time、requests、success、failures{4 类}、tokens（总）、cost（主币种）、cacheHitRate、latency{p50, p95, p99} |
| `GET /model-series?metric=cost\|requests\|tokens` | 用量趋势堆叠图 | Top 5 模型 + `Other` 的时间序列，以及上一周期总量序列 |
| `GET /errors?limit=5` | 可靠性面板里的热门错误 | [{statusCode, message, count}]；message 截断到 200 字符后再分组 |
| `GET /endpoints?limit=5` | Endpoint 健康表 | [{endpoint, requests, errorRate, p95LatencyMs, outputTokensPerSecond, cost[]}] |
| `GET /top?dimension=model\|user\|apikey\|harness&limit=5` | Top 排行和客户端分布条 | [{key, name, requests, tokens, cacheHitRate, cost[], costShare}]，以及 `otherShare` |
| `GET /heatmap?tz=Asia/Shanghai` | 活跃时段热力图 | 7×24 计数矩阵（weekday 0=周一） |

实现约束：

- 分层沿用现有结构：`handler/audit.go` → `application/audit/query`（按「admin/demo Handler + ByUser Handler」成对实现）→ `domain/modelcall.AuditRepository` 新方法 → `infrastructure/repository/audit_repository.go`。
- SQL 片段放在 `internal/common/constant/sql.go`（conv lint 禁止本地 const）；所有用户输入（tz、limit、dimension）都走参数绑定或白名单。
  - `tz` 先用 `time.LoadLocation` 校验，非法值返回 422，校验通过后以绑定参数传给 `AT TIME ZONE ?`。
  - `dimension` 和 `metric` 用 enum 白名单。
  - `limit` 限制在 1–10。
- 读路径沿用 `readCacheKey` + `cachedSlice` 读缓存，缓存键包含 scope、时间桶、粒度、维度；harness 相关的缓存键带规则指纹。
- `summary` 和 `series` 的本期与上一周期在服务端并发查询，只发一次 HTTP 请求。
- `user` / `apikey` 维度的名称通过现有 `BatchGetRelations` 回填，不在 SQL 里 join 业务表。
- DTO 遵守 `huma-dto-conventions` skill 与 `docs/agents/go-backend.md` 的约定（不允许 any，不导入 dbmodel，字段写全 doc 标签）。
- 性能预算：生产 audit 表约 4 万行量级（见 2026-07-27 分页基线），单接口 30d 区间 P95 < 300ms。`percentile_cont` 和 harness 子查询是成本最高的两类查询，实现后要在生产量级数据上 EXPLAIN 验证。

旧接口：8 个 `/stats/model|harness|request|token/*` 接口和对应的 handler、query、仓储方法只被旧 dashboard 图表和 e2e 测试使用。新页面上线后在同一 PR 中删除；删除前用 CodeGraph 再确认一遍没有其他调用方。e2e 测试中原本覆盖旧接口的断言迁移到新接口上，重点是 `empty_key_scope`（空 Key 不越权）、`demo`（demo 可访问且脱敏）、`model_pricing`（费用汇总）、`agent_harness`（客户端分组）这四组。

## 7. 前端结构

- `web/src/app/(dashboard)/page.tsx` 重写为页面编排：持有全局时间范围状态，按 §3 的网格摆放面板。
- 新建 `web/src/components/dashboard/`：`kpi-card.tsx`、`usage-trend-panel.tsx`、`reliability-panel.tsx`、`latency-panel.tsx`、`endpoint-health-panel.tsx`、`top-ranking-panel.tsx`、`activity-panel.tsx`，以及共享的 `use-dashboard-range.ts`（时间范围 → startTime/endTime/granularity/上一周期标签）。
- 每个面板各自请求数据，各自显示 skeleton、错误态（显示「重试」按钮）和空态；单个面板失败不影响其他面板。
- 图表继续用 recharts，色板用 `lib/theme` 现有 hook，动效遵守现有的 `useChartMotion` 和 reduced-motion 守卫；数字用现有的 `MetricNumber` 组件。
- 删除 8 个只被旧 dashboard 使用的图表组件（`model-cost-bar-chart` 等），`runtime-*` 和 `stacked-ratio-bar`（如果仍被复用）保留；同步更新 `shadcn-native.test.ts` 中对这些路径的引用。
- `api-client.ts` 删除旧的 8 个 fetch 方法，新增对应方法；`lib/types.ts` 跟随修改。
- 现有的 4 张资源计数卡片不再保留（用户确认其价值低）。
- i18n：新增的文案中英双语都要补齐；含翻译文本的 `useMemo` 依赖 `locale`，不依赖 `t`。
- 截断文案必须配 Tooltip（项目 lint 规则 `truncate-requires-tooltip`）。

## 8. 验证

- 后端：
  - 每个新 query handler 都要有单测，覆盖三路权限分流以及名下无 Key 时的短路。
  - 仓储层的 PG 专属 SQL 沿用双向守卫测试法：`nil` 必须落到 SQL 执行，空切片必须短路。
  - 在 e2e 中迁移并补齐越权与 demo 脱敏断言。
- 前端：
  - `npm run lint`、`npm run format:check`、`npm run test`、`npm run build` 全部通过。
  - 用 chrome-headless-shell 在 1440×900 和 1920×1080 两档下分别加载 anthropic 和 moonshot 主题，程序化断言 `document.scrollingElement.scrollHeight <= innerHeight`，并检查无文本溢出。
- 口径对账：同一时间区间内，summary 的请求数和成功率必须与审计列表页同条件筛选下的总数一致。

## 9. 留给实现者的决定

- 具体的图表组件写法（recharts 的 Area / Bar / Composed）以及 tooltip 的样式。
- 热力图用 CSS grid 还是 SVG。
- 面板内部间距、字号的微调，以满足一屏约束为准。
- 「其他」类别的配色。

## 10. 我替用户做的决定（请审阅时确认）

1. 每个面板一个接口，而不是一个大的 bundle 接口（便于各自加载、各自失败）。
2. Token、延迟、输出速度只统计成功请求；请求数和客户端分布统计全部请求，与现有接口口径一致。
3. 普通用户请求「用户」维度返回 422，前端隐藏该页签。
4. Demo 的「用户」和「API Key」维度显示脱敏名称，不隐藏这两个维度。
5. 旧的 8 个统计接口和 8 个图表组件在同一 PR 中删除。
6. 热力图按浏览器本地时区计算，由前端传 IANA 时区名。
7. 删除原有的 4 张资源计数卡片。
