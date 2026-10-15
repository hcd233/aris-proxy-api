# session/list keyword 搜索性能修复设计（两段式 + `= ANY` + user 部分索引）

> 日期：2026-10-15
> 分支：`bugfix/session-keyword-search-perf-2026-10-15`
> 目标：修复 `GET /api/web/v1/session/list?keyword=...` 高频词查询 10s~60s+ 的性能问题，
> 保持「中文/英文任意子串匹配、只搜用户消息」的搜索语义不变。

## 背景

### 现象

traceId `b844b0ce-949c-4c06-bd0e-206aced0bef2`：`GET /api/web/v1/session/list?keyword=shadcn`
总耗时 **10.06s**（HTTP 200），其中一条 GORM SQL 占 10.02s。生产复现时该 SQL
`EXPLAIN ANALYZE` 25s、60s 两次超时，实际最坏可达小时级。

### 根因（生产库只读实测证据链）

1. **数据形态**：messages 实际 **164,482 行 / 1,077 MB**（平均 ~120KB/行 JSON 长文本），
   其中 user 消息 **11,551 行 / 127 MB**；sessions 3,144 行，questions 元素共 14,144 个。
   `pg_stat_user_tables` 估算仅 9,050 行（**统计信息过时 18 倍**）。
2. **内层 ILIKE 单次 3.58s**：trigram bitmap 命中 1,931 候选（planner 估计 17 行，**85 倍低估**），
   回表 recheck 1,826 行长文本，`Buffers: shared hit=18053 read=4063`（~173MB IO）。
3. **放大机制（元凶）**：`EXISTS (… jsonb_array_elements_text(sessions.questions) arr
   WHERE arr.mid::bigint IN (SELECT id FROM messages WHERE ILIKE ?))` 被 planner 建成
   **correlated SubPlan**——EXISTS 引用外层 `sessions.questions`，SRF + IN 子查询组合无法去相关，
   messages 的 ILIKE 扫描对外层 3,144 个 session **逐行重复执行**。
   planner 自估总代价 2,493,241（≈3144×792/行）。
4. **2026-08-02 修复为何失效**：`IN` 子查询形态当时被去相关成「arr 驱动主键点查」（22ms）；
   trigram 统计漂移后退化为 correlated SubPlan + Hash Join。**该 SQL 形态的计划稳定性
   押注在 planner 统计信息上，本质上不可靠**，必须改查询结构。

### 搜索语义（用户确认）

- 只搜**用户消息**（`sessions.questions` 引用的 message，即 `role='user'` 的正文）；
- **中文/英文任意子串匹配是硬需求** → 排除 tsvector 全文检索路线（`zhparser` 运维重且失去子串能力）。

## 范围

### 本次改动

| 项 | 内容 |
|---|---|
| 索引 DDL | `idx_messages_user_content_trgm`：user 消息 `content` 的部分 trigram GIN 索引 |
| SQL 常量 | `SessionKeywordFilterSQL` 改 `= ANY(?::bigint[])` 形态；新增 `SessionKeywordMatchIDsSQL` |
| 查询流程 | `session_repository.go` 两个 keyword 注入点改两段式 + ids 缓存 |
| 测试 | SQL 形态断言、两段式单测、E2E 搜索用例 |

### 明确不做（及理由）

- **命中上限/截断/超限报错**：搜索范围锁定 user 消息后，命中 id 集合硬上界 = 用户消息总数
  （当前 11,551），全量参与仅毫秒级哈希探测，无雪崩风险；截断会造成「搜不到」的不可预测漏结果，
  报错则是为不可能场景写错误处理（YAGNI）。user 消息增长到数十万行、ids 数组膨胀到 MB 级时
  再考虑分页/降级（`// ponytail:` 注释标记）。
- **旧索引 `idx_messages_message_trgm` / `idx_messages_content_trgm`**：保留不动，可能有其他
  查询路径依赖，本次不扩大战场。
- **`COUNT(*) OVER ()` 改造**：keyword 路径修复后第二段为毫秒级，total 精确计数成本可忽略。
- **tsvector / zhparser 全文检索**：被中文子串硬需求排除。
- **非 keyword 的 session 列表路径**：不改。

## 方案选型

| 方案 | 思路 | 结论 |
|---|---|---|
| **A（采用）** | 两段式查询 + `= ANY(?)` 常量数组 + user 部分索引 + ids 缓存 | 计划退化空间从结构上焊死 |
| B | `MATERIALIZED CTE` 单 SQL 改写 | EXISTS 相关性仍在，planner 对「物化 CTE + SRF + IN + 相关 EXISTS」行为仍需反复验证；单语句长持连接 |
| C | tsvector + zhparser | 失去子串能力且运维重，被需求排除 |

选 A 的核心理由：本次事故的本质是「SQL 形态的计划稳定性押注 planner 统计信息」。
方案 A 把 ids 变成常量数组传入，**结构上不可能再产生 correlated SubPlan**；方案 B 仍在赌 planner 行为。

## 详细设计

### 1. 数据层：user 消息内容部分 trigram 索引

```sql
CREATE INDEX CONCURRENTLY idx_messages_user_content_trgm
ON messages USING gin ((message::jsonb->>'content') gin_trgm_ops)
WHERE message::jsonb->>'role' = 'user';
```

- 范围 11.5K 行 / 127MB（全表的 7%）；只索引 `content` 正文，不索引 role/tool_call 等
  JSON 元数据（消除搜 "user" 命中全部用户消息这类误报）。
- `CONCURRENTLY` 不阻塞写，不能在事务块内执行，失败会遗留 INVALID 索引
  （执行后查 `pg_indexes` 验证，INVALID 则 `DROP INDEX CONCURRENTLY` 清理后重建）。
- 执行通道：`query-prod-database` 索引 DDL 授权流程（展示完整 SQL → 用户授权 → 执行）。

### 2. SQL 形态（`internal/common/constant/sql.go`）

```go
// 第一段：keyword → 用户消息 id 集合
SessionKeywordMatchIDsSQL = "SELECT id FROM messages WHERE message::jsonb->>'role' = 'user' AND message::jsonb->>'content' ILIKE ?"

// 第二段：ids 以常量数组传入
SessionKeywordFilterSQL = "EXISTS (SELECT 1 FROM jsonb_array_elements_text(sessions.questions::jsonb) AS arr(mid) WHERE arr.mid::bigint = ANY(?::bigint[]))"
```

约束（沿革 fix #58 / #59，继续遵守）：

- 整段 SQL 只能有 **1 个 `?`**（gorm 占位符撞车）；`= ANY(?::bigint[])` 满足。
- 不要写 `messages.id = ANY(sessions.message_ids)`：`message_ids` 是 jsonb 文本非原生数组，
  会触发 SQLSTATE 42809。
- ids 由 Go 端拼 `"{1,2,3}"` 字符串（`lo.Map` + `strings.Join`，纯数字序列化，无注入面，零新依赖）。
- 同步更新常量注释的设计沿革（2026-06-07 JOIN → 2026-08-02 IN 子查询 → 本次 `= ANY` 两段式）。

### 3. 查询流程（`internal/infrastructure/repository/session_repository.go` 两个注入点）

`keyword != ""` 时：

1. `cache.GetOrLoad` 查命中 ids：key = 归一化 keyword（trim + 小写），TTL 60s±10s 抖动；
   空命中走 `mo.None` 空值标记（短 TTL）防穿透。
2. 命中 0 → **短路返回空页**，不执行第二段。
3. 否则第二段以 ids 过滤 sessions，原有排序/分页/`COUNT(*) OVER ()` 逻辑不动。

```go
// ponytail: ids 集合无上限保护，user 消息超数十万行时再考虑分页/降级/FTS，upgrade path: 分区表 + tsvector
```

- 缓存放 repository 层（lint 约束：`internal/application/**` 禁 import `internal/infrastructure/cache`）。
- 第一段同样作用于两个注入点（JWT 版 `:234`、owner 版 `:303`），抽公共私有方法避免重复。
- Redis 故障 fail-open（`ReadCache` 既有行为），缓存永不影响正确性。

### 4. 错误处理

- 第一段/第二段 DB 错误：`ierr.Wrap(ierr.ErrDBQuery, err, "...")`，与现状一致。
- 缓存故障：`GetOrLoad` fail-open 兜底，仅日志告警。
- 不引入新错误码（超限保护已砍）。

## 测试与验证

> 前提事实：测试基建全部基于 sqlite（`test/unit/sqlitetest`），PG 特有语法
> （`jsonb_array_elements_text`、`::jsonb`、`::bigint[]` cast）无法在现有单测/E2E 执行；
> keyword 路径历史上也只有 SQL 形态字符串断言，从未有行为级测试。本次沿用该边界，不引入 PG 测试容器（YAGNI）。

- **单测（SQL 形态）** `test/unit/session_keyword_filter/`：断言更新为 `= ANY` 形态 + 单占位符 +
  `role='user'` 条件；防回退钉死 SQL 形态（沿用既有断言风格）。
- **单测（编排逻辑）**：两段式流程以 fake 查询接口注入，不真跑 SQL——命中 / 空命中短路 /
  缓存命中不回源（GORM Query 计数断言：`db.Callback().Query().Before("gorm:query").Register(...)`）/
  ids 数组序列化正确性（`{1,2,3}` 拼装、空集合短路不进入第二段）。
- **本地 PG 行为验证**（手工清单，不进 CI）：docker 起 PG 灌最小夹具，验证中文子串 / 英文子串
  （`shadcn` / `Task:`）/ 无命中三类结果集正确，`EXPLAIN` 确认无 SubPlan、第一段走部分索引。
- **生产验证**：部署后重放 `keyword=shadcn` 对比 latency；`EXPLAIN` 复核计划。
- **E2E**：不新增（sqlite 无法执行 keyword 路径 SQL，历史上也不覆盖）；由本地 PG 清单 + 生产验证兜底。

## 性能预期

| 场景 | 现状 | 修复后 |
|---|---|---|
| 重复查询（60s 窗口） | 每次 10s+ | 缓存命中，0 DB |
| 首次高频词（`shadcn`） | 10s~60s+ | ~0.3–1s（127MB 部分索引 recheck） |
| 低频词 | 数秒 | ~10–50ms |
| 第二段 sessions 过滤 | 含在大查询内 | ~2–5ms |

## 风险与权衡

- **两段非原子**：缓存 60s 窗口 + 两次查询间的新消息可能不被搜到；搜索场景可接受（列表数据更新频率低）。
- **部分索引体积随 user 消息增长**：127MB 正文的 trigram 索引构建一次性 IO 开销
  （`CONCURRENTLY` 低峰执行）；高频词首次扫描成本随数据量线性增长，缓存摊薄，
  数十万行级再升级 FTS（ponytail 记号已标注）。
- **gorm 数组传参**：`= ANY(?::bigint[])` + 字符串数组为项目首次使用，sqlite 测试无法覆盖
  参数绑定路径，由本地 PG 行为验证清单 + 生产验证兜底（单测仅钉 SQL 形态与编排逻辑）。
- **缓存 60s 陈旧窗口**：新发消息在窗口内可能搜不到；搜索场景（找历史会话）可接受。
