# Session 时间范围过滤 + Session/Audit 排序

## 背景

Session 列表页仅支持分页，无时间过滤、无排序。Audit 页支持时间过滤但前端未暴露排序 UI。后端 Audit 已完整支持 sort/sortField，Session 不支持。

## 目标

1. Session 列表页新增时间范围过滤（复用 Audit 的 TimeRangePicker 组件）
2. Session 列表页新增表头排序（createdAt/updatedAt/messageCount/toolCount，默认 createdAt desc）
3. Audit 列表页新增表头排序（createdAt/inputTokens/outputTokens/firstTokenLatencyMs/streamDurationMs，默认 createdAt desc）
4. Session 后端从 raw SQL 迁移到 GORM builder，支持动态 WHERE/ORDER

## 后端改动

### DTO (`internal/dto/session.go`)

`ListSessionsByUserReq` 新增 `Sort`、`SortField`、`StartTime`、`EndTime` 字段，对齐 `ListAuditLogsReq`。

### Usecase (`internal/application/session/query/jwt_session_queries.go`)

`ListSessionsByUserQuery` 新增 `Sort`、`SortField`、`StartTime`、`EndTime` 字段。

增加排序字段白名单校验：`created_at`、`updated_at`、`message_count`、`tool_count`。

默认：`Sort=desc, SortField=created_at`。

### Domain Repository (`internal/domain/session/repository.go`)

`SessionReadRepository` 三个 List 方法签名更新为接收 `model.CommonParam` + `startTime`/`endTime`。

### Repository Impl (`internal/infrastructure/repository/session_repository.go`)

`listSessionsRaw` 迁移为 GORM builder 模式（对齐 `audit_repository.paginate`），支持动态时间过滤和排序。

### Handler (`internal/handler/session.go`)

`HandleListSessionsByUser` 传递 `Sort`/`SortField`/`StartTime`/`EndTime`。

### Constants (`internal/common/constant/sql.go`)

新增 `FieldMessageCount`、`FieldToolCount` 常量。

## 前端改动

### API Client (`web/src/lib/api-client.ts`)

`listSessions` 签名更新为对象参数，增加 `sort`/`sortField`/`startTime`/`endTime`。

`listAuditLogs` 签名增加 `sort`/`sortField`。

### Session 页面 (`web/src/app/(dashboard)/sessions/page.tsx`)

新增 TimeRangePicker 组件（复用 audit 的组件）。表头排序（点击切换 asc/desc）。

### Audit 页面 (`web/src/app/(dashboard)/audit/page.tsx`)

表头排序（点击切换 asc/desc）。`fetchLogs` 传递 `sort`/`sortField`。

## 排序字段

| 页面    | 字段           | sortField 值            |
|---------|----------------|------------------------|
| Session | Created        | `created_at`           |
| Session | Updated        | `updated_at`           |
| Session | Messages       | `message_count`        |
| Session | Tools          | `tool_count`           |
| Audit   | Time           | `created_at`           |
| Audit   | Input Tokens   | `input_tokens`         |
| Audit   | Output Tokens  | `output_tokens`        |
| Audit   | Latency        | `first_token_latency_ms` |
| Audit   | Duration       | `stream_duration_ms`   |

## 默认行为

- Session：`sort=desc, sortField=created_at`，无时间过滤
- Audit：`sort=desc, sortField=created_at`，默认时间范围 24h（保持现有行为）
