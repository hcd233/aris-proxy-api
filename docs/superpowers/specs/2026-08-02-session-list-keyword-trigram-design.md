# Session 列表 keyword 过滤性能优化（trigram 索引陷阱）— 设计文档

> 日期：2026-08-02
> 状态：已实施并上线（分支 `bugfix/session-list-keyword-perf-2026-08-02`，master `7957c46f`）
> 类型：性能缺陷修复（线上超时）

## 背景与目标

### 现象

生产环境 `GET /api/v1/session/list?page=1&pageSize=100&...&keyword=Task:` 请求超时（>30s 无响应）。不带 `keyword` 的同参数请求仅 ~1.1ms。Web 端 Sessions 页面的关键词搜索功能实际不可用。

### 目标

1. 修复 `keyword` 过滤路径的查询性能，使其恢复到与无 keyword 同量级（<1s）；
2. 保持响应结构与分页语义完全不变；
3. 防止 SQL 形态被回退（测试护栏）。

## 现状与数据规模（生产实测）

| 表 | 行数（估算） | 大小 |
|----|-------------|------|
| `sessions` | 2,486 | 19 MB |
| `messages` | 103,917 | 366 MB（`message` 为 jsonb 大字段） |
| `tools` | 965 | 1.4 MB |

关键索引：

- `sessions`：`idx_sessions_api_key_name_created_at (api_key_name, created_at)`、`idx_sessions_deleted_at_created_at (deleted_at, created_at)` — 覆盖分页排序与时间过滤；
- `messages`：`idx_messages_message_trgm`（`message gin_trgm_ops`，trigram GIN 索引）、`idx_message_checksum`、`messages_pkey`。

## 根因分析（生产 EXPLAIN ANALYZE 实测）

### 相关代码

`internal/common/constant/sql.go` 的 `SessionKeywordFilterSQL` 被 `session_repository.go` 两处使用（`ListAllSessions` 管理视角 / `ListSessionsByOwnerNames` 用户视角）：

```sql
-- 修复前
EXISTS (
  SELECT 1
  FROM jsonb_array_elements_text(sessions.questions::jsonb) AS arr(mid)
  JOIN messages ON messages.id = arr.mid::bigint
  WHERE messages.message::text ILIKE ?
)
```

语义：对每个候选 session，把其 `questions`（用户提问消息 ID 数组）展开为 K 个 ID，按 PK 回查 messages，对命中行做 `ILIKE '%keyword%'`。

### 执行计划暴露的问题（keyword=Task:，时间范围 24h）

```
Execution Time: 27410.270 ms   ← 27.4s，接口超时根源

Index Scan Backward (idx_sessions_deleted_at_created_at)
  Filter: EXISTS(SubPlan 1)            loops=12（时间范围内 12 个候选 session）
  SubPlan 1 (每 session 循环执行一次):
    Bitmap Heap Scan on messages       actual time=2283ms × 12 loops
      Recheck Cond: message ~~* '%Task:%'
      Rows Removed by Index Recheck: 8263   ← trigram 假阳性
      Heap Blocks: exact=54517              ← 回表 54517 个堆块
      Bitmap Index Scan on idx_messages_message_trgm
        rows=9701                      ← 索引命中 9701 条候选
```

三层叠加问题：

1. **trigram 索引假阳性爆炸**：planner 估计 `%Task:%` 命中 11 行（选择性好），选择 `idx_messages_message_trgm` bitmap 路径；实际索引命中 **9,701 条候选**，精确匹配仅 **230 条**（选择性 2.4%）。Bitmap Heap Scan 需回表 366MB 大表读取 **54,517 个堆块**做精确 recheck（`message ~~*`），单次循环 2.28s。
2. **相关子查询逐行循环**：EXISTS 是相关子查询，对时间范围内每个候选 session（12 个）**重复执行** bitmap 扫描 → 12 × 2.28s ≈ 27s。
3. **窗口函数叠加**：`COUNT(*) OVER ()`（`SessionSummarySelect` 折入的 total 计算）与分页 SELECT 共用同一 WHERE，同样承受慢 EXISTS 评估。

### 为什么"PK 回查"没生效

修复前的 SQL 语义上确实是"展开 questions → PK 回查 messages"，但 planner **被 trigram 索引误导**：它认为 `ILIKE '%kw%'` 用小索引更快（估计 11 行），选择了 bitmap 而非以 `arr.mid` 驱动主键点查。高频词（`Task:`、`error` 等）是 trigram 索引的典型陷阱——索引候选多、假阳性高、回表代价被放大。

## 方案对比

### 方案 A：IN 子查询形态改写（采纳 ✅）

```sql
-- 修复后
EXISTS (
  SELECT 1
  FROM jsonb_array_elements_text(sessions.questions::jsonb) AS arr(mid)
  WHERE arr.mid::bigint IN (SELECT id FROM messages WHERE messages.message::text ILIKE ?)
)
```

**思路**：把"JOIN messages 表"改为"IN (SELECT id FROM messages)"。planner 以 arr（每 session 少量 question ID，通常 3~50 个）为**驱动侧**，对 `messages_pkey` 主键索引逐 ID 点查（O(log N)），完全避开 trigram bitmap 回表。

- ✅ 一行常量改动（`internal/common/constant/sql.go`），调用方零改动
- ✅ 语义完全等价（同为"questions 中存在消息匹配 keyword"）
- ✅ 仍满足既有约束：整段 SQL 只有 1 个 `?` 占位符（gorm）
- ✅ 实测三场景全部稳定（见验证矩阵）

### 方案 B：先物化匹配消息 ID 集合 + jsonb 数组重叠

```sql
AND questions::jsonb ?| (SELECT ARRAY(SELECT id::text FROM messages WHERE message ILIKE ?)::text[])
```

- ❌ 实测 2.47s：获取匹配 ID 集合本身仍需 trigram bitmap 回表（9,706 候选 → 4,978 堆块），瓶颈未消除
- ❌ 需新增 `sessions.questions` 的 jsonb GIN 索引（DDL），改动面大
- ❌ 数组重叠对 text/jsonb 类型转换敏感，语义边界多

### 方案 C：应用层两段式查询（先查匹配消息 ID，再反查 sessions）

- ❌ 需要改 usecase 层逻辑（`jwt_session_queries.go`），破坏 repository 单一查询封装
- ❌ 同样绕不开"获取匹配 ID 集合"的 trigram 回表成本（同方案 B）
- ❌ 多一次 roundtrip，代码复杂化，无收益

### 决策

**采用方案 A**。核心洞察：**问题不在"是否回查 messages"，而在"planner 是否有机会选择 trigram bitmap"。IN 子查询形态天然以 arr 小集合驱动主键点查，不给 planner 留 bitmap 选项**。改动最小、语义等价、实测最优。

## 验证矩阵（生产库 EXPLAIN ANALYZE 实测）

| keyword | 修复前 | 修复后 | 说明 |
|---------|--------|--------|------|
| `Task:`（用户报障） | 27,410 ms | **22 ms** | 高频词，trigram 假阳性 9,701→230 |
| `error`（常见词） | —（同类风险） | **17 ms** | 验证常见词不劣化 |
| 罕见词（0 命中） | — | **19 ms** | 40 次主键点查全 miss，仍快 |

三场景覆盖高/中/低命中率，方案 A 均稳定。修复后生产接口实测 HTTP 200，总耗时 **0.229s**（含网络与 JWT），较修复前 >30s 提升 100+ 倍。

## 测试计划

| 层 | 位置 | 断言 |
|----|------|------|
| 单元 | `test/unit/session_keyword_filter/session_keyword_filter_test.go` | 形态护栏：必须含 `jsonb_array_elements_text(`、`IN (SELECT id FROM messages`、`ILIKE ?`；**禁止** `JOIN messages ON messages.id =`（防止 planner 陷阱回归）；必须恰 1 个 `?`；禁 `ANY(message_ids)`、`jsonb_exists(` |
| 生产验证 | 线上 EXPLAIN ANALYZE | 三 keyword 场景耗时 <100ms；执行计划含 `Index Scan using messages_pkey` 主键点查 |

## 边界与说明

- **行为等价**：仅改变查询计划，响应结构、分页、排序、过滤语义均不变。
- **数据量增长**：方案 A 复杂度为 O(候选 sessions × K)（K = 每 session question 数），随 sessions/messages 增长线性扩展；深页 offset 扫描与 Count 查询的长期优化由 `2026-07-27-cursor-pagination-design.md` 游标分页改造承接（待实施）。
- **trigram 索引去留**：`idx_messages_message_trgm` 保留——对低频词搜索仍有加速价值，只是高频词路径不应再被 planner 选中。
- **已知关联项**：`session_baseline_perf` 单测中关于 trigram 索引的注释为历史记录，无需变更。
