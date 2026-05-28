# Session Summary Count Fields 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `/api/v1/session/list` 响应中的 `messageIds`/`toolIds` 替换为 `messageCount`/`toolCount`，并在 repository 层用 SQL `array_length` 避免读取完整数组。

**Architecture:** 全链路改为 Count。Projection/View 层不再传 MessageIDs/ToolIDs，改用 SQL array_length 聚合。空摘要回退通过额外查 session.message_ids 列来定位用户消息。

**Tech Stack:** Go 1.25, GORM, huma, bytedance/sonic, PostgreSQL array_length, Next.js/TypeScript

---

### Task 1: 修改 Projection 和 View — 全链路改为 Count

**Files:**
- Modify: `internal/domain/session/repository.go:47-54` (SessionSummaryProjection)
- Modify: `internal/application/session/query/session_queries.go:17-24` (SessionSummaryView)

- [ ] **Step 1: 修改 SessionSummaryProjection**

将 `internal/domain/session/repository.go` 第 47-54 行：

```go
type SessionSummaryProjection struct {
	ID         uint
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Summary    string
	MessageIDs []uint
	ToolIDs    []uint
}
```

替换为：

```go
type SessionSummaryProjection struct {
	ID           uint
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Summary      string
	MessageCount int
	ToolCount    int
}
```

- [ ] **Step 2: 修改 SessionSummaryView**

将 `internal/application/session/query/session_queries.go` 第 17-24 行：

```go
type SessionSummaryView struct {
	ID         uint
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Summary    string
	MessageIDs []uint
	ToolIDs    []uint
}
```

替换为：

```go
type SessionSummaryView struct {
	ID           uint
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Summary      string
	MessageCount int
	ToolCount    int
}
```

---

### Task 2: Repository 层 — 用 SQL array_length 替代读完整数组

**Files:**
- Modify: `internal/common/constant/sql.go:85` (SessionRepoFieldsReadList)
- Modify: `internal/infrastructure/repository/session_repository.go:249-333` (三个 List 方法)
- Add new method: `FindSessionMessageIDsByIDs` 到 `SessionReadRepository` 接口和实现
- Modify: `internal/domain/session/repository.go:99-110` (接口)

- [ ] **Step 1: 更新 SessionRepoFieldsReadList**

将 `internal/common/constant/sql.go` 第 85 行：

```go
	SessionRepoFieldsReadList   = []string{FieldID, FieldCreatedAt, FieldUpdatedAt, FieldSummary, FieldMessageIDs, FieldToolIDs}
```

替换为：

```go
	SessionRepoFieldsReadList   = []string{FieldID, FieldCreatedAt, FieldUpdatedAt, FieldSummary}
```

- [ ] **Step 2: 修改三个 List 方法使用 Raw SQL 带 array_length**

因为 GORM 的 `Select(fields)` 不支持 `array_length()` 聚合表达式，需要用 Raw SQL。定义一个辅助结构体 `sessionSummaryRow` 用于扫描，三个 List 方法统一改用 Raw SQL。

在 `internal/infrastructure/repository/session_repository.go` 中，添加扫描结构体和通用查询方法：

```go
type sessionSummaryRow struct {
	ID           uint      `gorm:"column:id"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
	Summary      string    `gorm:"column:summary"`
	MessageCount int       `gorm:"column:message_count"`
	ToolCount    int       `gorm:"column:tool_count"`
}

func (r *sessionReadRepository) listSessionsRaw(ctx context.Context, where string, args []any, page, pageSize int) ([]*session.SessionSummaryProjection, *model.PageInfo, error) {
	db := r.db.WithContext(ctx)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	countSQL := "SELECT COUNT(*) FROM sessions WHERE deleted_at = 0"
	querySQL := `SELECT id, created_at, updated_at, summary,
		COALESCE(array_length(message_ids, 1), 0) AS message_count,
		COALESCE(array_length(tool_ids, 1), 0) AS tool_count
		FROM sessions WHERE deleted_at = 0`

	if where != "" {
		countSQL += " AND " + where
		querySQL += " AND " + where
	}
	querySQL += " ORDER BY id ASC LIMIT ? OFFSET ?"

	var total int64
	if err := db.Raw(countSQL, args...).Scan(&total).Error; err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "count sessions")
	}

	queryArgs := append(args, pageSize, offset)
	var rows []sessionSummaryRow
	if err := db.Raw(querySQL, queryArgs...).Scan(&rows).Error; err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "list sessions raw")
	}

	out := make([]*session.SessionSummaryProjection, 0, len(rows))
	for _, row := range rows {
		out = append(out, &session.SessionSummaryProjection{
			ID:           row.ID,
			CreatedAt:    row.CreatedAt,
			UpdatedAt:    row.UpdatedAt,
			Summary:      row.Summary,
			MessageCount: row.MessageCount,
			ToolCount:    row.ToolCount,
		})
	}
	return out, &model.PageInfo{Page: page, PageSize: pageSize, Total: total}, nil
}
```

然后修改三个 List 方法调用 `listSessionsRaw`：

- `ListSessions`:
```go
func (r *sessionReadRepository) ListSessions(ctx context.Context, owner string, page, pageSize int) ([]*session.SessionSummaryProjection, *model.PageInfo, error) {
	return r.listSessionsRaw(ctx, "api_key_name = ?", []any{owner}, page, pageSize)
}
```

- `ListAllSessions`:
```go
func (r *sessionReadRepository) ListAllSessions(ctx context.Context, page, pageSize int) ([]*session.SessionSummaryProjection, *model.PageInfo, error) {
	return r.listSessionsRaw(ctx, "", nil, page, pageSize)
}
```

- `ListSessionsByOwnerNames`:
```go
func (r *sessionReadRepository) ListSessionsByOwnerNames(ctx context.Context, ownerNames []string, page, pageSize int) ([]*session.SessionSummaryProjection, *model.PageInfo, error) {
	return r.listSessionsRaw(ctx, "api_key_name IN ?", []any{ownerNames}, page, pageSize)
}
```

- [ ] **Step 3: 新增 FindSessionMessageIDsByIDs 方法**

在 `internal/domain/session/repository.go` 的 `SessionReadRepository` 接口添加：

```go
	// FindSessionMessageIDsByIDs 按 session ID 列表批量查询 message_ids（用于空摘要回退）
	FindSessionMessageIDsByIDs(ctx context.Context, ids []uint) (map[uint][]uint, error)
```

在 `internal/infrastructure/repository/session_repository.go` 添加实现：

```go
func (r *sessionReadRepository) FindSessionMessageIDsByIDs(ctx context.Context, ids []uint) (map[uint][]uint, error) {
	if len(ids) == 0 {
		return map[uint][]uint{}, nil
	}
	db := r.db.WithContext(ctx)
	records, err := r.sessionDAO.BatchGetByField(db, constant.WhereFieldID, ids, constant.SessionRepoFieldsSummarize)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "batch get session message ids")
	}
	result := make(map[uint][]uint, len(records))
	for _, s := range records {
		result[s.ID] = s.MessageIDs
	}
	return result, nil
}
```

- [ ] **Step 4: 验证编译**

Run: `go build ./...`
Expected: 编译失败（query handler 尚未更新 MessageIDs 引用）

---

### Task 3: 修改 Query Handler — 适配新 Projection 和回退逻辑

**Files:**
- Modify: `internal/application/session/query/jwt_session_queries.go:55-116` (Handle 方法)

- [ ] **Step 1: 修改 listSessionsByUserHandler.Handle**

将空摘要回退逻辑改为：先收集空摘要 session ID → 查 message_ids → 查用户消息。

替换第 81-116 行为：

```go
	views := make([]*SessionSummaryView, 0, len(projections))

	var emptySummaryIDs []uint
	for _, p := range projections {
		if p.Summary == "" {
			emptySummaryIDs = append(emptySummaryIDs, p.ID)
		}
	}

	var sessionMsgIDs map[uint][]uint
	var msgByID map[uint]*session.MessageDetailProjection
	if len(emptySummaryIDs) > 0 {
		var batchErr error
		sessionMsgIDs, batchErr = h.readRepo.FindSessionMessageIDsByIDs(ctx, emptySummaryIDs)
		if batchErr != nil {
			log.Error("[SessionQuery] Failed to batch load message IDs for empty summary", zap.Error(batchErr))
		} else {
			var allMsgIDs []uint
			for _, ids := range sessionMsgIDs {
				allMsgIDs = append(allMsgIDs, ids...)
			}
			if len(allMsgIDs) > 0 {
				messages, msgErr := h.readRepo.FindMessagesByIDs(ctx, lo.Uniq(allMsgIDs))
				if msgErr != nil {
					log.Error("[SessionQuery] Failed to batch load messages for empty summary", zap.Error(msgErr))
				} else {
					msgByID = lo.SliceToMap(messages, func(m *session.MessageDetailProjection) (uint, *session.MessageDetailProjection) {
						return m.ID, m
					})
				}
			}
		}
	}

	for _, p := range projections {
		summary := p.Summary
		if summary == "" {
			summary = firstUserMessageContent(sessionMsgIDs[p.ID], msgByID)
		}

		views = append(views, &SessionSummaryView{
			ID:           p.ID,
			CreatedAt:    p.CreatedAt,
			UpdatedAt:    p.UpdatedAt,
			Summary:      summary,
			MessageCount: p.MessageCount,
			ToolCount:    p.ToolCount,
		})
	}
	return views, pageInfo, nil
```

- [ ] **Step 2: 验证编译**

Run: `go build ./...`
Expected: 编译失败（DTO 和 Handler 尚未更新）

---

### Task 4: 修改 DTO 和 Handler 映射

**Files:**
- Modify: `internal/dto/session.go:20-21`
- Modify: `internal/handler/session.go:91-100`
- Modify: `test/unit/session_dto/fixtures/cases.json:9-10`

- [ ] **Step 1: 修改 DTO 结构体**

将 `internal/dto/session.go` 第 20-21 行：

```go
	MessageIDs []uint            `json:"messageIds" doc:"消息ID列表"`
	ToolIDs    []uint            `json:"toolIds" doc:"工具ID列表"`
```

替换为：

```go
	MessageCount int `json:"messageCount" doc:"消息数量"`
	ToolCount    int `json:"toolCount" doc:"工具数量"`
```

- [ ] **Step 2: 修改 Handler 映射**

将 `internal/handler/session.go` 第 91-100 行的 `lo.Map` 闭包改为：

```go
	rsp.Sessions = lo.Map(views, func(v *sessionquery.SessionSummaryView, _ int) *dto.SessionSummary {
		return &dto.SessionSummary{
			ID:           v.ID,
			CreatedAt:    v.CreatedAt,
			UpdatedAt:    v.UpdatedAt,
			Summary:      v.Summary,
			MessageCount: v.MessageCount,
			ToolCount:    v.ToolCount,
		}
	})
```

- [ ] **Step 3: 更新测试 fixture**

将 `test/unit/session_dto/fixtures/cases.json` 第 9-10 行：

```json
"messageIds": [10, 20, 30],
"toolIds": [100, 200]
```

替换为：

```json
"messageCount": 3,
"toolCount": 2
```

- [ ] **Step 4: 验证编译**

Run: `go build ./...`
Expected: 编译通过

- [ ] **Step 5: 运行单元测试**

Run: `go test -v -count=1 -run TestSessionSummary ./test/unit/session_dto/`
Expected: PASS

- [ ] **Step 6: 运行 lint**

Run: `make lint`
Expected: PASS

- [ ] **Step 7: 运行全量测试**

Run: `go test -count=1 ./...`
Expected: PASS

---

### Task 5: 更新前端类型和 UI

**Files:**
- Modify: `web/src/lib/types.ts:80-86`
- Modify: `web/src/app/(dashboard)/page.tsx`
- Modify: `web/src/app/(dashboard)/sessions/page.tsx`

- [ ] **Step 1: 更新 SessionSummary 类型**

在 `web/src/lib/types.ts` 第 80-86 行，将：

```typescript
export interface SessionSummary {
  id: number;
  createdAt: string;
  updatedAt: string;
  summary: string;
  metadata?: Record<string, string>;
}
```

改为：

```typescript
export interface SessionSummary {
  id: number;
  createdAt: string;
  updatedAt: string;
  summary: string;
  messageCount: number;
  toolCount: number;
  metadata?: Record<string, string>;
}
```

- [ ] **Step 2: 在 dashboard 首页添加消息数 Badge**

在 `web/src/app/(dashboard)/page.tsx` 的 session 列表项中，在第 177 行 `</div>` 之后、第 179 行 `</Link>` 之前，添加：

```tsx
                      <Badge variant="secondary" className="ml-2 shrink-0 text-xs">
                        {s.messageCount ?? 0} msgs
                      </Badge>
```

确认文件顶部已导入 `Badge`（搜索 `Badge` import，若无则从 `@/components/ui/badge` 添加）。

- [ ] **Step 3: 在 sessions 列表页添加 Messages 列**

在 `web/src/app/(dashboard)/sessions/page.tsx`：

表头（约第 88-92 行）在 `Created` 列之前添加：

```tsx
                    <TableHead>Messages</TableHead>
```

表体行（约第 103-111 行）在 Created 单元格之前添加：

```tsx
                      <TableCell>{s.messageCount ?? 0}</TableCell>
```

- [ ] **Step 4: 验证前端构建**

Run: `cd web && npm run build`
Expected: PASS

---

### Task 6: 提交

- [ ] **Step 1: 提交变更**

```bash
git add internal/domain/session/repository.go internal/application/session/query/session_queries.go internal/application/session/query/jwt_session_queries.go internal/infrastructure/repository/session_repository.go internal/common/constant/sql.go internal/dto/session.go internal/handler/session.go test/unit/session_dto/fixtures/cases.json web/src/lib/types.ts web/src/app/\(dashboard\)/page.tsx web/src/app/\(dashboard\)/sessions/page.tsx
git commit -m "feat: replace messageIds/toolIds with messageCount/toolCount in session list API"
```
