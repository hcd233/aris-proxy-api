# Session List 性能优化：questions 列 + 删除 summary 列

- **日期**: 2026-06-08
- **状态**: 待实现
- **修订**: v2 — 增加删除 `summary`/`summarize_error` 列及 SummarizeAgent

## 问题

1. `GET /api/v1/session/list` 的 summary fallback 触发 N 次 message 查询
2. Keyword 搜索用 `message_ids` 做 EXISTS 子查询太重
3. `summary`/`summarize_error` 列维护成本高（AI 总结 cron + agent_pool），实际列表只用第一条用户消息

## 设计

### 一、新增 `questions` 列

`sessions` 表新增 `questions` JSONB 列（`messages.id` 数组），只存 role=user 且 tool_call_id="" 的消息 ID。

**判定条件**：`m.Message.Role == enum.RoleUser && m.Message.ToolCallID == ""`

**写时维护**（store_pool.go）：`deduplicateAndStoreMessages` 返回与 `messages` 同序的 `messageIDs`，按 role 过滤。

**Keyword 搜索**：`SessionKeywordFilterSQL` 中 `message_ids` → `questions`。

### 二、删除 `summary`/`summarize_error` 列及 SummarizeAgent

原因：列表不再需要 `summary` 列，summary 值从 `questions[0]` 动态获取。

**删列影响面**：

| 文件 | 操作 |
|------|------|
| `internal/infrastructure/database/model/session.go` | 删 `Summary`、`SummarizeError` 字段 |
| `internal/common/constant/sql.go` | 删 `FieldSummary`、`FieldSummarizeError`、`WhereFieldSummary`；`SessionSummarySelect` 去掉 summary；删 summary 回填迁移 SQL |
| `internal/common/constant/convcheck.go` | 删 `ConvCheckLegacyCronSessionSummary` 规则 |
| `internal/domain/session/aggregate/session.go` | 删 `summary` 字段、`UpdateSummary` 方法、`Summary()` 访问器 |
| `internal/domain/session/repository.go` | 删 `UpdateSummary` 接口方法；`SessionSummaryProjection` 删 `Summary` 字段 |
| `internal/domain/session/vo/session_summary.go` | 删整个文件 |
| `internal/infrastructure/repository/session_repository.go` | 删 `UpdateSummary` 方法、`applySummary` 函数、`sessionSummaryRow.Summary`、list 方法中的 Summary 赋值、`toSessionAggregate` 中的 summary 相关代码 |
| `internal/infrastructure/pool/store_pool.go` | 删 `sessionSummary` 变量及 `Session.Summary` 赋值 |
| `internal/application/session/query/jwt_session_queries.go` | 读时从 `questions[0]` 加载消息 content 作为 summary |

**删 SummarizeAgent 影响面**：

| 文件 | 操作 |
|------|------|
| `internal/infrastructure/pool/agent_pool.go` | 删整文件 |
| `internal/cron/session_summarize.go` | 删整文件 |
| `internal/cron/cron.go` | 删 `CronModuleSessionSummarize` 注册（含 import 清理） |
| `internal/infrastructure/agent/summarizer.go` | 删整文件 |
| `internal/common/constant/agent.go` | 删 `SessionSummarizerAgentName` 等常量 |
| `internal/common/constant/session.go` | 删 `CronModuleSessionSummarize`、`CronSpecSessionSummarize` |
| `internal/config/config.go` | 删 `CronSessionSummarizeEnabled` |
| `internal/dto/asynctask.go` | 删 `SummarizeTask` 结构体 |

### 三、读时从 questions[0] 获取 summary

`jwt_session_queries.go` 的 `Handle` 方法中：

1. 收集所有 projection 的 `questions[0]` message ID
2. 通过 `FindMessagesByIDs` 批量加载消息
3. 提取每条消息的 content text 作为 summary
4. 如果 `questions` 为空或消息不存在，summary = ""

这样每个请求最多增加 1 次 `SELECT ... FROM messages WHERE id IN (?)`（页大小最多 200 条 session × 1 条消息 = 200 IDs，单次查询即可）。

### 四、新请求 SQL 链路

| # | SQL | 执行条件 |
|---|-----|---------|
| 1 | `SELECT name FROM proxy_api_keys` | 非 admin |
| 2 | `SELECT id,created_at,updated_at,score,message_count,tool_count,COUNT(*)OVER() FROM sessions WHERE ...` | 必执行 |
| 3 | `SELECT id,model,message,created_at FROM messages WHERE id IN (?)` | 必执行（questions[0] 批量加载） |

3 条 SQL，带 keyword 仍为 3 条（SQL #2 的 keyword 过滤在 sessions 表内完成）。

### 五、存量数据迁移

仅保留一条回填（阶段二 summary 回填删除）：

```sql
UPDATE sessions SET questions = (
  SELECT COALESCE(jsonb_agg(m.id ORDER BY m.id), '[]'::jsonb)
  FROM messages m
  WHERE m.id IN (
    SELECT jsonb_array_elements_text(sessions.message_ids::jsonb)::bigint
  )
    AND m.message->>'role' = 'user'
    AND (m.message->>'tool_call_id' IS NULL OR m.message->>'tool_call_id' = '')
) WHERE questions IS NULL;
```

### 六、单元测试更新

| 文件 | 操作 |
|------|------|
| `test/unit/session_baseline_perf/` | `SessionSummarySelect` 断言去掉 summary；`BackfillIsIdempotent` 放宽 |
| `test/unit/domain_session_vo/` | 删 `NewSessionSummary` 相关测试 |
| `test/unit/session_dto/` | 更新 `SessionSummary` 相关测试 |
| `test/unit/session_list_batch_perf/` | 已删（上轮） |
| `test/unit/pool_manager/` | 如涉及 agent_pool，更新 |

### 七、不涉及

- 前端 response 结构不变（summary 字段保留，值从 questions[0] 填）
- 分页方案仍是 OFFSET/LIMIT
- `COUNT(*) OVER ()` 仍保留
