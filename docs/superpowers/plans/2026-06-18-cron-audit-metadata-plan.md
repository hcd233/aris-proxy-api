# Cron 审计日志 Metadata 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 `cron_call_audits` 表新增 `metadata` JSONB 字段，让每个 cron 任务在执行时写入本批次统计元数据，并在前端审计表格中展示。

**Architecture:** cron 任务函数返回 `map[string]any` → `wrapCronFunc` 收集后传给 `saveCronCallAudit` → 存入 GORM serializer:json 映射的 JSONB 列 → API 查询时映射到 DTO → 前端渲染为紧凑标签。

**Tech Stack:** Go 1.25.1, PostgreSQL JSONB, GORM serializer:json, Next.js + TypeScript

---

### Task 1: 数据库 Migration

**Files:**
- Create: `migrations/YYYYMMDD_add_cron_audit_metadata.sql`（按项目现有 migration 命名惯例）

- [ ] **Step 1: 创建 migration SQL 文件**

```sql
ALTER TABLE cron_call_audits ADD COLUMN metadata JSONB NOT NULL DEFAULT '{}';
```

- [ ] **Step 2: 执行 migration**

Run: 在 PostgreSQL 中执行该 SQL，或通过项目的 migration 工具执行

---

### Task 2: Model 层 — 添加 Metadata 字段

**Files:**
- Modify: `internal/infrastructure/database/model/cron.go:36-45`

- [ ] **Step 1: 修改 CronCallAudit 结构体**

```go
type CronCallAudit struct {
	BaseModel
	CronName   string         `json:"cron_name" gorm:"column:cron_name;not null;comment:任务名;index"`
	TraceID    string         `json:"trace_id" gorm:"column:trace_id;comment:Trace ID"`
	StartedAt  time.Time      `json:"started_at" gorm:"column:started_at;not null;comment:开始时间"`
	EndedAt    time.Time      `json:"ended_at" gorm:"column:ended_at;comment:结束时间"`
	DurationMs int64          `json:"duration_ms" gorm:"column:duration_ms;comment:执行耗时(ms)"`
	Status     string         `json:"status" gorm:"column:status;not null;comment:执行状态"`
	Message    string         `json:"message" gorm:"column:message;comment:附加信息"`
	Metadata   map[string]any `json:"metadata" gorm:"column:metadata;type:jsonb;default:'{}';not null;serializer:json"`
}
```

- [ ] **Step 2: 确认编译通过**

Run: `go build ./...`
Expected: 编译成功

---

### Task 3: Port 层 — 添加 Metadata 到 CronCallAuditView

**Files:**
- Modify: `internal/application/cronaudit/port/handler.go:15-25`

- [ ] **Step 1: 修改 CronCallAuditView**

```go
type CronCallAuditView struct {
	ID         uint
	CronName   string
	TraceID    string
	StartedAt  time.Time
	EndedAt    time.Time
	DurationMs int64
	Status     string
	Message    string
	Metadata   map[string]any
	CreatedAt  time.Time
}
```

- [ ] **Step 2: 确认编译通过**

Run: `go build ./...`
Expected: 编译成功

---

### Task 4: Runner — 修改 wrapCronFunc 支持 metadata

**Files:**
- Modify: `internal/cron/lock_runner.go:134-196`

- [ ] **Step 1: 修改 fn 签名，从 `func(ctx context.Context)` 改为 `func(ctx context.Context) map[string]any`**

```go
func wrapCronFunc(name string, locker lock.Locker, key string, opts LockOptions, fn func(ctx context.Context) map[string]any) func() {
	return func() {
		ctx := context.WithValue(getBootstrapContext(), constant.CtxKeyTraceID, uuid.New().String())
		start := time.Now()
		var metadata map[string]any
		defer func() {
			if r := recover(); r != nil {
				cronPanicHandler(ctx, name, r)
			}
		}()

		if cronJobStore != nil {
			job, err := cronJobStore.Get(ctx, name)
			if err == nil && job != nil && !job.Enabled {
				logger.WithCtx(ctx).Info("[Cron] Cron job is disabled in DB, skip", zap.String("name", name))
				saveCronCallAudit(ctx, name, constant.CronCallAuditStatusSkipped, 0, "", nil)
				return
			}
		}

		if !RunWithLock(ctx, locker, key, opts, func(lockCtx context.Context) {
			metadata = fn(lockCtx)
		}) {
			return
		}
		durationMs := time.Since(start).Milliseconds()
		saveCronCallAudit(ctx, name, constant.CronCallAuditStatusSuccess, durationMs, "", metadata)
	}
}
```

- [ ] **Step 2: 修改 saveCronCallAudit 接收 metadata 参数**

```go
func saveCronCallAudit(ctx context.Context, name, status string, durationMs int64, message string, metadata map[string]any) {
	if cronCallAuditStore == nil {
		return
	}
	now := time.Now().UTC()
	audit := &cronauditport.CronCallAuditView{
		CronName:   name,
		TraceID:    util.CtxValueString(ctx, constant.CtxKeyTraceID),
		StartedAt:  now.Add(-time.Duration(durationMs) * time.Millisecond),
		EndedAt:    now,
		DurationMs: durationMs,
		Status:     status,
		Message:    message,
		Metadata:   metadata,
	}
	if err := cronCallAuditStore.Save(ctx, audit); err != nil {
		logger.WithCtx(ctx).Error("[Cron] Save cron call audit failed",
			zap.String("name", name),
			zap.Error(err),
		)
	}
}
```

- [ ] **Step 3: 修改 cronPanicHandler 传入 nil metadata**

```go
func cronPanicHandler(ctx context.Context, name string, r any) {
	logger.WithCtx(ctx).Error("[Cron] Panic recovered",
		zap.String("name", name),
		zap.Any("panic", r),
		zap.Stack("stack"),
	)
	saveCronCallAudit(ctx, name, constant.CronCallAuditStatusPanic, 0, fmt.Sprintf(constant.CronPanicMessageTemplate, r), nil)
}
```

- [ ] **Step 4: 确认编译通过**

Run: `go build ./...`
Expected: 编译成功

---

### Task 5: Cron 任务实现 — SessionDeduplicateCron

**Files:**
- Modify: `internal/cron/session_dedup.go:106`

- [ ] **Step 1: 修改 deduplicate 函数签名和返回值**

```go
func (c *SessionDeduplicateCron) deduplicate(ctx context.Context) map[string]any {
	log := logger.WithCtx(ctx)
	db := c.db.WithContext(ctx)

	sessions, err := c.sessionDAO.BatchGet(db, &dbmodel.Session{}, constant.SessionRepoFieldsDedup)
	if err != nil {
		log.Error("[SessionDeduplicateCron] Failed to load sessions", zap.Error(err))
		return nil
	}

	checkedCount := len(sessions)

	if len(sessions) < 2 {
		log.Info("[SessionDeduplicateCron] Skip deduplication, not enough sessions", zap.Int("count", checkedCount))
		return map[string]any{
			"checked_sessions_count": checkedCount,
			"deduped_sessions_count": 0,
		}
	}

	mergeResult := FindRedundantSessionsWithMerge(sessions)

	messages, err := c.loadLastMessagesForTerminalToolCheck(db, sessions, mergeResult.RedundantIDs)
	if err != nil {
		log.Error("[SessionDeduplicateCron] Failed to load last messages for terminal tool call check", zap.Error(err))
	}

	if len(messages) > 0 {
		terminalToolCallResult := FindTerminalToolCallSessions(sessions, messages, mergeResult.RedundantIDs)
		if len(terminalToolCallResult.RedundantIDs) > 0 {
			mergeResult.RedundantIDs = append(mergeResult.RedundantIDs, terminalToolCallResult.RedundantIDs...)
			for sessionID, toolIDSet := range terminalToolCallResult.MergeMapping {
				mergeResult.MergeMapping[sessionID] = mergeToolIDs(mergeResult.MergeMapping[sessionID], toolIDSet)
			}
		}
	}

	if len(mergeResult.RedundantIDs) == 0 {
		log.Info("[SessionDeduplicateCron] No redundant sessions found", zap.Int("total", checkedCount))
		return map[string]any{
			"checked_sessions_count": checkedCount,
			"deduped_sessions_count": 0,
		}
	}

	mergedCount := 0
	for sessionID, toolIDSet := range mergeResult.MergeMapping {
		if len(toolIDSet) == 0 {
			continue
		}
		mergedToolIDs := lo.Keys(toolIDSet)
		slices.Sort(mergedToolIDs)
		err := c.sessionDAO.Update(db, &dbmodel.Session{ID: sessionID}, map[string]any{
			constant.FieldToolIDs: lo.Must1(sonic.MarshalString(mergedToolIDs)),
		})
		if err != nil {
			log.Error("[SessionDeduplicateCron] Failed to update session tool_ids",
				zap.Uint("sessionID", sessionID),
				zap.Error(err))
			continue
		}
		mergedCount++
	}

	err = c.sessionDAO.BatchDeleteByField(db, constant.WhereFieldID, mergeResult.RedundantIDs)
	if err != nil {
		log.Error("[SessionDeduplicateCron] Failed to delete redundant sessions", zap.Error(err))
		return nil
	}

	log.Info("[SessionDeduplicateCron] Deduplication completed",
		zap.Int("total", checkedCount),
		zap.Int("deleted", len(mergeResult.RedundantIDs)),
		zap.Int("merged", mergedCount))

	return map[string]any{
		"checked_sessions_count": checkedCount,
		"deduped_sessions_count": len(mergeResult.RedundantIDs),
	}
}
```

- [ ] **Step 2: 确认编译通过**

Run: `go build ./...`

---

### Task 6: Cron 任务实现 — SoftDeletePurgeCron

**Files:**
- Modify: `internal/cron/soft_delete_purge.go:103`

- [ ] **Step 1: 修改 purge 函数签名和返回值**

```go
func (c *SoftDeletePurgeCron) purge(ctx context.Context) map[string]any {
	log := logger.WithCtx(ctx)
	db := c.db.WithContext(ctx)

	softDeletedSessions, err := c.sessionDAO.FindAllForPurge(db, true)
	if err != nil {
		log.Error("[SoftDeletePurgeCron] Failed to find soft deleted sessions", zap.Error(err))
		return nil
	}

	if len(softDeletedSessions) == 0 {
		log.Info("[SoftDeletePurgeCron] No soft deleted sessions found")
		return map[string]any{
			"purged_messages_count": 0,
			"purged_tools_count":    0,
			"retention_days":        config.SoftDeleteRetentionDays(),
		}
	}

	candidateMessageIDs := lo.Uniq(lo.Flatten(lo.Map(softDeletedSessions, func(s dao.SessionPurgeView, _ int) []uint {
		return s.MessageIDs
	})))
	candidateToolIDs := lo.Uniq(lo.Flatten(lo.Map(softDeletedSessions, func(s dao.SessionPurgeView, _ int) []uint {
		return s.ToolIDs
	})))

	activeSessions, err := c.sessionDAO.FindAllForPurge(db, false)
	if err != nil {
		log.Error("[SoftDeletePurgeCron] Failed to find active sessions", zap.Error(err))
		return nil
	}

	usedMessageIDs := lo.Uniq(lo.Flatten(lo.Map(activeSessions, func(s dao.SessionPurgeView, _ int) []uint {
		return s.MessageIDs
	})))
	usedToolIDs := lo.Uniq(lo.Flatten(lo.Map(activeSessions, func(s dao.SessionPurgeView, _ int) []uint {
		return s.ToolIDs
	})))

	orphanMessageIDs, _ := lo.Difference(candidateMessageIDs, usedMessageIDs)
	orphanToolIDs, _ := lo.Difference(candidateToolIDs, usedToolIDs)

	var msgCount, toolCount int64
	if len(orphanMessageIDs) > 0 {
		msgCount, err = c.messageDAO.HardDeleteByIDs(db, orphanMessageIDs)
		if err != nil {
			log.Error("[SoftDeletePurgeCron] Failed to purge messages", zap.Error(err))
			return nil
		}
	}

	if len(orphanToolIDs) > 0 {
		toolCount, err = c.toolDAO.HardDeleteByIDs(db, orphanToolIDs)
		if err != nil {
			log.Error("[SoftDeletePurgeCron] Failed to purge tools", zap.Error(err))
			return nil
		}
	}

	sessionCount, err := c.sessionDAO.HardDeleteSoftDeleted(db)
	if err != nil {
		log.Error("[SoftDeletePurgeCron] Failed to purge sessions", zap.Error(err))
		return nil
	}

	log.Info("[SoftDeletePurgeCron] Purge completed",
		zap.Int64("sessionsDeleted", sessionCount),
		zap.Int64("messagesDeleted", msgCount),
		zap.Int64("toolsDeleted", toolCount))

	return map[string]any{
		"purged_messages_count": msgCount,
		"purged_tools_count":    toolCount,
		"retention_days":        config.SoftDeleteRetentionDays(),
	}
}
```

- [ ] **Step 2: 检查 config 包中 SoftDeleteRetentionDays 是否存在，如不存在则直接在 purge 中硬编码或从现有配置读取**

先搜索现有配置方式：
```go
// 需要在 import 中确认已有 config 包
```

- [ ] **Step 3: 确认编译通过**

Run: `go build ./...`

---

### Task 7: Cron 任务实现 — ThinkExtractCron

**Files:**
- Modify: `internal/cron/think_extract.go:105`

- [ ] **Step 1: 修改 extract 函数签名和返回值**

```go
func (c *ThinkExtractCron) extract(ctx context.Context) map[string]any {
	log := logger.WithCtx(ctx)
	startTime, endTime := currentDayRange(time.Now().UTC())

	var lastID uint
	totalScanned := 0
	totalProcessed := 0

	for {
		messages, err := c.repo.FindThinkExtractCandidates(ctx, lastID, startTime, endTime, config.SQLBatchSize)
		if err != nil {
			log.Error("[ThinkExtractCron] Query error", zap.Error(err))
			return nil
		}

		if len(messages) == 0 {
			break
		}

		totalScanned += len(messages)

		for _, msg := range messages {
			lastID = msg.ID

			if msg.Message == nil || msg.Message.ReasoningContent != "" {
				continue
			}

			extracted := extractThinkFromContent(msg.Message)
			if extracted == "" {
				continue
			}

			msg.Message.ReasoningContent = extracted
			if err := c.repo.UpdateMessageContent(ctx, msg.ID, msg.Message); err != nil {
				log.Error("[ThinkExtractCron] Update error", zap.Uint("id", msg.ID), zap.Error(err))
				continue
			}
			totalProcessed++
		}

		if len(messages) < config.SQLBatchSize {
			break
		}

		log.Info("[ThinkExtractCron] Batch processed",
			zap.Int("batchSize", len(messages)),
			zap.Uint("lastID", lastID))
	}

	log.Info("[ThinkExtractCron] Extract completed", zap.Int("totalProcessed", totalProcessed))

	return map[string]any{
		"scanned_messages_count":   totalScanned,
		"extracted_messages_count": totalProcessed,
	}
}
```

- [ ] **Step 2: 确认编译通过**

Run: `go build ./...`

---

### Task 8: Cron 任务实现 — BlockedHitSyncCron

**Files:**
- Modify: `internal/cron/blocked_hit_sync.go:62`

- [ ] **Step 1: 修改 sync 函数签名和返回值**

```go
func (c *blockedHitSyncCron) sync(ctx context.Context) map[string]any {
	hits, err := c.hitCache.PopAll(ctx)
	if err != nil {
		logger.WithCtx(ctx).Error("[BlockedHitSync] Failed to pop hit counts", zap.Error(err))
		return nil
	}
	if len(hits) == 0 {
		return map[string]any{
			"synced_hits_count": 0,
		}
	}
	err = c.blockedRepo.BatchIncrementHitCount(ctx, hits)
	if err != nil {
		logger.WithCtx(ctx).Error("[BlockedHitSync] Failed to batch increment hit counts", zap.Error(err))
		return nil
	}
	logger.WithCtx(ctx).Info("[BlockedHitSync] Synced hit counts",
		zap.Int("count", len(hits)))
	return map[string]any{
		"synced_hits_count": len(hits),
	}
}
```

- [ ] **Step 2: 确认编译通过**

Run: `go build ./...`

---

### Task 9: Repository — 映射 Metadata 字段

**Files:**
- Modify: `internal/infrastructure/repository/cron_audit_repository.go:44-59` (Save)
- Modify: `internal/infrastructure/repository/cron_audit_repository.go` (List 中 view 映射部分)

- [ ] **Step 1: Save 方法添加 Metadata 映射**

```go
func (r *cronCallAuditRepository) Save(ctx context.Context, audit *port.CronCallAuditView) error {
	record := &dbmodel.CronCallAudit{
		CronName:   audit.CronName,
		TraceID:    audit.TraceID,
		StartedAt:  audit.StartedAt,
		EndedAt:    audit.EndedAt,
		DurationMs: audit.DurationMs,
		Status:     audit.Status,
		Message:    audit.Message,
		Metadata:   audit.Metadata,
	}
	if err := r.dao.Create(r.db.WithContext(ctx), record); err != nil {
		return ierr.Wrap(ierr.ErrDBCreate, err, "create cron call audit")
	}
	audit.ID = record.ID
	return nil
}
```

- [ ] **Step 2: List 方法中追加 Metadata 映射**

读取 `cron_audit_repository.go` 的 List 方法，在 model → view 的映射中加入 `Metadata: record.Metadata`

- [ ] **Step 3: 确认编译通过**

Run: `go build ./...`

---

### Task 10: Handler — 映射 Metadata 到 DTO

**Files:**
- Modify: `internal/handler/cron.go:129-141`

- [ ] **Step 1: HandleListCronCallAudits 中追加 Metadata 映射**

```go
	rsp.Logs = lo.Map(logs, func(log *cronauditport.CronCallAuditView, _ int) *dto.CronCallAuditItem {
		return &dto.CronCallAuditItem{
			ID:         log.ID,
			CronName:   log.CronName,
			TraceID:    log.TraceID,
			StartedAt:  log.StartedAt,
			EndedAt:    log.EndedAt,
			DurationMs: log.DurationMs,
			Status:     log.Status,
			Message:    log.Message,
			Metadata:   log.Metadata,
			CreatedAt:  log.CreatedAt,
		}
	})
```

- [ ] **Step 2: 确认编译通过**

Run: `go build ./...`

---

### Task 11: DTO — 添加 Metadata 字段

**Files:**
- Modify: `internal/dto/cron.go:101-111`

- [ ] **Step 1: CronCallAuditItem 加 Metadata**

```go
type CronCallAuditItem struct {
	ID         uint           `json:"id" doc:"记录ID"`
	CronName   string         `json:"cronName" doc:"任务名"`
	TraceID    string         `json:"traceId" doc:"Trace ID"`
	StartedAt  time.Time      `json:"startedAt" doc:"开始时间"`
	EndedAt    time.Time      `json:"endedAt" doc:"结束时间"`
	DurationMs int64          `json:"durationMs" doc:"执行耗时(ms)"`
	Status     string         `json:"status" doc:"执行状态"`
	Message    string         `json:"message" doc:"附加信息"`
	Metadata   map[string]any `json:"metadata" doc:"执行元数据"`
	CreatedAt  time.Time      `json:"createdAt" doc:"创建时间"`
}
```

- [ ] **Step 2: 确认编译通过**

Run: `go build ./...`

---

### Task 12: 前端类型 — 添加 metadata 字段

**Files:**
- Modify: `web/src/lib/types.ts:522-532`

- [ ] **Step 1: CronCallAuditItem 加 metadata**

```typescript
export interface CronCallAuditItem {
  id: number;
  cronName: string;
  traceId: string;
  startedAt: string;
  endedAt: string;
  durationMs: number;
  status: string;
  message: string;
  metadata: Record<string, number>;
  createdAt: string;
}
```

- [ ] **Step 2: 确认 TypeScript 编译通过**

Run: `cd web && npx tsc --noEmit`
Expected: 无类型错误

---

### Task 13: 前端页面 — 新增 Metadata 列

**Files:**
- Modify: `web/src/app/(dashboard)/audit/cron/page.tsx`

- [ ] **Step 1: 添加 metadata 标签映射和格式化函数**

在 `statusLabelMap` 下方添加：

```typescript
const metadataLabelMap: Record<string, string> = {
  checked_sessions_count: "Checked",
  deduped_sessions_count: "Deduped",
  purged_messages_count: "Messages",
  purged_tools_count: "Tools",
  retention_days: "Retention",
  scanned_messages_count: "Scanned",
  extracted_messages_count: "Extracted",
  synced_hits_count: "Synced Hits",
};

function formatMetadata(metadata: Record<string, number> | undefined | null): string {
  if (!metadata || Object.keys(metadata).length === 0) return "—";
  return Object.entries(metadata)
    .map(([key, val]) => `${metadataLabelMap[key] ?? key}: ${val}`)
    .join(" | ");
}
```

- [ ] **Step 2: 表格头新增 Metadata 列**

在 `<TableHead>Error Message</TableHead>` 后添加：

```tsx
<TableHead>Metadata</TableHead>
```

- [ ] **Step 3: 表格体新增 Metadata 单元格**

在 Error Message 的 `<TableCell>` 后添加：

```tsx
<TableCell className="max-w-[250px] truncate text-xs text-muted-foreground">
  {formatMetadata(log.metadata)}
</TableCell>
```

- [ ] **Step 4: 确认前端编译通过**

Run: `cd web && npm run build`
Expected: 构建成功

---

### Task 14: 后端编译 & 测试验证

- [ ] **Step 1: 全量编译**

Run: `go build ./...`
Expected: 编译成功

- [ ] **Step 2: 运行 lint**

Run: `make lint`
Expected: 无新增 lint 错误

- [ ] **Step 3: 运行现有测试**

Run: `go test -count=1 ./...`
Expected: 所有现有测试通过

---

### Task 15: 集成验证

- [ ] **Step 1: 启动本地服务**

Run: `go run main.go server start --host localhost --port 8080`

- [ ] **Step 2: 验证 cron 审计日志 API 返回 metadata 字段**

Run: `curl -s http://localhost:8080/api/v1/audit/cron/log/list?page=1\&pageSize=5 | jq '.logs[0].metadata'`
Expected: 返回 `null` 或 `{}`（历史数据无 metadata）或包含预期字段的 object（新数据）

- [ ] **Step 3: 验证前端页面**

Run: `cd web && npm run dev`
Open: `http://localhost:3000/web/audit/cron`
Expected: 表格中出现 "Metadata" 列
