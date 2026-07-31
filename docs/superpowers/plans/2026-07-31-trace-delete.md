# Agent Trace 删除功能实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 agent trace 增加删除能力：软删除 trace + 级联软删 events，上报侧拦截防止已删 trace 复活，Web 前端列表页（单条+批量）与详情页删除入口。

**Architecture:** 沿用 Session 删除的成熟模式（软删除 + 逗号分隔批量 + 失败收集 + owner/admin 授权）。后端按分层自底向上：Domain 接口 → Infrastructure 实现 → Application command → DTO/Handler/Router/DI。前端复用 sessions 页的勾选批量删除交互。上报拦截在 `reportTraceEventHandler.Handle` 中用 `FindBySessionIDIncludingDeleted` 感知软删状态。

**Tech Stack:** Go 1.x（gorm + huma + dig + samber/lo）、Next.js（App Router + shadcn/ui + i18n）、PostgreSQL。

## Global Constraints

- 开发必须在 git worktree 中进行，禁止在主工作区直接开发（AGENTS.md workflow 规范）
- 分支命名：`feature/trace-delete-2026-07-31`
- 写 Go 代码必须加载 `golang-samber-lo` 与 `golang-samber-mo` skill；遵循 `golang-code-style` / `golang-naming`
- 实现遵循 ponytail 原则：YAGNI、不建投机抽象、只动必要代码
- TDD：每个功能先写失败测试，再实现，再验证
- 测试命令：聚焦 `go test -v -count=1 -run TestXxx ./test/unit/trace/`；全量 `make test`
- lint：`make lint`（lint-conv + lint-static）；前端 `cd web && npm run lint`
- 前端三语 locale 必须同步修改：`web/src/locales/{zh,en,ja}.json`
- 删除语义：软删除（`deleted_at` 置位，`time.Now().UTC().Unix()`），与 Session 一致
- 已软删 session 继续上报 → 全部 records `rejected`（message: trace deleted），不重建、不插入、不报错

---

### Task 0: 创建 worktree 与分支

**Files:**
- 无代码文件

**Interfaces:**
- 无

- [ ] **Step 1: 创建 worktree 并 checkout 分支**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api
git worktree add -b feature/trace-delete-2026-07-31 .worktrees/trace-delete-2026-07-31
cd .worktrees/trace-delete-2026-07-31
```

- [ ] **Step 2: 确认 worktree 就绪**

Run: `git branch --show-current && git log --oneline -1`
Expected: 输出 `feature/trace-delete-2026-07-31` 且最近提交包含 `fdb6521e docs(spec): agent trace 删除功能设计`

- [ ] **Step 3: 读 skill 与工程经验（写 Go 代码前必读）**

- 加载 `golang-samber-lo`、`golang-samber-mo` skill（AGENTS.md 强制）
- `serena_list_memories` 读取相关经验（`engineering/ponytail-dead-code-cleanup-2026-07-31` 等）
- 参考文件：`internal/application/session/command/delete_session.go`、`internal/handler/session.go` 的 `HandleDeleteSession` 与 `parseCommaSeparatedIDs`

---

### Task 1: 常量与 Domain 接口

**Files:**
- Modify: `internal/common/constant/string.go`（新增 4 个删除错误常量）
- Modify: `internal/common/constant/sql.go`（新增 `TraceRecordMessageTraceDeleted`）
- Modify: `internal/domain/trace/repository.go`（`Trace` 加 `DeletedAt`；`TraceRepository` 加 2 个方法）

**Interfaces:**
- Produces:
  - `constant.TraceDeleteErrorFindFailed = "failed to find trace"`
  - `constant.TraceDeleteErrorNotFound = "trace not found"`
  - `constant.TraceDeleteErrorNoPermission = "no permission"`
  - `constant.TraceDeleteErrorDeleteFailed = "failed to delete"`
  - `constant.TraceRecordMessageTraceDeleted = "trace deleted"`
  - `trace.Trace.DeletedAt int64` 字段
  - `TraceRepository.Delete(ctx context.Context, id uint) error`
  - `TraceRepository.FindBySessionIDIncludingDeleted(ctx context.Context, sessionID string) (*Trace, error)`（未找到返回 `(nil, nil)`）

- [ ] **Step 1: 加常量**

`internal/common/constant/string.go` 中 `SessionDeleteError*` 块之后追加：

```go
	TraceDeleteErrorFindFailed   = "failed to find trace"
	TraceDeleteErrorNotFound     = "trace not found"
	TraceDeleteErrorNoPermission = "no permission"
	TraceDeleteErrorDeleteFailed = "failed to delete"
```

`internal/common/constant/sql.go` 中 `TraceRecordMessageStorageFailed` 之后追加：

```go
	TraceRecordMessageTraceDeleted     = "trace deleted"
```

（注意该块为对齐格式，若相邻常量缩进不同以 gofmt 为准）

- [ ] **Step 2: 改 Domain**

`internal/domain/trace/repository.go`：

```go
// Trace 一次 agent 运行（领域结构体）
type Trace struct {
	ID         uint
	Agent      string
	SessionID  string
	APIKeyName string
	UserID     uint
	Model      string
	CWD        string
	Source     string
	Status     string
	Metadata   map[string]string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  int64
}
```

`TraceRepository` 接口中 `ListEvents` 之后追加：

```go
	// FindBySessionIDIncludingDeleted 按 session_id 查询（含软删）；未找到返回 (nil, nil)
	FindBySessionIDIncludingDeleted(ctx context.Context, sessionID string) (*Trace, error)
	// Delete 软删除 trace 并级联软删其 events（事务）
	Delete(ctx context.Context, id uint) error
```

- [ ] **Step 3: 编译验证**

Run: `go build ./internal/...`
Expected: 编译失败，提示 `TraceRepository` 缺少方法（fake repo 尚未实现，属预期）。确认仅此一类错误后继续 Task 2。

---

### Task 2: Infrastructure 实现

**Files:**
- Modify: `internal/infrastructure/repository/trace_repository.go`（`toTraceDomain` 加 `DeletedAt`；新增 2 个方法）

**Interfaces:**
- Consumes: Task 1 的 `trace.Trace.DeletedAt`、`constant.FieldDeletedAt`、`constant.FieldTraceID`
- Produces: `traceRepository` 实现 `Delete` / `FindBySessionIDIncludingDeleted`

- [ ] **Step 1: `toTraceDomain` 回填 DeletedAt**

```go
func toTraceDomain(m *dbmodel.Trace) *trace.Trace {
	return &trace.Trace{
		ID: m.ID, Agent: m.Agent, SessionID: m.SessionID, APIKeyName: m.APIKeyName,
		UserID: m.UserID, Model: m.Model, CWD: m.CWD, Source: m.Source,
		Status: m.Status, Metadata: m.Metadata, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
		DeletedAt: m.DeletedAt,
	}
}
```

- [ ] **Step 2: 实现 `FindBySessionIDIncludingDeleted`**

在 `FindBySessionID` 方法后新增：

```go
func (r *traceRepository) FindBySessionIDIncludingDeleted(ctx context.Context, sessionID string) (*trace.Trace, error) {
	db := r.db.WithContext(ctx)
	var rec dbmodel.Trace
	err := db.Unscoped().Where(&dbmodel.Trace{SessionID: sessionID}).First(&rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "find trace by session including deleted")
	}
	return toTraceDomain(&rec), nil
}
```

- [ ] **Step 3: 实现 `Delete`（事务：软删 trace + 级联软删 events）**

```go
func (r *traceRepository) Delete(ctx context.Context, id uint) error {
	db := r.db.WithContext(ctx)
	now := time.Now().UTC().Unix()
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&dbmodel.Trace{}).Where(constant.FieldID+" = ?", id).
			Update(constant.FieldDeletedAt, now).Error; err != nil {
			return ierr.Wrap(ierr.ErrDBDelete, err, "soft delete trace")
		}
		if err := tx.Model(&dbmodel.TraceEvent{}).Where(constant.FieldTraceID+" = ?", id).
			Update(constant.FieldDeletedAt, now).Error; err != nil {
			return ierr.Wrap(ierr.ErrDBDelete, err, "soft delete trace events")
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}
```

确认 `internal/common/constant/sql.go` 存在 `FieldID`（如不存在则改用字符串 `"id"`）。

- [ ] **Step 4: 编译验证**

Run: `go build ./internal/...`
Expected: 仅剩 fake repository 未实现 `TraceRepository` 接口的编译错误（Task 4 修复）。

---

### Task 3: Port 定义

**Files:**
- Modify: `internal/application/trace/port/handler.go`（文件末尾追加删除命令/结果/handler 接口）

**Interfaces:**
- Produces:
  - `port.DeleteTraceCommand{ UserID uint; IsAdmin bool; IDs []uint }`
  - `port.DeleteTraceFailedItem{ ID uint; Error string }`
  - `port.DeleteTraceResult{ DeletedCount int; Failures []DeleteTraceFailedItem }`
  - `port.DeleteTraceHandler interface { Handle(ctx context.Context, cmd DeleteTraceCommand) (*DeleteTraceResult, error) }`

- [ ] **Step 1: 追加 port 定义**

`internal/application/trace/port/handler.go` 文件末尾追加：

```go
// DeleteTraceCommand 删除 Trace 命令
type DeleteTraceCommand struct {
	UserID  uint
	IsAdmin bool
	IDs     []uint
}

// DeleteTraceFailedItem 删除失败项
type DeleteTraceFailedItem struct {
	ID    uint
	Error string
}

// DeleteTraceResult 删除结果
type DeleteTraceResult struct {
	DeletedCount int
	Failures     []DeleteTraceFailedItem
}

// DeleteTraceHandler 删除命令处理器接口（支持单个和批量）
type DeleteTraceHandler interface {
	Handle(ctx context.Context, cmd DeleteTraceCommand) (*DeleteTraceResult, error)
}
```

- [ ] **Step 2: 编译验证**

Run: `go build ./internal/...`
Expected: 同上，仅剩 fake repo 接口缺失错误。

---

### Task 4: 上报拦截 + fake repository 更新 + 单测

**Files:**
- Modify: `internal/application/trace/command/report_trace_event.go`（`Handle` 开头拦截 + `ensureTrace` 签名改造）
- Modify: `test/unit/trace/fake_repository.go`（实现 2 个新方法 + `FindBySessionID`/`PaginateByOwners` 过滤软删）
- Test: `test/unit/trace/report_deleted_test.go`（新建）

**Interfaces:**
- Consumes: Task 1/2 的 `FindBySessionIDIncludingDeleted`、`trace.Trace.DeletedAt`、`constant.TraceRecordMessageTraceDeleted`
- Produces: 上报拦截行为（已软删 → 全 rejected）；`ensureTrace(ctx, cmd, agent string, existing *trace.Trace) (*trace.Trace, error)`

- [ ] **Step 1: 写失败测试**

`test/unit/trace/report_deleted_test.go`：

```go
package trace

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/command"
	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// TestReportRejectsTraceDeleted 已软删 session 继续上报 → 全部 rejected 且不重建
func TestReportRejectsTraceDeleted(t *testing.T) {
	repo := NewFakeRepo()
	h := command.NewReportTraceEventHandler(repo)

	ctx := context.Background()
	// 先建一条 trace，再软删
	created, err := repo.UpsertBySessionID(ctx, &trace.Trace{Agent: constant.TraceAgentCodex, SessionID: "s-deleted", APIKeyName: "k", UserID: 1})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	results, err := h.Handle(ctx, port.ReportTraceEventCommand{
		SessionID:  "s-deleted",
		Agent:      constant.TraceAgentCodex,
		APIKeyName: "k",
		UserID:     1,
		Records: []port.ReportTraceRecord{{
			Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent,
			Event: "UserPromptSubmit", DedupKey: "hook:deleted:1", Payload: []byte(`{"x":1}`),
		}},
	})
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if len(results) != 1 || results[0].Status != constant.TraceRecordStatusRejected || results[0].Message != constant.TraceRecordMessageTraceDeleted {
		t.Fatalf("expected rejected with trace deleted, got %+v", results)
	}
	// 不重建
	again, err := repo.FindBySessionIDIncludingDeleted(ctx, "s-deleted")
	if err != nil || again == nil || again.DeletedAt == 0 {
		t.Fatalf("trace should remain soft-deleted, got %+v err=%v", again, err)
	}
}
```

注意补 import：`"github.com/hcd233/aris-proxy-api/internal/domain/trace"`。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -v -count=1 -run TestReportRejectsTraceDeleted ./test/unit/trace/`
Expected: 编译失败（fake repo 缺 `Delete` / `FindBySessionIDIncludingDeleted`）或测试失败（拦截逻辑未实现）。

- [ ] **Step 3: 更新 fake repository**

`test/unit/trace/fake_repository.go`：

- import 增加 `"time"`
- `FindBySessionID` 改为过滤软删：

```go
func (f *FakeRepo) FindBySessionID(_ context.Context, sid string) (*trace.Trace, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := f.traces[sid]
	if t != nil && t.DeletedAt != 0 {
		return nil, nil
	}
	return t, nil
}
```

- 新增两个方法（`ListEvents` 之后）：

```go
func (f *FakeRepo) FindBySessionIDIncludingDeleted(_ context.Context, sid string) (*trace.Trace, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.traces[sid], nil
}

func (f *FakeRepo) Delete(_ context.Context, id uint) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t, ok := f.byID[id]; ok {
		t.DeletedAt = time.Now().UTC().Unix()
	}
	for _, e := range f.events {
		if e.TraceID == id {
			e.DeletedAt = time.Now().UTC().Unix()
		}
	}
	return nil
}
```

- `PaginateByOwners` 过滤软删（`for _, t := range f.traces` 循环内开头加）：

```go
		if t.DeletedAt != 0 {
			continue
		}
```

- [ ] **Step 4: 实现上报拦截**

`internal/application/trace/command/report_trace_event.go`：

`Handle` 中 `records := normalizeRecords(cmd)` 之后、`t, err := h.ensureTrace(...)` 之前改为：

```go
	existing, err := h.repo.FindBySessionIDIncludingDeleted(ctx, cmd.SessionID)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.DeletedAt != 0 {
		return lo.Map(records, func(r port.ReportTraceRecord, _ int) port.ReportTraceRecordResult {
			return port.ReportTraceRecordResult{
				DedupKey: r.DedupKey,
				Status:   constant.TraceRecordStatusRejected,
				Message:  constant.TraceRecordMessageTraceDeleted,
			}
		}), nil
	}
	t, err := h.ensureTrace(ctx, cmd, agent, existing)
```

`ensureTrace` 签名与开头改为：

```go
func (h *reportTraceEventHandler) ensureTrace(
	ctx context.Context,
	cmd port.ReportTraceEventCommand,
	agent string,
	existing *trace.Trace,
) (*trace.Trace, error) {
	if existing == nil {
		return h.repo.UpsertBySessionID(ctx, &trace.Trace{
			Agent:      agent,
			SessionID:  cmd.SessionID,
			APIKeyName: cmd.APIKeyName,
			UserID:     cmd.UserID,
			Model:      cmd.Model,
			CWD:        cmd.CWD,
			Source:     cmd.Source,
			Status:     constant.TraceStatusActive,
		})
	}
	if existing.Agent != "" && existing.Agent != agent {
		return nil, ierr.New(ierr.ErrValidation, "trace agent mismatch for session")
	}

	modelName := existing.Model
	if cmd.Model != "" {
		modelName = cmd.Model
	}
	cwd := existing.CWD
	if cmd.CWD != "" {
		cwd = cmd.CWD
	}
	source := existing.Source
	if cmd.Source != "" {
		source = cmd.Source
	}
	return h.repo.UpsertBySessionID(ctx, &trace.Trace{
		ID:         existing.ID,
		Agent:      agent,
		SessionID:  cmd.SessionID,
		APIKeyName: existing.APIKeyName,
		UserID:     existing.UserID,
		Model:      modelName,
		CWD:        cwd,
		Source:     source,
		Status:     existing.Status,
		Metadata:   existing.Metadata,
	})
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `go test -v -count=1 -run 'TestReportRejectsTraceDeleted|TestReport' ./test/unit/trace/`
Expected: 新增测试 PASS，既有上报相关测试（`usecase_test.go` 等）全部 PASS（回归确认 `ensureTrace` 重构未破坏原逻辑）。

- [ ] **Step 6: Commit**

```bash
git add internal/common/constant/string.go internal/common/constant/sql.go internal/domain/trace/repository.go internal/infrastructure/repository/trace_repository.go internal/application/trace/port/handler.go internal/application/trace/command/report_trace_event.go test/unit/trace/fake_repository.go test/unit/trace/report_deleted_test.go
git commit -m "feat(trace): 上报侧拦截已软删 session，防止 trace 复活"
```

---

### Task 5: 删除命令处理器 + 单测

**Files:**
- Create: `internal/application/trace/command/delete_trace.go`
- Test: `test/unit/trace/delete_test.go`（新建）

**Interfaces:**
- Consumes: Task 1 常量、Task 3 port 定义、`apikey.APIKeyRepository`、`trace.TraceRepository`
- Produces: `command.NewDeleteTraceHandler(repo trace.TraceRepository, apiKeyRepo apikey.APIKeyRepository) port.DeleteTraceHandler`

- [ ] **Step 1: 写失败测试**

`test/unit/trace/delete_test.go`：

```go
package trace

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/command"
	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

func mustUpsert(t *testing.T, repo *FakeRepo, sessionID, owner string) uint {
	t.Helper()
	tr, err := repo.UpsertBySessionID(context.Background(), &trace.Trace{
		Agent: constant.TraceAgentCodex, SessionID: sessionID, APIKeyName: owner, UserID: 1,
	})
	if err != nil {
		t.Fatalf("upsert %s: %v", sessionID, err)
	}
	return tr.ID
}

// TestDeleteOwnerCanDeleteOwn 普通用户可删自己 API Key 名下的 trace
func TestDeleteOwnerCanDeleteOwn(t *testing.T) {
	repo := NewFakeRepo()
	keyRepo := newFakeAPIKeyRepo(map[uint][]string{1: {"key-a"}})
	h := command.NewDeleteTraceHandler(repo, keyRepo)

	id := mustUpsert(t, repo, "s-own", "key-a")

	result, err := h.Handle(context.Background(), port.DeleteTraceCommand{UserID: 1, IsAdmin: false, IDs: []uint{id}})
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if result.DeletedCount != 1 || len(result.Failures) != 0 {
		t.Fatalf("expected 1 deleted 0 failures, got %+v", result)
	}
	tr, _ := repo.FindBySessionIDIncludingDeleted(context.Background(), "s-own")
	if tr == nil || tr.DeletedAt == 0 {
		t.Fatal("trace should be soft deleted")
	}
}

// TestDeleteOwnerCannotDeleteOthers 普通用户不能删他人 trace
func TestDeleteOwnerCannotDeleteOthers(t *testing.T) {
	repo := NewFakeRepo()
	keyRepo := newFakeAPIKeyRepo(map[uint][]string{1: {"key-a"}})
	h := command.NewDeleteTraceHandler(repo, keyRepo)

	id := mustUpsert(t, repo, "s-other", "key-b")

	result, err := h.Handle(context.Background(), port.DeleteTraceCommand{UserID: 1, IsAdmin: false, IDs: []uint{id}})
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if result.DeletedCount != 0 || len(result.Failures) != 1 || result.Failures[0].Error != constant.TraceDeleteErrorNoPermission {
		t.Fatalf("expected no-permission failure, got %+v", result)
	}
}

// TestDeleteAdminCanDeleteAny admin 可删任意 trace
func TestDeleteAdminCanDeleteAny(t *testing.T) {
	repo := NewFakeRepo()
	keyRepo := newFakeAPIKeyRepo(map[uint][]string{1: {"key-a"}})
	h := command.NewDeleteTraceHandler(repo, keyRepo)

	id := mustUpsert(t, repo, "s-any", "key-b")

	result, err := h.Handle(context.Background(), port.DeleteTraceCommand{UserID: 1, IsAdmin: true, IDs: []uint{id}})
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if result.DeletedCount != 1 {
		t.Fatalf("expected 1 deleted, got %+v", result)
	}
}

// TestDeleteNotFound 不存在的 trace → NotFound 失败项
func TestDeleteNotFound(t *testing.T) {
	repo := NewFakeRepo()
	keyRepo := newFakeAPIKeyRepo(map[uint][]string{1: {"key-a"}})
	h := command.NewDeleteTraceHandler(repo, keyRepo)

	result, err := h.Handle(context.Background(), port.DeleteTraceCommand{UserID: 1, IsAdmin: true, IDs: []uint{999}})
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if len(result.Failures) != 1 || result.Failures[0].Error != constant.TraceDeleteErrorNotFound {
		t.Fatalf("expected not-found failure, got %+v", result)
	}
}

// TestDeleteBatchMixed 批量混合成功/失败
func TestDeleteBatchMixed(t *testing.T) {
	repo := NewFakeRepo()
	keyRepo := newFakeAPIKeyRepo(map[uint][]string{1: {"key-a"}})
	h := command.NewDeleteTraceHandler(repo, keyRepo)

	okID := mustUpsert(t, repo, "s-ok", "key-a")
	otherID := mustUpsert(t, repo, "s-no", "key-b")

	result, err := h.Handle(context.Background(), port.DeleteTraceCommand{
		UserID: 1, IsAdmin: false, IDs: []uint{okID, otherID, 999},
	})
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if result.DeletedCount != 1 || len(result.Failures) != 2 {
		t.Fatalf("expected 1 deleted 2 failures, got %+v", result)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -v -count=1 -run TestDelete ./test/unit/trace/`
Expected: 编译失败（`NewDeleteTraceHandler` 未定义）。

- [ ] **Step 3: 实现删除命令**

`internal/application/trace/command/delete_trace.go`：

```go
package command

import (
	"context"
	"slices"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// NewDeleteTraceHandler 构造删除命令处理器
func NewDeleteTraceHandler(repo trace.TraceRepository, apiKeyRepo apikey.APIKeyRepository) port.DeleteTraceHandler {
	return &deleteTraceHandler{repo: repo, apiKeyRepo: apiKeyRepo}
}

type deleteTraceHandler struct {
	repo       trace.TraceRepository
	apiKeyRepo apikey.APIKeyRepository
}

func (h *deleteTraceHandler) Handle(ctx context.Context, cmd port.DeleteTraceCommand) (*port.DeleteTraceResult, error) {
	log := logger.WithCtx(ctx)

	var ownerNames []string
	if !cmd.IsAdmin {
		names, lookupErr := h.apiKeyRepo.LookupOwnerNamesByUserID(ctx, cmd.UserID)
		if lookupErr != nil {
			log.Error("[TraceCommand] Delete: lookup owner names failed",
				zap.Error(lookupErr), zap.Uint("userID", cmd.UserID))
			return nil, lookupErr
		}
		ownerNames = names
	}

	result := &port.DeleteTraceResult{}

	for _, id := range cmd.IDs {
		t, err := h.repo.FindByID(ctx, id)
		if err != nil {
			log.Error("[TraceCommand] Delete: FindByID failed", zap.Error(err), zap.Uint("traceID", id))
			result.Failures = append(result.Failures, port.DeleteTraceFailedItem{ID: id, Error: constant.TraceDeleteErrorFindFailed})
			continue
		}
		if t == nil {
			result.Failures = append(result.Failures, port.DeleteTraceFailedItem{ID: id, Error: constant.TraceDeleteErrorNotFound})
			continue
		}

		if !cmd.IsAdmin && !slices.Contains(ownerNames, t.APIKeyName) {
			result.Failures = append(result.Failures, port.DeleteTraceFailedItem{ID: id, Error: constant.TraceDeleteErrorNoPermission})
			continue
		}

		if err := h.repo.Delete(ctx, id); err != nil {
			log.Error("[TraceCommand] Delete: delete failed", zap.Error(err), zap.Uint("traceID", id))
			result.Failures = append(result.Failures, port.DeleteTraceFailedItem{ID: id, Error: constant.TraceDeleteErrorDeleteFailed})
			continue
		}

		result.DeletedCount++
		log.Info("[TraceCommand] Trace deleted",
			zap.Uint("traceID", id),
			zap.Uint("requesterID", cmd.UserID),
			zap.String("owner", t.APIKeyName))
	}

	log.Info("[TraceCommand] Delete completed",
		zap.Int("total", len(cmd.IDs)),
		zap.Int("deleted", result.DeletedCount),
		zap.Int("failed", len(result.Failures)),
		zap.Uint("requesterID", cmd.UserID))

	return result, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test -v -count=1 -run TestDelete ./test/unit/trace/`
Expected: 5 个用例全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/application/trace/command/delete_trace.go test/unit/trace/delete_test.go
git commit -m "feat(trace): 删除命令处理器（owner/admin 授权 + 批量失败收集）"
```

---

### Task 6: DTO + Handler + Router + DI

**Files:**
- Modify: `internal/dto/trace.go`
- Modify: `internal/handler/trace.go`
- Modify: `internal/router/trace.go`
- Modify: `internal/bootstrap/modules/application.go`
- Modify: `internal/bootstrap/modules/handler.go`

**Interfaces:**
- Consumes: Task 5 的 `port.DeleteTraceHandler`；`parseCommaSeparatedIDs`（handler 包内既有函数）
- Produces:
  - `dto.DeleteTraceReq{ IDs string \`query:"ids" required:"true" minLength:"1"\` }`
  - `dto.DeleteTraceRsp{ CommonRsp; DeletedCount int; Failures []DeleteFailed }`
  - `TraceHandler.HandleDeleteTraces(ctx, req *dto.DeleteTraceReq) (*dto.HTTPResponse[*dto.DeleteTraceRsp], error)`
  - 路由：OperationID `deleteTrace`，`DELETE` + `Path: ""`，JWT + PermissionUser
  - DI：`NewDeleteTraceHandler` 注册；`NewTraceDependencies` 增加 `delete` 参数

- [ ] **Step 1: DTO**

`internal/dto/trace.go` 的 `GetTraceReq` 之后新增：

```go
// DeleteTraceReq 删除 Trace 请求（支持逗号分隔批量）
type DeleteTraceReq struct {
	IDs string `query:"ids" required:"true" minLength:"1" doc:"Trace ID 列表，逗号分隔，如 123 或 123,456,789"`
}

// DeleteTraceRsp 删除响应
type DeleteTraceRsp struct {
	CommonRsp
	DeletedCount int            `json:"deletedCount,omitempty" doc:"成功删除数量"`
	Failures     []DeleteFailed `json:"failures,omitempty" doc:"删除失败列表"`
}
```

- [ ] **Step 2: Handler 接口与依赖**

`internal/handler/trace.go`：

- `TraceHandler` 接口新增：

```go
	HandleDeleteTraces(ctx context.Context, req *dto.DeleteTraceReq) (*dto.HTTPResponse[*dto.DeleteTraceRsp], error)
```

- `TraceDependencies` 新增字段：

```go
	Delete port.DeleteTraceHandler
```

- `traceHandler` 结构体新增字段与构造器赋值：

```go
type traceHandler struct {
	report       port.ReportTraceEventHandler
	list         port.ListTracesHandler
	get          port.GetTraceHandler
	events       port.ListTraceEventsHandler
	conversation port.ListTraceConversationHandler
	delete       port.DeleteTraceHandler
}
```

```go
	return &traceHandler{
		report:       deps.Report,
		list:         deps.List,
		get:          deps.Get,
		events:       deps.Events,
		conversation: deps.Conversation,
		delete:       deps.Delete,
	}
```

- [ ] **Step 3: Handler 实现**

`internal/handler/trace.go` 文件末尾（`HandleListTraceEvents` 之后）新增：

```go
// HandleDeleteTraces 删除 traces（JWT，支持逗号分隔批量）
func (h *traceHandler) HandleDeleteTraces(ctx context.Context, req *dto.DeleteTraceReq) (*dto.HTTPResponse[*dto.DeleteTraceRsp], error) {
	rsp := &dto.DeleteTraceRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValuePermission(ctx)
	isAdmin := permission.Level() >= enum.PermissionAdmin.Level()

	ids, parseErr := parseCommaSeparatedIDs(req.IDs)
	if parseErr != nil {
		rsp.Error = ierr.ToBizErrorLocalized(ctx, parseErr, ierr.ErrValidation.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	result, err := h.delete.Handle(ctx, port.DeleteTraceCommand{UserID: userID, IsAdmin: isAdmin, IDs: ids})
	if err != nil {
		logger.WithCtx(ctx).Error("[TraceHandler] Delete traces failed", zap.Error(err))
		rsp.Error = ierr.ToBizErrorLocalized(ctx, err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.DeletedCount = result.DeletedCount
	rsp.Failures = lo.Map(result.Failures, func(f port.DeleteTraceFailedItem, _ int) dto.DeleteFailed {
		return dto.DeleteFailed{ID: f.ID, Error: f.Error}
	})

	logger.WithCtx(ctx).Info("[TraceHandler] Trace(s) deleted",
		zap.Int("total", len(ids)),
		zap.Int("deleted", result.DeletedCount),
		zap.Int("failed", len(result.Failures)))

	return apiutil.WrapHTTPResponse(rsp, nil)
}
```

- [ ] **Step 4: Router**

`internal/router/trace.go` 的 queryGroup 中、`getTraceConversation` 注册之后新增：

```go
	huma.Register(queryGroup, huma.Operation{
		OperationID: "deleteTrace", Method: http.MethodDelete, Path: "",
		Summary: "DeleteTrace", Description: "Delete traces by IDs (owner or admin, comma separated)",
		Tags:        []string{constant.TagTrace},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("deleteTrace", enum.PermissionUser)},
	}, deps.TraceHandler.HandleDeleteTraces)
```

- [ ] **Step 5: DI（application.go）**

`internal/bootstrap/modules/application.go` 中 `NewListTraceConversationHandler` 之后新增：

```go
func NewDeleteTraceHandler(repo trace.TraceRepository, apiKeyRepo apikey.APIKeyRepository) traceport.DeleteTraceHandler {
	return tracecommand.NewDeleteTraceHandler(repo, apiKeyRepo)
}
```

- [ ] **Step 6: DI（handler.go）**

`internal/bootstrap/modules/handler.go`：

- `NewTraceDependencies` 签名增加参数并在返回结构中赋值：

```go
func NewTraceDependencies(
	report traceport.ReportTraceEventHandler,
	list traceport.ListTracesHandler,
	get traceport.GetTraceHandler,
	events traceport.ListTraceEventsHandler,
	conversation traceport.ListTraceConversationHandler,
	delete traceport.DeleteTraceHandler,
) handler.TraceDependencies {
	return handler.TraceDependencies{
		Report:       report,
		List:         list,
		Get:          get,
		Events:       events,
		Conversation: conversation,
		Delete:       delete,
	}
}
```

- [ ] **Step 7: 编译验证**

Run: `go build ./...`
Expected: 编译通过。若有 dig provider 解析问题，检查 `internal/bootstrap/modules/` 的 fx/dig 分组注册是否已自动包含（`NewDeleteTraceHandler` 的依赖 `trace.TraceRepository` 与 `apikey.APIKeyRepository` 均已注册）。

- [ ] **Step 8: Commit**

```bash
git add internal/dto/trace.go internal/handler/trace.go internal/router/trace.go internal/bootstrap/modules/application.go internal/bootstrap/modules/handler.go
git commit -m "feat(trace): DELETE /api/v1/trace?ids= 接口与 DI 注册"
```

---

### Task 7: 后端全量验证

**Files:**
- 无新增代码

- [ ] **Step 1: 跑 trace 相关全部单测**

Run: `go test -v -count=1 ./test/unit/trace/`
Expected: 全部 PASS。

- [ ] **Step 2: 跑全量测试**

Run: `make test`
Expected: 全部 PASS（无回归）。

- [ ] **Step 3: lint**

Run: `make lint`
Expected: 无错误。若 lint-conv 提示 OperationID/Description 格式问题，按提示修正后重跑。

- [ ] **Step 4: 确认无未提交改动**

Run: `git status --short`
Expected: 工作区干净。

---

### Task 8: E2E 测试

**Files:**
- Create: `test/e2e/trace/delete_test.go`

**Interfaces:**
- Consumes: `handler.NewTraceHandler`、`command.NewReportTraceEventHandler`、`command.NewDeleteTraceHandler`、`tracefake.NewFakeRepo`、`tracefake` 的 fakeAPIKeyRepo（通过导出函数 `NewFakeRepo` 已可用；owner 查询需要 fakeAPIKeyRepo —— 若 `newFakeAPIKeyRepo` 未导出，则在 e2e 文件中内联一个最小 owner repo 实现，只实现 `LookupOwnerNamesByUserID`，其余方法返回零值）

- [ ] **Step 1: 写 E2E 测试**

`test/e2e/trace/delete_test.go`：

```go
package trace_e2e

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/command"
	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	traceschema "github.com/hcd233/aris-proxy-api/internal/dto/schema"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	tracefake "github.com/hcd233/aris-proxy-api/test/unit/trace"
)

// TestE2E_TraceDeleteFlow 上报建 trace → 删除 → 列表不含 → 同 session 再上报被拒
func TestE2E_TraceDeleteFlow(t *testing.T) {
	t.Parallel()

	repo := tracefake.NewFakeRepo()
	apiKeyRepo := newE2EAPIKeyRepo(map[uint][]string{7: {"e2e-key"}})
	h := handler.NewTraceHandler(handler.TraceDependencies{
		Report: command.NewReportTraceEventHandler(repo),
		Delete: command.NewDeleteTraceHandler(repo, apiKeyRepo),
	})

	ctx := context.WithValue(context.Background(), constant.CtxKeyUserID, uint(7))
	ctx = context.WithValue(ctx, constant.CtxKeyAPIKeyName, "e2e-key")

	body := &dto.ReportTraceEventReqBody{
		SessionID: "e2e-del",
		Agent:     constant.TraceAgentCodex,
		Records: []*dto.ReportTraceRecordReq{{
			Source:        constant.TraceRecordSourceHook,
			RecordType:    constant.TraceRecordTypeHookEvent,
			HookEventName: "UserPromptSubmit",
			DedupKey:      "hook:e2e-del:1",
			Payload:       traceschema.RawJSON(`{"hook_event_name":"UserPromptSubmit","session_id":"e2e-del"}`),
		}},
	}
	if _, err := h.HandleReportTraceEvent(ctx, &dto.ReportTraceEventReq{Body: body}); err != nil {
		t.Fatalf("report: %v", err)
	}
	tr, _ := repo.FindBySessionID(context.Background(), "e2e-del")
	if tr == nil {
		t.Fatal("trace not persisted before delete")
	}

	// 删除
	delRsp, err := h.HandleDeleteTraces(ctx, &dto.DeleteTraceReq{IDs: "1"})
	if err != nil || delRsp == nil || delRsp.Body == nil {
		t.Fatalf("delete: rsp=%+v err=%v", delRsp, err)
	}
	if delRsp.Body.DeletedCount != 1 || len(delRsp.Body.Failures) != 0 {
		t.Fatalf("expected 1 deleted, got %+v", delRsp.Body)
	}

	// 列表不再包含（fake 的 FindBySessionID 过滤软删）
	if again, _ := repo.FindBySessionID(context.Background(), "e2e-del"); again != nil {
		t.Fatal("trace should be gone from normal lookup after delete")
	}

	// 同 session 再上报 → 全部 rejected
	reRsp, err := h.HandleReportTraceEvent(ctx, &dto.ReportTraceEventReq{Body: body})
	if err != nil {
		t.Fatalf("re-report: %v", err)
	}
	if reRsp == nil || reRsp.Body == nil || len(reRsp.Body.Results) != 1 ||
		reRsp.Body.Results[0].Status != constant.TraceRecordStatusRejected {
		t.Fatalf("expected rejected on re-report, got %+v", reRsp.Body)
	}
	if n, _ := repo.CountEvents(context.Background(), tr.ID); n != 1 {
		t.Fatalf("expected events unchanged after rejected re-report, got %d", n)
	}
}
```

在文件末尾追加最小 owner repo（仅实现删除流程需要的接口方法，其余返回零值）：

```go
// e2eAPIKeyRepo 最小 API Key 仓储，仅供删除流程 owner 查询
type e2eAPIKeyRepo struct {
	owners map[uint][]string
}

func newE2EAPIKeyRepo(owners map[uint][]string) *e2eAPIKeyRepo {
	return &e2eAPIKeyRepo{owners: owners}
}

func (r *e2eAPIKeyRepo) LookupOwnerNamesByUserID(_ context.Context, userID uint) ([]string, error) {
	return r.owners[userID], nil
}
```

注意：`e2eAPIKeyRepo` 需要实现 `apikey.APIKeyRepository` 全接口。若接口方法较多，参考 `test/unit/trace/fake_repository.go` 中 `fakeAPIKeyRepo` 的全部方法签名逐个补齐（Save / FindByID / ListByUser / ListAll / PaginateByUser / PaginateAll / CountByUser / Delete / LookupOwnerNamesByUserID / LookupIDsByUserID，全部返回零值）。如 `test/unit/trace` 已导出可复用的 owner repo，则直接 `tracefake.NewFakeAPIKeyRepo(...)` 复用（若存在）。

- [ ] **Step 2: 运行 E2E**

Run: `go test -v -count=1 -run TestE2E_TraceDeleteFlow ./test/e2e/trace/`
Expected: PASS。

- [ ] **Step 3: 全量测试 + lint**

Run: `make test && make lint`
Expected: 全部通过。

- [ ] **Step 4: Commit**

```bash
git add test/e2e/trace/delete_test.go
git commit -m "test(trace): e2e 删除流程（删除后列表消失 + 再上报被拒）"
```

---

### Task 9: 前端 API client + types + i18n

**Files:**
- Modify: `web/src/lib/api-client.ts`
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/locales/zh.json`
- Modify: `web/src/locales/en.json`
- Modify: `web/src/locales/ja.json`

**Interfaces:**
- Produces:
  - `api.deleteTrace(traceId: number): Promise<DeleteTraceRsp>`
  - `api.batchDeleteTraces(ids: number[]): Promise<DeleteTraceRsp>`
  - `interface DeleteTraceRsp extends CommonRsp { deletedCount?: number; failures?: { id: number; error: string }[] }`
  - locale keys：`trace.delete_confirm` / `trace.delete_success` / `trace.delete_error` / `trace.batch_delete_title` / `trace.batch_delete_desc` / `trace.batch_delete_warning` / `trace.batch_delete_success` / `trace.batch_delete_error` / `trace.delete_dialog_title` / `trace.delete_dialog_desc` / `trace.delete_aria`

- [ ] **Step 1: types.ts**

`web/src/lib/types.ts` 中 `DeleteSessionRsp` 定义附近新增：

```ts
export interface DeleteTraceRsp extends CommonRsp {
  deletedCount?: number;
  failures?: { id: number; error: string }[];
}
```

（参考 `DeleteSessionRsp` 的现有定义结构；若其 failures 类型为命名 interface 则复用同名结构。）

- [ ] **Step 2: api-client.ts**

`web/src/lib/api-client.ts` 的 `getTraceConversation` 之后、Trace 区块末尾新增：

```ts
  async deleteTrace(traceId: number): Promise<DeleteTraceRsp> {
    return this.request<DeleteTraceRsp>(
      `/api/v1/trace?ids=${traceId}`,
      { method: "DELETE" }
    );
  }

  async batchDeleteTraces(ids: number[]): Promise<DeleteTraceRsp> {
    return this.request<DeleteTraceRsp>(
      `/api/v1/trace?ids=${ids.join(",")}`,
      { method: "DELETE" }
    );
  }
```

并在文件头部 import 列表加入 `DeleteTraceRsp`（与 `ListTracesRsp` 等并列）。

- [ ] **Step 3: zh.json**

`trace.install_close` 之后新增：

```json
  "trace.delete_confirm": "确定要删除该轨迹吗？",
  "trace.delete_success": "轨迹已删除",
  "trace.delete_error": "删除轨迹失败",
  "trace.delete_dialog_title": "删除轨迹？",
  "trace.delete_dialog_desc": "这将删除轨迹 \"{name}\" 及其全部事件。此操作不可撤销。",
  "trace.delete_aria": "删除轨迹",
  "trace.batch_delete_title": "批量删除轨迹？",
  "trace.batch_delete_desc": "这将删除选中的 {count} 条轨迹及其全部事件。此操作不可撤销。",
  "trace.batch_delete_warning": "已删除 {deleted}，{failed} 个失败",
  "trace.batch_delete_success": "已删除 {count} 条轨迹",
  "trace.batch_delete_error": "批量删除轨迹失败"
```

- [ ] **Step 4: en.json**

对应 key 英文文案（保持键名一致）：

```json
  "trace.delete_confirm": "Delete this trace?",
  "trace.delete_success": "Trace deleted",
  "trace.delete_error": "Failed to delete trace",
  "trace.delete_dialog_title": "Delete trace?",
  "trace.delete_dialog_desc": "This will delete trace \"{name}\" and all its events. This action cannot be undone.",
  "trace.delete_aria": "Delete trace",
  "trace.batch_delete_title": "Delete traces?",
  "trace.batch_delete_desc": "This will delete {count} selected traces and all their events. This action cannot be undone.",
  "trace.batch_delete_warning": "Deleted {deleted}, {failed} failed",
  "trace.batch_delete_success": "Deleted {count} traces",
  "trace.batch_delete_error": "Failed to batch delete traces"
```

- [ ] **Step 5: ja.json**

对应 key 日语文案（键名一致，翻译参考 sessions 页既有 ja 文案风格）。

- [ ] **Step 6: 验证**

Run: `cd web && npx tsc --noEmit && npm run lint`
Expected: 无类型错误、无 lint 错误。

- [ ] **Step 7: Commit**

```bash
git add web/src/lib/api-client.ts web/src/lib/types.ts web/src/locales/zh.json web/src/locales/en.json web/src/locales/ja.json
git commit -m "feat(web): trace 删除 API client 与多语言文案"
```

---

### Task 10: 前端列表页删除 UI

**Files:**
- Modify: `web/src/app/(dashboard)/trace/page.tsx`

**Interfaces:**
- Consumes: Task 9 的 `api.deleteTrace` / `api.batchDeleteTraces`、locale keys
- Produces: 列表页单条删除 + 勾选批量删除

- [ ] **Step 1: 参照 sessions 页补 state**

`web/src/app/(dashboard)/trace/page.tsx` 组件内新增 state（参考 `web/src/app/(dashboard)/sessions/page.tsx` 的 `openDeleteConfirm` / `handleDelete` / `toggleSelect` / `handleBatchDelete` 模式）：

```tsx
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [deleteTarget, setDeleteTarget] = useState<TraceSummary | null>(null);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [batchDeleteConfirmOpen, setBatchDeleteConfirmOpen] = useState(false);
  const [deleting, setDeleting] = useState<number | null>(null);
  const [batchDeleting, setBatchDeleting] = useState(false);
```

import 增加 `Trash2`（lucide-react）、`ConfirmDialog`（若项目有该组件；否则复用 sessions 页所用的对话框组件与 import）。

- [ ] **Step 2: 单条删除回调**

```tsx
  const openDeleteConfirm = (tr: TraceSummary, e: React.MouseEvent) => {
    e.stopPropagation();
    setDeleteTarget(tr);
    setDeleteConfirmOpen(true);
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(deleteTarget.id);
    try {
      await api.deleteTrace(deleteTarget.id);
      toast.success(t("trace.delete_success"));
      fetchTraces(pageInfo.page, pageInfo.pageSize, keyword, true);
    } catch (err) {
      showErrorToast(err, { title: t("trace.delete_error") });
    } finally {
      setDeleting(null);
      setDeleteConfirmOpen(false);
      setDeleteTarget(null);
    }
  };

  const toggleSelect = (id: number, e: React.MouseEvent) => {
    e.stopPropagation();
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const toggleSelectAll = () => {
    if (selected.size === traces.length) {
      setSelected(new Set());
    } else {
      setSelected(new Set(traces.map((tr) => tr.id)));
    }
  };

  const handleBatchDelete = async () => {
    if (selected.size === 0) return;
    setBatchDeleting(true);
    try {
      const ids = Array.from(selected);
      const rsp = await api.batchDeleteTraces(ids);
      const failed = rsp.failures?.length ?? 0;
      if (failed > 0) {
        toast.warning(t("trace.batch_delete_warning").replace("{deleted}", String(rsp.deletedCount)).replace("{failed}", String(failed)));
      } else {
        toast.success(t("trace.batch_delete_success").replace("{count}", String(rsp.deletedCount)));
      }
      setSelected(new Set());
      fetchTraces(1, pageInfo.pageSize, keyword, true);
    } catch (err) {
      showErrorToast(err, { title: t("trace.batch_delete_error") });
    } finally {
      setBatchDeleting(false);
      setBatchDeleteConfirmOpen(false);
    }
  };
```

- [ ] **Step 3: 表格加勾选列与行内删除按钮**

- TableHeader 首列加 checkbox：`<TableHead className="w-10"><input type="checkbox" checked={selected.size === traces.length && traces.length > 0} onChange={toggleSelectAll} /></TableHead>`（项目若有 Checkbox 组件则用组件）
- TableBody 每行：首列 `<input type="checkbox" checked={selected.has(tr.id)} onChange={(e) => toggleSelect(tr.id, e as unknown as React.MouseEvent)} />`
- 每行操作列（TableHead 加 `<TableHead className="w-20">{t("common.actions")}</TableHead>`），单元格内删除按钮：

```tsx
  <TableCell onClick={(e) => e.stopPropagation()}>
    <button
      type="button"
      className="text-muted-foreground hover:text-destructive"
      title={t("trace.delete_aria")}
      onClick={(e) => openDeleteConfirm(tr, e)}
      disabled={deleting === tr.id}
    >
      <Trash2 className="size-4" />
    </button>
  </TableCell>
```

- 移动端卡片视图（`isMobile` 分支）也加删除按钮（stopPropagation 避免触发跳转）。

- [ ] **Step 4: 批量操作栏与确认对话框**

- 表头卡片标题区（`CardHeader` 的 `CardTitle` 旁）或表格上方加批量按钮，选中数 > 0 时显示：

```tsx
  {selected.size > 0 && (
    <Button
      variant="destructive"
      size="sm"
      onClick={() => setBatchDeleteConfirmOpen(true)}
      disabled={batchDeleting}
    >
      <Trash2 className="size-4" />
      {t("common.delete")} ({selected.size})
    </Button>
  )}
```

- 两个确认对话框（单条 + 批量），复用 sessions 页使用的对话框组件与 `sessions.delete_dialog_title` / `sessions.batch_delete_title` 的呈现方式，文案用 `trace.*` 对应 key：

```tsx
  <ConfirmDialog
    open={deleteConfirmOpen}
    title={t("trace.delete_dialog_title")}
    description={t("trace.delete_dialog_desc").replace("{name}", deleteTarget?.sessionId ?? String(deleteTarget?.id ?? ""))}
    confirmLabel={t("common.delete")}
    onConfirm={handleDelete}
    onOpenChange={(open) => { if (!deleting) setDeleteConfirmOpen(open); }}
  />
  <ConfirmDialog
    open={batchDeleteConfirmOpen}
    title={t("trace.batch_delete_title")}
    description={t("trace.batch_delete_desc").replace("{count}", String(selected.size))}
    confirmLabel={t("common.delete")}
    onConfirm={handleBatchDelete}
    onOpenChange={(open) => { if (!batchDeleting) setBatchDeleteConfirmOpen(open); }}
  />
```

（`ConfirmDialog` 组件名/Props 以 sessions 页实际使用的为准；删除后应清空 `selected`。）

- [ ] **Step 5: 验证**

Run: `cd web && npx tsc --noEmit && npm run lint`
Expected: 无错误。

- [ ] **Step 6: Commit**

```bash
git add "web/src/app/(dashboard)/trace/page.tsx"
git commit -m "feat(web): trace 列表页单条与批量删除"
```

---

### Task 11: 前端详情页删除 UI

**Files:**
- Modify: `web/src/components/trace-detail/trace-detail-client.tsx`

**Interfaces:**
- Consumes: Task 9 的 `api.deleteTrace`、locale keys、现有 `useRouter` / `router.push`
- Produces: 详情页删除按钮，删除成功后跳回 `/trace/`

- [ ] **Step 1: 加 state 与回调**

`trace-detail-client.tsx` 组件内（现有 `router` 附近）新增：

```tsx
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);

  const handleDelete = async () => {
    setDeleting(true);
    try {
      await api.deleteTrace(traceId);
      toast.success(t("trace.delete_success"));
      router.push("/trace/");
    } catch (err) {
      showErrorToast(err, { title: t("trace.delete_error") });
      setDeleting(false);
      setDeleteConfirmOpen(false);
    }
  };
```

import 增加 `Trash2`（lucide-react）与确认对话框组件（同 Task 10）。

- [ ] **Step 2: 头部加删除按钮**

在返回按钮（`router.push("/trace/")` 的返回元素）所在 header 区域旁加删除按钮：

```tsx
  <Button
    variant="ghost"
    size="sm"
    className="text-muted-foreground hover:text-destructive"
    aria-label={t("trace.delete_aria")}
    title={t("trace.delete_aria")}
    disabled={deleting}
    onClick={() => setDeleteConfirmOpen(true)}
  >
    <Trash2 className="size-4" />
  </Button>
```

- [ ] **Step 3: 确认对话框**

```tsx
  <ConfirmDialog
    open={deleteConfirmOpen}
    title={t("trace.delete_dialog_title")}
    description={t("trace.delete_dialog_desc").replace("{name}", String(traceId))}
    confirmLabel={t("common.delete")}
    onConfirm={handleDelete}
    onOpenChange={(open) => { if (!deleting) setDeleteConfirmOpen(open); }}
  />
```

（组件名/Props 以项目实际对话框组件为准。）

- [ ] **Step 4: 验证**

Run: `cd web && npx tsc --noEmit && npm run lint`
Expected: 无错误。

- [ ] **Step 5: Commit**

```bash
git add web/src/components/trace-detail/trace-detail-client.tsx
git commit -m "feat(web): trace 详情页删除入口"
```

---

### Task 12: 前端构建 + 全量回归 + 过度工程审查

**Files:**
- 无新增代码

- [ ] **Step 1: 前端构建**

Run: `make web-build`
Expected: 构建成功。

- [ ] **Step 2: 后端全量测试 + lint**

Run: `make test && make lint`
Expected: 全部通过。

- [ ] **Step 3: ponytail-review 审查本次 diff**

使用 `ponytail-review` skill 审查 `git diff master...HEAD`，确认无投机抽象、重复造轮子、死代码。按审查结论修正（如有）。

- [ ] **Step 4: 最终提交与状态确认**

Run: `git status --short && git log --oneline master..HEAD`
Expected: 工作区干净，提交历史包含 Task 0~11 的提交。

---

## 自审记录（Self-Review）

- **Spec 覆盖**：软删除（Task 2）✓；上报拦截（Task 4）✓；级联软删 events（Task 2 Delete 事务）✓；owner/admin 权限（Task 5）✓；单个+批量（Task 6 逗号分隔）✓；前端列表页单条+批量（Task 10）✓；详情页删除（Task 11）✓；E2E（Task 8）✓
- **占位符扫描**：无 TBD/TODO；所有代码步骤均给出完整代码或精确到行/函数的修改指令
- **类型一致性**：`DeleteTraceCommand{UserID,IsAdmin,IDs}`、`DeleteTraceResult{DeletedCount,Failures}`、`TraceRepository.Delete(ctx,id)`、`FindBySessionIDIncludingDeleted(ctx,sessionID)` 在 Task 1~8 之间签名一致；`ensureTrace` 新签名 `(ctx, cmd, agent, existing)` 在 Task 4 中定义并被同一文件使用；handler 复用既有 `parseCommaSeparatedIDs`
- **注意**：`ConfirmDialog` 组件名、`Checkbox` 组件、`DeleteSessionRsp` failures 的具体结构、`FieldID` 常量是否存在、fakeAPIKeyRepo 是否可复用导出——Task 6/9/10/11 步骤中以"以实际代码为准"标注，执行时先读对应文件确认再落地
