# Audit 管理页面实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 改造 `GET /api/v1/audit/logs`：鉴权切到 `jwtAuth`，按用户权限分级返回审计日志（admin 看全量、user 看自己名下所有 key 的）；前端在 `(dashboard)/audit` 新建表格列表页，支持分页、时间范围、关键词搜索、TraceID 复制。

**Architecture:** 后端删除 `ListByAPIKeyID` 路径（query handler / repository 方法），新增 `ListAllAuditLogsHandler` + `ListAuditLogsByUserHandler`，repository 增 `ListAll` + `ListByAPIKeyIDs`（不 JOIN，多次小 SQL），handler 用 switch 按 `enum.Permission` 分发并做三步关联查询补 `apiKeyName/userName/userEmail`。前端复制 `sessions/page.tsx` 骨架到 `audit/page.tsx`，扩展 `lib/api-client.ts` 与 `lib/types.ts`，sidebar 加导航项。

**Tech Stack:** Go 1.25 + huma/v2 + GORM + dig；Next.js 16 + React 19 + Tailwind v4 + Sonner。

**关键事实（spec 阶段后核实）：**
- JWT 中间件写入的 `permission` 类型是 `enum.Permission`（`internal/middleware/jwt.go:99,112`），用 `util.CtxValuePermission(ctx)` 读取。
- `baseDAO[T].Paginate` 不支持 `WHERE ... IN (?)`——`ListByAPIKeyIDs` 必须在 repository 层手写 SQL。
- `baseDAO[T].BatchGetByField(field, values, fields)` 已存在，可用于 `ListByIDs` 语义，无需新增 DAO 方法。
- `constant.DBConditionInTemplate = "%s IN ?"`、`DBConditionDeletedAtZero = "deleted_at = 0"` 已定义。
- 错误类别断言用 `errors.Is(err, ierr.ErrValidation)`（`*InternalError.Is` 已实现）。
- 现有 `test/unit/audit_query/audit_query_test.go` 完全针对要被删除的 ByAPIKey handler，整体替换。
- 现有 `test/unit/audit_dto/audit_dto_test.go` 不涉及被删 handler，仅扩展用例。

---

## File Structure

**后端**
- 删除：`list_audit_logs.go` 内的 `ListAuditLogsHandler` / `ListAuditLogsQuery`、`AuditRepository.ListByAPIKeyID`、`bootstrap` 的 `auditquery.NewListAuditLogsHandler` provider 与 `AuditDependencies.List`
- 修改：`internal/dto/audit.go`（DTO 加 3 字段）
- 重写：`internal/application/audit/query/list_audit_logs.go`（两个新 query handler + 共享参数清洗 helper）
- 修改：`internal/domain/modelcall/repository.go`（接口加 `ListAll` + `ListByAPIKeyIDs`）
- 修改：`internal/infrastructure/repository/audit_repository.go`（实现 `ListAll` + `ListByAPIKeyIDs`，手写 SQL）
- 修改：`internal/handler/audit.go`（依赖切换 + switch 分发 + 三步关联查询）
- 修改：`internal/router/audit.go` + `internal/router/router.go`（鉴权切 JWT、签名加 cache）
- 修改：`internal/bootstrap/container.go`（DI 重连）

**测试**
- 重写：`test/unit/audit_query/audit_query_test.go`
- 修改：`test/unit/audit_dto/audit_dto_test.go`

**前端**
- 新增：`web/src/app/(dashboard)/audit/page.tsx`
- 修改：`web/src/app/(dashboard)/layout.tsx`（navItems）
- 修改：`web/src/lib/types.ts`（加 `AuditLogItem`、`ListAuditLogsRsp`）
- 修改：`web/src/lib/api-client.ts`（加 `listAuditLogs`）

**编译策略**：Task 2-7 期间项目处于编译失败/测试失败状态（删除接口后实现尚未对齐）。这是有意的 TDD"红"状态，到 Task 8 修测试后整体回到绿。Task 9 单独提交一次大变更。前端 Task 10-12 独立提交。

---

## Task 1：后端 DTO 补充关联字段

**Files:**
- Modify: `internal/dto/audit.go`
- Modify: `test/unit/audit_dto/audit_dto_test.go`

- [ ] **Step 1: 在 `internal/dto/audit.go` 的 `AuditLogItem` 结构体的 `TraceID` 字段后追加三个字段**

```go
APIKeyName string `json:"apiKeyName" doc:"调用所用 API Key 名称"`
UserName   string `json:"userName" doc:"调用方用户名"`
UserEmail  string `json:"userEmail" doc:"调用方邮箱"`
```

- [ ] **Step 2: 在 `test/unit/audit_dto/audit_dto_test.go` 的 `TestAuditLogItem_JSONTags` 函数中扩展用例**

`item := dto.AuditLogItem{...}` 字面量增加三个字段：

```go
APIKeyName: "my-key",
UserName:   "alice",
UserEmail:  "alice@example.com",
```

在 `inputTokens` 断言后追加：

```go
if v, ok := obj["apiKeyName"]; !ok {
    t.Errorf("apiKeyName field missing")
} else if v != "my-key" {
    t.Errorf("apiKeyName = %v, want my-key", v)
}
if v, ok := obj["userName"]; !ok {
    t.Errorf("userName field missing")
} else if v != "alice" {
    t.Errorf("userName = %v, want alice", v)
}
if v, ok := obj["userEmail"]; !ok {
    t.Errorf("userEmail field missing")
} else if v != "alice@example.com" {
    t.Errorf("userEmail = %v, want alice@example.com", v)
}
```

- [ ] **Step 3: 运行 DTO 测试**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/feature/audit-admin-page-2026-05-29
go test -v -count=1 ./test/unit/audit_dto/...
```
Expected: PASS。

- [ ] **Step 4: 提交**

```bash
git add internal/dto/audit.go test/unit/audit_dto/audit_dto_test.go
git commit -m "feat(audit): DTO 增加 apiKeyName/userName/userEmail 关联字段"
```

---

## Task 2：补充字段常量（如缺失）

**Files:**
- Modify: `internal/common/constant/sql.go`

- [ ] **Step 1: 检查现有 SQL 字段常量**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/feature/audit-admin-page-2026-05-29
grep -nE 'FieldAPIKeyID|FieldUserID|FieldName|FieldEmail|FieldID ' internal/common/constant/sql.go
```

预期：`FieldAPIKeyID`、`FieldUserID`、`FieldID`、`FieldName`、`FieldEmail` 全部存在。如缺失则补充。

- [ ] **Step 2: 若有缺失，编辑 `internal/common/constant/sql.go`，在 const 块中补全**

```go
FieldID       = "id"
FieldAPIKeyID = "api_key_id"
FieldUserID   = "user_id"
FieldName     = "name"
FieldEmail    = "email"
```

- [ ] **Step 3: 编译验证**

```bash
go build ./internal/common/...
```
Expected: 编译通过。

- [ ] **Step 4: 若有改动则提交**

```bash
git add internal/common/constant/sql.go
git commit -m "chore(constant): 补全审计页面所需字段常量"
```

> 若 Step 1 全部命中无需新增，跳过 Step 2-4。

---

## Task 3：Repository 接口替换

**Files:**
- Modify: `internal/domain/modelcall/repository.go`

- [ ] **Step 1: 重写 `internal/domain/modelcall/repository.go` 全文**

```go
// Package modelcall ModelCall 域根（仓储接口）
//
// TODO: 此域尚未被 use case 层接入。LLM 代理当前通过 pool.SubmitModelCallAuditTask() 直接写入审计记录。
// 计划在后续迭代中将审计写入迁移至 aggregate + repository 模式，届时本包将被激活。
package modelcall

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/aggregate"
)

// AuditRepository ModelCallAudit 聚合仓储接口
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type AuditRepository interface {
	// Save 持久化审计聚合（首次 Save 后回填 ID）
	Save(ctx context.Context, audit *aggregate.ModelCallAudit) error

	// ListAll 全量分页查询审计记录，支持时间范围过滤、关键词搜索和多字段排序（admin 用）
	ListAll(ctx context.Context, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error)

	// ListByAPIKeyIDs 按 api_key_id IN (...) 分页查询；apiKeyIDs 为空时返回空结果且不打 SQL
	ListByAPIKeyIDs(ctx context.Context, apiKeyIDs []uint, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error)
}
```

- [ ] **Step 2: 不在此步运行测试或提交**

> 编译会失败（实现还引用旧接口）。Task 4 / 5 / 6 完成后才会恢复。

---

## Task 4：Repository 实现

**Files:**
- Modify: `internal/infrastructure/repository/audit_repository.go`

- [ ] **Step 1: 重写 `internal/infrastructure/repository/audit_repository.go` 全文**

完整内容见本计划 **附录 A**（位于文档末尾）。直接复制粘贴。

> 关键点：
> - `Save` 与既有实现一致，不动。
> - `ListAll` 与 `ListByAPIKeyIDs` 都调内部 helper `paginate(...)`，后者手写 SQL 而非调 `dao.Paginate`（因为基类不支持 IN 条件）。
> - `ListByAPIKeyIDs` 在 `len(apiKeyIDs) == 0` 时直接返回空结果不打 SQL。
> - 模糊搜索把 OR 条件包成子 expression 用 `Where(sub)` 注入，避免 OR 与外层 AND 优先级混乱。

- [ ] **Step 2: 不在此步运行测试或提交**

---

## Task 5：Application Query Handler 重写

**Files:**
- Modify: `internal/application/audit/query/list_audit_logs.go`

- [ ] **Step 1: 重写 `internal/application/audit/query/list_audit_logs.go` 全文**

完整内容见本计划 **附录 B**。直接复制粘贴。

> 关键点：
> - 包内私有 `sanitizeListParam` helper 集中所有默认值/上下界/SortField 校验。
> - `ListAllAuditLogsHandler` 只依赖 `modelcall.AuditRepository`。
> - `ListAuditLogsByUserHandler` 依赖 repo + `*dao.ProxyAPIKeyDAO` + `*gorm.DB`，`Handle` 内部先用 `BatchGetByField("user_id", []uint{userID}, ["id"])` 取该用户名下所有 key 的 ID，再调 `repo.ListByAPIKeyIDs`。

- [ ] **Step 2: 不在此步运行测试或提交**

---
### 附录 A：`internal/infrastructure/repository/audit_repository.go`

```go
package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/vo"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// auditRepository AuditRepository 的 GORM 实现
type auditRepository struct {
	dao *dao.ModelCallAuditDAO
	db  *gorm.DB
}

// NewAuditRepository 构造审计仓储
//
//	@return modelcall.AuditRepository
//	@author centonhuang
//	@update 2026-05-29 14:00:00
func NewAuditRepository(db *gorm.DB) modelcall.AuditRepository {
	return &auditRepository{dao: dao.GetModelCallAuditDAO(), db: db}
}

// Save 持久化审计聚合
//
//	@receiver r *auditRepository
//	@param ctx context.Context
//	@param audit *aggregate.ModelCallAudit
//	@return error
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func (r *auditRepository) Save(ctx context.Context, audit *aggregate.ModelCallAudit) error {
	db := r.db.WithContext(ctx)
	record := &dbmodel.ModelCallAudit{
		APIKeyID:                 audit.APIKeyID(),
		ModelID:                  audit.ModelID(),
		Model:                    audit.Model(),
		UpstreamProvider:         audit.UpstreamProvider(),
		APIProvider:              audit.APIProvider(),
		InputTokens:              audit.Tokens().Input(),
		OutputTokens:             audit.Tokens().Output(),
		CacheCreationInputTokens: audit.Tokens().CacheCreation(),
		CacheReadInputTokens:     audit.Tokens().CacheRead(),
		FirstTokenLatencyMs:      audit.Latency().FirstTokenMs(),
		StreamDurationMs:         audit.Latency().StreamMs(),
		UserAgent:                audit.UserAgent(),
		UpstreamStatusCode:       audit.Status().UpstreamStatusCode(),
		ErrorMessage:             audit.Status().ErrorMessage(),
		TraceID:                  audit.TraceID(),
	}
	if err := r.dao.Create(db, record); err != nil {
		return ierr.Wrap(ierr.ErrDBCreate, err, "create model call audit")
	}
	audit.SetID(record.ID)
	return nil
}

// ListAll 全量分页查询审计记录（admin 用）
//
//	@receiver r *auditRepository
//	@author centonhuang
//	@update 2026-05-29 14:00:00
func (r *auditRepository) ListAll(ctx context.Context, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
	db := r.db.WithContext(ctx)
	return r.paginate(db, param, startTime, endTime)
}

// ListByAPIKeyIDs 按 api_key_id IN (...) 分页查询；apiKeyIDs 为空时返回空结果且不打 SQL
//
//	@receiver r *auditRepository
//	@author centonhuang
//	@update 2026-05-29 14:00:00
func (r *auditRepository) ListByAPIKeyIDs(ctx context.Context, apiKeyIDs []uint, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
	if len(apiKeyIDs) == 0 {
		page, pageSize := param.Page, param.PageSize
		if page < 1 {
			page = 1
		}
		if pageSize < 1 {
			pageSize = 20
		}
		return nil, &model.PageInfo{Page: page, PageSize: pageSize, Total: 0}, nil
	}
	db := r.db.WithContext(ctx).Where(fmt.Sprintf(constant.DBConditionInTemplate, constant.FieldAPIKeyID), apiKeyIDs)
	return r.paginate(db, param, startTime, endTime)
}

// paginate 通用分页：在调用方已附加范围过滤的 db 上做时间范围、模糊搜索、排序、count、limit/offset。
//
// 不复用 baseDAO.Paginate，因为后者只接受 *ModelT 等值 where 不支持 IN 条件。
func (r *auditRepository) paginate(db *gorm.DB, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
	if param.Page < 1 {
		param.Page = 1
	}
	if param.PageSize < 1 {
		param.PageSize = 20
	}

	sql := db.Model(&dbmodel.ModelCallAudit{}).Select(constant.AuditRepoFields).Where(constant.DBConditionDeletedAtZero)

	if !startTime.IsZero() {
		sql = sql.Where(constant.FieldCreatedAt+" >= ?", startTime)
	}
	if !endTime.IsZero() {
		sql = sql.Where(constant.FieldCreatedAt+" <= ?", endTime)
	}

	if param.Query != "" && len(constant.AuditQueryFields) > 0 {
		like := "%" + param.Query + "%"
		expressions := make([]clause.Expression, 0, len(constant.AuditQueryFields))
		for _, field := range constant.AuditQueryFields {
			if field == "" {
				continue
			}
			expressions = append(expressions, clause.Like{Column: clause.Column{Name: field}, Value: like})
		}
		if len(expressions) > 0 {
			sub := db.Session(&gorm.Session{NewDB: true}).Where(expressions[0])
			for _, expr := range expressions[1:] {
				sub = sub.Or(expr)
			}
			sql = sql.Where(sub)
		}
	}

	if param.Sort != "" && param.SortField != "" {
		sql = sql.Order(clause.OrderByColumn{Column: clause.Column{Name: param.SortField}, Desc: param.Sort == enum.SortDesc})
	}

	pageInfo := &model.PageInfo{Page: param.Page, PageSize: param.PageSize}
	if err := sql.Count(&pageInfo.Total).Error; err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "count audit logs")
	}

	limit, offset := param.PageSize, (param.Page-1)*param.PageSize
	var records []*dbmodel.ModelCallAudit
	if err := sql.Limit(limit).Offset(offset).Find(&records).Error; err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "paginate audit logs")
	}

	audits := make([]*aggregate.ModelCallAudit, 0, len(records))
	for _, rec := range records {
		a := aggregate.ReconstructAudit(aggregate.ReconstructAuditInput{
			APIKeyID:         rec.APIKeyID,
			ModelID:          rec.ModelID,
			Model:            rec.Model,
			UpstreamProvider: rec.UpstreamProvider,
			APIProvider:      rec.APIProvider,
			Tokens:           vo.NewTokenBreakdown(rec.InputTokens, rec.OutputTokens, rec.CacheCreationInputTokens, rec.CacheReadInputTokens),
			Latency:          vo.NewCallLatency(time.Duration(rec.FirstTokenLatencyMs)*time.Millisecond, time.Duration(rec.StreamDurationMs)*time.Millisecond),
			Status:           vo.NewCallStatus(rec.UpstreamStatusCode, rec.ErrorMessage),
			UserAgent:        rec.UserAgent,
			TraceID:          rec.TraceID,
			CreatedAt:        rec.CreatedAt,
		})
		a.SetID(rec.ID)
		audits = append(audits, a)
	}
	return audits, pageInfo, nil
}
```
### 附录 B：`internal/application/audit/query/list_audit_logs.go`

```go
package query

import (
	"context"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

var validSortFields = map[string]bool{
	constant.FieldCreatedAt:           true,
	constant.FieldInputTokens:         true,
	constant.FieldOutputTokens:        true,
	constant.FieldFirstTokenLatencyMs: true,
	constant.FieldStreamDurationMs:    true,
}

// ─── 共享：参数清洗 ─────────────────────────────────────────

type listAuditLogsParam struct {
	Page      int
	PageSize  int
	Query     string
	Sort      enum.Sort
	SortField string
}

// sanitizeListParam 校验并填充默认值；非法 SortField 返回 ErrValidation
func sanitizeListParam(ctx context.Context, in listAuditLogsParam) (model.CommonParam, error) {
	if in.PageSize < 1 {
		in.PageSize = 20
	}
	if in.PageSize > constant.AuditMaxPageSize {
		in.PageSize = constant.AuditMaxPageSize
	}
	if in.Page < 1 {
		in.Page = 1
	}
	if in.SortField != "" && !validSortFields[in.SortField] {
		logger.WithCtx(ctx).Warn("[AuditQuery] Invalid sort field", zap.String("sortField", in.SortField))
		return model.CommonParam{}, ierr.New(ierr.ErrValidation, "invalid sort field: "+in.SortField)
	}
	if in.Sort == "" {
		in.Sort = enum.SortDesc
	}
	if in.SortField == "" {
		in.SortField = constant.FieldCreatedAt
	}
	return model.CommonParam{
		PageParam:  model.PageParam{Page: in.Page, PageSize: in.PageSize},
		QueryParam: model.QueryParam{Query: in.Query},
		SortParam:  model.SortParam{Sort: in.Sort, SortField: in.SortField},
	}, nil
}

// ─── ListAllAuditLogsHandler（admin） ─────────────────────────

// ListAllAuditLogsQuery admin 全量审计列表查询
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type ListAllAuditLogsQuery struct {
	Page      int
	PageSize  int
	Query     string
	Sort      enum.Sort
	SortField string
	StartTime time.Time
	EndTime   time.Time
}

// ListAllAuditLogsHandler 全量审计列表查询处理器
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type ListAllAuditLogsHandler interface {
	Handle(ctx context.Context, q ListAllAuditLogsQuery) ([]*aggregate.ModelCallAudit, *model.PageInfo, error)
}

type listAllAuditLogsHandler struct {
	repo modelcall.AuditRepository
}

// NewListAllAuditLogsHandler 构造 admin 全量审计查询处理器
//
//	@param repo modelcall.AuditRepository
//	@return ListAllAuditLogsHandler
//	@author centonhuang
//	@update 2026-05-29 14:00:00
func NewListAllAuditLogsHandler(repo modelcall.AuditRepository) ListAllAuditLogsHandler {
	return &listAllAuditLogsHandler{repo: repo}
}

// Handle 执行全量审计分页查询
func (h *listAllAuditLogsHandler) Handle(ctx context.Context, q ListAllAuditLogsQuery) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
	param, err := sanitizeListParam(ctx, listAuditLogsParam{
		Page: q.Page, PageSize: q.PageSize, Query: q.Query, Sort: q.Sort, SortField: q.SortField,
	})
	if err != nil {
		return nil, nil, err
	}
	return h.repo.ListAll(ctx, param, q.StartTime, q.EndTime)
}

// ─── ListAuditLogsByUserHandler（普通 user） ─────────────────

// ListAuditLogsByUserQuery 按 user 维度审计列表查询
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type ListAuditLogsByUserQuery struct {
	UserID    uint
	Page      int
	PageSize  int
	Query     string
	Sort      enum.Sort
	SortField string
	StartTime time.Time
	EndTime   time.Time
}

// ListAuditLogsByUserHandler user 自己名下所有 key 的审计列表查询处理器
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type ListAuditLogsByUserHandler interface {
	Handle(ctx context.Context, q ListAuditLogsByUserQuery) ([]*aggregate.ModelCallAudit, *model.PageInfo, error)
}

type listAuditLogsByUserHandler struct {
	repo      modelcall.AuditRepository
	apiKeyDAO *dao.ProxyAPIKeyDAO
	db        *gorm.DB
}

// NewListAuditLogsByUserHandler 构造 user 维度审计查询处理器
//
//	@param repo modelcall.AuditRepository
//	@param apiKeyDAO *dao.ProxyAPIKeyDAO
//	@param db *gorm.DB
//	@return ListAuditLogsByUserHandler
//	@author centonhuang
//	@update 2026-05-29 14:00:00
func NewListAuditLogsByUserHandler(repo modelcall.AuditRepository, apiKeyDAO *dao.ProxyAPIKeyDAO, db *gorm.DB) ListAuditLogsByUserHandler {
	return &listAuditLogsByUserHandler{repo: repo, apiKeyDAO: apiKeyDAO, db: db}
}

// Handle 执行 user 维度审计分页查询
//
// 内部两步：
//  1. 用 ProxyAPIKeyDAO.BatchGetByField 按 user_id 查 user 名下所有 key 的 ID 列表
//  2. 调 repo.ListByAPIKeyIDs(keyIDs, ...)；空 keyIDs 时不打 SQL 直接返回空
func (h *listAuditLogsByUserHandler) Handle(ctx context.Context, q ListAuditLogsByUserQuery) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
	param, err := sanitizeListParam(ctx, listAuditLogsParam{
		Page: q.Page, PageSize: q.PageSize, Query: q.Query, Sort: q.Sort, SortField: q.SortField,
	})
	if err != nil {
		return nil, nil, err
	}

	db := h.db.WithContext(ctx)
	keys, err := h.apiKeyDAO.BatchGetByField(db, constant.FieldUserID, []uint{q.UserID}, []string{constant.FieldID})
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "list api keys by user id")
	}
	keyIDs := make([]uint, 0, len(keys))
	for _, k := range keys {
		keyIDs = append(keyIDs, k.ID)
	}
	return h.repo.ListByAPIKeyIDs(ctx, keyIDs, param, q.StartTime, q.EndTime)
}
```
### 附录 C：`internal/handler/audit.go`

```go
package handler

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	auditquery "github.com/hcd233/aris-proxy-api/internal/application/audit/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// AuditHandler 审计处理器
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type AuditHandler interface {
	HandleListAuditLogs(ctx context.Context, req *dto.ListAuditLogsReq) (*dto.HTTPResponse[*dto.ListAuditLogsRsp], error)
}

// AuditDependencies AuditHandler 依赖项
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type AuditDependencies struct {
	ListAll    auditquery.ListAllAuditLogsHandler
	ListByUser auditquery.ListAuditLogsByUserHandler
	APIKeyDAO  *dao.ProxyAPIKeyDAO
	UserDAO    *dao.UserDAO
	DB         *gorm.DB
}

type auditHandler struct {
	listAll    auditquery.ListAllAuditLogsHandler
	listByUser auditquery.ListAuditLogsByUserHandler
	apiKeyDAO  *dao.ProxyAPIKeyDAO
	userDAO    *dao.UserDAO
	db         *gorm.DB
}

// NewAuditHandler 创建审计处理器
//
//	@param deps AuditDependencies
//	@return AuditHandler
//	@author centonhuang
//	@update 2026-05-29 14:00:00
func NewAuditHandler(deps AuditDependencies) AuditHandler {
	return &auditHandler{
		listAll:    deps.ListAll,
		listByUser: deps.ListByUser,
		apiKeyDAO:  deps.APIKeyDAO,
		userDAO:    deps.UserDAO,
		db:         deps.DB,
	}
}

// HandleListAuditLogs 分页获取审计日志列表，按当前 JWT 用户权限分级返回数据范围
//
//	@receiver h *auditHandler
//	@param ctx context.Context
//	@param req *dto.ListAuditLogsReq
//	@return *dto.HTTPResponse[*dto.ListAuditLogsRsp]
//	@return error
//	@author centonhuang
//	@update 2026-05-29 14:00:00
func (h *auditHandler) HandleListAuditLogs(ctx context.Context, req *dto.ListAuditLogsReq) (*dto.HTTPResponse[*dto.ListAuditLogsRsp], error) {
	rsp := &dto.ListAuditLogsRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValuePermission(ctx)

	var (
		audits   []*aggregate.ModelCallAudit
		pageInfo *model.PageInfo
		err      error
	)

	switch permission {
	case enum.PermissionAdmin:
		audits, pageInfo, err = h.listAll.Handle(ctx, auditquery.ListAllAuditLogsQuery{
			Page:      req.Page,
			PageSize:  req.PageSize,
			Query:     req.Query,
			Sort:      req.Sort,
			SortField: req.SortField,
			StartTime: req.StartTime,
			EndTime:   req.EndTime,
		})
	case enum.PermissionUser:
		audits, pageInfo, err = h.listByUser.Handle(ctx, auditquery.ListAuditLogsByUserQuery{
			UserID:    userID,
			Page:      req.Page,
			PageSize:  req.PageSize,
			Query:     req.Query,
			Sort:      req.Sort,
			SortField: req.SortField,
			StartTime: req.StartTime,
			EndTime:   req.EndTime,
		})
	default:
		rsp.Error = ierr.ErrUnauthorized.BizError()
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] List audit logs failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	keyByID, userByID, err := h.fetchRelations(ctx, audits)
	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] Fetch audit relations failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.Logs = lo.Map(audits, func(a *aggregate.ModelCallAudit, _ int) *dto.AuditLogItem {
		item := &dto.AuditLogItem{
			ID:                       a.AggregateID(),
			CreatedAt:                a.CreatedAt(),
			Model:                    a.Model(),
			UpstreamProvider:         a.UpstreamProvider(),
			APIProvider:              a.APIProvider(),
			InputTokens:              a.Tokens().Input(),
			OutputTokens:             a.Tokens().Output(),
			CacheCreationInputTokens: a.Tokens().CacheCreation(),
			CacheReadInputTokens:     a.Tokens().CacheRead(),
			FirstTokenLatencyMs:      a.Latency().FirstTokenMs(),
			StreamDurationMs:         a.Latency().StreamMs(),
			UserAgent:                a.UserAgent(),
			UpstreamStatusCode:       a.Status().UpstreamStatusCode(),
			ErrorMessage:             a.Status().ErrorMessage(),
			TraceID:                  a.TraceID(),
		}
		if k, ok := keyByID[a.APIKeyID()]; ok {
			item.APIKeyName = k.Name
			if u, ok := userByID[k.UserID]; ok {
				item.UserName = u.Name
				item.UserEmail = u.Email
			}
		}
		return item
	})
	rsp.PageInfo = pageInfo
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// fetchRelations 批量拉取 audit 涉及的 ProxyAPIKey 和 User，返回按 ID 索引的 map
func (h *auditHandler) fetchRelations(ctx context.Context, audits []*aggregate.ModelCallAudit) (map[uint]*dbmodel.ProxyAPIKey, map[uint]*dbmodel.User, error) {
	if len(audits) == 0 {
		return map[uint]*dbmodel.ProxyAPIKey{}, map[uint]*dbmodel.User{}, nil
	}
	db := h.db.WithContext(ctx)

	apiKeyIDs := lo.Uniq(lo.Map(audits, func(a *aggregate.ModelCallAudit, _ int) uint { return a.APIKeyID() }))
	keys, err := h.apiKeyDAO.BatchGetByField(db, constant.FieldID, apiKeyIDs, []string{constant.FieldID, constant.FieldName, constant.FieldUserID})
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "batch get proxy api keys")
	}
	keyByID := lo.SliceToMap(keys, func(k *dbmodel.ProxyAPIKey) (uint, *dbmodel.ProxyAPIKey) { return k.ID, k })

	userIDs := lo.Uniq(lo.Map(keys, func(k *dbmodel.ProxyAPIKey, _ int) uint { return k.UserID }))
	users, err := h.userDAO.BatchGetByField(db, constant.FieldID, userIDs, []string{constant.FieldID, constant.FieldName, constant.FieldEmail})
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "batch get users")
	}
	userByID := lo.SliceToMap(users, func(u *dbmodel.User) (uint, *dbmodel.User) { return u.ID, u })

	return keyByID, userByID, nil
}
```
### 附录 D：`test/unit/audit_query/audit_query_test.go`

```go
package audit_query

import (
	"context"
	"errors"
	"testing"
	"time"

	auditquery "github.com/hcd233/aris-proxy-api/internal/application/audit/query"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/aggregate"
)

// ─── fake repository ─────────────────────────────────────

type fakeAuditRepo struct {
	listAllFunc       func(ctx context.Context, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error)
	listByAPIKeyIDsFn func(ctx context.Context, apiKeyIDs []uint, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error)

	listAllCalls       int
	listByAPIKeyIDsCnt int
	lastAPIKeyIDs      []uint
}

func (f *fakeAuditRepo) Save(ctx context.Context, a *aggregate.ModelCallAudit) error { return nil }

func (f *fakeAuditRepo) ListAll(ctx context.Context, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
	f.listAllCalls++
	if f.listAllFunc != nil {
		return f.listAllFunc(ctx, param, startTime, endTime)
	}
	return nil, &model.PageInfo{Page: param.Page, PageSize: param.PageSize}, nil
}

func (f *fakeAuditRepo) ListByAPIKeyIDs(ctx context.Context, apiKeyIDs []uint, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
	f.listByAPIKeyIDsCnt++
	f.lastAPIKeyIDs = apiKeyIDs
	if f.listByAPIKeyIDsFn != nil {
		return f.listByAPIKeyIDsFn(ctx, apiKeyIDs, param, startTime, endTime)
	}
	return nil, &model.PageInfo{Page: param.Page, PageSize: param.PageSize}, nil
}

var _ modelcall.AuditRepository = (*fakeAuditRepo)(nil)

// ─── ListAllAuditLogsHandler 测试 ───────────────────────────

func TestListAllAuditLogs_DefaultsAndClamp(t *testing.T) {
	repo := &fakeAuditRepo{
		listAllFunc: func(ctx context.Context, param model.CommonParam, _, _ time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
			if param.Page != 1 {
				t.Errorf("Page = %d, want 1 (default for 0)", param.Page)
			}
			if param.PageSize != 100 {
				t.Errorf("PageSize = %d, want 100 (clamped from 999)", param.PageSize)
			}
			if param.Sort != enum.SortDesc {
				t.Errorf("Sort = %q, want desc (default)", param.Sort)
			}
			if param.SortField != "created_at" {
				t.Errorf("SortField = %q, want created_at (default)", param.SortField)
			}
			return nil, &model.PageInfo{Page: 1, PageSize: 100}, nil
		},
	}
	h := auditquery.NewListAllAuditLogsHandler(repo)
	if _, _, err := h.Handle(context.Background(), auditquery.ListAllAuditLogsQuery{Page: 0, PageSize: 999}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if repo.listAllCalls != 1 {
		t.Errorf("ListAll calls = %d, want 1", repo.listAllCalls)
	}
}

func TestListAllAuditLogs_InvalidSortField(t *testing.T) {
	repo := &fakeAuditRepo{}
	h := auditquery.NewListAllAuditLogsHandler(repo)
	_, _, err := h.Handle(context.Background(), auditquery.ListAllAuditLogsQuery{
		Page: 1, PageSize: 20, SortField: "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ierr.ErrValidation) {
		t.Errorf("err want ErrValidation, got %v", err)
	}
	if repo.listAllCalls != 0 {
		t.Errorf("ListAll should NOT be called on validation error, but called %d times", repo.listAllCalls)
	}
}

func TestListAllAuditLogs_TimeRangePassthrough(t *testing.T) {
	start := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	repo := &fakeAuditRepo{
		listAllFunc: func(ctx context.Context, param model.CommonParam, s, e time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
			if !s.Equal(start) || !e.Equal(end) {
				t.Errorf("time range mismatch: got [%v, %v], want [%v, %v]", s, e, start, end)
			}
			return nil, &model.PageInfo{}, nil
		},
	}
	h := auditquery.NewListAllAuditLogsHandler(repo)
	if _, _, err := h.Handle(context.Background(), auditquery.ListAllAuditLogsQuery{
		Page: 1, PageSize: 20, StartTime: start, EndTime: end,
	}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

// ─── ListAuditLogsByUserHandler 测试 ────────────────────────

// 注：ListAuditLogsByUserHandler 还需要 *dao.ProxyAPIKeyDAO 与 *gorm.DB 依赖。
// 单元测试无法替换 *dao.ProxyAPIKeyDAO（具体类型，无法 mock），因此 ByUser handler 的
// SQL 执行路径无法在纯单元测试中覆盖。这里只覆盖：
//   - 参数清洗（SortField 非法时返回 ErrValidation 且不调任何 IO）
// SQL 路径正确性留给 plan 后的本地手工冒烟测试验证。

func TestListAuditLogsByUser_InvalidSortField(t *testing.T) {
	repo := &fakeAuditRepo{}
	// db / apiKeyDAO 在 SortField 非法时不会被调用，可以传 nil
	h := auditquery.NewListAuditLogsByUserHandler(repo, nil, nil)
	_, _, err := h.Handle(context.Background(), auditquery.ListAuditLogsByUserQuery{
		UserID: 1, Page: 1, PageSize: 20, SortField: "drop_table",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ierr.ErrValidation) {
		t.Errorf("err want ErrValidation, got %v", err)
	}
	if repo.listByAPIKeyIDsCnt != 0 {
		t.Errorf("repo should NOT be called on validation error, but called %d times", repo.listByAPIKeyIDsCnt)
	}
}
```
### 附录 E：`web/src/app/(dashboard)/audit/page.tsx`

```tsx
"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { api } from "@/lib/api-client";
import type { AuditLogItem, PageInfo } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  ChevronLeft,
  ChevronRight,
  ScrollText,
  Search,
  ListFilter,
  Check,
  Clock,
} from "lucide-react";
import { toast } from "sonner";
import { useIsMobile } from "@/hooks/use-mobile";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

type TimeRangeKey = "1h" | "24h" | "7d" | "custom";

const TIME_RANGE_LABELS: Record<TimeRangeKey, string> = {
  "1h": "Last 1 hour",
  "24h": "Last 24 hours",
  "7d": "Last 7 days",
  custom: "Custom",
};

function computeRange(key: TimeRangeKey, customStart?: string, customEnd?: string): { startTime?: string; endTime?: string } {
  if (key === "custom") {
    return {
      startTime: customStart ? new Date(customStart).toISOString() : undefined,
      endTime: customEnd ? new Date(customEnd).toISOString() : undefined,
    };
  }
  const now = new Date();
  const start = new Date(now);
  if (key === "1h") start.setHours(start.getHours() - 1);
  else if (key === "24h") start.setHours(start.getHours() - 24);
  else if (key === "7d") start.setDate(start.getDate() - 7);
  return { startTime: start.toISOString(), endTime: now.toISOString() };
}

function formatTokens(input: number, output: number): string {
  const fmt = (n: number) => (n >= 1000 ? `${(n / 1000).toFixed(1)}k` : String(n));
  return `${fmt(input)} / ${fmt(output)}`;
}

export default function AuditPage() {
  const isMobile = useIsMobile();
  const [logs, setLogs] = useState<AuditLogItem[]>([]);
  const [pageInfo, setPageInfo] = useState<PageInfo>({ page: 1, pageSize: 20, total: 0 });
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [timeRange, setTimeRange] = useState<TimeRangeKey>("24h");
  const [customStart, setCustomStart] = useState("");
  const [customEnd, setCustomEnd] = useState("");
  const [pageInputValue, setPageInputValue] = useState("1");

  const fetchLogs = useCallback(
    async (page: number, pageSize: number, query: string, range: TimeRangeKey, cs: string, ce: string) => {
      setLoading(true);
      try {
        const { startTime, endTime } = computeRange(range, cs, ce);
        const rsp = await api.listAuditLogs({
          page,
          pageSize,
          query: query || undefined,
          startTime,
          endTime,
        });
        if (rsp.error) {
          toast.error(rsp.error.message ?? "Failed to load audit logs");
          return;
        }
        setLogs(rsp.logs ?? []);
        if (rsp.pageInfo) {
          setPageInfo(rsp.pageInfo);
          setPageInputValue(String(rsp.pageInfo.page));
        }
      } catch (err) {
        toast.error(err instanceof Error ? err.message : "Failed to load audit logs");
      } finally {
        setLoading(false);
      }
    },
    []
  );

  /* eslint-disable react-hooks/set-state-in-effect -- Initial data fetch on mount */
  useEffect(() => {
    fetchLogs(1, 20, "", "24h", "", "");
  }, [fetchLogs]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const totalPages = useMemo(
    () => Math.max(1, Math.ceil(pageInfo.total / pageInfo.pageSize)),
    [pageInfo]
  );

  const refresh = (page: number, pageSize?: number) =>
    fetchLogs(page, pageSize ?? pageInfo.pageSize, searchQuery, timeRange, customStart, customEnd);

  const handleCopyTrace = (traceId: string) => {
    if (!traceId) return;
    navigator.clipboard.writeText(traceId).then(
      () => toast.success("TraceID copied"),
      () => toast.error("Copy failed")
    );
  };

  return (
    <div className="space-y-8">
      <div>
        <h1 className="font-display text-2xl md:text-3xl font-semibold tracking-tight text-foreground">Audit</h1>
        <p className="mt-1.5 text-sm text-muted-foreground">
          Inspect model call records, latency, errors, and trace IDs.
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="font-display">Audit Logs</CardTitle>
        </CardHeader>
        <CardContent>
          {/* 筛选区 */}
          <div className="mb-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <div className="flex flex-wrap items-center gap-2">
              <DropdownMenu>
                <DropdownMenuTrigger render={<Button variant="outline" size="sm" className="gap-1.5" />}>
                  <Clock className="size-3.5" />
                  {TIME_RANGE_LABELS[timeRange]}
                </DropdownMenuTrigger>
                <DropdownMenuContent align="start">
                  {(Object.keys(TIME_RANGE_LABELS) as TimeRangeKey[]).map((k) => (
                    <DropdownMenuItem
                      key={k}
                      onClick={() => {
                        setTimeRange(k);
                        if (k !== "custom") {
                          fetchLogs(1, pageInfo.pageSize, searchQuery, k, customStart, customEnd);
                        }
                      }}
                    >
                      {k === timeRange && <Check className="size-4" />}
                      <span className={k === timeRange ? "ml-0" : "ml-6"}>{TIME_RANGE_LABELS[k]}</span>
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>
              {timeRange === "custom" && (
                <div className="flex items-center gap-2">
                  <input
                    type="datetime-local"
                    value={customStart}
                    onChange={(e) => setCustomStart(e.target.value)}
                    onBlur={() => fetchLogs(1, pageInfo.pageSize, searchQuery, "custom", customStart, customEnd)}
                    className="h-8 rounded-md border border-input bg-transparent px-2 py-1 text-xs"
                  />
                  <span className="text-xs text-muted-foreground">–</span>
                  <input
                    type="datetime-local"
                    value={customEnd}
                    onChange={(e) => setCustomEnd(e.target.value)}
                    onBlur={() => fetchLogs(1, pageInfo.pageSize, searchQuery, "custom", customStart, customEnd)}
                    className="h-8 rounded-md border border-input bg-transparent px-2 py-1 text-xs"
                  />
                </div>
              )}
            </div>
            <div className="relative w-full md:max-w-sm">
              <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                placeholder="Search by traceID or model..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") refresh(1);
                }}
                className="pl-9"
              />
            </div>
          </div>

          {/* 列表 */}
          {loading ? (
            <div className="space-y-3">
              {Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} className="h-10 w-full" />
              ))}
            </div>
          ) : logs.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-center">
              <ScrollText className="mb-3 size-10 text-muted-foreground/50" />
              <p className="text-sm text-muted-foreground">No audit logs in selected range</p>
            </div>
          ) : isMobile ? (
            <div className="space-y-3">
              {logs.map((log) => {
                const ok = log.upstreamStatusCode === 200;
                return (
                  <div key={log.id} className="rounded-lg border border-border bg-card p-4">
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0 flex-1">
                        <p className="truncate text-sm font-medium">{log.model || "—"}</p>
                        <p className="mt-0.5 truncate text-xs text-muted-foreground">
                          {log.userName || "—"} · {log.apiKeyName || "—"}
                        </p>
                      </div>
                      <Badge variant={ok ? "secondary" : "destructive"} className="shrink-0 text-xs" title={ok ? undefined : log.errorMessage}>
                        {log.upstreamStatusCode}
                      </Badge>
                    </div>
                    <div className="mt-2 flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
                      <span>{new Date(log.createdAt).toLocaleString()}</span>
                      <span>{formatTokens(log.inputTokens, log.outputTokens)}</span>
                      <span>{log.firstTokenLatencyMs}ms</span>
                      <span
                        className="cursor-pointer font-mono underline-offset-2 hover:underline"
                        onClick={() => handleCopyTrace(log.traceId)}
                        title="Click to copy full traceID"
                      >
                        {log.traceId.slice(-6) || "—"}
                      </span>
                    </div>
                  </div>
                );
              })}
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Time</TableHead>
                  <TableHead>Model</TableHead>
                  <TableHead>User</TableHead>
                  <TableHead>API Key</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Tokens</TableHead>
                  <TableHead>Latency</TableHead>
                  <TableHead>TraceID</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {logs.map((log) => {
                  const ok = log.upstreamStatusCode === 200;
                  return (
                    <TableRow key={log.id}>
                      <TableCell className="whitespace-nowrap text-muted-foreground">
                        {new Date(log.createdAt).toLocaleString()}
                      </TableCell>
                      <TableCell className="max-w-[180px] truncate">{log.model || "—"}</TableCell>
                      <TableCell>
                        <div className="text-sm">{log.userName || "—"}</div>
                        <div className="text-xs text-muted-foreground">{log.userEmail || ""}</div>
                      </TableCell>
                      <TableCell className="max-w-[140px] truncate">{log.apiKeyName || "—"}</TableCell>
                      <TableCell>
                        <Badge variant={ok ? "secondary" : "destructive"} className="text-xs" title={ok ? undefined : log.errorMessage}>
                          {log.upstreamStatusCode}
                        </Badge>
                      </TableCell>
                      <TableCell className="whitespace-nowrap">{formatTokens(log.inputTokens, log.outputTokens)}</TableCell>
                      <TableCell className="whitespace-nowrap text-muted-foreground">
                        {log.firstTokenLatencyMs}ms
                        {log.streamDurationMs > 0 && <span className="ml-1 text-xs">/ {log.streamDurationMs}ms</span>}
                      </TableCell>
                      <TableCell
                        className="cursor-pointer font-mono text-xs underline-offset-2 hover:underline"
                        onClick={() => handleCopyTrace(log.traceId)}
                        title="Click to copy full traceID"
                      >
                        {log.traceId.slice(-6) || "—"}
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          )}

          {/* 分页 */}
          {pageInfo.total > 0 && (
            <div className="mt-4 flex flex-wrap items-center justify-between gap-4">
              <div className="hidden items-center gap-3 md:flex">
                <DropdownMenu>
                  <DropdownMenuTrigger render={<Button variant="outline" size="sm" className="gap-1.5" />}>
                    <ListFilter className="size-3.5" />
                    {pageInfo.pageSize} / page
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="start">
                    {[20, 50, 100].map((size) => (
                      <DropdownMenuItem key={size} onClick={() => refresh(1, size)}>
                        {size === pageInfo.pageSize && <Check className="size-4" />}
                        <span className={size === pageInfo.pageSize ? "ml-0" : "ml-6"}>{size} per page</span>
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuContent>
                </DropdownMenu>
                <p className="hidden text-sm text-muted-foreground md:block">
                  {pageInfo.total} log{pageInfo.total !== 1 ? "s" : ""} total
                </p>
              </div>
              <div className="flex items-center gap-2">
                <Button variant="outline" size="sm" disabled={pageInfo.page <= 1} onClick={() => refresh(pageInfo.page - 1)}>
                  <ChevronLeft className="size-4" />
                </Button>
                <div className="flex items-center gap-1.5 text-sm">
                  <span className="text-muted-foreground">Page</span>
                  <input
                    type="number"
                    min={1}
                    max={totalPages}
                    value={pageInputValue}
                    onChange={(e) => setPageInputValue(e.target.value)}
                    className="h-8 w-14 rounded-md border border-input bg-transparent px-2 py-1 text-center text-sm tabular-nums focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/40 focus-visible:outline-none dark:bg-input/30"
                    onKeyDown={(e) => {
                      if (e.key === "Enter") {
                        let page = parseInt(pageInputValue, 10);
                        if (Number.isNaN(page)) page = 1;
                        page = Math.max(1, Math.min(page, totalPages));
                        refresh(page);
                      }
                    }}
                    onBlur={() => {
                      let page = parseInt(pageInputValue, 10);
                      if (Number.isNaN(page)) page = 1;
                      page = Math.max(1, Math.min(page, totalPages));
                      refresh(page);
                    }}
                  />
                  <span className="text-muted-foreground">/ {totalPages}</span>
                </div>
                <Button variant="outline" size="sm" disabled={pageInfo.page >= totalPages} onClick={() => refresh(pageInfo.page + 1)}>
                  <ChevronRight className="size-4" />
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
```
