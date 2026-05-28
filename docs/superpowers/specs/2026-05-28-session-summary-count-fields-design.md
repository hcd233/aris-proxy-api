# Session Summary Count Fields 设计

## 背景

`/api/v1/session/list` 接口当前返回 `messageIds: number[]` 和 `toolIds: number[]`，前端仅使用其 `.length` 展示数量。传输完整 ID 数组浪费带宽且暴露不必要的内部信息。

## 目标

将 `SessionSummary` DTO 的 `messageIds`/`toolIds` 替换为 `messageCount`/`toolCount`，减少响应体积，同时在 repository 层用 SQL `array_length` 避免读取完整数组到 Go 内存。

## 方案

**方案 A（已采纳）：全链路改为 Count，列表查询用 SQL array_length 聚合**

### 1. 列表查询：用 SQL array_length 代替读完整数组

- `SessionRepoFieldsReadList` 去掉 `message_ids`/`tool_ids`
- 列表查询改为 `SELECT id, created_at, updated_at, summary, array_length(message_ids,1) AS message_count, array_length(tool_ids,1) AS tool_count`
- 不能再走通用 `Paginate`（它只支持 `Select(fields)` 简单字段名），需要写原生 SQL 或 GORM Raw 查询
- `SessionSummaryProjection` 将 `MessageIDs []uint` / `ToolIDs []uint` 替换为 `MessageCount int` / `ToolCount int`

### 2. 空摘要回退：仍用 message_ids 做子查询

- 先从列表结果中找出 summary 为空的 session
- 再查一次 sessions 表只取 `message_ids` 列（通过 `SessionRepoFieldsSummarize`）
- 然后用这些 message_ids 去查 messages 表找 role='user' 的消息

### 3. 全链路影响

| 层 | 变更 |
|---|---|
| `SessionSummaryProjection` | `MessageIDs []uint` / `ToolIDs []uint` → `MessageCount int` / `ToolCount int` |
| `SessionSummaryView` | `MessageIDs []uint` / `ToolIDs []uint` → `MessageCount int` / `ToolCount int` |
| `dto.SessionSummary` | `MessageIDs []uint` / `ToolIDs []uint` → `MessageCount int` / `ToolCount int` |
| Repository 列表查询 | 用 SQL array_length 替代读完整数组 |
| 空摘要回退逻辑 | 额外查 session.message_ids 用于定位用户消息 |
| Handler 映射 | 直接传 count |

## 不改动

- `SessionDetailProjection` — 保留 MessageIDs/ToolIDs（详情查询需要）
- `SessionDetailView` — 保留 MessageIDs/ToolIDs（详情查询需要）
- `dto.SessionDetail` — 不变
- 数据库 schema / 聚合根 — 不变

## API 兼容性

Breaking change：`messageIds`/`toolIds` 被替换为 `messageCount`/`toolCount`。前端需同步部署。

## 验证

1. 单元测试 fixture 和断言更新后 `go test -count=1 ./test/unit/session_dto/...` 通过
2. `make lint` 通过
3. 前端 `npm run build` 通过
