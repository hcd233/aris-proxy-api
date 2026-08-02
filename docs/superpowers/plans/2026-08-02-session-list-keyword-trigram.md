# Session 列表 keyword 过滤性能优化实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 `GET /api/v1/session/list` 带 `keyword` 时的高频词查询超时（27.4s → <100ms），通过改写 `SessionKeywordFilterSQL` 为 IN 子查询形态，强制 planner 以 question-id 小集合驱动 messages 主键点查。

**Architecture:** 根因是 `SessionKeywordFilterSQL` 的 EXISTS + `JOIN messages ON messages.id = arr.mid::bigint` 形态被 messages 的 trigram GIN 索引（`idx_messages_message_trgm`）误导：planner 估计 `%keyword%` 命中少而选 bitmap 路径，高频词（`Task:`）实际命中 9701 候选/230 精确（选择性 2.4%），回表 366MB 大表 54517 堆块 recheck，相关子查询对每个候选 session 循环执行 → 27.4s。改写为 `arr.mid::bigint IN (SELECT id FROM messages WHERE messages.message::text ILIKE ?)` 后，planner 以 arr 为驱动侧对 `messages_pkey` 主键点查，彻底避开 trigram bitmap。

**Tech Stack:** Go + GORM + PostgreSQL（pg_trgm GIN 索引）；测试 `test/unit/` + `test/e2e/`。

**参考设计文档:** `docs/superpowers/specs/2026-08-02-session-list-keyword-trigram-design.md`

## Global Constraints

- 测试目录：`test/unit/<topic>/`、`test/e2e/<topic>/`；禁止 `internal/` 内测试。
- 业务错误走 `internal/common/ierr`；HTTP 状态码用 `fiber.StatusXxx`。
- 日志 `logger.WithCtx(ctx)`，消息前缀 `[PascalCaseModule]`。
- 常量放 `internal/common/constant/`；禁止业务包本地 const 块。
- 占位符约束：`SessionKeywordFilterSQL` 整段 SQL 只能有 1 个 `?`（gorm 占位符，ILIKE ?）。
- 代码风格：samber/lo 函数式优先；ponytail full 级别最简实现。
- 开发在 `.worktrees/` 分支进行；提交前跑 `make test` + `make lint`；推送 master 触发自动部署。

---

### Task 1: 根因确认（生产 EXPLAIN ANALYZE）

**Files:**
- 只读：`internal/common/constant/sql.go:150-173`（SessionKeywordFilterSQL）、`internal/infrastructure/repository/session_repository.go:241-373`（ListAllSessions / ListSessionsByOwnerNames）
- 只读：生产库 `sessions` / `messages` 表（SSH `api.lvlvko.top`，`query-prod-database` skill）

**Interfaces:**
- Consumes: `constant.SessionKeywordFilterSQL` 当前值、`idx_messages_message_trgm` 索引
- Produces: 根因结论（27.4s 执行计划证据：loops=12、rows=9701、Heap Blocks=54517、Rows Removed by Index Recheck=8263）

- [x] **Step 1: 确认生产数据规模与索引**

```sql
SELECT relname, n_live_tup AS est_rows, pg_size_pretty(pg_total_relation_size(relid)) AS size
FROM pg_stat_user_tables WHERE relname IN ('sessions','messages');
SELECT indexdef FROM pg_indexes WHERE tablename IN ('sessions','messages');
```
Expected: sessions≈2486 行/19MB、messages≈103917 行/366MB；确认存在 `idx_messages_message_trgm`、`idx_sessions_deleted_at_created_at`。

- [x] **Step 2: 复现原 SQL 执行计划**

```sql
EXPLAIN (ANALYZE, BUFFERS) SELECT id, created_at, updated_at, score,
  COALESCE(jsonb_array_length(message_ids::jsonb),0) AS message_count,
  COALESCE(jsonb_array_length(tool_ids::jsonb),0) AS tool_count,
  questions, model_ids, COUNT(*) OVER () AS total_count
FROM sessions
WHERE deleted_at = 0
  AND created_at >= '2026-08-01T04:29:34.092Z' AND created_at <= '2026-08-02T04:29:34.092Z'
  AND EXISTS (SELECT 1 FROM jsonb_array_elements_text(sessions.questions::jsonb) AS arr(mid)
              JOIN messages ON messages.id = arr.mid::bigint
              WHERE messages.message::text ILIKE '%Task:%')
ORDER BY created_at DESC LIMIT 100 OFFSET 0;
```
Expected: `Execution Time: 27410.270 ms`；Bitmap Index Scan on idx_messages_message_trgm rows=9701；SubPlan loops=12。

### Task 2: 修改 SQL 常量

**Files:**
- Modify: `internal/common/constant/sql.go:172`（SessionKeywordFilterSQL 值 + 注释）

**Interfaces:**
- Consumes: 无（常量独立）
- Produces: 新 `constant.SessionKeywordFilterSQL`（IN 子查询形态）

- [x] **Step 1: 改写常量为 IN 子查询形态**

```go
// 原
SessionKeywordFilterSQL = "EXISTS (SELECT 1 FROM jsonb_array_elements_text(sessions.questions::jsonb) AS arr(mid) JOIN messages ON messages.id = arr.mid::bigint WHERE messages.message::text ILIKE ?)"
// 新
SessionKeywordFilterSQL = "EXISTS (SELECT 1 FROM jsonb_array_elements_text(sessions.questions::jsonb) AS arr(mid) WHERE arr.mid::bigint IN (SELECT id FROM messages WHERE messages.message::text ILIKE ?))"
```

同步更新常量上方设计注释：说明 2026-06-07 JOIN 形态被 trigram 索引误导选 bitmap 的教训、本次 IN 形态如何强制主键点查、实测三场景数据。

### Task 3: 更新单元测试护栏

**Files:**
- Modify: `test/unit/session_keyword_filter/session_keyword_filter_test.go:29-68`（TestSessionKeywordFilterSQL_UsesPKJoinShape）

**Interfaces:**
- Consumes: `constant.SessionKeywordFilterSQL` 新值
- Produces: 防回退断言（钉死 IN 形态、禁止 JOIN 形态）

- [x] **Step 1: 更新 `TestSessionKeywordFilterSQL_UsesPKJoinShape` 断言**

将原 `strings.Contains(fragment, "messages.id =")` 断言替换为：
```go
if !strings.Contains(fragment, "IN (SELECT id FROM messages") {
    t.Errorf("SessionKeywordFilterSQL must PK-lookup messages via IN (SELECT id FROM messages ...) so the planner drives from the small question-id set instead of the trigram bitmap on the large messages table, got %q", fragment)
}
if strings.Contains(fragment, "JOIN messages ON messages.id =") {
    t.Errorf("SessionKeywordFilterSQL must not JOIN messages (the planner may pick the trigram bitmap path and time out on high-frequency keywords); use IN (SELECT id FROM messages ...) instead, got %q", fragment)
}
```

- [x] **Step 2: 更新测试注释与 @update**

注释补充 2026-08-02 IN 形态缘由；`@update 2026-06-07 21:20:00` → `@update 2026-08-02 12:40:00`。

- [x] **Step 3: 运行单元测试确认绿**

Run: `go test -count=1 ./test/unit/session_keyword_filter/`
Expected: 2 passed。

### Task 4: 全量验证与提交

**Files:**
- 验证：`go build ./cmd/server`、`go test -count=1 ./test/unit/...`、`make lint`
- 生产验证：部署后对线上执行同参数 curl 与 EXPLAIN ANALYZE

- [x] **Step 1: 编译 + 全量单测 + lint**

Run: `go build ./cmd/server && go test -count=1 ./test/unit/... && make lint`
Expected: build OK；836 passed（web-build 完成后，含先前 2 个因 `web/dist` 缺失而失败的包）；lint errors=0。

- [x] **Step 2: 提交分支并推送 master（触发自动部署）**

```bash
git add internal/common/constant/sql.go test/unit/session_keyword_filter/session_keyword_filter_test.go
git commit -m "fix(session): keyword 过滤改写为 IN 子查询，修复高频词查询超时（27.4s→22ms）"
# 合并到 master 并 push
```
Expected: master `7957c46f`；`docker-publish.yml` workflow 构建 + deploy-k8s 成功。

- [x] **Step 3: 生产实测验证**

```bash
curl -w "HTTP %{http_code} | total %{time_total}s" 'https://api.lvlvko.top/api/v1/session/list?page=1&pageSize=100&sort=desc&sortField=created_at&startTime=2026-08-01T04%3A29%3A34.092Z&endTime=2026-08-02T04%3A29%3A34.092Z&keyword=Task%3A' -H 'authorization: Bearer <JWT>'
```
Expected: HTTP 200，total 0.229s（修复前 >30s 超时）。

- [x] **Step 4: 沉淀工程经验**

`serena_write_memory` 记录根因、修复、验证方法（`perf/session-list-keyword-trigram-2026-08-02`）。

### Task 5: 沉淀 spec 与 plan 文档（本文档）

**Files:**
- Create: `docs/superpowers/specs/2026-08-02-session-list-keyword-trigram-design.md`
- Create: `docs/superpowers/plans/2026-08-02-session-list-keyword-trigram.md`
- 提交到 master

- [x] **Step 1: 写设计文档**（背景/根因/方案对比/验证矩阵）
- [x] **Step 2: 写实施计划**（本文件）
- [x] **Step 3: 提交推送 master**
