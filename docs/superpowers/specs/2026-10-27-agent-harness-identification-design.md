# Agent Harness 识别：可运行时维护的 UA 规则 + 会话筛选 + 调用量走势

- 日期：2026-10-27
- 状态：待实施
- 相关：Session、ModelCallAudit、Filter DSL、Dashboard 图表

## 1. 背景

平台同时被多种 Agent Harness 调用。生产审计表（2026-09-27 ~ 2026-10-27，15,519 次调用）的实际分布：

| UA 首段 | 调用数 | 占比 |
|---|---|---|
| `deepseek-harness/0.2.1-rc.x` | 7,031 | 45.3% |
| `ZCode/3.14.x–3.16.x ai-sdk/...` | 5,392 | 34.7% |
| `pi (darwin …; x64/arm64)` | 2,822 | 18.2% |
| `OpenAI/Python 2.24.0` | 236 | 1.5% |
| 浏览器 / `curl` / `node` / `WorkBuddy` / `Codex Desktop` / 压测 | 27 | 0.2% |

当前无法回答两个问题：会话列表里这条会话来自哪个 harness、某段时间各 harness 的调用占比。

**现状约束**：

- `sessions` 表没有任何客户端身份字段。`Session.metadata` 是客户端在请求体里自报的 `metadata`（OpenAI Response / Anthropic），大多为空且客户端可控，不能作为来源依据。
- `model_call_audits.user_agent` 存了全量调用的原始 UA，且历史完整（30 天前也在）。
- 会话与审计之间没有关联键，无法事后 JOIN 出会话的来源。

## 2. 目标

1. 会话列表展示来源 harness，并可按 harness 筛选（沿用现有 Filter DSL）。
2. Dashboard 增加一张「按 harness 的调用量走势」图（时间桶 × harness × 调用次数）。
3. **新增 harness 不需要发版**；**改规则后历史数据自动重标，不做任何回填**。
4. 顺带修复 demo 视角的会话筛选失效（否则 demo 上的 harness 筛选必然是坏功能），见 D6。

## 3. 非目标

- 不做按 harness 的计费、配额、限流或鉴权（UA 可伪造，不是可信身份）。
- 不改 `model_call_audits` 表结构，不做历史回填。
- 不做「会话 ↔ 审计」的精确关联。
- 不给公开分享页（`/share`）暴露 harness 或原始 UA。

## 4. 核心设计

### D1：原始 UA 是事实，harness 是派生物（不落库）

`sessions` 新增原始 UA 列 `client_ua`，**写入时机是会话创建那一刻**，存原样。**不新增** `agent_harness` 列。

语义明确为**会话起源**：会话合并（`DeduplicateSessionGroup` / `writeBackSnapshot`）**不**把 `client_ua` 加进前滚列表。一段对话前半段 DSH、后半段换 ZCode，整段会话始终显示 DSH。代价是上线前创建的 5,702 个存量会话永远是 `Other`（续聊也不会自愈）——这是刻意选择：归因稳定，不随最近一次请求漂移。

harness 在每次查询/展示时由「规则表 + `client_ua`」现算：

- 会话列表 / 会话详情展示：Go 侧对 `client_ua` 求值，随响应返回 `agentHarness`。
- 会话筛选：由同一份规则生成 SQL 条件（见 D3）。
- 走势图：审计表上按规则生成 `CASE` 分组。

这条是「改规则历史自动重标」的全部机制：只要原始 UA 还在，规则一改，下一次查询就把旧会话从 `Other` 里认出来。**任何形式的 `agent_harness` 落库都会破坏这个性质**，因此明确禁止。

### D2：规则存库，运行时可维护

新表 `agent_harness_rules`：

| 列 | 类型 | 说明 |
|---|---|---|
| `id` | uint PK | |
| `pattern` | text not null, unique | 小写化的**前缀**，如 `deepseek-harness/` |
| `name` | text not null | 展示名，如 `DSH`；同名多行表示同一 harness 的多种 UA |
| `sort` | int not null default 0 | 升序，**先命中者优先** |
| `created_at` / `updated_at` | time | |

匹配语义（唯一权威定义）：

1. `lower(ua)` 与 `pattern` 做**前缀**比较（`HasPrefix`），pattern 建行时统一转小写。
2. 按 `sort ASC, id ASC` 顺序取**第一条**命中。
3. 命中不到或 UA 为空 → `Other`（常量，不建行、不可编辑）。
4. 硬删除（`Unscoped`），删除即回到 `Other`。不用软删除：唯一索引 + 软删除会让「重建同名规则」被存量行挡住。

**不缓存**：规则表只有个位数到几十行，每次请求读一次（`SELECT ... ORDER BY sort, id`），代价是一次小表查询，换来多副本部署下无失效问题、改完立即全量生效。升级路径（写进注释）：量级变大时加 Redis 30s TTL，参照 `auth:apikey` 守卫缓存的模式。

种子数据：迁移时若表为空，写入 6 条（`deepseek-harness/`→`DSH`、`zcode/`→`ZCode`、`pi (`→`Pi`、`codex_cli_rs/`→`Codex`、`Codex Desktop/`→`Codex`、`claude-cli/`→`Claude Code`、`opencode/`→`OpenCode`）。种子只插一次，之后完全由管理员维护。

### D3：一份规则，三个消费方

| 消费方 | 实现 | 说明 |
|---|---|---|
| 会话展示 | Go `Match(ua, rules)` | 纯函数，单测覆盖 |
| 会话筛选 | SQL 标量子查询：UA → 首命中 `name`，与请求值比较 | `Other` 用 `IS NULL`，空 UA 天然落入 |
| 走势图 | 同一个标量子查询 + `COALESCE(..., 'Other')` 作为分组键 | `GROUP BY harness, 时间桶` |

会话筛选条件（`harness:ZCode` / `harness:Other`）：

```sql
-- 具体 harness
(SELECT r.name FROM agent_harness_rules r
  WHERE lower(s.client_ua) LIKE r.pattern || '%'
  ORDER BY r.sort, r.id LIMIT 1) = $1
-- Other
(SELECT r.name FROM agent_harness_rules r
  WHERE lower(s.client_ua) LIKE r.pattern || '%'
  ORDER BY r.sort, r.id LIMIT 1) IS NULL
```

走势图分组键：

```sql
COALESCE((SELECT r.name FROM agent_harness_rules r
  WHERE lower(a.user_agent) LIKE r.pattern || '%'
  ORDER BY r.sort, r.id LIMIT 1), 'Other')
```

要点：

- 规则是**库里的数据**，直接当数据用：不生成 `CASE`、不拼接 SQL、不需要转义，新增规则不用重新编译任何表达式。
- 子查询与 Go `Match` 完全同语义（`sort, id` 升序首命中），不会出现「展示 ZCode、筛 ZCode 查不到」。
- `LIKE` 的通配符风险由**写入校验**兜住（`pattern` 拒绝 `%` / `_`），因此 `pattern || '%'` 的 `%` 是唯一的通配符。
- 代价是每行一次小表嵌套查询：`sessions` 5,702 行、规则十几行、仅在用到该 facet 时执行；审计表 30 天 1.5 万行同理，可忽略。

**框架改动**：现有 `filter.FieldConfig` 只能表达「列/表达式 + 操作符 + 值」，表达不了「值决定子查询」。给 `FieldConfig` 增加可选钩子

```go
Custom func(f Filter) (sql string, args []any, err error)
```

在 `buildCondition` 最前面短路，`harness` 用该钩子；`score` / `model` / `messageCount` 三个既有字段一行不改。

### D4：展示与统计口径

- 展示：`Other` 对用户可见，命中不到就是 `Other`，不区分「SDK 直连 / 浏览器 / 压测」。
- 统计：走势图统计**全部调用**（含失败），与 `RequestRate` 口径一致；不使用 `ModelTrend` 的「仅 upstream 成功」口径，因为占比要反映真实流量。
- 空 `client_ua`（存量会话）→ `Other`，详情页 tooltip 说明「该会话创建于功能上线前或该轮请求未携带 UA」。

### D5：走势图沿用读缓存，但缓存键带上规则指纹

`/audit/stats/*` 全部走 `cachedSlice`（TTL 5min + 0~1min 抖动）。走势图**保留缓存**（与其余 7 张图一致，dashboard 首屏不打满数据库），但缓存键在 `readCacheKey(...)` 原有成分之外追加**规则指纹**——`sha256(pattern|name|sort 有序拼接)[:8]`。

于是：改规则 → 下一次查询的键不同 → 直接未命中回源，图例与分组立刻反映新规则，旧键自然过期。「改规则立即生效」在图表上同样成立，且不需要跨模块失效通道，多副本下天然一致。

规则本来就要读（用来生成 `CASE`），指纹是顺手算的，没有额外查询。

### D6：本期一并修复 demo 视角的会话筛选

现状是既有缺陷：`jwt_session_queries.go` 的 demo 分支调用 `ListSessionsByIDs(ids, param)`，不接 `criteria` / `keyword` / `startTime` / `endTime`，所以 demo 用户选评分、模型、消息数或改时间范围全都无效。harness facet 会原样继承这个坑。

本期一起修，做法是把 `ListSessionsByOwnerIDs` 里「keyword + 时间范围 + filter criteria」的 WHERE 构造抽成共用 helper，`ListSessionsByOwnerIDs`（按 owner scope）与 `ListSessionsByIDs`（按抽样 ID scope）都套用：

- demo 的抽样口径不变（`id % K == 0` 先定 ID 集合），只是在集合之上再叠加筛选条件；
- 产品层面的可感知变化：demo 的评分/模型/消息数/时间范围/关键词筛选**开始真正生效**，属于预期内的修复，需要在发布说明里点明；
- 不改抽样语义，也不放宽 demo 的数据可见范围（筛选只会缩小结果）。

## 5. 数据模型变更

1. `sessions.client_ua`：`text not null default ''`，AutoMigrate 加列，**不回填**。
   - 同步更新 `constant.SessionRepoFields*` 中需要的投影（列表、详情、read 路径），漏列会读出空串。
   - **不加入**合并前滚字段列表（`writeBackSnapshot` 只前滚 message_ids / questions / model_ids / metadata），保持「首轮 UA」语义。
2. `agent_harness_rules`：新模型，进 `model.Models`，随 AutoMigrate 建表；唯一索引 `idx_harness_rules_pattern (pattern)`。

## 6. API 变更

### 6.1 会话（读）

- `SessionSummary` / `SessionDetail` / `SessionMetadata` 增加 `agentHarness string`（展示值，`Other` 兜底）。
- `clientUA` **只加在 `SessionMetadata`**（详情页 tooltip 显示原始 UA 与「无 UA」解释），列表页不带原始 UA：`Other` 在列表里只显示通用徽标，把 UA 的暴露面限制在单条详情。
  列表页若也带 UA，投影、DTO、以及「谁能在列表里看到别人的 UA」都要重新论证一遍，收益只是列表上的一个 tooltip。
- `SessionOptionListReq.Field` 的 huma 枚举补 `harness`（**历史教训：漏改 DTO 枚举会 422，应用层永远走不到**）。
- `sessionFieldConfigs` 增加 `harness` 字段；`option_list` 返回规则里的 `name` 去重列表 + `Other`。
- demo 视角的筛选依赖 D6 的修复才真正生效；D6 落地前不得把 `harness` facet 只加到 demo 可达的 UI 上而不修后端。

### 6.2 规则管理（admin only）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/web/v1/harness/rule/list` | 全部规则（按 sort, id） |
| POST | `/api/web/v1/harness/rule` | 新增（pattern 规范化 + 唯一性校验） |
| PATCH | `/api/web/v1/harness/rule` | 改 pattern / name / sort |
| DELETE | `/api/web/v1/harness/rule` | 硬删除 |
| POST | `/api/web/v1/harness/rule/preview` | 粘贴一条 UA，返回命中的规则与最终 harness |

校验：`pattern` 去空白后转小写、非空、长度 ≤ 200、不允许 `%`/`_`（它们在前缀语义下没有意义，直接拒绝比转义更难误用）；`name` 非空、≤ 64 字符、不等于保留字 `Other`（大小写不敏感）。

### 6.3 走势图

`GET /api/web/v1/audit/stats/harness/trend?startTime&endTime&granularity`

响应沿用 `ModelTrend` 的形状：`series: [{harness, points: [{time, count}]}]`，按 harness 名排序（`Other` 放最后），时间桶按 `granularity`（minute/hour/day/week）零值补齐。

权限与作用域：与 `queryModelTrend` 完全一致（`LimitUserPermissionWithDemoMiddleware`，user 只看自己 Key 的调用，demo 走 audit 模块白名单 + 抽样口径）。

## 7. 前端变更

1. **会话列表**：新增窄列「Agent」展示 harness 徽标（`Other` 用 muted 样式）；移动端卡片同样展示。`FilterBar` 增加 `harness` facet（异步 options 走 option list）。列表与会话详情**都要展示**（已与需求方确认）。
2. **会话详情**：头部展示 harness 徽标 + 原始 UA tooltip。
3. **Dashboard**：新增 `HarnessTrendChart` 卡片，堆叠面积图（`Other` 置底、固定灰色），图例可点选隐藏，粒度沿用现有小时/天切换。
4. **规则管理页** `/harness`（admin 导航项）：表格（pattern / name / sort / 最近更新）+ 新增/编辑/删除 + 顶部「UA 预览」输入框（粘贴一条 UA 立即显示命中规则与最终 harness，用于新增 harness 前的验证）。
5. i18n：zh/en 同步。

## 8. 权限与安全

- 规则增删改、预览：`PermissionAdmin`。
- 规则列表：admin（普通用户不需要看到规则表）。
- harness / clientUA 展示：随会话权限（demo 在抽样范围内可见）。
- UA 不可信：只用于统计与展示，永不参与鉴权、计费、路由。

## 9. 风险与兼容

| 风险 | 处理 |
|---|---|
| 筛选条件无法走索引 | 已知接受：`sessions` 5,702 行，访问模式是「按 API Key + 时间范围翻页」；升级路径是加回填列 |
| 存量会话无 UA | 显示 `Other` + tooltip 说明 |
| 规则配错（如 `z` 前缀吞掉一切） | 预览接口 + 列表页可即时验证；规则按 sort 排序，排错可调 |
| 前端/后端规则不一致 | 只有一份规则，SQL 与 Go 匹配器同语义，均由规则表驱动 |
| 审计 UA 脏数据（生产有 13 行被截断覆盖） | 归 `Other`，不影响统计正确性 |
| demo 筛选开始生效（D6 的副作用） | 属修复但产品可感知：demo 用户会突然看到筛选/时间范围/关键词起作用；发布说明点明，E2E 覆盖 |

## 10. 测试策略

- 单测：
  - `Match`：命中/未命中、大小写、排序优先级、空 UA、同 name 多 pattern。
  - 规则校验：pattern 非法字符、`Other` 保留名、重复 pattern。
  - SQL 片段生成：参数个数、转义、`Other` 条件等价性（「全部规则都不命中」）。
  - 会话响应：`agentHarness` 注入与 `Other` 兜底。
  - demo 列表筛选：`criteria` + keyword + 时间范围在 ID scope 下生成的 SQL 与非 demo 路径等价（同一个 helper）。
- E2E（`test/e2e/agent_harness/`）：
  1. 新增规则 → 带该 UA 发一次模型调用 → 会话列表返回对应 harness 且可按其筛选。
  2. 修改规则（改 name 或加新 pattern）→ **不重启、不迁移**，同一会话的 harness 立即变化（历史自动重标的核心断言）。
  3. 走势图按 harness 分组返回时间序列，含 `Other`；改规则后同一时间范围的分组立即变化（验证缓存键指纹）。
  4. 普通用户访问规则管理接口 → 403。
- E2E（`test/e2e/session_demo_filter/`）：
  5. demo 账户按 score / model / messageCount / harness 筛选、改时间范围、关键词搜索，结果集与筛选条件一致（当前会被忽略）。
  6. 抽样口径不变：筛选后的结果仍全部落在 `id % K == 0` 的集合内。

## 11. 决策记录

**需求方拍板**：

- 会话的 harness 取**首轮**（会话起源），不随最近一次请求漂移；合并时不前滚 `client_ua`。
- demo 视角筛选**本期一并修复**（D6），接受它带来的产品可感知变化。
- harness 在会话列表与会话详情**都展示**。

**实现者默认（可推翻）**：

- 规则匹配用**前缀**而非子串：真实 33 条 UA 全部可用首段前缀区分，子串会带来误命中。
- `Other` 是常量而非可编辑规则行：避免「规则名被占用/被删除」的边界。
- 规则不缓存：正确性与运维简单性优先于一次小表查询。
- 走势图统计全部调用（含失败）。
- 走势图沿用 5 分钟读缓存 + 规则指纹（D5），而不是关掉缓存。
- 原始 UA 只出现在详情页 DTO（D4/§6.1）。
- 规则管理做成独立 admin 页而非塞进会话页弹窗：新增 harness 是低频但需要「预览 + 排序」的运维动作。

## 12. 需同步的文档

- `CONTEXT.md` 增补词条：**AgentHarness**（派生值 + `Other`）、**HarnessRule**（有序前缀规则）。
