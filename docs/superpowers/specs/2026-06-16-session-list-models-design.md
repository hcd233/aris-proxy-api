# Session 列表返回模型列表设计

## 背景

当前 `/api/v1/session/list` 返回的 `SessionSummary` 只包含消息数、工具数、评分、总结等字段。用户希望在列表里直接看到每个 session 中「回答」使用过的模型列表，并在 Web 端用模型图标展示。

## 目标

1. 后端 `/api/v1/session/list` 响应的 `SessionSummary` 新增 `models` 字段，返回该 session 中所有 assistant 消息使用过的模型名（去重）。
2. Web 端 Session 列表新增一列「Models」，使用现有 `ProviderIcon` 组件展示模型图标。
3. 历史 session 通过一次性 SQL 回填 `models`。

## 关键决策

- **预抽取 vs 实时聚合**：选择「预抽取」。在 `sessions` 表新增 `models` JSON 列，创建 session 时从 assistant 消息中抽取模型名写入。读取时直接返回该列，避免列表查询时再 JOIN/Batch。
- **历史数据**：通过一次性 UPDATE SQL 回填，由运维在部署后执行。

## 数据模型

### DB 模型变更

`internal/infrastructure/database/model/session.go`

```go
type Session struct {
    BaseModel
    ID         uint              `json:"id" gorm:"column:id;primary_key;auto_increment;comment:会话ID"`
    APIKeyName string            `json:"api_key_name" gorm:"column:api_key_name;not null;default:'';comment:API密钥名称"`
    MessageIDs []uint            `json:"message_ids" gorm:"column:message_ids;not null;comment:消息ID列表;serializer:json"`
    ToolIDs    []uint            `json:"tool_ids" gorm:"column:tool_ids;not null;comment:工具ID列表;serializer:json"`
    Questions  []uint            `json:"questions" gorm:"column:questions;comment:用户提问消息ID列表(仅role=user且tool_call_id为空);serializer:json"`
    Models     []string          `json:"models" gorm:"column:models;comment:回答模型列表;serializer:json"`
    Metadata   map[string]string `json:"metadata" gorm:"column:metadata;comment:请求元数据;serializer:json"`
    Score      *int              `json:"score" gorm:"column:score;comment:人工评分(1-5)"`
    ScoredAt   *time.Time        `json:"scored_at" gorm:"column:scored_at;comment:评分时间"`
}
```

GORM `AutoMigrate` 会自动添加 `models` 列。该列允许为空（存量 session 回填前为 NULL）。

## 后端实现

### 1. 投影链路

- `internal/domain/session/repository.go`：`SessionSummaryProjection` 增加 `Models []string`。
- `internal/infrastructure/repository/session_repository.go`：
  - `sessionSummaryRow` 增加 `Models []string`。
  - `SessionSummarySelect` 增加 `models` 列。
  - `ListAllSessions` / `ListSessionsByOwnerNames` 映射 `Models`。
- `internal/application/session/port/handler.go`：`SessionSummaryView` 增加 `Models []string`。
- `internal/application/session/query/jwt_session_queries.go`：在 `Handle` 返回 view 时把 `p.Models` 带过去。
- `internal/dto/session.go`：`SessionSummary` 增加 `Models []string`（带 `omitempty`，为空时不返回）。
- `internal/handler/session.go`：`HandleListSessionsByUser` 映射 `v.Models` 到 DTO。

### 2. 写入逻辑

`internal/infrastructure/pool/store_pool.go` 的 `runMessageStoreTask` 在构造 `dbmodel.Session` 时，从 `messages` 中收集：

```go
var models []string
seen := make(map[string]struct{})
for _, m := range messages {
    if m.Message.Role == enum.RoleAssistant && m.Model != "" {
        if _, ok := seen[m.Model]; !ok {
            seen[m.Model] = struct{}{}
            models = append(models, m.Model)
        }
    }
}
```

写入 `Models: models`。保持去重且保留出现顺序。

### 3. 历史回填 SQL

```sql
-- 回填存量 session 的 models 列
-- 说明：从 message_ids 聚合 assistant 消息的 model 字段（非空），去重后写回 sessions.models
UPDATE sessions s
SET models = sub.models
FROM (
    SELECT
        s.id,
        COALESCE(
            jsonb_agg(DISTINCT m.model ORDER BY m.model) FILTER (WHERE m.model IS NOT NULL AND m.model <> ''),
            '[]'::jsonb
        ) AS models
    FROM sessions s
    CROSS JOIN LATERAL jsonb_array_elements_text(s.message_ids::jsonb) AS mid
    JOIN messages m ON m.id = mid::bigint
    WHERE s.deleted_at = 0
      AND m.message::jsonb ->> 'role' = 'assistant'
    GROUP BY s.id
) sub
WHERE s.id = sub.id
  AND (s.models IS NULL OR s.models = '[]'::jsonb);
```

> 注意：该 SQL 一次性处理全表，数据量大时建议分批或低峰期执行；执行前请备份。

## 前端实现

### 1. 类型

`web/src/lib/types.ts`

```ts
export interface SessionSummary {
  id: number;
  createdAt: string;
  updatedAt: string;
  summary: string;
  score?: number;
  messageCount: number;
  toolCount: number;
  metadata?: Record<string, string>;
  models?: string[];
}
```

### 2. 桌面端表格

`web/src/app/(dashboard)/sessions/page.tsx`

- 表头在 `Tools` 右侧新增 `Models` 列（不可排序）。
- 单元格渲染：
  - 若 `s.models` 为空，显示 `—`。
  - 否则遍历 `s.models`，每个模型调用 `<ProviderIcon protocol={model} size={14} />`。
  - 多个图标横向排列，间距 `gap-1`；溢出时截断或换行（视表格宽度决定，保持紧凑）。
  - 无匹配图标的模型显示短文本标签（首字母或截断）。

### 3. 移动端卡片

在卡片底部 `tools` 信息旁增加 models 展示，同样使用 `ProviderIcon`。

## 测试

- 确保 `SessionSummarySelect` 单测（`session_baseline_perf`）仍通过：新增列不改变 `jsonb_array_length`、`message_count`、`tool_count`、`COUNT(*) OVER ()` 等断言。
- 可考虑补充 store_pool 对 `Models` 抽取的单元测试。
- Web 端运行 `npm run lint && npm run build` 验证类型与构建。

## 部署与迁移

1. 合并代码后，GORM `AutoMigrate` 会自动创建 `models` 列。
2. 部署完成后执行上面的回填 SQL。
3. 之后新 session 会在创建时自动写入 `models`。
