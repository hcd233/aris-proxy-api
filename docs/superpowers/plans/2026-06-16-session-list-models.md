# Session 列表返回模型列表 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `/api/v1/session/list` 响应中返回每个 session 的回答模型列表，并在 Web 端 Sessions 页面新增带模型图标的 Models 列。

**Architecture:** 采用预抽取方案，在 `sessions` 表新增 `models` JSON 列；创建 session 时从 assistant 消息抽取模型名写入；列表查询直接读取该列。历史 session 通过一次性 SQL 回填。

**Tech Stack:** Go 1.25.1 + GORM + PostgreSQL；Next.js 16 + React 19 + TypeScript + Tailwind v4 + shadcn/ui + @lobehub/icons。

---

## File Map

| 文件 |  responsibility |
|------|-----------------|
| `internal/infrastructure/database/model/session.go` | DB 模型，新增 `Models []string` 字段 |
| `internal/domain/session/repository.go` | 读模型 `SessionSummaryProjection` 增加 `Models` |
| `internal/infrastructure/repository/session_repository.go` | 列表查询行模型、SQL Select、映射逻辑 |
| `internal/common/constant/sql.go` | `SessionSummarySelect` 常量 |
| `internal/application/session/port/handler.go` | `SessionSummaryView` 增加 `Models` |
| `internal/application/session/query/jwt_session_queries.go` | 列表 handler，投影→视图映射 |
| `internal/dto/session.go` | `SessionSummary` DTO 增加 `Models` |
| `internal/handler/session.go` | HTTP handler，视图→DTO 映射 |
| `internal/infrastructure/pool/store_pool.go` | 创建 session 时抽取 models 写入 |
| `web/src/lib/types.ts` | 前端 `SessionSummary` 类型 |
| `web/src/app/(dashboard)/sessions/page.tsx` | Sessions 列表页面，新增 Models 列/卡片展示 |

---

### Task 1: DB 模型新增 `models` 列

**Files:**
- Modify: `internal/infrastructure/database/model/session.go`

- [ ] **Step 1: 添加 `Models []string` 字段**

```go
Models []string `json:"models" gorm:"column:models;comment:回答模型列表;serializer:json"`
```

位置：在 `Questions` 字段之后、`Metadata` 之前。

- [ ] **Step 2: 编译检查**

Run: `go build ./internal/infrastructure/database/model/...`
Expected: 无错误。

---

### Task 2: 投影链路新增 `Models`

**Files:**
- Modify: `internal/domain/session/repository.go`
- Modify: `internal/infrastructure/repository/session_repository.go`
- Modify: `internal/common/constant/sql.go`

- [ ] **Step 1: `SessionSummaryProjection` 增加字段**

```go
type SessionSummaryProjection struct {
    ID           uint
    CreatedAt    time.Time
    UpdatedAt    time.Time
    Score        *int
    MessageCount int
    ToolCount    int
    Questions    []uint
    Models       []string
}
```

- [ ] **Step 2: `sessionSummaryRow` 增加字段并更新 SQL**

```go
type sessionSummaryRow struct {
    ID           uint      `gorm:"column:id"`
    CreatedAt    time.Time `gorm:"column:created_at"`
    UpdatedAt    time.Time `gorm:"column:updated_at"`
    Score        *int      `gorm:"column:score"`
    MessageCount int       `gorm:"column:message_count"`
    ToolCount    int       `gorm:"column:tool_count"`
    Questions    []uint    `gorm:"column:questions;serializer:json"`
    Models       []string  `gorm:"column:models;serializer:json"`
    TotalCount   int64     `gorm:"column:total_count"`
}
```

- [ ] **Step 3: `SessionSummarySelect` 加入 `models`**

更新 `internal/common/constant/sql.go` 中的 `SessionSummarySelect`：

```go
SessionSummarySelect = "id, created_at, updated_at, score, COALESCE(jsonb_array_length(message_ids::jsonb), 0) AS message_count, COALESCE(jsonb_array_length(tool_ids::jsonb), 0) AS tool_count, questions, models, COUNT(*) OVER () AS total_count"
```

- [ ] **Step 4: 列表查询映射 `Models`**

在 `ListAllSessions` 和 `ListSessionsByOwnerNames` 的 `lo.Map` 中，给 `SessionSummaryProjection` 加上 `Models: row.Models`。

- [ ] **Step 5: 编译并跑相关单测**

Run: `go test -count=1 ./test/unit/session_baseline_perf/...`
Expected: PASS。

---

### Task 3: Application / Handler 链路新增 `Models`

**Files:**
- Modify: `internal/application/session/port/handler.go`
- Modify: `internal/application/session/query/jwt_session_queries.go`
- Modify: `internal/dto/session.go`
- Modify: `internal/handler/session.go`

- [ ] **Step 1: `SessionSummaryView` 增加 `Models []string`**

```go
type SessionSummaryView struct {
    ID           uint
    CreatedAt    time.Time
    UpdatedAt    time.Time
    Summary      string
    Score        *int
    MessageCount int
    ToolCount    int
    Models       []string
}
```

- [ ] **Step 2: `ListSessionsByUserHandler` 映射 `Models`**

在 `jwt_session_queries.go` 的 `Handle` 返回 view 时：

```go
return &sessionport.SessionSummaryView{
    ID:           p.ID,
    CreatedAt:    p.CreatedAt,
    UpdatedAt:    p.UpdatedAt,
    Summary:      summary,
    Score:        p.Score,
    MessageCount: p.MessageCount,
    ToolCount:    p.ToolCount,
    Models:       p.Models,
}
```

- [ ] **Step 3: DTO `SessionSummary` 增加 `Models`**

```go
type SessionSummary struct {
    ID           uint              `json:"id" doc:"Session ID"`
    CreatedAt    time.Time         `json:"createdAt" doc:"创建时间"`
    UpdatedAt    time.Time         `json:"updatedAt" doc:"更新时间"`
    Summary      string            `json:"summary" doc:"会话总结"`
    Score        *int              `json:"score,omitempty" doc:"人工评分(1-5)"`
    MessageCount int               `json:"messageCount" doc:"消息数量"`
    ToolCount    int               `json:"toolCount" doc:"工具数量"`
    Metadata     map[string]string `json:"metadata,omitempty" doc:"请求元数据"`
    Models       []string          `json:"models,omitempty" doc:"回答模型列表"`
}
```

- [ ] **Step 4: HTTP handler 映射 `Models`**

在 `internal/handler/session.go` 的 `HandleListSessionsByUser` 中：

```go
return &dto.SessionSummary{
    ID:           v.ID,
    CreatedAt:    v.CreatedAt,
    UpdatedAt:    v.UpdatedAt,
    Summary:      v.Summary,
    MessageCount: v.MessageCount,
    ToolCount:    v.ToolCount,
    Score:        v.Score,
    Models:       v.Models,
}
```

- [ ] **Step 5: 编译检查**

Run: `go build ./...`
Expected: 无错误。

---

### Task 4: 创建 session 时抽取 models

**Files:**
- Modify: `internal/infrastructure/pool/store_pool.go`

- [ ] **Step 1: 在 `runMessageStoreTask` 中收集 models**

在 `messageIDs` 生成之后、`session` 构造之前，插入：

```go
var models []string
seenModels := make(map[string]struct{})
for _, m := range messages {
    if m.Message.Role == enum.RoleAssistant && m.Model != "" {
        if _, ok := seenModels[m.Model]; !ok {
            seenModels[m.Model] = struct{}{}
            models = append(models, m.Model)
        }
    }
}
```

- [ ] **Step 2: 写入 `dbmodel.Session`**

```go
session := &dbmodel.Session{
    APIKeyName: task.APIKeyName,
    MessageIDs: messageIDs,
    Questions:  questions,
    Models:     models,
    ToolIDs:    toolIDs,
    Metadata:   task.Metadata,
}
```

- [ ] **Step 3: 编译检查**

Run: `go build ./internal/infrastructure/pool/...`
Expected: 无错误。

---

### Task 5: 前端类型与列表展示

**Files:**
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/app/(dashboard)/sessions/page.tsx`

- [ ] **Step 1: 更新 `SessionSummary` 类型**

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

- [ ] **Step 2: 导入 `ProviderIcon` 和 `Tooltip`**

```ts
import { ProviderIcon } from "@/components/provider-icon";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
```

- [ ] **Step 3: 新增 Models 列渲染 helper**

在组件内添加：

```ts
function ModelBadge({ model }: { model: string }) {
  const hasIcon = ProviderIcon({ protocol: model, size: 14 }) !== null;
  return (
    <TooltipProvider delayDuration={200}>
      <Tooltip>
        <TooltipTrigger asChild>
          <div className="flex items-center gap-1 rounded border border-border bg-secondary/50 px-1.5 py-0.5">
            <ProviderIcon protocol={model} size={14} />
            {!hasIcon && (
              <span className="max-w-[80px] truncate text-xs text-muted-foreground">
                {model}
              </span>
            )}
          </div>
        </TooltipTrigger>
        <TooltipContent side="top">
          <p className="text-xs">{model}</p>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}
```

> 注意：`ProviderIcon` 内部会返回 `null` 当无匹配图标；这里用同样逻辑判断是否需要文本回退。

- [ ] **Step 4: 桌面端表格新增 Models 列**

在 `<TableHead>Tools ...</TableHead>` 之后、`<TableHead className="w-16 sr-only">Actions</TableHead>` 之前新增：

```tsx
<TableHead className="w-[140px]">Models</TableHead>
```

在表格 body 的 `<TableCell>{s.toolCount ?? 0}</TableCell>` 之后新增：

```tsx
<TableCell>
  <div className="flex flex-wrap items-center gap-1">
    {s.models && s.models.length > 0 ? (
      s.models.map((m) => <ModelBadge key={m} model={m} />)
    ) : (
      <span className="text-muted-foreground">—</span>
    )}
  </div>
</TableCell>
```

- [ ] **Step 5: 移动端卡片新增 Models 信息**

在移动端卡片底部：

```tsx
<div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
  <span>ID: {s.id}</span>
  <span>{s.toolCount ?? 0} tools</span>
  {s.models && s.models.length > 0 && (
    <div className="flex items-center gap-1">
      {s.models.map((m) => <ProviderIcon key={m} protocol={m} size={12} />)}
    </div>
  )}
  <span>{formatDateTime(s.createdAt)}</span>
</div>
```

- [ ] **Step 6: 前端类型与构建检查**

Run: `cd web && npm run lint && npm run build`
Expected: 无类型/构建错误。

---

### Task 6: 迁移 SQL（回填存量数据）

**Files:**
- 新增（不进入代码仓库，交付给用户）：一次性回填 SQL

- [ ] **Step 1: 准备 SQL 脚本**

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

- [ ] **Step 2: 将 SQL 附在最终交付说明中**

---

### Task 7: 验证

- [ ] **Step 1: Go 全量测试**

Run: `go test -count=1 ./...`
Expected: PASS。

- [ ] **Step 2: Go lint**

Run: `make lint`
Expected: PASS。

- [ ] **Step 3: Web lint & build**

Run: `cd web && npm run lint && npm run build`
Expected: PASS。

---

## 交付物

1. 代码改动（上述文件）。
2. 一次性回填 SQL（见 Task 6）。
3. 本地验证通过（`go test ./...`、`make lint`、`npm run lint && npm run build`）。
