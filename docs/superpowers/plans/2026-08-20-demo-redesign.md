# Demo 数据展示逻辑重设计 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 demo 从「取模抽样 + 模块白名单」重设计为「独立 tab + demo sessions 白名单 + 非 session 模块全量脱敏 + demo 接口 IP 限流」。

**Architecture:** 删除 `sample_modulus` 全链路，新增 `demo_sessions` 白名单表替代取模；session 查询按白名单过滤、非 session 模块按 `isDemo` 脱敏；`TokenBucketRateLimiterMiddleware` 增加 `WithPermissionFilter` option 对 demo 接口按 IP 限流；前端新增 `/demo` 页。

**Tech Stack:** Go 1.25 + Huma + GORM(AutoMigrate) + Redis + fx DI；前端 Next.js 静态导出 + Tailwind v4 + shadcn/ui。

## Global Constraints

- Go 代码必须加载 `golang-samber-lo` / `golang-samber-mo` / `golang-code-style` / `golang-naming` skill 遵守风格；用 `lo` 做切片变换。
- 编码阶段激活 `ponytail`（full）：不做投机抽象、复用 `MaskSecret`、新表最小字段。
- 脱敏范围（B 方案）：身份类 `UserName`/`UserEmail` 用 `MaskIdentity`（固定 `"***"`）；连接类 `APIKeyName`/`Endpoint`/`TraceID`/`UpstreamModel`/BaseURL 用 `MaskSecret`（前 4 + 后 4）。
- 限流阈值常量：`PeriodDemoAccess = 5 * time.Second`、`LimitDemoAccess = 30`。
- 所有新文档/注释/计划用中文；代码标识符保持英文。
- 提交前必须跑单测 + lint；E2E 沉淀到 `test/e2e/demo/`。
- 开发在 worktree `.worktrees/feature-demo-redesign-2026-08-20`（分支 `feature/demo-redesign-2026-08-20`）。

---

### Task 1: 限流中间件增加 functional options

**Files:**
- Modify: `internal/middleware/rate.go`
- Modify: `internal/common/constant/oauth.go`
- Test: `test/unit/rate_limit/rate_limit_option_test.go`（若目录不存在则 Create）

**Interfaces:**
- Consumes: `util.CtxValuePermission(ctx context.Context) enum.Permission`（`internal/util`），`enum.Permission`。
- Produces: `WithPermissionFilter(p enum.Permission) RateLimiterOption`；新签名 `TokenBucketRateLimiterMiddleware(cache *redis.Client, serviceName string, key enum.CtxKey, period time.Duration, capacity int64, opts ...RateLimiterOption) func(ctx huma.Context, next func(huma.Context))`。

- [ ] **Step 1: 新增常量**

`internal/common/constant/oauth.go`，紧邻 `PeriodDemoLogin`/`LimitDemoLogin` 之后追加：

```go
	// Demo 登录后的接口访问限流（IP 维度，防单访客刷爆共享账户）
	PeriodDemoAccess = 5 * time.Second
	LimitDemoAccess  = 30
```

- [ ] **Step 2: 新增 option 类型与过滤器**

`internal/middleware/rate.go`：在 `import` 块补 `"github.com/hcd233/aris-proxy-api/internal/util"`；在 `tokenBucketLua` 之前新增：

```go
// RateLimiterOption 限流中间件可选项
type RateLimiterOption func(*rateLimiterConfig)

// rateLimiterConfig 限流中间件配置
type rateLimiterConfig struct {
	// permissionFilter 仅对匹配权限的用户生效；空串表示不过滤（现状行为）
	permissionFilter enum.Permission
}

// WithPermissionFilter 仅对指定权限的用户启用限流（如 demo），其余用户零开销放行
func WithPermissionFilter(p enum.Permission) RateLimiterOption {
	return func(c *rateLimiterConfig) {
		c.permissionFilter = p
	}
}
```

- [ ] **Step 3: 改签名并在入口加权限过滤**

`rate.go` 中 `TokenBucketRateLimiterMiddleware` 签名改为：

```go
func TokenBucketRateLimiterMiddleware(cache *redis.Client, serviceName string, key enum.CtxKey, period time.Duration, capacity int64, opts ...RateLimiterOption) func(ctx huma.Context, next func(huma.Context)) {
	cfg := &rateLimiterConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	refillRate := float64(capacity) / float64(period.Microseconds())
	expireMs := period.Milliseconds() * 2
	retryAfterSeconds := int(math.Ceil(1.0 / (refillRate * 1e6)))

	return func(ctx huma.Context, next func(huma.Context)) {
		// 权限过滤：不匹配时直接放行，零开销（不碰 Redis）
		if cfg.permissionFilter != "" && util.CtxValuePermission(ctx.Context()) != cfg.permissionFilter {
			next(ctx)
			return
		}
		logger := logger.WithCtx(ctx.Context())
		// ... 以下为既有逻辑，原样保留（cache 判空、keyValue/value 计算、Lua 执行、响应头）
```

> 注意：仅插入上述 `cfg` 构造与入口过滤两段，函数体其余部分（`if cache == nil` 起的全部既有逻辑）保持原样，不要改动。

- [ ] **Step 4: 写单测**

`test/unit/rate_limit/rate_limit_option_test.go`：

```go
package rate_limit_test

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

func TestWithPermissionFilter(t *testing.T) {
	// 仅验证 option 可正常构造且不 panic；权限过滤行为由 Task 11 的 E2E 覆盖
	opt := middleware.WithPermissionFilter(enum.PermissionDemo)
	if opt == nil {
		t.Fatal("WithPermissionFilter returned nil")
	}
	_ = middleware.TokenBucketRateLimiterMiddleware(nil, "t", "", 0, 0, opt)
}
```

- [ ] **Step 5: 编译 + 测试**

Run: `rtk go build ./... && rtk go test ./internal/middleware/... ./test/unit/rate_limit/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/middleware/rate.go internal/common/constant/oauth.go test/unit/rate_limit/
git commit -m "feat(middleware): 限流中间件增加 WithPermissionFilter option"
```

---

### Task 2: `demo_sessions` 表模型 + 仓储 + 端口

**Files:**
- Create: `internal/infrastructure/database/model/demo_session.go`
- Modify: `internal/infrastructure/database/model/base.go`（注册 `&DemoSession{}`）
- Modify: `internal/common/constant/string.go`（`DemoSessionTableName`）
- Modify: `internal/application/demo/port/handler.go`（`DemoSessionRepository`、`DemoSessionAccessor` 接口）
- Create: `internal/infrastructure/repository/demo_session_repository.go`
- Modify: `internal/bootstrap/modules/repository.go`（`NewDemoSessionRepository`）
- Test: `test/unit/demo_session/demo_session_repository_test.go`

**Interfaces:**
- Consumes: `*gorm.DB`；`constant.DemoSessionTableName`。
- Produces: `model.DemoSession`（`SessionID uint` 唯一索引）；`demoport.DemoSessionRepository`（`List/Add/Remove`）。

- [ ] **Step 1: 表模型**

`internal/infrastructure/database/model/demo_session.go`：

```go
package model

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// DemoSession demo 会话白名单（admin 选取供 demo 只读访问的会话）
type DemoSession struct {
	ID        uint      `json:"id" gorm:"column:id;primary_key;auto_increment;comment:ID"`
	SessionID uint      `json:"session_id" gorm:"column:session_id;uniqueIndex;not null;comment:会话ID"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at;comment:创建时间"`
}

func (DemoSession) TableName() string {
	return constant.DemoSessionTableName
}
```

- [ ] **Step 2: 常量 + 模型注册**

`constant/string.go` 在 `DemoConfigTableName` 附近追加 `DemoSessionTableName = "demo_sessions"`；`model/base.go` 的 `Models` 列表追加 `&DemoSession{}`。

- [ ] **Step 3: 端口接口**

`internal/application/demo/port/handler.go` 末尾追加：

```go
// DemoSessionRepository demo 会话白名单仓储
type DemoSessionRepository interface {
	// List 返回全部白名单 sessionID（升序）
	List(ctx context.Context) ([]uint, error)
	// Add 批量插入（去重，已存在忽略）
	Add(ctx context.Context, ids []uint) error
	// Remove 批量删除
	Remove(ctx context.Context, ids []uint) error
}

// DemoSessionAccessor demo 会话白名单放行判断（session 查询视角，读取失败 fail-closed）
type DemoSessionAccessor interface {
	// AllowedIDs 返回白名单 sessionID 集合；读取失败返回 error（调用方拒绝请求）
	AllowedIDs(ctx context.Context) ([]uint, error)
	// IsAllowed 判断 sessionID 是否在白名单；读取失败返回 false
	IsAllowed(ctx context.Context, sessionID uint) (bool, error)
}
```

- [ ] **Step 4: 仓储实现**

`internal/infrastructure/repository/demo_session_repository.go`：

```go
package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	demoport "github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/samber/lo"
)

type demoSessionRepository struct {
	db *gorm.DB
}

func NewDemoSessionRepository(db *gorm.DB) demoport.DemoSessionRepository {
	return &demoSessionRepository{db: db}
}

func (r *demoSessionRepository) List(ctx context.Context) ([]uint, error) {
	var rows []dbmodel.DemoSession
	if err := r.db.WithContext(ctx).Order("session_id ASC").Find(&rows).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "list demo sessions")
	}
	return lo.Map(rows, func(m dbmodel.DemoSession, _ int) uint { return m.SessionID }), nil
}

func (r *demoSessionRepository) Add(ctx context.Context, ids []uint) error {
	ids = lo.Uniq(ids)
	if len(ids) == 0 {
		return nil
	}
	rows := lo.Map(ids, func(id uint, _ int) dbmodel.DemoSession {
		return dbmodel.DemoSession{SessionID: id}
	})
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error; err != nil {
		return ierr.Wrap(ierr.ErrDBCreate, err, "add demo sessions")
	}
	return nil
}

func (r *demoSessionRepository) Remove(ctx context.Context, ids []uint) error {
	ids = lo.Uniq(ids)
	if len(ids) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).
		Where("session_id IN ?", ids).
		Delete(&dbmodel.DemoSession{}).Error; err != nil {
		return ierr.Wrap(ierr.ErrDBDelete, err, "remove demo sessions")
	}
	return nil
}
```

> `constant` 已被 `dbmodel.DemoSession.TableName()` 引用，import 无需额外处理。

- [ ] **Step 5: DI 注册**

`repository.go` 在 `RepositoryModule` 的 `fx.Provide` 列表 `NewDemoConfigRepository,` 后追加 `NewDemoSessionRepository,`，并新增：

```go
func NewDemoSessionRepository(db *gorm.DB) demoport.DemoSessionRepository {
	return repository.NewDemoSessionRepository(db)
}
```

- [ ] **Step 6: 单测**

`test/unit/demo_session/demo_session_repository_test.go`：用 `gorm.Open` + sqlite 内存库（参考现有单测 fixture），验证 `Add` 去重、`List` 排序、`Remove`。若仓库无 sqlite fixture，则本任务只跑 `go build` 验证编译，仓储行为交由 Task 13 E2E 覆盖。

- [ ] **Step 7: 编译**

Run: `rtk go build ./...`
Expected: 编译通过

- [ ] **Step 8: Commit**

```bash
git add internal/infrastructure/database/model/ internal/common/constant/string.go internal/application/demo/port/handler.go internal/infrastructure/repository/demo_session_repository.go internal/bootstrap/modules/repository.go
git commit -m "feat(demo): demo_sessions 白名单表模型与仓储"
```

---

### Task 3: `DemoSessionAccessor` 应用层实现

**Files:**
- Create: `internal/application/demo/query/demo_session_accessor.go`
- Modify: `internal/bootstrap/modules/application.go`（注册 `NewDemoSessionAccessor`）

**Interfaces:**
- Consumes: `demoport.DemoSessionRepository`。
- Produces: `demoport.DemoSessionAccessor`。

- [ ] **Step 1: 实现**

`internal/application/demo/query/demo_session_accessor.go`：

```go
package query

import (
	"context"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

type demoSessionAccessor struct {
	repo port.DemoSessionRepository
}

func NewDemoSessionAccessor(repo port.DemoSessionRepository) port.DemoSessionAccessor {
	return &demoSessionAccessor{repo: repo}
}

func (a *demoSessionAccessor) AllowedIDs(ctx context.Context) ([]uint, error) {
	ids, err := a.repo.List(ctx)
	if err != nil {
		logger.WithCtx(ctx).Error("[DemoQuery] List demo sessions failed (fail-closed)")
		return nil, err
	}
	return ids, nil
}

func (a *demoSessionAccessor) IsAllowed(ctx context.Context, sessionID uint) (bool, error) {
	ids, err := a.AllowedIDs(ctx)
	if err != nil {
		return false, err
	}
	return lo.Contains(ids, sessionID), nil
}
```

- [ ] **Step 2: 注册**

`application.go` 的 `ApplicationModule` `fx.Provide` 列表，在 `demoquery.NewDemoScopeProvider,`（Task 5 将删除）位置改为 `demoquery.NewDemoSessionAccessor,`。

- [ ] **Step 3: 编译**

Run: `rtk go build ./...`
Expected: 编译通过

- [ ] **Step 4: Commit**

```bash
git add internal/application/demo/query/demo_session_accessor.go internal/bootstrap/modules/application.go
git commit -m "feat(demo): DemoSessionAccessor 白名单放行判断"
```

---

### Task 4: demo sessions 管理接口（list/add/remove + 路由 + DI）

**Files:**
- Modify: `internal/application/demo/port/handler.go`（`DemoSessionView`、三个 handler 接口、命令/查询）
- Create: `internal/application/demo/query/list_demo_sessions.go`
- Create: `internal/application/demo/command/demo_sessions.go`
- Modify: `internal/dto/demo.go`（`DemoSession`、`ListDemoSessionsRsp`、`AddDemoSessionsReq`）
- Modify: `internal/handler/demo.go`（`DemoHandler` 接口 + 实现 + 依赖）
- Modify: `internal/router/demo.go`（三个路由）
- Modify: `internal/bootstrap/modules/handler.go`（`NewDemoDependencies`）
- Modify: `internal/bootstrap/modules/application.go`（注册三个 handler 构造函数）

**Interfaces:**
- Consumes: `session.SessionReadRepository`（`ListSessionsByIDs`，Task 6 提供）；`demoport.DemoSessionRepository`。
- Produces: `port.ListDemoSessionsHandler`、`port.AddDemoSessionsHandler`、`port.RemoveDemoSessionsHandler`。

- [ ] **Step 0: `ListSessionsByIDs` 仓储方法（additive，先于本任务其余步骤）**

本任务依赖 `session.SessionReadRepository` 新增一个按 ID 集合分页查询的方法（不触碰现有 `sampleModulus` 逻辑，纯新增）。

`internal/domain/session/repository.go` 的 `SessionReadRepository` 接口新增：

```go
	// ListSessionsByIDs 按会话 ID 集合分页查询列表投影（demo 白名单视角 / admin 管理列表）
	ListSessionsByIDs(ctx context.Context, ids []uint, param model.CommonParam) ([]*SessionSummaryProjection, *model.PageInfo, error)
```

`internal/infrastructure/repository/session_repository.go` 新增实现（复制 `ListAllSessions` 主体，在构造 `sql` 后追加 `sql = sql.Where("id IN ?", ids)`；`len(ids)==0` 时直接返回 `[]*SessionSummaryProjection{}` 与 `Total:0` 的 pageInfo）。

- [ ] **Step 1: 端口**

`port/handler.go` 追加：

```go
// DemoSessionView 白名单会话摘要视图（用于 admin 已选列表）
type DemoSessionView struct {
	ID           uint
	Summary      string
	MessageCount int
	ToolCount    int
	CreatedAt    time.Time
}

// ListDemoSessionsQuery 查询白名单会话
type ListDemoSessionsQuery struct {
	Page     int
	PageSize int
}

// ListDemoSessionsHandler 列出白名单会话
type ListDemoSessionsHandler interface {
	Handle(ctx context.Context, q ListDemoSessionsQuery) ([]*DemoSessionView, *model.PageInfo, error)
}

// AddDemoSessionsCommand 批量添加白名单会话
type AddDemoSessionsCommand struct {
	SessionIDs []uint
}

// AddDemoSessionsHandler 批量添加
type AddDemoSessionsHandler interface {
	Handle(ctx context.Context, cmd AddDemoSessionsCommand) ([]uint, error)
}

// RemoveDemoSessionsCommand 批量移除白名单会话
type RemoveDemoSessionsCommand struct {
	SessionIDs []uint
}

// RemoveDemoSessionsHandler 批量移除
type RemoveDemoSessionsHandler interface {
	Handle(ctx context.Context, cmd RemoveDemoSessionsCommand) error
}
```

> `port/handler.go` 需新增 `"github.com/hcd233/aris-proxy-api/internal/common/model"` import。

- [ ] **Step 2: List handler**

`internal/application/demo/query/list_demo_sessions.go`：

```go
package query

import (
	"context"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
)

type listDemoSessionsHandler struct {
	demoRepo  port.DemoSessionRepository
	readRepo  session.SessionReadRepository
}

func NewListDemoSessionsHandler(demoRepo port.DemoSessionRepository, readRepo session.SessionReadRepository) port.ListDemoSessionsHandler {
	return &listDemoSessionsHandler{demoRepo: demoRepo, readRepo: readRepo}
}

func (h *listDemoSessionsHandler) Handle(ctx context.Context, q port.ListDemoSessionsQuery) ([]*port.DemoSessionView, *model.PageInfo, error) {
	ids, err := h.demoRepo.List(ctx)
	if err != nil {
		return nil, nil, err
	}
	if len(ids) == 0 {
		return []*port.DemoSessionView{}, &model.PageInfo{Page: q.Page, PageSize: q.PageSize, Total: 0}, nil
	}
	param := model.CommonParam{
		PageParam: model.PageParam{Page: max(q.Page, 1), PageSize: q.PageSize},
	}
	projections, pageInfo, err := h.readRepo.ListSessionsByIDs(ctx, ids, param)
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "list demo sessions by ids")
	}
	views := lo.Map(projections, func(p *session.SessionSummaryProjection, _ int) *port.DemoSessionView {
		return &port.DemoSessionView{
			ID:           p.ID,
			MessageCount: p.MessageCount,
			ToolCount:    p.ToolCount,
			CreatedAt:    p.CreatedAt,
		}
	})
	return views, pageInfo, nil
}
```

> 此处 `ListSessionsByIDs(ctx, ids, param)` 采用 Task 6 定义的最小签名（仅 `CommonParam`）；`Summary` 字段留空（列表页按需展示 ID/时间/计数即可，避免额外拉消息）。

- [ ] **Step 3: Add/Remove handler**

`internal/application/demo/command/demo_sessions.go`：

```go
package command

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
	"github.com/samber/lo"
)

type addDemoSessionsHandler struct {
	demoRepo port.DemoSessionRepository
	readRepo session.SessionReadRepository
}

func NewAddDemoSessionsHandler(demoRepo port.DemoSessionRepository, readRepo session.SessionReadRepository) port.AddDemoSessionsHandler {
	return &addDemoSessionsHandler{demoRepo: demoRepo, readRepo: readRepo}
}

func (h *addDemoSessionsHandler) Handle(ctx context.Context, cmd port.AddDemoSessionsCommand) ([]uint, error) {
	ids := lo.Uniq(cmd.SessionIDs)
	if len(ids) == 0 {
		return nil, ierr.New(ierr.ErrValidation, "sessionIds is required")
	}
	// 校验存在性：仅存在的 session 可加入（fail-closed：查询失败拒绝全部）
	existing := existingSessionIDs(ctx, h.readRepo, ids)
	valid := lo.Filter(ids, func(id uint, _ int) bool { return existing[id] })
	if err := h.demoRepo.Add(ctx, valid); err != nil {
		return nil, err
	}
	return valid, nil
}

// existingSessionIDs 返回 ids 中实际存在的会话 ID 集合；查询失败返回空集（fail-closed 拒绝添加）
func existingSessionIDs(ctx context.Context, readRepo session.SessionReadRepository, ids []uint) map[uint]bool {
	param := model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: len(ids)}}
	projections, _, err := readRepo.ListSessionsByIDs(ctx, ids, param)
	if err != nil || len(projections) == 0 {
		return map[uint]bool{}
	}
	return lo.SliceToMap(projections, func(p *session.SessionSummaryProjection) (uint, bool) {
		return p.ID, true
	})
}

type removeDemoSessionsHandler struct {
	demoRepo port.DemoSessionRepository
}

func NewRemoveDemoSessionsHandler(demoRepo port.DemoSessionRepository) port.RemoveDemoSessionsHandler {
	return &removeDemoSessionsHandler{demoRepo: demoRepo}
}

func (h *removeDemoSessionsHandler) Handle(ctx context.Context, cmd port.RemoveDemoSessionsCommand) error {
	if len(cmd.SessionIDs) == 0 {
		return ierr.New(ierr.ErrValidation, "sessionIds is required")
	}
	return h.demoRepo.Remove(ctx, cmd.SessionIDs)
}
```

> `demo_sessions.go` 需 import：`"github.com/hcd233/aris-proxy-api/internal/common/model"`、`"github.com/hcd233/aris-proxy-api/internal/domain/session"`。

- [ ] **Step 4: DTO**

`internal/dto/demo.go` 追加：

```go
// DemoSession 白名单会话摘要
type DemoSession struct {
	ID           uint      `json:"id" doc:"Session ID"`
	Summary      string    `json:"summary,omitempty" doc:"会话摘要"`
	MessageCount int       `json:"messageCount" doc:"消息数"`
	ToolCount    int       `json:"toolCount" doc:"工具调用数"`
	CreatedAt    time.Time `json:"createdAt,omitzero" doc:"创建时间"`
}

// ListDemoSessionsRsp 白名单会话列表响应
type ListDemoSessionsRsp struct {
	CommonRsp
	Sessions []*DemoSession `json:"sessions,omitempty" doc:"白名单会话列表"`
}

// AddDemoSessionsReq 批量添加白名单会话请求
type AddDemoSessionsReq struct {
	Body *AddDemoSessionsReqBody `json:"body" doc:"请求体"`
}

// AddDemoSessionsReqBody 批量添加白名单会话请求体
type AddDemoSessionsReqBody struct {
	SessionIDs []uint `json:"sessionIds" required:"true" minItems:"1" doc:"会话 ID 列表"`
}

// RemoveDemoSessionsRsp 批量移除白名单会话响应
type RemoveDemoSessionsRsp struct {
	CommonRsp
}
```

- [ ] **Step 5: Handler**

`internal/handler/demo.go`：`DemoHandler` 接口新增三个方法；`DemoHandlerDependencies` 新增 `ListDemoSessions port.ListDemoSessionsHandler`、`AddDemoSessions port.AddDemoSessionsHandler`、`RemoveDemoSessions port.RemoveDemoSessionsHandler`；`NewDemoHandler`/结构体同步。实现：

```go
func (h *demoHandler) HandleListDemoSessions(ctx context.Context, req *dto.ListDemoSessionsReq) (*dto.HTTPResponse[*dto.ListDemoSessionsRsp], error) {
	rsp := &dto.ListDemoSessionsRsp{}
	views, pageInfo, err := h.listDemoSessions.Handle(ctx, port.ListDemoSessionsQuery{Page: req.Page, PageSize: req.PageSize})
	if err != nil {
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	rsp.Sessions = lo.Map(views, func(v *port.DemoSessionView, _ int) *dto.DemoSession {
		return &dto.DemoSession{ID: v.ID, Summary: v.Summary, MessageCount: v.MessageCount, ToolCount: v.ToolCount, CreatedAt: v.CreatedAt}
	})
	rsp.PageInfo = pageInfo
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *demoHandler) HandleAddDemoSessions(ctx context.Context, req *dto.AddDemoSessionsReq) (*dto.HTTPResponse[*dto.ListDemoSessionsRsp], error) {
	_, err := h.addDemoSessions.Handle(ctx, port.AddDemoSessionsCommand{SessionIDs: req.Body.SessionIDs})
	if err != nil {
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return h.HandleListDemoSessions(ctx, &dto.ListDemoSessionsReq{Page: 1, PageSize: 100})
}

func (h *demoHandler) HandleRemoveDemoSessions(ctx context.Context, req *dto.RemoveDemoSessionsReq) (*dto.HTTPResponse[*dto.RemoveDemoSessionsRsp], error) {
	err := h.removeDemoSessions.Handle(ctx, port.RemoveDemoSessionsCommand{SessionIDs: req.IDs})
	if err != nil {
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(&dto.RemoveDemoSessionsRsp{}, nil)
}
```

> `ListDemoSessionsReq` / `RemoveDemoSessionsReq` DTO 需在 `dto/demo.go` 定义（`ListDemoSessionsReq` 含 `Page`/`PageSize` query；`RemoveDemoSessionsReq` 含 `IDs []uint` query，key `ids`，逗号分隔）。`handler/demo.go` 需补 `"github.com/samber/lo"` import。

- [ ] **Step 6: 路由**

`internal/router/demo.go` 在 `initDemoRouter` 末尾追加：

```go
	demoSessionsGroup := huma.NewGroup(demoGroup, "/sessions")
	demoSessionsGroup.UseMiddleware(middleware.JwtMiddleware(db, cache, accessSigner))

	huma.Register(demoSessionsGroup, huma.Operation{
		OperationID: "listDemoSessions",
		Method:      http.MethodGet,
		Path:        "/list",
		Summary:     "ListDemoSessions",
		Description: "List sessions whitelisted for the demo account (admin only)",
		Tags:        []string{constant.TagDemo},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listDemoSessions", enum.PermissionAdmin)},
	}, demoHandler.HandleListDemoSessions)

	huma.Register(demoSessionsGroup, huma.Operation{
		OperationID: "addDemoSessions",
		Method:      http.MethodPost,
		Path:        "",
		Summary:     "AddDemoSessions",
		Description: "Batch add sessions to the demo whitelist (admin only)",
		Tags:        []string{constant.TagDemo},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("addDemoSessions", enum.PermissionAdmin)},
	}, demoHandler.HandleAddDemoSessions)

	huma.Register(demoSessionsGroup, huma.Operation{
		OperationID: "removeDemoSessions",
		Method:      http.MethodDelete,
		Path:        "",
		Summary:     "RemoveDemoSessions",
		Description: "Batch remove sessions from the demo whitelist (admin only)",
		Tags:        []string{constant.TagDemo},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("removeDemoSessions", enum.PermissionAdmin)},
	}, demoHandler.HandleRemoveDemoSessions)
```

- [ ] **Step 7: DI 注册**

`application.go` 的 `fx.Provide` 追加 `demoquery.NewListDemoSessionsHandler, democommand.NewAddDemoSessionsHandler, democommand.NewRemoveDemoSessionsHandler,`；`handler.go` 的 `NewDemoDependencies` 签名扩展三个入参并填充 `handler.DemoHandlerDependencies`。

- [ ] **Step 8: 编译**

Run: `rtk go build ./...`
Expected: 编译通过（若 `ListSessionsByIDs` 尚未定义，先完成 Task 6 再回来自测；本任务可先 commit 接口层，Task 6 后再跑全量 build）

- [ ] **Step 9: Commit**

```bash
git add internal/application/demo/ internal/dto/demo.go internal/handler/demo.go internal/router/demo.go internal/bootstrap/modules/
git commit -m "feat(demo): demo sessions 白名单管理接口（list/add/remove）"
```

---

### Task 5: 移除 `SampleModulus` 与 `DemoScopeProvider`

**Files:**
- Modify: `internal/infrastructure/database/model/demo_config.go`
- Modify: `internal/infrastructure/repository/demo_config_repository.go`
- Modify: `internal/application/demo/port/handler.go`（删 `DemoScopeProvider`、`SampleModulus` 字段）
- Modify: `internal/application/demo/query/get_config.go`（删 `demoScopeProvider`）
- Modify: `internal/application/demo/command/update_config.go`、`login.go`
- Modify: `internal/dto/demo.go`、`internal/handler/demo.go`
- Modify: `internal/common/constant/string.go`（删 `DemoDefaultSampleModulus`）

**Interfaces:**
- Consumes: 无。
- Produces: 删除 `port.DemoScopeProvider`；`DemoConfigEntity/View`、`dto.DemoConfig` 均不含 `SampleModulus`。

- [ ] **Step 1: 删除字段与接口**

按设计文档 §4.1 逐文件删除 `SampleModulus` 字段/参数/`DemoScopeProvider` 接口与 `demoScopeProvider` 实现；`update_config.go` 删除 `if cmd.SampleModulus != nil && *cmd.SampleModulus < 2` 校验块及赋值；`get_config.go` `toDemoConfigView` 去掉 `SampleModulus`；`handler/demo.go` `toDemoConfigDTO` 去掉该字段；`constant/string.go` 删 `DemoDefaultSampleModulus`。

> 此阶段 `session`/`audit` 仍引用 `DemoScopeProvider`，会编译失败——这是预期中间态，Task 6/7 会移除这些引用。为保持每个任务可独立编译，本任务与 Task 6、Task 7 需**连续执行后统一 `go build`**，或按顺序在 Task 7 末尾统一编译。推荐：Task 5/6/7 作为一组，最终统一 build + commit。

- [ ] **Step 2: Commit（与 Task 6/7 合并提交，见 Task 7）**

---

### Task 6: session 查询改白名单视角 + repo `ListSessionsByIDs`

**Files:**
- Modify: `internal/domain/session/repository.go`（接口：删 `sampleModulus`，增 `ListSessionsByIDs`）
- Modify: `internal/infrastructure/repository/session_repository.go`（实现）
- Modify: `internal/application/session/port/handler.go`（query 字段替换）
- Modify: `internal/application/session/query/jwt_session_queries.go`、`session_meta_query.go`、`session_message_list_query.go`、`session_tool_list_query.go`、`option_list.go`
- Modify: `internal/handler/session.go`（`resolveDemoScope` → `resolveDemoAccess`，依赖替换）

**Interfaces:**
- Consumes: `demoport.DemoSessionAccessor`（Task 3）。
- Produces: `session.SessionReadRepository.ListSessionsByIDs(ctx, ids []uint, param model.CommonParam) ([]*SessionSummaryProjection, *model.PageInfo, error)`；`ListDistinctScores/Models/MessageCountStats` 的 `sampleModulus` 参数改为 `sessionIDs []uint`。

- [ ] **Step 1: 仓储接口**

`domain/session/repository.go`：`ListAllSessions` 删 `sampleModulus uint` 参数（`ListSessionsByIDs` 已由 Task 4 Step 0 定义，无需重复）。

`ListDistinctScores/ListDistinctModels/ListMessageCountStats` 的 `sampleModulus uint` 参数改为 `sessionIDs []uint`（nil=不过滤，非 nil=`WHERE id IN (?)`）。

- [ ] **Step 2: 仓储实现**

`session_repository.go`：`ListAllSessions` 删除 `if sampleModulus > 1 { sql = sql.Where(constant.DBConditionIDModuloZero, sampleModulus) }` 块；新增 `ListSessionsByIDs`（复制 `ListAllSessions` 主体，起始 `sql := r.db...` 前加 `sql = sql.Where("id IN ?", ids)`，空 ids 直接返回空页）；`ListDistinctScores/Models/MessageCountStats` 的 `if sampleModulus > 1 { ... }` 改为 `if len(sessionIDs) > 0 { query = query.Where("id IN ?", sessionIDs) }`（注意 distinct model/messageCount 的查询别名与 `id` 归属，参考现有 `ListSessionsByOwnerNames` 的 owner 过滤写法）。

- [ ] **Step 3: session port**

`application/session/port/handler.go`：`ListSessionsByUserQuery` 删 `SampleModulus`，增 `IsDemo bool`、`SessionIDs []uint`；`GetSessionByUserQuery`/`GetSessionMetaByUserQuery`/`ListSessionMessagesQuery`/`ListSessionToolsQuery` 删 `SampleModulus`，增 `IsDemo bool`、`AllowedSessionIDs []uint`；`ListSessionOptionQuery` 删 `SampleModulus`，增 `SessionIDs []uint`。

- [ ] **Step 4: session query**

`jwt_session_queries.go` `listSessionsByUserHandler.Handle`：分支改为

```go
	if q.IsAdmin {
		projections, pageInfo, err = h.readRepo.ListAllSessions(ctx, param, q.StartTime, q.EndTime, q.Keyword, criteria)
	} else if q.IsDemo {
		projections, pageInfo, err = h.readRepo.ListSessionsByIDs(ctx, q.SessionIDs, param)
	} else {
		// 原 ListSessionsByOwnerNames 分支不变
	}
```

`getSessionByUserHandler.Handle` 的越权校验改为：

```go
	if q.IsDemo && !slices.Contains(q.AllowedSessionIDs, q.SessionID) {
		log.Info(...)
		return nil, ierr.New(ierr.ErrDataNotExists, "session not found")
	}
```

`session_meta_query.go` 同样替换；`session_message_list_query.go`/`session_tool_list_query.go` 把传给 `metaQuery` 的 `SampleModulus` 改为 `IsDemo: q.IsDemo, AllowedSessionIDs: q.AllowedSessionIDs`；`option_list.go` 的 `q.SampleModulus` 改为 `q.SessionIDs`。

- [ ] **Step 5: session handler**

`handler/session.go`：`SessionDependencies.DemoScope demoport.DemoScopeProvider` → `DemoAccess demoport.DemoSessionAccessor`；`resolveDemoScope` 替换为：

```go
func (h *sessionHandler) resolveDemoAccess(ctx context.Context, permission enum.Permission) (isDemo bool, allowedIDs []uint, err error) {
	if permission != enum.PermissionDemo {
		return false, nil, nil
	}
	ids, err := h.demoAccess.AllowedIDs(ctx)
	if err != nil {
		return false, nil, err
	}
	return true, ids, nil
}
```

各 `Handle*` 方法：`isDemo, allowedIDs, scopeErr := h.resolveDemoAccess(...)`；列表传 `IsAdmin: isAdmin, IsDemo: isDemo, SessionIDs: allowedIDs`；详情/meta/messages/tools 传 `IsAdmin: isAdmin || isDemo, IsDemo: isDemo, AllowedSessionIDs: allowedIDs`；option 传 `SessionIDs: allowedIDs`。

- [ ] **Step 6: 编译**

Run: `rtk go build ./...`
Expected: 通过（依赖 Task 7 完成 audit 侧删除后整体通过）

---

### Task 7: audit 查询移除取模 + 传递 isDemo

**Files:**
- Modify: `internal/domain/modelcall/repository.go`（删 `sampleModulus` 参数）
- Modify: `internal/infrastructure/repository/audit_repository.go`（删过滤块）
- Modify: `internal/application/audit/query/service.go`（删 `resolveSampleModulus`、`demoScope` 依赖）
- Modify: `internal/application/audit/query/list_audit_logs.go`（`ListAllAuditLogsQuery` 删 `SampleModulus` 增 `IsDemo`；`buildAuditViews` 增 `isDemo`）
- Modify: `internal/application/audit/query/option_list.go`（删 `SampleModulus`）
- Modify: 各聚合 query（`model_trend.go`/`request_rate.go`/`token_throughput.go`/`token_rate.go`/`token_usage.go`/`first_token_latency.go`）删 `SampleModulus`
- Modify: `internal/bootstrap/modules/application.go`（`NewAuditService` 去掉 `demoScope` 参数）

**Interfaces:**
- Consumes: 无（删除 `demoport.DemoScopeProvider` 依赖）。
- Produces: `AuditRepository` 各方法无 `sampleModulus` 参数。

- [ ] **Step 1: 仓储接口与实现**

`modelcall/repository.go`：所有 `sampleModulus uint` 参数删除；`audit_repository.go` 删除所有 `if sampleModulus > 1 { ... }` 块。

- [ ] **Step 2: audit service**

`service.go`：删除 `demoScope demoport.DemoScopeProvider` 字段、构造参数、`resolveSampleModulus` 方法；`ListLogs` 的 admin/demo 分支改为直接调用 `listAll.Handle`（不再解析 modulus），demo 分支传 `IsDemo: true`；`ListAuditOption`/各聚合方法删除 `resolveSampleModulus` 调用。

- [ ] **Step 3: list_audit_logs**

`list_audit_logs.go`：`ListAllAuditLogsQuery` 删 `SampleModulus uint`，增 `IsDemo bool`；`Handle` 里 `h.repo.ListAll(ctx, param, q.StartTime, q.EndTime, criteria)`（无 modulus），`buildAuditViews(ctx, h.repo, audits, q.IsDemo)`；`buildAuditViews` 增 `isDemo bool` 参数，在组装 view 时对 `APIKeyName/UserName/UserEmail/Endpoint/TraceID` 按 Task 9 的脱敏逻辑处理（本任务先传 `isDemo` 但脱敏体在 Task 9 补全，可先传 false 占位使编译通过）。

> 为避免中间态编译失败：本任务先让 `buildAuditViews` 签名带 `isDemo bool`，函数体内暂不脱敏（直接赋值原值），Task 9 再补脱敏逻辑。

- [ ] **Step 4: 编译 + 提交**

Run: `rtk go build ./... && rtk go test ./...`
Expected: 通过

```bash
git add internal/domain/ internal/infrastructure/repository/ internal/application/session/ internal/application/audit/ internal/handler/session.go internal/application/demo/ internal/dto/demo.go internal/handler/demo.go internal/bootstrap/modules/ internal/common/constant/
git commit -m "refactor(demo): 移除取模抽样，session 改白名单视角，audit 全量"
```

---

### Task 8: `MaskIdentity` 脱敏工具

**Files:**
- Modify: `internal/common/util/secret.go`
- Test: `test/unit/secret/secret_test.go`（或复用现有）

**Interfaces:**
- Produces: `MaskIdentity(s string) string`（非空返回 `"***"`，空返回 `""`）。

- [ ] **Step 1: 实现**

`secret.go` 追加：

```go
// MaskIdentity 掩码身份类信息（姓名/邮箱），非空一律返回固定占位符，避免反推
func MaskIdentity(s string) string {
	if s == "" {
		return ""
	}
	return constant.MaskSecretPlaceholder
}
```

- [ ] **Step 2: 单测**

验证：`MaskIdentity("alice@example.com") == "***"`、`MaskIdentity("") == ""`、`MaskSecret("sk-1234567890abcd") == "sk-1***abcd"`。

- [ ] **Step 3: Commit**

```bash
git add internal/common/util/secret.go test/unit/secret/
git commit -m "feat(util): MaskIdentity 身份类脱敏工具"
```

---

### Task 9: audit 脱敏

**Files:**
- Modify: `internal/application/audit/query/list_audit_logs.go`
- Modify: `internal/application/audit/query/service.go`（`ListAuditOption` user 字段脱敏）

**Interfaces:**
- Consumes: `commonutil.MaskSecret`、`commonutil.MaskIdentity`。

- [ ] **Step 1: buildAuditViews 脱敏**

`list_audit_logs.go` `buildAuditViews` 内，组装 `view` 后、返回前，若 `isDemo`：

```go
		if isDemo {
			view.APIKeyName = commonutil.MaskSecret(view.APIKeyName)
			view.UserName = commonutil.MaskIdentity(view.UserName)
			view.UserEmail = commonutil.MaskIdentity(view.UserEmail)
			view.Endpoint = commonutil.MaskSecret(view.Endpoint)
			view.TraceID = commonutil.MaskSecret(view.TraceID)
		}
```

> `commonutil` 为 `"github.com/hcd233/aris-proxy-api/internal/common/util"`（文件已 import）。

- [ ] **Step 2: service.ListAuditOption 脱敏**

`service.go` `ListAuditOption`：demo 且 field 为 `constant.AuditFilterFieldUser` 时，对返回 items 做 `lo.Map(MaskIdentity)` 再 `lo.Uniq`；其余字段不处理。

- [ ] **Step 3: 单测**

新增/更新 `test/unit/audit_query/`：构造 demo 视角 `ListLogs`，断言 `UserName/UserEmail/APIKeyName/Endpoint/TraceID` 已脱敏。

- [ ] **Step 4: Commit**

```bash
git add internal/application/audit/query/ test/unit/audit_query/
git commit -m "feat(audit): demo 视角脱敏身份与连接字段"
```

---

### Task 10: models / endpoints 脱敏

**Files:**
- Modify: `internal/application/model/port/handler.go`（`ListModelsQuery` 增 `IsDemo bool`）
- Modify: `internal/application/model/query/list_models.go`
- Modify: `internal/application/endpoint/port/handler.go`（`ListEndpointsQuery` 增 `IsDemo bool`）
- Modify: `internal/application/endpoint/query/list_endpoints.go`
- Modify: `internal/handler/model.go`、`internal/handler/endpoint.go`（传 `IsDemo`）

**Interfaces:**
- Consumes: `util.CtxValuePermission(ctx)`；`commonutil.MaskSecret`。

- [ ] **Step 1: port 增字段**

`ListModelsQuery`/`ListEndpointsQuery` 增 `IsDemo bool`。

- [ ] **Step 2: query 脱敏**

`list_models.go`：视图组装时若 `q.IsDemo`，对 `UpstreamModel` 与 `Endpoint` 的 `Name/OpenaiBaseURL/AnthropicBaseURL` 用 `MaskSecret`；`list_endpoints.go`：若 `q.IsDemo`，对 `OpenaiBaseURL/AnthropicBaseURL` 用 `MaskSecret`。

- [ ] **Step 3: handler 传 isDemo**

`handler/model.go` `HandleListModels`、`handler/endpoint.go` `HandleListEndpoints`：`IsDemo: util.CtxValuePermission(ctx) == enum.PermissionDemo`。

- [ ] **Step 4: 单测**

`test/unit/`：demo 视角 `ListModelsQuery{IsDemo:true}` 断言 `UpstreamModel`/Endpoint BaseURL 已脱敏。

- [ ] **Step 5: Commit**

```bash
git add internal/application/model/ internal/application/endpoint/ internal/handler/model.go internal/handler/endpoint.go test/unit/
git commit -m "feat(model,endpoint): demo 视角脱敏上游与连接字段"
```

---

### Task 11: demo 接口 IP 限流挂载

**Files:**
- Modify: `internal/router/session.go`、`endpoint.go`、`model.go`、`audit.go`、`cron.go`、`trigger.go`、`metrics.go`、`demo.go`

**Interfaces:**
- Consumes: `middleware.TokenBucketRateLimiterMiddleware`（Task 1）、`middleware.WithPermissionFilter`、`constant.PeriodDemoAccess/LimitDemoAccess`、`enum.PermissionDemo`。

- [ ] **Step 1: 各 group 挂载**

在每个 demo 可访问 group 的 `UseMiddleware(middleware.JwtMiddleware(...))` 之后追加同一行：

```go
	group.UseMiddleware(middleware.TokenBucketRateLimiterMiddleware(cache, "demoAccess", "", constant.PeriodDemoAccess, constant.LimitDemoAccess, middleware.WithPermissionFilter(enum.PermissionDemo)))
```

涉及：`initSessionJWTRouter`、`initEndpointRouter`、`initModelRouter`、`initAuditRouter`、`initCronRouter`、`initTriggerRouter`、`initMetricsRouter`，以及 `initDemoRouter` 的 `demoConfigGroup`（`UseMiddleware(JwtMiddleware)` 之后）。

> 各 init 函数签名已含 `cache *redis.Client`；若某文件未 import `enum`/`constant`，补相应 import。

- [ ] **Step 2: 编译**

Run: `rtk go build ./...`
Expected: 通过

- [ ] **Step 3: Commit**

```bash
git add internal/router/
git commit -m "feat(router): demo 接口按 IP 限流（WithPermissionFilter）"
```

---

### Task 12: 前端（/demo 页 + 侧边栏 + 类型/接口）

**Files:**
- Modify: `web/src/lib/types.ts`（`DemoConfig` 删 `sampleModulus`；新增 `DemoSession`、`ListDemoSessionsRsp`、`AddDemoSessionsReqBody`）
- Modify: `web/src/lib/api-client.ts`（`listDemoSessions`/`addDemoSessions`/`removeDemoSessions`）
- Create: `web/src/app/(dashboard)/demo/page.tsx`
- Create: `web/src/components/demo-sessions-manager.tsx`
- Modify: `web/src/components/demo-config-card.tsx`（删 sampleModulus 输入）
- Modify: `web/src/app/(dashboard)/users/page.tsx`（删 `DemoConfigCard` 引用）
- Modify: `web/src/app/(dashboard)/layout.tsx`（`getNavItems` 增 `nav.demo`）
- Modify: `web/src/locales/en.json`、`zh.json`、`ja.json`

**Interfaces:**
- Consumes: `api.listSessions`（选择器候选）、`api.listDemoSessions/addDemoSessions/removeDemoSessions`。

- [ ] **Step 1: 类型与接口**

`types.ts`：`DemoConfig` 移除 `sampleModulus`；新增

```ts
export interface DemoSession {
  id: number;
  summary?: string;
  messageCount: number;
  toolCount: number;
  createdAt?: string;
}
export interface ListDemoSessionsRsp extends CommonRsp {
  sessions?: DemoSession[];
  pageInfo?: PageInfo;
}
export interface AddDemoSessionsReqBody {
  sessionIds: number[];
}
```

`api-client.ts` 新增：

```ts
  async listDemoSessions(page = 1, pageSize = 100): Promise<ListDemoSessionsRsp> {
    return this.request<ListDemoSessionsRsp>(`/api/v1/demo/sessions/list?page=${page}&pageSize=${pageSize}`);
  }
  async addDemoSessions(body: AddDemoSessionsReqBody): Promise<ListDemoSessionsRsp> {
    return this.request<ListDemoSessionsRsp>("/api/v1/demo/sessions", { method: "POST", body: JSON.stringify(body) });
  }
  async removeDemoSessions(ids: number[]): Promise<CommonRsp> {
    return this.request<CommonRsp>(`/api/v1/demo/sessions?ids=${ids.join(",")}`, { method: "DELETE" });
  }
```

- [ ] **Step 2: demo 页 + 组件**

`demo/page.tsx`：`<PermissionGuard adminOnly>` 包裹，渲染 `<DemoConfigCard />` + `<DemoSessionsManager />` + 页头。

`demo-config-card.tsx`：删除 `sampleModulus` 的 `Input` 与 state 字段，`save` payload 只保留 `loginEnabled`/`modules`。

`demo-sessions-manager.tsx`（`"use client"`）：
- 上半：已选列表（`listDemoSessions` 拉取，勾选 + `removeDemoSessions` 批量移除）。
- 下半：选择器（复用 `listSessions` 分页搜索，勾选候选 + `addDemoSessions` 批量加入，加入后刷新已选列表）。

- [ ] **Step 3: 侧边栏 + users 页 + locales**

`layout.tsx` `getNavItems`：新增 `{ labelKey: "nav.demo", href: "/demo/", icon: <Settings2 className="size-4" />, adminOnly: true }`（`Settings2` 从 lucide 引入或复用 `SlidersHorizontal`）。

`users/page.tsx`：删除 `<DemoConfigCard />` 及其 import。

`locales/*.json`：新增 `nav.demo`、`demo.*`（sessions 管理相关）文案。

- [ ] **Step 4: 构建**

Run: `cd web && rtk pnpm lint && rtk pnpm build`（或项目 Makefile 对应命令）
Expected: 通过

- [ ] **Step 5: Commit**

```bash
git add web/src/
git commit -m "feat(web): 新增 demo 独立页与 demo sessions 管理"
```

---

### Task 13: E2E 用例

**Files:**
- Modify/Create: `test/e2e/demo/demo_account_test.go`（扩展）

**Interfaces:**
- Consumes: 现有 E2E 骨架（登录/权限 helper）。

- [ ] **Step 1: 用例**

新增用例覆盖：
1. admin 批量 add demo sessions → list 含该会话 → demo 登录后可读该 session、非白名单 session 返回"不存在"。
2. demo 视角 audit logs 的 `userName/userEmail/apiKeyName/endpoint/traceId` 已脱敏。
3. demo 接口高频访问返回 429（IP 限流）。
4. admin 批量 remove demo sessions → list 为空。

- [ ] **Step 2: 跑通**

Run: `rtk go test ./test/e2e/demo/...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add test/e2e/demo/
git commit -m "test(e2e): demo 白名单/脱敏/IP 限流用例"
```

---

### Task 14: 收尾（全量测试 + lint + ponytail-review）

- [ ] **Step 1: 全量验证**

Run: `rtk go build ./... && rtk go test ./... && cd web && rtk pnpm lint`

- [ ] **Step 2: ponytail-review**

对本次 diff 做 `ponytail-review`，删除投机抽象/死代码。

- [ ] **Step 3: 沉淀经验**

`serena_write_memory`：记录 demo 白名单/脱敏/限流的架构决策与踩坑。

- [ ] **Step 4: 汇报**

汇报证据（测试通过数、E2E 结果），询问用户是否提 MR / 合并。
