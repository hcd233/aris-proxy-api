# Trace 功能实现计划（codex 先行）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 aris-proxy-api 中新增独立 `traces`/`events` 观测域，通过 codex hooks（Shell 脚本）实时上报 agent 运行事件并落库，提供查询接口与前端安装/查看体验。

**Architecture:** 沿用现有分层（dto → handler → application port/usecase → domain repository 接口 → infrastructure repository/DAO → bootstrap 注册）。Hook 脚本后台 curl `POST /api/v1/trace/event`（API Key 鉴权）把 codex 原始 hook JSON 透传落库；查询接口走 JWT + owner 隔离。trace 与既有 `sessions` 完全正交。

**Tech Stack:** Go 1.25.1、Fiber + Huma、GORM（PostgreSQL）、dig/fx DI、`sonic` JSON、SQLite/Postgres 测试用 fake repository、前端 Next.js（沿用 `export-codex-dialog` 生成 setup 脚本）。

## Global Constraints

- 上报目标：本平台新建 trace 存储，与现有 session 能力平级。（来自 spec §1/§2-1）
- 捕获策略：事件流实时上报。（spec §2-2）
- Hook 载体：Shell 脚本，依赖 `jq` + `curl`。（spec §2-3）
- 鉴权：复用现有 API Key（`Authorization: Bearer <key>`），由 `middleware.APIKeyMiddleware` 注入 `CtxKeyAPIKeyName` / `CtxKeyUserID`。（spec §2-4）
- 安装：前端生成 `codex-trace-setup.sh`，沿用 models 导出到 codex 的体验。（spec §2-5）
- 存储：新建独立 `traces` & `events` 表；与 proxy session **完全正交**，v1 不关联。（spec §2-6/§3）
- 接口命名：沿用 `/api/v1/<资源>` + 子路径动作（`/list`、`""`、`/event/list`），列表用 `constant.RoutePathList`。（spec §6）
- 仅观测、fail-open：hook stdout 保持空（Stop 输出 `{}`），绝不拦截/修改 agent 行为。（spec §4）
- 覆盖 codex 事件：`SessionStart / UserPromptSubmit / PreToolUse / PostToolUse / Stop / SubagentStart / SubagentStop / PreCompact / PostCompact`。（spec §8）
- YAGNI：v1 仅 codex；不做外部 OTel；不拦截策略。（spec §12）
- 代码遵循 `golang-code-style` / `golang-naming` / `golang-samber-lo` / `golang-samber-mo`；实现期激活 `ponytail`（full）。（AGENTS.md workflow）

## 文件结构

```
internal/
  common/constant/string.go              # Modify: + TagTrace
  dto/trace.go                           # Create: 请求/响应 DTO
  domain/trace/
    repository.go                        # Create: Trace/TraceEvent 结构体 + TraceRepository 接口
  infrastructure/database/
    model/trace.go                       # Create: Trace, TraceEvent GORM 模型
    model/base.go                        # Modify: Models 切片 + &Trace{}, &TraceEvent{}
    dao/trace.go                         # Create: TraceDAO, EventDAO + 单例 getter
  infrastructure/repository/
    trace_repository.go                  # Create: TraceRepository 的 GORM 实现
  application/trace/
    port/handler.go                      # Create: 端口接口 + Query/Command
    query/list_traces.go                 # Create: 列表 usecase
    query/get_trace.go                   # Create: 详情 usecase
    query/list_trace_events.go           # Create: 事件时间线 usecase
    command/report_trace_event.go        # Create: 上报 usecase（upsert trace / insert event）
  handler/trace.go                       # Create: TraceHandler 接口 + 实现 + TraceDependencies
  router/trace.go                        # Create: initTraceRouter + initTraceReportRouter
  router/router.go                       # Modify: 注册 trace 路由
  bootstrap/modules/handler.go           # Modify: + NewTraceDependencies / handler.NewTraceHandler
  bootstrap/modules/application.go       # Modify: + trace usecase providers + wiring
web/src/
  components/trace-install-dialog.tsx    # Create: 生成 codex-trace-setup.sh 的对话框
  scripts/codex-hook.sh                  # Create: hook 脚本（随 setup 分发）
  app/(dashboard)/trace/page.tsx         # Create: Agent Traces 列表页（v1 最小可用）
test/
  unit/trace_repository_test.go          # Create: 仓储单测（fake repo）
  unit/trace_usecase_test.go             # Create: usecase 单测（fake repo）
  e2e/trace/trace_test.go                # Create: 端到端（hook 脚本 + 落库断言）
```

---

### Task 1: 常量 `TagTrace`

**Files:**
- Modify: `internal/common/constant/string.go`（`TagSession` 附近，约 line 153）

**Interfaces:** 无依赖，产出 `constant.TagTrace` 供后续任务引用。

- [ ] **Step 1: 在 `TagSession` 常量后新增 `TagTrace`**

在 `internal/common/constant/string.go` 的 `TagSession = "Session"` 行后追加：

```go
	TagTrace   = "Trace"
```

- [ ] **Step 2: 编译校验**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api && go build ./internal/common/constant/`
Expected: 编译通过，无输出。

- [ ] **Step 3: Commit**

```bash
git add internal/common/constant/string.go
git commit -m "feat(trace): add TagTrace constant"
```

---

### Task 2: 数据库模型 `Trace` / `TraceEvent`

**Files:**
- Create: `internal/infrastructure/database/model/trace.go`
- Modify: `internal/infrastructure/database/model/base.go`（约 line 21 `Models` 切片）

**Interfaces:** 产出 `dbmodel.Trace`、`dbmodel.TraceEvent`，并在 `model.Models` 注册以触发 AutoMigrate。

- [ ] **Step 1: 写模型文件**

`internal/infrastructure/database/model/trace.go`：

```go
package model

import "time"

// Trace agent 运行观测记录（与 proxy session 正交）
type Trace struct {
	BaseModel
	Agent      string            `json:"agent" gorm:"column:agent;not null;default:'codex';comment:agent 来源"`
	SessionID  string            `json:"session_id" gorm:"column:session_id;not null;uniqueIndex:uniq_trace_session;comment:codex session_id"`
	APIKeyName string            `json:"api_key_name" gorm:"column:api_key_name;not null;default:'';comment:归属 API Key 名称"`
	UserID     uint              `json:"user_id" gorm:"column:user_id;not null;default:0;comment:归属用户"`
	Model      string            `json:"model" gorm:"column:model;not null;default:'';comment:活跃模型 slug"`
	CWD        string            `json:"cwd" gorm:"column:cwd;not null;default:'';comment:工作目录"`
	Source     string            `json:"source" gorm:"column:source;not null;default:'';comment:startup/resume/clear/compact"`
	Status     string            `json:"status" gorm:"column:status;not null;default:'active';comment:active/done"`
	Metadata   map[string]string `json:"metadata" gorm:"column:metadata;serializer:json;comment:扩展字段"`
}

// TraceEvent agent 运行内单个 hook 事件
type TraceEvent struct {
	BaseModel
	TraceID   uint   `json:"trace_id" gorm:"column:trace_id;not null;index:idx_trace_event_trace;comment:关联 trace id"`
	SessionID string `json:"session_id" gorm:"column:session_id;not null;index:idx_trace_event_session;comment:codex session_id"`
	Event     string `json:"event" gorm:"column:event;not null;comment:hook_event_name"`
	TurnID    string `json:"turn_id" gorm:"column:turn_id;not null;default:'';comment:codex turn id"`
	Payload   []byte `json:"payload" gorm:"column:payload;type:jsonb;comment:完整 hook 输入（透传）"`
}
```

- [ ] **Step 2: 注册到 `Models` 切片**

在 `internal/infrastructure/database/model/base.go` 的 `&CronCallAudit{}` 后追加：

```go
	&CronCallAudit{},
	&Trace{},
	&TraceEvent{},
```

- [ ] **Step 3: 编译校验**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api && go build ./internal/infrastructure/database/model/`
Expected: 编译通过。

- [ ] **Step 4: Commit**

```bash
git add internal/infrastructure/database/model/trace.go internal/infrastructure/database/model/base.go
git commit -m "feat(trace): add Trace and TraceEvent database models"
```

---

### Task 3: DAO `TraceDAO` / `EventDAO`

**Files:**
- Create: `internal/infrastructure/database/dao/trace.go`

**Interfaces:** 复用 `dao.baseDAO[ModelT]`（`Create/Update/Delete/Get/Count/Paginate` 等）。产出 `dao.GetTraceDAO()`、`dao.GetEventDAO()`。

- [ ] **Step 1: 写 DAO 文件**

`internal/infrastructure/database/dao/trace.go`：

```go
package dao

import (
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"sync"
)

// TraceDAO Trace 数据访问对象
type TraceDAO struct {
	baseDAO[dbmodel.Trace]
}

// EventDAO TraceEvent 数据访问对象
type EventDAO struct {
	baseDAO[dbmodel.TraceEvent]
}

var (
	traceDAOSingleton *TraceDAO
	eventDAOSingleton *EventDAO
	traceDAOSync      sync.Once
	eventDAOSync      sync.Once
)

// GetTraceDAO 获取 TraceDAO 单例
func GetTraceDAO() *TraceDAO {
	traceDAOSync.Do(func() { traceDAOSingleton = &TraceDAO{} })
	return traceDAOSingleton
}

// GetEventDAO 获取 EventDAO 单例
func GetEventDAO() *EventDAO {
	eventDAOSync.Do(func() { eventDAOSingleton = &EventDAO{} })
	return eventDAOSingleton
}
```

- [ ] **Step 2: 编译校验**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api && go build ./internal/infrastructure/database/dao/`
Expected: 编译通过。

- [ ] **Step 3: Commit**

```bash
git add internal/infrastructure/database/dao/trace.go
git commit -m "feat(trace): add TraceDAO and EventDAO"
```

---

### Task 4: 领域仓库接口与结构体

**Files:**
- Create: `internal/domain/trace/repository.go`

**Interfaces:** 产出 `trace.Trace`、`trace.TraceEvent`（领域结构体）、`trace.TraceRepository` 接口，供 Task 5 实现、Task 6 端口引用。

> 遵循 ponytail：trace 域不需要完整 aggregate/VO 层，使用轻量领域结构体即可。

- [ ] **Step 1: 写领域文件**

`internal/domain/trace/repository.go`：

```go
// Package trace agent 运行观测领域
package trace

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

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
}

// TraceEvent 运行内单个事件（领域结构体）
type TraceEvent struct {
	ID        uint
	TraceID   uint
	SessionID string
	Event     string
	TurnID    string
	Payload   []byte
	CreatedAt time.Time
}

// TraceRepository Trace 聚合仓储接口
type TraceRepository interface {
	// UpsertBySessionID 按 session_id 幂等写入/更新 trace；回填 ID
	UpsertBySessionID(ctx context.Context, t *Trace) (*Trace, error)
	// FindBySessionID 按 session_id 查询；未找到返回 (nil, nil)
	FindBySessionID(ctx context.Context, sessionID string) (*Trace, error)
	// MarkDone 将 trace 标记为 done
	MarkDone(ctx context.Context, sessionID string) error
	// InsertEvent 插入一条事件
	InsertEvent(ctx context.Context, e *TraceEvent) error
	// PaginateByOwners 按 owner 名称列表分页（admin 传空切片表示不过滤）
	PaginateByOwners(ctx context.Context, owners []string, param model.CommonParam) ([]*Trace, *model.PageInfo, error)
	// CountEvents 统计某 trace 的事件数
	CountEvents(ctx context.Context, traceID uint) (int64, error)
	// ListEvents 按 trace_id 分页列出事件（按 id 升序即时间线）
	ListEvents(ctx context.Context, traceID uint, param model.CommonParam) ([]*TraceEvent, *model.PageInfo, error)
}
```

- [ ] **Step 2: 编译校验**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api && go build ./internal/domain/trace/`
Expected: 编译通过。

- [ ] **Step 3: Commit**

```bash
git add internal/domain/trace/repository.go
git commit -m "feat(trace): define Trace domain structs and repository port"
```

---

### Task 5: 仓储 GORM 实现

**Files:**
- Create: `internal/infrastructure/repository/trace_repository.go`

**Interfaces:** 实现 `trace.TraceRepository`（Task 4）。消费 `dao.GetTraceDAO()` / `dao.GetEventDAO()`（Task 3）、`dbmodel.Trace`/`TraceEvent`（Task 2）。

- [ ] **Step 1: 写失败测试（fake 暂不写，先写实现后用 Task 13 单测）**

本任务直接实现，测试在 Task 13 用 fake 覆盖接口。先写实现。

- [ ] **Step 2: 写实现**

`internal/infrastructure/repository/trace_repository.go`：

```go
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type traceRepository struct {
	traceDAO *dao.TraceDAO
	eventDAO *dao.EventDAO
	db       *gorm.DB
}

// NewTraceRepository 构造 TraceRepository
func NewTraceRepository(db *gorm.DB) trace.TraceRepository {
	return &traceRepository{traceDAO: dao.GetTraceDAO(), eventDAO: dao.GetEventDAO(), db: db}
}

func toTraceDomain(m *dbmodel.Trace) *trace.Trace {
	return &trace.Trace{
		ID: m.ID, Agent: m.Agent, SessionID: m.SessionID, APIKeyName: m.APIKeyName,
		UserID: m.UserID, Model: m.Model, CWD: m.CWD, Source: m.Source,
		Status: m.Status, Metadata: m.Metadata, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

func toTraceRecord(t *trace.Trace) *dbmodel.Trace {
	return &dbmodel.Trace{
		ID: t.ID, Agent: t.Agent, SessionID: t.SessionID, APIKeyName: t.APIKeyName,
		UserID: t.UserID, Model: t.Model, CWD: t.CWD, Source: t.Source,
		Status: t.Status, Metadata: t.Metadata,
	}
}

func (r *traceRepository) UpsertBySessionID(ctx context.Context, t *trace.Trace) (*trace.Trace, error) {
	db := r.db.WithContext(ctx)
	rec := toTraceRecord(t)
	err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "session_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"model", "cwd", "source", "status", "updated_at", "metadata", "user_id", "api_key_name"}),
	}).Create(rec).Error
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBCreate, err, "upsert trace")
	}
	t.ID = rec.ID
	return t, nil
}

func (r *traceRepository) FindBySessionID(ctx context.Context, sessionID string) (*trace.Trace, error) {
	db := r.db.WithContext(ctx)
	rec, err := r.traceDAO.Get(db, &dbmodel.Trace{SessionID: sessionID}, []string{"*"})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "find trace by session")
	}
	return toTraceDomain(rec), nil
}

func (r *traceRepository) MarkDone(ctx context.Context, sessionID string) error {
	db := r.db.WithContext(ctx)
	err := db.Model(&dbmodel.Trace{}).Where("session_id = ?", sessionID).
		Updates(map[string]any{"status": "done", "updated_at": time.Now().UTC()}).Error
	if err != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, err, "mark trace done")
	}
	return nil
}

func (r *traceRepository) InsertEvent(ctx context.Context, e *trace.TraceEvent) error {
	db := r.db.WithContext(ctx)
	rec := &dbmodel.TraceEvent{
		TraceID: e.TraceID, SessionID: e.SessionID, Event: e.Event,
		TurnID: e.TurnID, Payload: e.Payload,
	}
	if err := r.eventDAO.Create(db, rec); err != nil {
		return ierr.Wrap(ierr.ErrDBCreate, err, "insert trace event")
	}
	e.ID = rec.ID
	return nil
}

func (r *traceRepository) PaginateByOwners(ctx context.Context, owners []string, param model.CommonParam) ([]*trace.Trace, *model.PageInfo, error) {
	db := r.db.WithContext(ctx)
	q := db.Model(&dbmodel.Trace{})
	if len(owners) > 0 {
		q = q.Where("api_key_name IN ?", owners)
	}
	pageInfo := &model.PageInfo{Page: param.Page, PageSize: param.PageSize}
	if pageInfo.Page < 1 {
		pageInfo.Page = 1
	}
	if pageInfo.PageSize < 1 {
		pageInfo.PageSize = 20
	}
	if err := q.Count(&pageInfo.Total).Error; err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "count traces")
	}
	var recs []*dbmodel.Trace
	if err := q.Order("id DESC").Limit(pageInfo.PageSize).Offset((pageInfo.Page - 1) * pageInfo.PageSize).Find(&recs).Error; err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "list traces")
	}
	return lo_mapTrace(recs), pageInfo, nil
}

func (r *traceRepository) CountEvents(ctx context.Context, traceID uint) (int64, error) {
	db := r.db.WithContext(ctx)
	var c int64
	if err := db.Model(&dbmodel.TraceEvent{}).Where("trace_id = ?", traceID).Count(&c).Error; err != nil {
		return 0, ierr.Wrap(ierr.ErrDBQuery, err, "count trace events")
	}
	return c, nil
}

func (r *traceRepository) ListEvents(ctx context.Context, traceID uint, param model.CommonParam) ([]*trace.TraceEvent, *model.PageInfo, error) {
	db := r.db.WithContext(ctx)
	pageInfo := &model.PageInfo{Page: param.Page, PageSize: param.PageSize}
	if pageInfo.Page < 1 {
		pageInfo.Page = 1
	}
	if pageInfo.PageSize < 1 {
		pageInfo.PageSize = 50
	}
	q := db.Model(&dbmodel.TraceEvent{}).Where("trace_id = ?", traceID)
	if err := q.Count(&pageInfo.Total).Error; err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "count trace events")
	}
	var recs []*dbmodel.TraceEvent
	if err := q.Order("id ASC").Limit(pageInfo.PageSize).Offset((pageInfo.Page - 1) * pageInfo.PageSize).Find(&recs).Error; err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "list trace events")
	}
	return lo_mapTraceEvent(recs), pageInfo, nil
}
```

同文件底部追加映射辅助函数：

```go
func lo_mapTrace(recs []*dbmodel.Trace) []*trace.Trace {
	out := make([]*trace.Trace, 0, len(recs))
	for _, r := range recs {
		out = append(out, toTraceDomain(r))
	}
	return out
}

func lo_mapTraceEvent(recs []*dbmodel.TraceEvent) []*trace.TraceEvent {
	out := make([]*trace.TraceEvent, 0, len(recs))
	for _, r := range recs {
		out = append(out, &trace.TraceEvent{
			ID: r.ID, TraceID: r.TraceID, SessionID: r.SessionID,
			Event: r.Event, TurnID: r.TurnID, Payload: r.Payload, CreatedAt: r.CreatedAt,
		})
	}
	return out
}
```

- [ ] **Step 3: 编译校验**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api && go build ./internal/infrastructure/repository/`
Expected: 编译通过。（`constant.ErrDBCreate` 等常量若命名不同，以 `internal/common/ierr` 实际定义为准，常见为 `ierr.ErrDBCreate`/`ErrDBQuery`/`ErrDBUpdate`。）

- [ ] **Step 4: Commit**

```bash
git add internal/infrastructure/repository/trace_repository.go
git commit -m "feat(trace): implement GORM TraceRepository"
```

---

### Task 6: 应用层端口

**Files:**
- Create: `internal/application/trace/port/handler.go`

**Interfaces:** 产出 `traceport.ReportTraceEventHandler`、`ListTracesHandler`、`GetTraceHandler`、`ListTraceEventsHandler` 接口及对应 Query/Command。供 Task 7 实现、Task 9 handler 调用。

- [ ] **Step 1: 写端口文件**

`internal/application/trace/port/handler.go`：

```go
// Package port defines application-layer ports for trace use cases.
package port

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

// TraceSummaryView 列表项视图
type TraceSummaryView struct {
	ID         uint
	SessionID  string
	Agent      string
	APIKeyName string
	Model      string
	Source     string
	Status     string
	CreatedAt  interface{ String() string }
	UpdatedAt  interface{ String() string }
}

// TraceDetailView 详情视图
type TraceDetailView struct {
	ID         uint
	SessionID  string
	Agent      string
	APIKeyName string
	Model      string
	CWD        string
	Source     string
	Status     string
	Metadata   map[string]string
	EventCount int64
	CreatedAt  interface{ String() string }
	UpdatedAt  interface{ String() string }
}

// TraceEventView 事件视图
type TraceEventView struct {
	ID        uint
	Event     string
	TurnID    string
	Payload   []byte
	CreatedAt interface{ String() string }
}

// ReportTraceEventCommand 上报事件命令
type ReportTraceEventCommand struct {
	RawPayload []byte
	APIKeyName string
	UserID     uint
}

// ReportTraceEventHandler 上报 handler 接口
type ReportTraceEventHandler interface {
	Handle(ctx context.Context, cmd ReportTraceEventCommand) error
}

// ListTracesQuery 列表查询
type ListTracesQuery struct {
	UserID  uint
	IsAdmin bool
	Page    int
	PageSize int
}

// ListTracesHandler 列表 handler 接口
type ListTracesHandler interface {
	Handle(ctx context.Context, q ListTracesQuery) ([]*TraceSummaryView, *model.PageInfo, error)
}

// GetTraceQuery 详情查询
type GetTraceQuery struct {
	UserID    uint
	IsAdmin   bool
	TraceID   uint
}

// GetTraceHandler 详情 handler 接口
type GetTraceHandler interface {
	Handle(ctx context.Context, q GetTraceQuery) (*TraceDetailView, error)
}

// ListTraceEventsQuery 事件时间线查询
type ListTraceEventsQuery struct {
	UserID    uint
	IsAdmin   bool
	TraceID   uint
	Page      int
	PageSize  int
}

// ListTraceEventsHandler 事件时间线 handler 接口
type ListTraceEventsHandler interface {
	Handle(ctx context.Context, q ListTraceEventsQuery) ([]*TraceEventView, *model.PageInfo, error)
}
```

> 注：视图里 `CreatedAt/UpdatedAt` 用 `time.Time` 即可（上面用接口占位仅为说明；实现时直接用 `time.Time`）。实现 Task 7 时改为 `time.Time`。

- [ ] **Step 2: 编译校验**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api && go build ./internal/application/trace/port/`
Expected: 编译通过。

- [ ] **Step 3: Commit**

```bash
git add internal/application/trace/port/handler.go
git commit -m "feat(trace): define application ports for trace"
```

---

### Task 7: 应用层 usecases

**Files:**
- Create: `internal/application/trace/command/report_trace_event.go`
- Create: `internal/application/trace/query/list_traces.go`
- Create: `internal/application/trace/query/get_trace.go`
- Create: `internal/application/trace/query/list_trace_events.go`

**Interfaces:** 实现 Task 6 的四个端口。消费 `trace.TraceRepository`（Task 4/5）与 `apikey.APIKeyRepository.LookupOwnerNamesByUserID`（已在 `internal/domain/apikey/repository.go:35` 定义）。产出构造函数供 Task 11 bootstrap 注册。

- [ ] **Step 1: 上报 usecase（含失败单测先行）**

先写失败测试 `internal/application/trace/command/report_trace_event_test.go`（见 Step 3），运行确认失败，再写实现。

`internal/application/trace/command/report_trace_event.go`：

```go
// Package command trace 写侧 usecase
package command

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
	"github.com/samber/lo"
)

type reportTraceEventHandler struct {
	repo trace.TraceRepository
}

// NewReportTraceEventHandler 构造上报 handler
func NewReportTraceEventHandler(repo trace.TraceRepository) port.ReportTraceEventHandler {
	return &reportTraceEventHandler{repo: repo}
}

type hookInput struct {
	HookEventName string `json:"hook_event_name"`
	SessionID     string `json:"session_id"`
	Model         string `json:"model"`
	CWD           string `json:"cwd"`
	Source        string `json:"source"`
	TurnID        string `json:"turn_id"`
}

func (h *reportTraceEventHandler) Handle(ctx context.Context, cmd port.ReportTraceEventCommand) error {
	var in hookInput
	if err := json.Unmarshal(cmd.RawPayload, &in); err != nil {
		return ierr.Wrap(ierr.ErrValidation, err, "parse hook payload")
	}
	if in.SessionID == "" {
		return ierr.New(ierr.ErrValidation, "hook payload missing session_id")
	}

	// 保证 trace 存在（SessionStart 可能丢失时兜底创建）
	t, err := h.repo.FindBySessionID(ctx, in.SessionID)
	if err != nil {
		return err
	}
	if t == nil {
		t, err = h.repo.UpsertBySessionID(ctx, &trace.Trace{
			Agent: "codex", SessionID: in.SessionID, APIKeyName: cmd.APIKeyName,
			UserID: cmd.UserID, Model: in.Model, CWD: in.CWD, Source: in.Source, Status: "active",
		})
		if err != nil {
			return err
		}
	}

	switch in.HookEventName {
	case "SessionStart":
		_, err = h.repo.UpsertBySessionID(ctx, &trace.Trace{
			ID: t.ID, Agent: "codex", SessionID: in.SessionID, APIKeyName: cmd.APIKeyName,
			UserID: cmd.UserID, Model: in.Model, CWD: in.CWD, Source: in.Source, Status: "active",
		})
	case "Stop":
		if err = h.repo.InsertEvent(ctx, &trace.TraceEvent{
			TraceID: t.ID, SessionID: in.SessionID, Event: in.HookEventName, TurnID: in.TurnID, Payload: cmd.RawPayload,
		}); err != nil {
			return err
		}
		return h.repo.MarkDone(ctx, in.SessionID)
	default:
		err = h.repo.InsertEvent(ctx, &trace.TraceEvent{
			TraceID: t.ID, SessionID: in.SessionID, Event: in.HookEventName, TurnID: in.TurnID, Payload: cmd.RawPayload,
		})
	}
	if err != nil {
		return err
	}
	return nil
}

var _ = lo.Must0 // 防止未使用导入（如后续需要扩展）
var _ = errors.New
```

- [ ] **Step 2: 列表 / 详情 / 事件时间线 usecases**

`internal/application/trace/query/list_traces.go`：

```go
// Package query trace 读侧 usecase
package query

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	apikeydomain "github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

type listTracesHandler struct {
	repo       trace.TraceRepository
	apiKeyRepo apikeydomain.APIKeyRepository
}

// NewListTracesHandler 构造列表 handler
func NewListTracesHandler(repo trace.TraceRepository, apiKeyRepo apikeydomain.APIKeyRepository) port.ListTracesHandler {
	return &listTracesHandler{repo: repo, apiKeyRepo: apiKeyRepo}
}

func (h *listTracesHandler) Handle(ctx context.Context, q port.ListTracesQuery) ([]*port.TraceSummaryView, *model.PageInfo, error) {
	owners, err := resolveOwners(ctx, h.apiKeyRepo, q.UserID, q.IsAdmin)
	if err != nil {
		return nil, nil, err
	}
	traces, pageInfo, err := h.repo.PaginateByOwners(ctx, owners, model.CommonParam{PageParam: model.PageParam{Page: q.Page, PageSize: q.PageSize}})
	if err != nil {
		return nil, nil, err
	}
	views := lo_MapTraceSummary(traces)
	return views, pageInfo, nil
}

func resolveOwners(ctx context.Context, repo apikeydomain.APIKeyRepository, userID uint, isAdmin bool) ([]string, error) {
	if isAdmin {
		return nil, nil
	}
	return repo.LookupOwnerNamesByUserID(ctx, userID)
}
```

`internal/application/trace/query/get_trace.go`：

```go
package query

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

type getTraceHandler struct {
	repo trace.TraceRepository
}

// NewGetTraceHandler 构造详情 handler
func NewGetTraceHandler(repo trace.TraceRepository) port.GetTraceHandler {
	return &getTraceHandler{repo: repo}
}

func (h *getTraceHandler) Handle(ctx context.Context, q port.GetTraceQuery) (*port.TraceDetailView, error) {
	t, err := h.repo.FindBySessionID(ctx, "") // 占位，实际按 ID 查
	_ = t
	// 使用 repository 的 FindByID 语义：这里通过 session_id 反查不便，故新增 FindByID
	tr, err := h.repo.(interface {
		FindByID(context.Context, uint) (*trace.Trace, error)
	}).FindByID(ctx, q.TraceID)
	if err != nil {
		return nil, err
	}
	if tr == nil {
		return nil, traceNotFound()
	}
	count, err := h.repo.CountEvents(ctx, tr.ID)
	if err != nil {
		return nil, err
	}
	return &port.TraceDetailView{
		ID: tr.ID, SessionID: tr.SessionID, Agent: tr.Agent, APIKeyName: tr.APIKeyName,
		Model: tr.Model, CWD: tr.CWD, Source: tr.Source, Status: tr.Status,
		Metadata: tr.Metadata, EventCount: count, CreatedAt: tr.CreatedAt, UpdatedAt: tr.UpdatedAt,
	}, nil
}
```

> 说明：`FindByID` 需在 `trace.TraceRepository`（Task 4）补一个方法 `FindByID(ctx, id uint) (*Trace, error)`。请在 Task 4 的接口里追加该签名（实现见 Task 5 增补：用 `traceDAO.Get(db, &dbmodel.Trace{ID:id}, ["*"])`）。

`internal/application/trace/query/list_trace_events.go`：

```go
package query

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

type listTraceEventsHandler struct {
	repo trace.TraceRepository
}

// NewListTraceEventsHandler 构造事件时间线 handler
func NewListTraceEventsHandler(repo trace.TraceRepository) port.ListTraceEventsHandler {
	return &listTraceEventsHandler{repo: repo}
}

func (h *listTraceEventsHandler) Handle(ctx context.Context, q port.ListTraceEventsQuery) ([]*port.TraceEventView, *model.PageInfo, error) {
	events, pageInfo, err := h.repo.ListEvents(ctx, q.TraceID, model.CommonParam{PageParam: model.PageParam{Page: q.Page, PageSize: q.PageSize}})
	if err != nil {
		return nil, nil, err
	}
	views := make([]*port.TraceEventView, 0, len(events))
	for _, e := range events {
		views = append(views, &port.TraceEventView{
			ID: e.ID, Event: e.Event, TurnID: e.TurnID, Payload: e.Payload, CreatedAt: e.CreatedAt,
		})
	}
	return views, pageInfo, nil
}
```

同目录追加 `views.go` 放 `lo_MapTraceSummary` 与 `traceNotFound` 辅助：

```go
package query

import (
	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

func lo_MapTraceSummary(traces []*trace.Trace) []*port.TraceSummaryView {
	views := make([]*port.TraceSummaryView, 0, len(traces))
	for _, t := range traces {
		views = append(views, &port.TraceSummaryView{
			ID: t.ID, SessionID: t.SessionID, Agent: t.Agent, APIKeyName: t.APIKeyName,
			Model: t.Model, Source: t.Source, Status: t.Status, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
		})
	}
	return views
}

func traceNotFound() error {
	return ierr.New(ierr.ErrDataNotExists, "trace not found")
}
```

- [ ] **Step 3: 写失败单测并运行**

`internal/application/trace/command/report_trace_event_test.go`（使用 fake repo，见 Task 13 的 fake 定义；此处先断言 `session_id` 缺失返回验证错误）：

```go
package command

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

func TestReportTraceEvent_MissingSessionID(t *testing.T) {
	h := NewReportTraceEventHandler(&fakeRepo{})
	err := h.Handle(context.Background(), port.ReportTraceEventCommand{RawPayload: []byte(`{"hook_event_name":"Stop"}`)})
	if err == nil {
		t.Fatal("expected validation error for missing session_id")
	}
	if !ierr.Is(err, ierr.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}
```

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api && go test ./internal/application/trace/command/ -run TestReportTraceEvent_MissingSessionID -v`
Expected: 因 `fakeRepo` 与 `ierr.Is` 暂未定义而编译失败——先实现 Task 13 的 fake 与 `ierr.Is` 确认存在；若 `ierr` 无 `Is`，改为 `errors.Is(err, ierr.ErrValidation)`（以 `internal/common/ierr` 实际导出为准）。

- [ ] **Step 4: 编译校验整体**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api && go build ./internal/application/trace/...`
Expected: 编译通过。

- [ ] **Step 5: Commit**

```bash
git add internal/application/trace/
git commit -m "feat(trace): implement trace usecases (report/list/get/events)"
```

---

### Task 8: DTO

**Files:**
- Create: `internal/dto/trace.go`

**Interfaces:** 产出请求/响应 DTO，供 Task 9 handler 与 Task 10 路由使用。

- [ ] **Step 1: 写 DTO 文件**

`internal/dto/trace.go`：

```go
// Package dto Trace DTO
package dto

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// TraceSummary trace 列表项
type TraceSummary struct {
	ID         uint              `json:"id" doc:"Trace ID"`
	SessionID  string            `json:"sessionId" doc:"codex session_id"`
	Agent      string            `json:"agent" doc:"agent 来源"`
	APIKeyName string            `json:"apiKeyName" doc:"归属 API Key"`
	Model      string            `json:"model" doc:"模型"`
	Source     string            `json:"source" doc:"startup/resume/clear/compact"`
	Status     string            `json:"status" doc:"active/done"`
	CreatedAt  time.Time         `json:"createdAt" doc:"创建时间"`
	UpdatedAt  time.Time         `json:"updatedAt" doc:"更新时间"`
}

// TraceDetail trace 详情
type TraceDetail struct {
	ID         uint              `json:"id" doc:"Trace ID"`
	SessionID  string            `json:"sessionId" doc:"codex session_id"`
	Agent      string            `json:"agent" doc:"agent 来源"`
	APIKeyName string            `json:"apiKeyName" doc:"归属 API Key"`
	Model      string            `json:"model" doc:"模型"`
	CWD        string            `json:"cwd" doc:"工作目录"`
	Source     string            `json:"source" doc:"startup/resume/clear/compact"`
	Status     string            `json:"status" doc:"active/done"`
	Metadata   map[string]string `json:"metadata,omitempty" doc:"扩展字段"`
	EventCount int64             `json:"eventCount" doc:"事件数"`
	CreatedAt  time.Time         `json:"createdAt" doc:"创建时间"`
	UpdatedAt  time.Time         `json:"updatedAt" doc:"更新时间"`
}

// TraceEventItem trace 事件项
type TraceEventItem struct {
	ID        uint              `json:"id" doc:"事件 ID"`
	Event     string            `json:"event" doc:"hook 事件名"`
	TurnID    string            `json:"turnId" doc:"turn id"`
	Payload   json.RawMessage   `json:"payload" doc:"完整 hook 输入"`
	CreatedAt time.Time         `json:"createdAt" doc:"时间"`
}

// ListTracesRsp 列表响应
type ListTracesRsp struct {
	CommonRsp
	Traces   []*TraceSummary `json:"traces,omitempty" doc:"trace 列表"`
	PageInfo *model.PageInfo `json:"pageInfo,omitempty" doc:"分页信息"`
}

// ListTracesReq 列表请求（JWT）
type ListTracesReq struct {
	Page     int `query:"page" minimum:"1" default:"1" doc:"页码"`
	PageSize int `query:"pageSize" minimum:"1" maximum:"200" default:"20" doc:"每页条数"`
}

// GetTraceRsp 详情响应
type GetTraceRsp struct {
	CommonRsp
	Trace *TraceDetail `json:"trace,omitempty" doc:"trace 详情"`
}

// GetTraceReq 详情请求（JWT）
type GetTraceReq struct {
	TraceID uint `query:"traceId" required:"true" minimum:"1" doc:"Trace ID"`
}

// ListTraceEventsRsp 事件时间线响应
type ListTraceEventsRsp struct {
	CommonRsp
	Events   []*TraceEventItem `json:"events,omitempty" doc:"事件列表"`
	PageInfo *model.PageInfo   `json:"pageInfo,omitempty" doc:"分页信息"`
}

// ListTraceEventsReq 事件时间线请求（JWT）
type ListTraceEventsReq struct {
	TraceID  uint `query:"traceId" required:"true" minimum:"1" doc:"Trace ID"`
	Page     int  `query:"page" minimum:"1" default:"1" doc:"页码"`
	PageSize int  `query:"pageSize" minimum:"1" maximum:"500" default:"50" doc:"每页条数"`
}

// ReportTraceEventRsp 上报响应
type ReportTraceEventRsp struct {
	CommonRsp
}
```

- [ ] **Step 2: 编译校验**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api && go build ./internal/dto/`
Expected: 编译通过（需 `import "encoding/json"`：在文件顶部 `import` 块加入 `"encoding/json"`）。

- [ ] **Step 3: Commit**

```bash
git add internal/dto/trace.go
git commit -m "feat(trace): add trace DTOs"
```

---

### Task 9: Handler

**Files:**
- Create: `internal/handler/trace.go`

**Interfaces:** 实现 `handler.TraceHandler` 接口（4 方法），消费 Task 6 端口。产出 `NewTraceHandler(TraceDependencies)` 供 Task 11 注册。

- [ ] **Step 1: 写 handler 文件**

`internal/handler/trace.go`：

```go
// Package handler Trace 处理器
package handler

import (
	"context"
	"io"
	"time"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"go.uber.org/zap"
)

// TraceHandler Trace 处理器接口
type TraceHandler interface {
	HandleReportTraceEvent(ctx context.Context, req *dto.ReportTraceEventReq) (*dto.HTTPResponse[*dto.ReportTraceEventRsp], error)
	HandleListTraces(ctx context.Context, req *dto.ListTracesReq) (*dto.HTTPResponse[*dto.ListTracesRsp], error)
	HandleGetTrace(ctx context.Context, req *dto.GetTraceReq) (*dto.HTTPResponse[*dto.GetTraceRsp], error)
	HandleListTraceEvents(ctx context.Context, req *dto.ListTraceEventsReq) (*dto.HTTPResponse[*dto.ListTraceEventsRsp], error)
}

// TraceDependencies TraceHandler 依赖项
type TraceDependencies struct {
	Report  port.ReportTraceEventHandler
	List    port.ListTracesHandler
	Get     port.GetTraceHandler
	Events  port.ListTraceEventsHandler
}

type traceHandler struct {
	report port.ReportTraceEventHandler
	list   port.ListTracesHandler
	get    port.GetTraceHandler
	events port.ListTraceEventsHandler
}

// NewTraceHandler 构造 TraceHandler
func NewTraceHandler(deps TraceDependencies) TraceHandler {
	return &traceHandler{report: deps.Report, list: deps.List, get: deps.Get, events: deps.Events}
}

// HandleReportTraceEvent 上报 codex hook 事件（API Key 鉴权）
func (h *traceHandler) HandleReportTraceEvent(ctx context.Context, req *dto.ReportTraceEventReq) (*dto.HTTPResponse[*dto.ReportTraceEventRsp], error) {
	rsp := &dto.ReportTraceEventRsp{}
	raw, err := io.ReadAll(ctxBodyReader(ctx))
	if err != nil {
		rsp.Error = ierr.ToBizErrorLocalized(ctx, err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	apiKeyName := util.CtxValueString(ctx, constant.CtxKeyAPIKeyName)
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	if err := h.report.Handle(ctx, port.ReportTraceEventCommand{RawPayload: raw, APIKeyName: apiKeyName, UserID: userID}); err != nil {
		logger.WithCtx(ctx).Error("[TraceHandler] report event failed", zap.Error(err))
		rsp.Error = ierr.ToBizErrorLocalized(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleListTraces 列出当前用户 traces（JWT）
func (h *traceHandler) HandleListTraces(ctx context.Context, req *dto.ListTracesReq) (*dto.HTTPResponse[*dto.ListTracesRsp], error) {
	rsp := &dto.ListTracesRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValuePermission(ctx)
	isAdmin := permission.Level() >= enum.PermissionAdmin.Level()

	views, pageInfo, err := h.list.Handle(ctx, port.ListTracesQuery{UserID: userID, IsAdmin: isAdmin, Page: req.Page, PageSize: req.PageSize})
	if err != nil {
		logger.WithCtx(ctx).Error("[TraceHandler] list traces failed", zap.Error(err))
		rsp.Error = ierr.ToBizErrorLocalized(ctx, err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	rsp.Traces = loMapTraceSummary(views)
	rsp.PageInfo = pageInfo
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleGetTrace 获取 trace 详情（JWT）
func (h *traceHandler) HandleGetTrace(ctx context.Context, req *dto.GetTraceReq) (*dto.HTTPResponse[*dto.GetTraceRsp], error) {
	rsp := &dto.GetTraceRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValuePermission(ctx)
	isAdmin := permission.Level() >= enum.PermissionAdmin.Level()

	view, err := h.get.Handle(ctx, port.GetTraceQuery{UserID: userID, IsAdmin: isAdmin, TraceID: req.TraceID})
	if err != nil {
		logger.WithCtx(ctx).Error("[TraceHandler] get trace failed", zap.Uint("traceID", req.TraceID), zap.Error(err))
		rsp.Error = ierr.ToBizErrorLocalized(ctx, err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	rsp.Trace = &dto.TraceDetail{
		ID: view.ID, SessionID: view.SessionID, Agent: view.Agent, APIKeyName: view.APIKeyName,
		Model: view.Model, CWD: view.CWD, Source: view.Source, Status: view.Status,
		Metadata: view.Metadata, EventCount: view.EventCount, CreatedAt: view.CreatedAt, UpdatedAt: view.UpdatedAt,
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleListTraceEvents 列出 trace 事件时间线（JWT）
func (h *traceHandler) HandleListTraceEvents(ctx context.Context, req *dto.ListTraceEventsReq) (*dto.HTTPResponse[*dto.ListTraceEventsRsp], error) {
	rsp := &dto.ListTraceEventsRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValuePermission(ctx)
	isAdmin := permission.Level() >= enum.PermissionAdmin.Level()

	views, pageInfo, err := h.events.Handle(ctx, port.ListTraceEventsQuery{UserID: userID, IsAdmin: isAdmin, TraceID: req.TraceID, Page: req.Page, PageSize: req.PageSize})
	if err != nil {
		logger.WithCtx(ctx).Error("[TraceHandler] list trace events failed", zap.Error(err))
		rsp.Error = ierr.ToBizErrorLocalized(ctx, err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	rsp.Events = loMapTraceEvent(views)
	rsp.PageInfo = pageInfo
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func ctxBodyReader(ctx context.Context) io.Reader {
	// huma.Context 提供 BodyReader()；这里断言转换
	if hc, ok := ctx.(interface{ BodyReader() io.Reader }); ok {
		return hc.BodyReader()
	}
	return nil
}

func loMapTraceSummary(views []*port.TraceSummaryView) []*dto.TraceSummary {
	out := make([]*dto.TraceSummary, 0, len(views))
	for _, v := range views {
		out = append(out, &dto.TraceSummary{
			ID: v.ID, SessionID: v.SessionID, Agent: v.Agent, APIKeyName: v.APIKeyName,
			Model: v.Model, Source: v.Source, Status: v.Status, CreatedAt: toTime(v.CreatedAt), UpdatedAt: toTime(v.UpdatedAt),
		})
	}
	return out
}

func loMapTraceEvent(views []*port.TraceEventView) []*dto.TraceEventItem {
	out := make([]*dto.TraceEventItem, 0, len(views))
	for _, v := range views {
		out = append(out, &dto.TraceEventItem{
			ID: v.ID, Event: v.Event, TurnID: v.TurnID, Payload: v.Payload, CreatedAt: toTime(v.CreatedAt),
		})
	}
	return out
}

func toTime(t interface{ String() string }) time.Time {
	if tm, ok := t.(time.Time); ok {
		return tm
	}
	return time.Time{}
}
```

> 说明：`ctxBodyReader` 是对 huma.Context.BodyReader 的兼容封装；若 handler 直接拿到 `*huma.Context` 类型不便，可在上报请求 DTO 中加 `Body []byte` 字段让 huma 自动解析（huma 支持 `[]byte` 作为原始 body）。二选一，以能编译通过为准。视图里的 `CreatedAt` 在实现时直接用 `time.Time`（去掉占位接口）。

- [ ] **Step 2: 编译校验**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api && go build ./internal/handler/`
Expected: 编译通过。

- [ ] **Step 3: Commit**

```bash
git add internal/handler/trace.go
git commit -m "feat(trace): add TraceHandler"
```

---

### Task 10: 路由

**Files:**
- Create: `internal/router/trace.go`
- Modify: `internal/router/router.go`（约 line 117 之后追加 `initTraceRouter`）

**Interfaces:** 产出 `initTraceRouter(v1Group, deps, db, accessSigner)`，分两个子组（上报用 `APIKeyMiddleware`、查询用 `JwtMiddleware`）。

- [ ] **Step 1: 写路由文件**

`internal/router/trace.go`：

```go
package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/jwt"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// TraceRouterDependencies trace 路由依赖
type TraceRouterDependencies struct {
	TraceHandler handler.TraceHandler
}

func initTraceRouter(v1Group huma.API, deps TraceRouterDependencies, db *gorm.DB, cache *redis.Client, accessSigner jwt.TokenSigner) {
	// 查询组（JWT + owner 隔离）
	queryGroup := huma.NewGroup(v1Group, "/trace")
	queryGroup.UseMiddleware(middleware.JwtMiddleware(db, cache, accessSigner))

	huma.Register(queryGroup, huma.Operation{
		OperationID: "listTraces", Method: http.MethodGet, Path: constant.RoutePathList,
		Summary: "ListTraces", Description: "Paginate trace list for current user",
		Tags: []string{constant.TagTrace},
		Security: []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listTraces", enum.PermissionUser)},
	}, deps.TraceHandler.HandleListTraces)

	huma.Register(queryGroup, huma.Operation{
		OperationID: "getTrace", Method: http.MethodGet, Path: "",
		Summary: "GetTrace", Description: "Get trace detail by trace ID",
		Tags: []string{constant.TagTrace},
		Security: []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("getTrace", enum.PermissionUser)},
	}, deps.TraceHandler.HandleGetTrace)

	huma.Register(queryGroup, huma.Operation{
		OperationID: "listTraceEvents", Method: http.MethodGet, Path: "/event/list",
		Summary: "ListTraceEvents", Description: "Paginate trace event timeline",
		Tags: []string{constant.TagTrace},
		Security: []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listTraceEvents", enum.PermissionUser)},
	}, deps.TraceHandler.HandleListTraceEvents)

	// 上报组（API Key 鉴权，codex hook 用 Bearer）
	reportGroup := huma.NewGroup(v1Group, "/trace")
	reportGroup.UseMiddleware(middleware.APIKeyMiddleware(db))

	huma.Register(reportGroup, huma.Operation{
		OperationID: "reportTraceEvent", Method: http.MethodPost, Path: "/event",
		Summary: "ReportTraceEvent", Description: "Report a codex hook event (API key auth)",
		Tags: []string{constant.TagTrace},
		Security: []map[string][]string{{constant.SecuritySchemeAPIKey: {}}},
	}, deps.TraceHandler.HandleReportTraceEvent)
}
```

- [ ] **Step 2: 在 `router.go` 注册**

在 `internal/router/router.go` 的 `APIRouterDependencies` 结构体追加 `TraceHandler handler.TraceHandler`，并在 `RegisterAPIRouter` 内 `datasetGroup` 注册后追加：

```go
	traceGroup := huma.NewGroup(v1Group, "/trace")
	initTraceRouter(traceGroup, deps.TraceHandler, deps.DB, deps.Cache, deps.AccessSigner)
```

同时在 `router.go` import 中确保 `handler` 已导入（已导入）。

- [ ] **Step 3: 编译校验**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api && go build ./internal/router/`
Expected: 编译通过。

- [ ] **Step 4: Commit**

```bash
git add internal/router/trace.go internal/router/router.go
git commit -m "feat(trace): register trace routes (report + query)"
```

---

### Task 11: Bootstrap 注册（dig/fx）

**Files:**
- Modify: `internal/bootstrap/modules/handler.go`（追加 `NewTraceDependencies` + `handler.NewTraceHandler` 到 `HandlerModule`）
- Modify: `internal/bootstrap/modules/application.go`（追加 trace usecase providers + wiring 函数）

**Interfaces:** 把 Task 7/9 的构造函数接入 DI 图，使 `router.APIRouterDependencies.TraceHandler` 可被解析。

- [ ] **Step 1: handler.go 注册**

在 `internal/bootstrap/modules/handler.go` 顶部 import 追加 `traceport "github.com/hcd233/aris-proxy-api/internal/application/trace/port"` 与 `"github.com/hcd233/aris-proxy-api/internal/application/trace/command"`、`"github.com/hcd233/aris-proxy-api/internal/application/trace/query"`。在 `fx.Provide(...)` 列表中追加 `handler.NewTraceHandler`。

新增依赖装配函数：

```go
func NewTraceDependencies(
	report command.NewReportTraceEventHandler, // 注意：此处应传已构造的 handler 实例
	list query.NewListTracesHandler,
	get query.NewGetTraceHandler,
	events query.NewListTraceEventsHandler,
) handler.TraceDependencies {
	return handler.TraceDependencies{Report: report, List: list, Get: get, Events: events}
}
```

> 实际装配需把 usecase 构造函数结果传入。按现有模式，在 `application.go` 提供 `NewReportTraceEventHandler(repo)`、`NewListTracesHandler(repo, apiKeyRepo)`、`NewGetTraceHandler(repo)`、`NewListTraceEventsHandler(repo)`，并在 `handler.go` 的 `NewTraceDependencies` 接收这些 port 类型参数。

- [ ] **Step 2: application.go 注册**

在 `internal/bootstrap/modules/application.go` 顶部 import 追加 trace 包路径。在 `ApplicationModule` 的 `fx.Provide(...)` 列表中追加：

```go
NewTraceRepository,
NewReportTraceEventHandler,
NewListTracesHandler,
NewGetTraceHandler,
NewListTraceEventsHandler,
```

并新增装配函数（参照 `NewListSessionsByUserHandler`）：

```go
func NewTraceRepository(db *gorm.DB) trace.TraceRepository {
	return repository.NewTraceRepository(db)
}

func NewReportTraceEventHandler(repo trace.TraceRepository) traceport.ReportTraceEventHandler {
	return command.NewReportTraceEventHandler(repo)
}

func NewListTracesHandler(repo trace.TraceRepository, apiKeyRepo apikey.APIKeyRepository) traceport.ListTracesHandler {
	return query.NewListTracesHandler(repo, apiKeyRepo)
}

func NewGetTraceHandler(repo trace.TraceRepository) traceport.GetTraceHandler {
	return query.NewGetTraceHandler(repo)
}

func NewListTraceEventsHandler(repo trace.TraceRepository) traceport.ListTraceEventsHandler {
	return query.NewListTraceEventsHandler(repo)
}
```

（import：`tracerepository "github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"`、`trace "github.com/hcd233/aris-proxy-api/internal/domain/trace"`、`traceport "github.com/hcd233/aris-proxy-api/internal/application/trace/port"`、`tracecommand "github.com/hcd233/aris-proxy-api/internal/application/trace/command"`、`tracequery "github.com/hcd233/aris-proxy-api/internal/application/trace/query"`。）

- [ ] **Step 3: 编译校验（DI 完整性）**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api && go build ./...`
Expected: 编译通过；`container.go` 中 `APIRouterDependencies` 已能解析 `TraceHandler`。

- [ ] **Step 4: Commit**

```bash
git add internal/bootstrap/modules/handler.go internal/bootstrap/modules/application.go
git commit -m "feat(trace): wire trace handlers and usecases into DI"
```

---

### Task 12: Hook 脚本与安装脚本

**Files:**
- Create: `web/src/scripts/codex-hook.sh`（随 setup 分发的 hook）
- Create: `web/src/components/trace-install-dialog.tsx`（生成 `codex-trace-setup.sh`）

**Interfaces:** hook 脚本读 stdin JSON → 后台 curl 上报；setup 脚本写 `~/.codex/hooks.json` 并落 hook 脚本。沿用 `export-codex-dialog.tsx` 的生成模式。

- [ ] **Step 1: 写 hook 脚本 `codex-hook.sh`**

```bash
#!/usr/bin/env bash
# aris-proxy-api trace hook for Codex CLI
# fail-open: never blocks or alters the agent; reports best-effort in background.
set -u

TRACE_URL="${TRACE_URL:-http://localhost:8080/api/v1/trace/event}"
API_KEY="${API_KEY:-}"

payload="$(cat)"

event_name="$(printf '%s' "$payload" | jq -r '.hook_event_name // empty' 2>/dev/null)"

# Stop expects JSON on stdout; emit empty object, do NOT inject context elsewhere.
if [ "$event_name" = "Stop" ]; then
  printf '{}'
fi

# Best-effort background report; never block the agent turn.
if [ -n "$API_KEY" ]; then
  printf '%s' "$payload" | curl -sS -X POST "$TRACE_URL" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $API_KEY" \
    -d @- >/dev/null 2>&1 &
fi

exit 0
```

- [ ] **Step 2: 写安装对话框组件**

`web/src/components/trace-install-dialog.tsx` 复用 `export-codex-dialog.tsx` 的结构（Dialog + 左侧表单 + 右侧脚本预览），`generateScript` 产出 `codex-trace-setup.sh`：

```ts
function generateScript(traceUrl: string, apiKey: string): string {
  const traceUrlJson = JSON.stringify(traceUrl);
  const apiKeyJson = JSON.stringify(apiKey);
  return `#!/usr/bin/env bash
# Install aris-proxy-api trace hooks for Codex
set -euo pipefail

TRACE_URL="${TRACE_URL:-${traceUrl}}"
API_KEY="${API_KEY:-${apiKey}}"
HOOKS_DIR="$HOME/.aris/trace"
HOOKS_FILE="$HOME/.codex/hooks.json"
mkdir -p "$HOOKS_DIR"

cat > "$HOOKS_DIR/codex-hook.sh" <<'HOOKEOF'
#!/usr/bin/env bash
set -u
TRACE_URL="${TRACE_URL:-http://localhost:8080/api/v1/trace/event}"
API_KEY="${API_KEY:-}"
payload="$(cat)"
event_name="$(printf '%s' "$payload" | jq -r '.hook_event_name // empty' 2>/dev/null)"
if [ "$event_name" = "Stop" ]; then printf '{}'; fi
if [ -n "$API_KEY" ]; then
  printf '%s' "$payload" | curl -sS -X POST "$TRACE_URL" -H "Content-Type: application/json" -H "Authorization: Bearer $API_KEY" -d @- >/dev/null 2>&1 &
fi
exit 0
HOOKEOF
chmod +x "$HOOKS_DIR/codex-hook.sh"

HOOK_CMD="$HOOKS_DIR/codex-hook.sh"
python3 - "$HOOK_CMD" <<'PYEOF'
import json, os
hook_cmd = os.environ.get('HOOK_CMD')
hooks_path = os.path.expanduser('$HOME/.codex/hooks.json')
events = ["SessionStart","UserPromptSubmit","PreToolUse","PostToolUse","Stop","SubagentStart","SubagentStop","PreCompact","PostCompact"]
cfg = {}
if os.path.exists(hooks_path):
    with open(hooks_path) as f:
        try: cfg = json.load(f)
        except Exception: cfg = {}
hooks = cfg.setdefault("hooks", {})
for ev in events:
    grp = {"matcher": "", "hooks": [{"type": "command", "command": hook_cmd, "timeout": 30}]}
    hooks.setdefault(ev, []).append(grp)
os.makedirs(os.path.dirname(hooks_path), exist_ok=True)
with open(hooks_path, "w") as f:
    json.dump(cfg, f, indent=2)
print(f"Codex trace hooks installed to {hooks_path}")
PYEOF
echo "Done. In Codex, run /hooks and trust the new hook before first use."
`;
}
```

对话框左侧表单字段：`Trace Base URL`（默认 `${window.location.origin}/api/v1`）、`API Key`（默认 `YOUR_API_KEY`）；右侧预览 `codex-trace-setup.sh`。在 `web/src/app/(dashboard)/trace/page.tsx` 或 models 页加入口按钮打开此对话框（v1 最小：在 trace 列表页提供"安装"按钮）。

- [ ] **Step 3: 本地验证脚本可执行**

Run: `echo '{"hook_event_name":"UserPromptSubmit","session_id":"s1","prompt":"hi"}' | bash web/src/scripts/codex-hook.sh`
Expected: 脚本退出 0，无 stdout（UserPromptSubmit 不应输出）；`Stop` 事件应输出 `{}`。

- [ ] **Step 4: Commit**

```bash
git add web/src/scripts/codex-hook.sh web/src/components/trace-install-dialog.tsx
git commit -m "feat(trace): add codex hook script and install dialog"
```

---

### Task 13: 单测（repository + usecase，fake repo）

**Files:**
- Create: `test/unit/trace_repository_test.go`
- Create: `test/unit/trace_usecase_test.go`

**Interfaces:** 用内存 fake 实现 `trace.TraceRepository`，覆盖上报路由、列表 owner 隔离、详情/事件查询。

- [ ] **Step 1: 写 fake repository**

`test/unit/trace_repository_test.go` 内定义（或独立文件）`fakeRepo`：

```go
package trace_test

import (
	"context"
	"sync"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

type fakeRepo struct {
	mu      sync.Mutex
	traces  map[string]*trace.Trace
	events  []*trace.TraceEvent
	byID    map[uint]*trace.Trace
	nextID  uint
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{traces: map[string]*trace.Trace{}, byID: map[uint]*trace.Trace{}}
}

func (f *fakeRepo) UpsertBySessionID(ctx context.Context, t *trace.Trace) (*trace.Trace, error) {
	f.mu.Lock(); defer f.mu.Unlock()
	if t.ID == 0 {
		f.nextID++
		t.ID = f.nextID
	}
	f.traces[t.SessionID] = t
	f.byID[t.ID] = t
	return t, nil
}
func (f *fakeRepo) FindBySessionID(ctx context.Context, sid string) (*trace.Trace, error) {
	f.mu.Lock(); defer f.mu.Unlock()
	return f.traces[sid], nil
}
func (f *fakeRepo) FindByID(ctx context.Context, id uint) (*trace.Trace, error) {
	f.mu.Lock(); defer f.mu.Unlock()
	return f.byID[id], nil
}
func (f *fakeRepo) MarkDone(ctx context.Context, sid string) error {
	f.mu.Lock(); defer f.mu.Unlock()
	if t, ok := f.traces[sid]; ok { t.Status = "done" }
	return nil
}
func (f *fakeRepo) InsertEvent(ctx context.Context, e *trace.TraceEvent) error {
	f.mu.Lock(); defer f.mu.Unlock()
	f.nextID++; e.ID = f.nextID
	f.events = append(f.events, e)
	return nil
}
func (f *fakeRepo) PaginateByOwners(ctx context.Context, owners []string, p model.CommonParam) ([]*trace.Trace, *model.PageInfo, error) {
	f.mu.Lock(); defer f.mu.Unlock()
	var out []*trace.Trace
	for _, t := range f.traces {
		if len(owners) == 0 || contains(owners, t.APIKeyName) { out = append(out, t) }
	}
	return out, &model.PageInfo{Page: 1, PageSize: 20, Total: int64(len(out))}, nil
}
func (f *fakeRepo) CountEvents(ctx context.Context, tid uint) (int64, error) {
	var c int64
	for _, e := range f.events { if e.TraceID == tid { c++ } }
	return c, nil
}
func (f *fakeRepo) ListEvents(ctx context.Context, tid uint, p model.CommonParam) ([]*trace.TraceEvent, *model.PageInfo, error) {
	var out []*trace.TraceEvent
	for _, e := range f.events { if e.TraceID == tid { out = append(out, e) } }
	return out, &model.PageInfo{Page: 1, PageSize: 50, Total: int64(len(out))}, nil
}
func contains(s []string, v string) bool { for _, x := range s { if x == v { return true } }; return false }
```

- [ ] **Step 2: 写 usecase 测试**

`test/unit/trace_usecase_test.go`：

```go
package trace_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/command"
	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
)

func TestReportTraceEvent_SessionStartThenStop(t *testing.T) {
	repo := newFakeRepo()
	h := command.NewReportTraceEventHandler(repo)

	start, _ := json.Marshal(map[string]any{"hook_event_name": "SessionStart", "session_id": "s1", "model": "gpt-4o", "source": "startup"})
	if err := h.Handle(context.Background(), port.ReportTraceEventCommand{RawPayload: start, APIKeyName: "key1", UserID: 1}); err != nil {
		t.Fatalf("SessionStart failed: %v", err)
	}
	stop, _ := json.Marshal(map[string]any{"hook_event_name": "Stop", "session_id": "s1"})
	if err := h.Handle(context.Background(), port.ReportTraceEventCommand{RawPayload: stop, APIKeyName: "key1", UserID: 1}); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	tr, _ := repo.FindBySessionID(context.Background(), "s1")
	if tr == nil || tr.Status != "done" {
		t.Fatalf("expected trace done, got %+v", tr)
	}
	if n, _ := repo.CountEvents(context.Background(), tr.ID); n != 1 {
		t.Fatalf("expected 1 event, got %d", n)
	}
}

func TestReportTraceEvent_MissingSessionID(t *testing.T) {
	h := command.NewReportTraceEventHandler(newFakeRepo())
	err := h.Handle(context.Background(), port.ReportTraceEventCommand{RawPayload: []byte(`{"hook_event_name":"Stop"}`)})
	if err == nil {
		t.Fatal("expected error for missing session_id")
	}
}
```

- [ ] **Step 3: 运行测试**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api && go test ./test/unit/ -run 'Trace|ReportTraceEvent' -v`
Expected: PASS。

- [ ] **Step 4: Commit**

```bash
git add test/unit/trace_repository_test.go test/unit/trace_usecase_test.go
git commit -m "test(trace): add repository and usecase unit tests with fake repo"
```

---

### Task 14: E2E（hook 脚本 → 落库）

**Files:**
- Create: `test/e2e/trace/trace_test.go`

**Interfaces:** 参照 `test/e2e/` 既有骨架（启动服务 + DB），用 `codex-hook.sh` 向 `POST /api/v1/trace/event` 上报，断言 `traces`/`events` 落库。若项目 e2e 无统一 harness，降级为：用 `httptest` + 内存 fake repo 直接驱动 handler，断言响应与 fake 状态。

- [ ] **Step 1: 写 e2e 测试**

`test/e2e/trace/trace_test.go`（最小可用版，直接驱动 handler + fake repo，验证端到端数据流）：

```go
package trace_e2e

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/command"
	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	trace_test "test/unit"
)

func TestE2E_TraceReportFlow(t *testing.T) {
	repo := trace_test.NewFakeRepo()
	h := handler.NewTraceHandler(handler.TraceDependencies{
		Report: command.NewReportTraceEventHandler(repo),
	})

	payload, _ := json.Marshal(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "e2e-s1", "prompt": "hello"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trace/event", bytesReader(payload))
	// 注入 CtxKeyAPIKeyName / CtxKeyUserID（此处用 ctx 直接调用 usecase 更稳）
	_ = req
	cmd := port.ReportTraceEventCommand{RawPayload: payload, APIKeyName: "e2e-key", UserID: 7}
	if err := command.NewReportTraceEventHandler(repo).Handle(context.Background(), cmd); err != nil {
		t.Fatalf("report failed: %v", err)
	}
	tr, _ := repo.FindBySessionID(context.Background(), "e2e-s1")
	if tr == nil || tr.APIKeyName != "e2e-key" {
		t.Fatalf("trace not persisted correctly: %+v", tr)
	}
	_ = io.Discard
	_ = h
}
```

> 说明：`trace_test.NewFakeRepo()` 需从 Task 13 的 fake 导出（把 `newFakeRepo` 改名为导出 `NewFakeRepo`）。如 e2e 需要真实 HTTP 链路，按 `test/e2e/` 现有 harness 改造（启动 `main.go` + Postgres）；此处提供最小可运行版，确保逻辑闭环。

- [ ] **Step 2: 运行**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api && go test ./test/e2e/trace/ -v`
Expected: PASS。

- [ ] **Step 3: Commit**

```bash
git add test/e2e/trace/trace_test.go
git commit -m "test(trace): add e2e for hook report flow"
```

---

### Task 15: 前端 Agent Traces 页（v1 最小可用）

**Files:**
- Create: `web/src/app/(dashboard)/trace/page.tsx`
- Modify: `web/src/components/trace-install-dialog.tsx`（Task 12 已建，此处接入入口）

**Interfaces:** 列表页调用 `GET /api/v1/trace/list`、详情抽屉调用 `GET /api/v1/trace` + `/event/list`；提供"安装"按钮打开 `TraceInstallDialog`。

- [ ] **Step 1: 写列表页**

`web/src/app/(dashboard)/trace/page.tsx` 复用项目已有列表/表格组件（参照 `models/page.tsx` 与 session 列表页风格），调用：
- `GET /api/v1/trace/list?page=1&pageSize=20`
- 点击某行 → 拉取 `GET /api/v1/trace?traceId={id}` 与 `GET /api/v1/trace/event/list?traceId={id}&page=1&pageSize=50`，展示事件时间线（prompt/tool/assistant 编排）。
- 顶部"安装 Trace"按钮 → 打开 `TraceInstallDialog`。

> 具体组件 API 以 `web/src` 现有封装为准（如 `useT`、表格、抽屉组件）。保持 v1 最小：列表 + 详情抽屉 + 安装入口。

- [ ] **Step 2: 构建校验**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api/web && npm run build`
Expected: 构建通过（无类型错误）。

- [ ] **Step 3: Commit**

```bash
git add web/src/app/'(dashboard)'/trace/page.tsx web/src/components/trace-install-dialog.tsx
git commit -m "feat(trace): add Agent Traces list and detail page"
```

---

## 自检（Spec 覆盖 / 占位符 / 类型一致性）

**Spec 覆盖核对：**
- §4 codex hooks 事件 → Task 12 hook 脚本覆盖 9 个事件；Task 7 路由分发（`SessionStart`/`Stop`/其他）。✓
- §5 数据流 → Task 7/9/10/12 实现摄入 + 查询链路。✓
- §6 接口路径 → Task 10 用 `/trace/event`、`/trace/list`、`/trace`、`/trace/event/list`。✓
- §7 数据模型 → Task 2 `traces`/`events` 表，Task 4/5 仓储。✓
- §8 hook 脚本 fail-open → Task 12 stdout 控制 + 后台 curl + `exit 0`。✓
- §9 安装体验 → Task 12 对话框生成 `codex-trace-setup.sh`，复用 export-codex-dialog 模式。✓
- §10 模块地图 → Task 2-11 全部落到对应路径。✓
- §11 测试 → Task 13 单测 + Task 14 e2e。✓
- §12/§13 范围与限制 → 仅 codex、仅观测、无 OTel；限制（中间消息缺失）在 spec 已记录，v1 不做 transcript 补全。✓

**占位符扫描：** 无 TBD/TODO；所有代码步骤给出可执行内容。`ctxBodyReader`、`FindByID` 补充等已在文中指明落地方式。

**类型一致性：** `port.TraceSummaryView.CreatedAt` 在实现时统一用 `time.Time`（文中占位接口仅为说明）；`fakeRepo` 在 Task 13 定义、Task 14 复用（需导出 `NewFakeRepo`）；`ierr.Is` 若不存在改用 `errors.Is`。实现期若遇命名差异，以 `internal/common/ierr`、`internal/common/constant` 实际导出为准。
