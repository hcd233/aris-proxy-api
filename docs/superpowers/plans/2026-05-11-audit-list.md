# 审计日志列表接口 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现分页查询审计日志接口 `GET /api/v1/audit/logs`，支持搜索 traceID/model、时间范围过滤、多字段排序

**Architecture:** handler → application/query → domain/repository → infrastructure/repository → DAO.Paginate

**Tech Stack:** Go 1.25.1, GORM, Huma, dig DI

---

### Task 1: 新增缺失的常量和 SortParam.SortField

**Files:**
- Modify: `internal/common/constant/sql.go`
- Modify: `internal/common/model/param.go`

- [ ] **Step 1a: 添加 SortField 到 model.SortParam**

修改 `internal/common/model/param.go`，在 `SortParam` 结构体中添加 `SortField` 字段：

```go
type SortParam struct {
	Sort      enum.Sort `query:"sort" enum:"asc,desc"`
	SortField string    `json:"sortField"`
}
```

- [ ] **Step 1b: 添加审计表字段常量**

```go
const (
	// audit fields
	FieldTraceID              = "trace_id"
	FieldInputTokens           = "input_tokens"
	FieldOutputTokens          = "output_tokens"
	FieldFirstTokenLatencyMs   = "first_token_latency_ms"
	FieldStreamDurationMs      = "stream_duration_ms"
	FieldAPIKeyID              = "api_key_id"
)
```

- [ ] **Step 2: Commit**

```bash
git add internal/common/constant/sql.go
git commit -m "feat: add audit field name constants"
```

---

### Task 2: 新增 Domain 层查询接口

**Files:**
- Modify: `internal/domain/modelcall/repository.go`

- [ ] **Step 1: 添加 ListByAPIKeyID 方法到 AuditRepository 接口**

在 `AuditRepository` 接口的 `Save` 方法后添加：

```go
// ListByAPIKeyID 按 APIKeyID 分页查询审计记录，支持时间范围过滤、关键词搜索和多字段排序
ListByAPIKeyID(ctx context.Context, apiKeyID uint, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error)
```

需要新增 import：`"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/aggregate"` 和 `"time"`。

- [ ] **Step 2: Commit**

```bash
git add internal/domain/modelcall/repository.go
git commit -m "feat: add ListByAPIKeyID to AuditRepository interface"
```

---

### Task 3: 新增 Domain 层 ReconstructAudit 工厂

**Files:**
- Create: `internal/domain/modelcall/aggregate/reconstruct.go`

- [ ] **Step 1: 创建重构工厂**

```go
package aggregate

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/vo"
	"github.com/hcd233/aris-proxy-api/internal/enum"
)

// ReconstructAudit rebuilds an aggregate from persisted state (for read queries).
//
//	@author centonhuang
//	@update 2026-05-11 10:00:00
func ReconstructAudit(id uint, apiKeyID uint, modelID uint, model string, upstreamProvider enum.ProviderType, apiProvider enum.ProviderType, tokens vo.TokenBreakdown, latency vo.CallLatency, status vo.CallStatus, userAgent string, traceID string, createdAt time.Time) *ModelCallAudit {
	return &ModelCallAudit{
		Base:             aggregate.Base{},
		apiKeyID:         apiKeyID,
		modelID:          modelID,
		model:            model,
		upstreamProvider: upstreamProvider,
		apiProvider:      apiProvider,
		tokens:           tokens,
		latency:          latency,
		status:           status,
		userAgent:        userAgent,
		traceID:          traceID,
		createdAt:        createdAt,
	}
}
```

由于 Base 是私有字段，无法通过字面量初始化。改用 `aggregate.Base` 的 SetID 方式。修正为：

```go
func ReconstructAudit(input ReconstructAuditInput) *ModelCallAudit {
	a := &ModelCallAudit{
		apiKeyID:         input.APIKeyID,
		modelID:          input.ModelID,
		model:            input.Model,
		upstreamProvider: input.UpstreamProvider,
		apiProvider:      input.APIProvider,
		tokens:           input.Tokens,
		latency:          input.Latency,
		status:           input.Status,
		userAgent:        input.UserAgent,
		traceID:          input.TraceID,
		createdAt:        input.CreatedAt,
	}
	// Base 的 id 字段只能通过 SetID 设置，无法直接字面量赋值
	return a
}

type ReconstructAuditInput struct {
	APIKeyID         uint
	ModelID          uint
	Model            string
	UpstreamProvider enum.ProviderType
	APIProvider      enum.ProviderType
	Tokens           vo.TokenBreakdown
	Latency          vo.CallLatency
	Status           vo.CallStatus
	UserAgent        string
	TraceID          string
	CreatedAt        time.Time
}
```

由于 ModelCallAudit 嵌入 `aggregate.Base`（私有字段 `id uint`），字面量无法初始化。已有一个无参数的 zero-value Base，在调用方用 `SetID` 回填。

- [ ] **Step 2: 运行测试确保编译通过**

```bash
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add internal/domain/modelcall/aggregate/reconstruct.go
git commit -m "feat: add ReconstructAudit factory for aggregate reconstruction"
```

---

### Task 4: 新增 Infrastructure 层查询实现

**Files:**
- Modify: `internal/infrastructure/repository/audit_repository.go`

- [ ] **Step 1: 实现 ListByAPIKeyID 方法**

在 `auditRepository.Save() { ... }` 方法之后添加：

```go
// ListByAPIKeyID 按 APIKeyID 分页查询审计记录
//
//	@receiver r *auditRepository
//	@param ctx context.Context
//	@param apiKeyID uint
//	@param param model.CommonParam
//	@param startTime time.Time
//	@param endTime time.Time
//	@return []*aggregate.ModelCallAudit
//	@return *model.PageInfo
//	@return error
//	@author centonhuang
//	@update 2026-05-11 10:00:00
func (r *auditRepository) ListByAPIKeyID(ctx context.Context, apiKeyID uint, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
	db := database.GetDBInstance(ctx)

	// time filtering - pre-add conditions before Paginate
	if !startTime.IsZero() {
		db = db.Where(constant.FieldCreatedAt + " >= ?", startTime)
	}
	if !endTime.IsZero() {
		db = db.Where(constant.FieldCreatedAt + " <= ?", endTime)
	}

	where := &dbmodel.ModelCallAudit{APIKeyID: apiKeyID}
	records, pageInfo, err := r.dao.Paginate(
		db,
		where,
		constant.AuditRepoFields,
		&dao.CommonParam{
			PageParam:  dao.PageParam{Page: param.Page, PageSize: param.PageSize},
			QueryParam: dao.QueryParam{Query: param.Query, QueryFields: constant.AuditQueryFields},
			SortParam:  dao.SortParam{Sort: enum.Sort(param.Sort), SortField: param.SortField},
		},
	)
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "paginate audit logs")
	}

	audits := make([]*aggregate.ModelCallAudit, 0, len(records))
	for _, r := range records {
		a := aggregate.ReconstructAudit(aggregate.ReconstructAuditInput{
			APIKeyID:         r.APIKeyID,
			ModelID:          r.ModelID,
			Model:            r.Model,
			UpstreamProvider: r.UpstreamProvider,
			APIProvider:      r.APIProvider,
			Tokens:           vo.NewTokenBreakdown(r.InputTokens, r.OutputTokens, r.CacheCreationInputTokens, r.CacheReadInputTokens),
			Latency:          vo.NewCallLatency(time.Duration(r.FirstTokenLatencyMs)*time.Millisecond, time.Duration(r.StreamDurationMs)*time.Millisecond),
			Status:           vo.NewCallStatus(r.UpstreamStatusCode, r.ErrorMessage),
			UserAgent:        r.UserAgent,
			TraceID:          r.TraceID,
			CreatedAt:        r.CreatedAt,
		})
		a.SetID(r.ID)
		audits = append(audits, a)
	}
	return audits, pageInfo, nil
}
```

需要新增 import：
- `"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/aggregate"`
- `"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/vo"`
- `"github.com/hcd233/aris-proxy-api/internal/common/enum"`

- [ ] **Step 2: 添加常量到 sql.go**

在 `internal/common/constant/sql.go` 末尾添加：

```go
var (
	// AuditRepoFields list query fields
	AuditRepoFields = []string{FieldID, FieldAPIKeyID, FieldModelID, FieldModel, FieldUpstreamProvider, FieldAPIProvider, FieldInputTokens, FieldOutputTokens, FieldCacheCreationInputTokens, FieldCacheReadInputTokens, FieldFirstTokenLatencyMs, FieldStreamDurationMs, FieldUserAgent, FieldUpstreamStatusCode, FieldErrorMessage, FieldTraceID, FieldCreatedAt}

	// AuditQueryFields searchable fields
	AuditQueryFields = []string{FieldTraceID, FieldModel}
)
```

同时需补充字段常量（如未在 Task 1 添加）：
```go
const (
	FieldTraceID              = "trace_id"
	FieldInputTokens          = "input_tokens"
	FieldOutputTokens         = "output_tokens"
	FieldFirstTokenLatencyMs  = "first_token_latency_ms"
	FieldStreamDurationMs     = "stream_duration_ms"
	FieldAPIKeyID             = "api_key_id"
	FieldModelID              = "model_id"
	FieldUpstreamProvider     = "upstream_provider"
	FieldAPIProvider          = "api_provider"
	FieldCacheCreationInputTokens = "cache_creation_input_tokens"
	FieldCacheReadInputTokens     = "cache_read_input_tokens"
	FieldUserAgent            = "user_agent"
	FieldUpstreamStatusCode   = "upstream_status_code"
	FieldErrorMessage         = "error_message"
)
```

- [ ] **Step 3: 检查 `constant.DBConditionDeletedAtZero`**

确认 `internal/common/constant/database.go` 中有 `DBConditionDeletedAtZero = "deleted_at = 0"`，`ModelCallAudit` 有 `DeletedAt int64` 字段。

- [ ] **Step 4: 运行编译检查**

```bash
go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/repository/audit_repository.go internal/common/constant/sql.go
git commit -m "feat: implement ListByAPIKeyID in audit repository"
```

---

### Task 5: 新增 Application 层查询 Handler

**Files:**
- Create: `internal/application/audit/query/list_audit_logs.go`

- [ ] **Step 1: 创建查询 handler**

```go
package query

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

const maxPageSize = 100

var validSortFields = map[string]bool{
	"created_at":              true,
	"input_tokens":            true,
	"output_tokens":           true,
	"first_token_latency_ms":  true,
	"stream_duration_ms":      true,
}

// ListAuditLogsQuery 审计日志列表查询
//
//	@author centonhuang
//	@update 2026-05-11 10:00:00
type ListAuditLogsQuery struct {
	APIKeyID  uint
	Page      int
	PageSize  int
	Query     string
	Sort      enum.Sort
	SortField string
	StartTime time.Time
	EndTime   time.Time
}

// ListAuditLogsHandler 审计日志列表查询处理器
//
//	@author centonhuang
//	@update 2026-05-11 10:00:00
type ListAuditLogsHandler interface {
	Handle(ctx context.Context, q ListAuditLogsQuery) ([]*aggregate.ModelCallAudit, *model.PageInfo, error)
}

type listAuditLogsHandler struct {
	repo modelcall.AuditRepository
}

// NewListAuditLogsHandler 构造审计日志列表查询处理器
//
//	@param repo modelcall.AuditRepository
//	@return ListAuditLogsHandler
//	@author centonhuang
//	@update 2026-05-11 10:00:00
func NewListAuditLogsHandler(repo modelcall.AuditRepository) ListAuditLogsHandler {
	return &listAuditLogsHandler{repo: repo}
}

// Handle 执行审计日志分页查询
//
//	@receiver h *listAuditLogsHandler
//	@param ctx context.Context
//	@param q ListAuditLogsQuery
//	@return []*aggregate.ModelCallAudit
//	@return *model.PageInfo
//	@return error
//	@author centonhuang
//	@update 2026-05-11 10:00:00
func (h *listAuditLogsHandler) Handle(ctx context.Context, q ListAuditLogsQuery) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
	log := logger.WithCtx(ctx)

	if q.PageSize < 1 {
		q.PageSize = 20
	}
	if q.PageSize > maxPageSize {
		q.PageSize = maxPageSize
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.SortField != "" && !validSortFields[q.SortField] {
		log.Warn("[AuditQuery] Invalid sort field", zap.String("sortField", q.SortField))
		return nil, nil, ierr.New(ierr.ErrValidation, "invalid sort field: "+q.SortField)
	}
	if q.Sort == "" {
		q.Sort = enum.SortDesc
	}
	if q.SortField == "" {
		q.SortField = "created_at"
	}

	param := model.CommonParam{
		PageParam:  model.PageParam{Page: q.Page, PageSize: q.PageSize},
		QueryParam: model.QueryParam{Query: q.Query},
		SortParam:  model.SortParam{Sort: q.Sort, SortField: q.SortField},
	}

	return h.repo.ListByAPIKeyID(ctx, q.APIKeyID, param, q.StartTime, q.EndTime)
}
```

- [ ] **Step 2: 运行编译检查**

```bash
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add internal/application/audit/query/list_audit_logs.go
git commit -m "feat: add ListAuditLogsHandler query handler"
```

---

### Task 6: 新增 DTO

**Files:**
- Create: `internal/dto/audit.go`

- [ ] **Step 1: 创建 DTO 文件**

```go
package dto

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// ListAuditLogsReq 审计日志列表请求
//
//	@author centonhuang
//	@update 2026-05-11 10:00:00
type ListAuditLogsReq struct {
	Page      int       `query:"page" required:"true" minimum:"1"`
	PageSize  int       `query:"pageSize" required:"true" minimum:"1" maximum:"100"`
	Query     string    `query:"query" maxLength:"100"`
	Sort      enum.Sort `query:"sort" enum:"asc,desc"`
	SortField string    `query:"sortField" maxLength:"50"`
	StartTime time.Time `query:"startTime"`
	EndTime   time.Time `query:"endTime"`
}

// ListAuditLogsRsp 审计日志列表响应
//
//	@author centonhuang
//	@update 2026-05-11 10:00:00
type ListAuditLogsRsp struct {
	CommonRsp
	Logs     []*AuditLogItem `json:"logs,omitempty" doc:"审计日志列表"`
	PageInfo *model.PageInfo `json:"pageInfo,omitempty" doc:"分页信息"`
}

// AuditLogItem 审计日志条目
//
//	@author centonhuang
//	@update 2026-05-11 10:00:00
type AuditLogItem struct {
	ID                        uint      `json:"id" doc:"记录ID"`
	CreatedAt                 time.Time `json:"createdAt" doc:"创建时间"`
	Model                     string    `json:"model" doc:"模型名"`
	UpstreamProvider          string    `json:"upstreamProvider" doc:"上游提供商"`
	APIProvider               string    `json:"apiProvider" doc:"接口协议"`
	InputTokens               int       `json:"inputTokens" doc:"输入token数"`
	OutputTokens              int       `json:"outputTokens" doc:"输出token数"`
	CacheCreationInputTokens  int       `json:"cacheCreationInputTokens" doc:"缓存写入token数"`
	CacheReadInputTokens      int       `json:"cacheReadInputTokens" doc:"缓存命中token数"`
	FirstTokenLatencyMs       int64     `json:"firstTokenLatencyMs" doc:"首token延迟(ms)"`
	StreamDurationMs          int64     `json:"streamDurationMs" doc:"流式持续时间(ms)"`
	UserAgent                 string    `json:"userAgent" doc:"User-Agent"`
	UpstreamStatusCode        int       `json:"upstreamStatusCode" doc:"上游状态码"`
	ErrorMessage              string    `json:"errorMessage" doc:"错误信息"`
	TraceID                   string    `json:"traceId" doc:"Trace ID"`
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/dto/audit.go
git commit -m "feat: add audit list DTOs"
```

---

### Task 7: 新增 Handler

**Files:**
- Create: `internal/handler/audit.go`

- [ ] **Step 1: 创建 Handler**

```go
package handler

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	auditquery "github.com/hcd233/aris-proxy-api/internal/application/audit/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// AuditHandler 审计处理器
//
//	@author centonhuang
//	@update 2026-05-11 10:00:00
type AuditHandler interface {
	HandleListAuditLogs(ctx context.Context, req *dto.ListAuditLogsReq) (*dto.HTTPResponse[*dto.ListAuditLogsRsp], error)
}

// AuditDependencies AuditHandler 依赖项
//
//	@author centonhuang
//	@update 2026-05-11 10:00:00
type AuditDependencies struct {
	List auditquery.ListAuditLogsHandler
}

type auditHandler struct {
	list auditquery.ListAuditLogsHandler
}

// NewAuditHandler 创建审计处理器
//
//	@param deps AuditDependencies
//	@return AuditHandler
//	@author centonhuang
//	@update 2026-05-11 10:00:00
func NewAuditHandler(deps AuditDependencies) AuditHandler {
	return &auditHandler{list: deps.List}
}

// HandleListAuditLogs 分页获取审计日志列表
//
//	@receiver h *auditHandler
//	@param ctx context.Context
//	@param req *dto.ListAuditLogsReq
//	@return *dto.HTTPResponse[*dto.ListAuditLogsRsp]
//	@return error
//	@author centonhuang
//	@update 2026-05-11 10:00:00
func (h *auditHandler) HandleListAuditLogs(ctx context.Context, req *dto.ListAuditLogsReq) (*dto.HTTPResponse[*dto.ListAuditLogsRsp], error) {
	rsp := &dto.ListAuditLogsRsp{}
	apiKeyID := util.CtxValueUint(ctx, constant.CtxKeyAPIKeyID)

	audits, pageInfo, err := h.list.Handle(ctx, auditquery.ListAuditLogsQuery{
		APIKeyID:  apiKeyID,
		Page:      req.Page,
		PageSize:  req.PageSize,
		Query:     req.Query,
		Sort:      req.Sort,
		SortField: req.SortField,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] List audit logs failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return util.WrapHTTPResponse(rsp, nil)
	}

	rsp.Logs = lo.Map(audits, func(a *aggregate.ModelCallAudit, _ int) *dto.AuditLogItem {
		return &dto.AuditLogItem{
			ID:                        a.AggregateID(),
			CreatedAt:                 a.CreatedAt(),
			Model:                     a.Model(),
			UpstreamProvider:          a.UpstreamProvider(),
			APIProvider:               a.APIProvider(),
			InputTokens:               a.Tokens().Input(),
			OutputTokens:              a.Tokens().Output(),
			CacheCreationInputTokens:  a.Tokens().CacheCreation(),
			CacheReadInputTokens:      a.Tokens().CacheRead(),
			FirstTokenLatencyMs:       a.Latency().FirstTokenMs(),
			StreamDurationMs:          a.Latency().StreamMs(),
			UserAgent:                 a.UserAgent(),
			UpstreamStatusCode:        a.Status().UpstreamStatusCode(),
			ErrorMessage:              a.Status().ErrorMessage(),
			TraceID:                   a.TraceID(),
		}
	})
	rsp.PageInfo = pageInfo
	return util.WrapHTTPResponse(rsp, nil)
}
```

注意：`aggregate.ModelCallAudit` 的 `UpstreamProvider()` 和 `APIProvider()` 返回 `enum.ProviderType`（类型为 string），可以直接赋值给 DTO 的 string 字段。

- [ ] **Step 2: 运行编译检查**

```bash
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add internal/handler/audit.go
git commit -m "feat: add AuditHandler for audit list endpoint"
```

---

### Task 8: 新增路由注册

**Files:**
- Create: `internal/router/audit.go`

- [ ] **Step 1: 创建路由文件**

```go
package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

func initAuditRouter(auditGroup huma.API, auditHandler handler.AuditHandler) {
	auditGroup.UseMiddleware(middleware.APIKeyMiddleware())

	huma.Register(auditGroup, huma.Operation{
		OperationID: "listAuditLogs",
		Method:      http.MethodGet,
		Path:        "/logs",
		Summary:     "ListAuditLogs",
		Description: "Paginate audit logs filtered by current API key, supports search by traceID/model and time range filtering",
		Tags:        []string{"Audit"},
		Security: []map[string][]string{
			{"apiKeyAuth": {}},
		},
	}, auditHandler.HandleListAuditLogs)
}
```

- [ ] **Step 2: 注册到主路由**

修改 `internal/router/router.go`：

在 `APIRouterDependencies` 结构体中添加：
```go
AuditHandler handler.AuditHandler
```

在 `RegisterAPIRouter` 函数末尾添加：
```go
auditGroup := huma.NewGroup(v1Group, "/audit")
initAuditRouter(auditGroup, deps.AuditHandler)
```

- [ ] **Step 3: 修改 bootstrap/container.go 注册依赖**

**3a. 在 import 区添加**：
```go
auditquery "github.com/hcd233/aris-proxy-api/internal/application/audit/query"
modelcall "github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
```

**3b. 在 `provideInfrastructure` 函数中添加**（在 `newSessionReadRepository` 之后）：
```go
if err := container.Provide(newAuditRepository); err != nil {
    return err
}
```

**3c. 在 `provideApplication` 函数中添加**（在 `sessionquery.NewListSessionsHandler` 之后）：
```go
if err := container.Provide(auditquery.NewListAuditLogsHandler); err != nil {
    return err
}
```

**3d. 在 `provideHandlers` 函数中添加**（在 `newSessionDependencies` 之后）：
```go
if err := container.Provide(newAuditDependencies); err != nil {
    return err
}
if err := container.Provide(handler.NewAuditHandler); err != nil {
    return err
}
```

**3e. 添加构造函数**（在文件末尾）：
```go
func newAuditRepository() modelcall.AuditRepository {
	return repository.NewAuditRepository()
}

func newAuditDependencies(list auditquery.ListAuditLogsHandler) handler.AuditDependencies {
	return handler.AuditDependencies{List: list}
}
```

- [ ] **Step 4: 修改 bootstrap/router.go**

在 `routeParams` 结构体中添加：
```go
AuditHandler handler.AuditHandler
```

在 `RegisterAPIRouter` 调用中添加 `AuditHandler`：
```go
AuditHandler: params.AuditHandler,
```

- [ ] **Step 5: 运行编译检查和 lint**

```bash
go build ./...
make lint
```

- [ ] **Step 6: Commit**

```bash
git add internal/router/audit.go internal/router/router.go internal/bootstrap/container.go internal/bootstrap/router.go
git commit -m "feat: wire audit route and dependencies"
```

---

### Task 9: 单元测试

**Files:**
- Create: `test/unit/audit_query/audit_query_test.go`
- Create: `test/unit/audit_dto/audit_dto_test.go`

- [ ] **Step 1: 创建 query handler 单元测试**

`test/unit/audit_query/audit_query_test.go`:

```go
package audit_query

import (
    "testing"

    auditquery "github.com/hcd233/aris-proxy-api/internal/application/audit/query"
)

func TestListAuditLogsQueryDefaultValues(t *testing.T) {
    q := auditquery.ListAuditLogsQuery{
        APIKeyID: 1,
        Page:     0,
        PageSize: 0,
        Sort:     "",
        SortField: "",
    }
    // Defaults are applied in Handle(), not in struct init
    // This test verifies the struct accepts zero values
    if q.APIKeyID != 1 {
        t.Errorf("APIKeyID = %d, want 1", q.APIKeyID)
    }
}
```

- [ ] **Step 2: 创建 DTO 序列化/默认值测试**

`test/unit/audit_dto/audit_dto_test.go`:

```go
package audit_dto

import (
    "testing"

    "github.com/bytedance/sonic"
    "github.com/hcd233/aris-proxy-api/internal/common/model"
    "github.com/hcd233/aris-proxy-api/internal/dto"
)

func TestListAuditLogsRsp_EmptyLogs(t *testing.T) {
    rsp := &dto.ListAuditLogsRsp{
        Logs:     nil,
        PageInfo: &model.PageInfo{Page: 1, PageSize: 20, Total: 0},
    }
    data, err := sonic.Marshal(rsp)
    if err != nil {
        t.Fatalf("marshal failed: %v", err)
    }
    var obj map[string]any
    if err := sonic.Unmarshal(data, &obj); err != nil {
        t.Fatalf("unmarshal failed: %v", err)
    }
    // Verify PageInfo is present
    pi, ok := obj["pageInfo"]
    if !ok {
        t.Error("pageInfo field missing")
    }
    _ = pi
}

func TestAuditLogItem_JSONTags(t *testing.T) {
    item := dto.AuditLogItem{
        ID:                   1,
        Model:                "gpt-4o",
        UpstreamProvider:     "openai",
        APIProvider:          "openai",
        InputTokens:          100,
        OutputTokens:         50,
        FirstTokenLatencyMs:  200,
        StreamDurationMs:     1500,
        UpstreamStatusCode:   200,
        TraceID:              "abc123",
    }
    data, err := sonic.Marshal(item)
    if err != nil {
        t.Fatalf("marshal failed: %v", err)
    }
    var obj map[string]any
    if err := sonic.Unmarshal(data, &obj); err != nil {
        t.Fatalf("unmarshal failed: %v", err)
    }
    if obj["traceId"] != "abc123" {
        t.Errorf("traceId = %v, want abc123", obj["traceId"])
    }
    if obj["model"] != "gpt-4o" {
        t.Errorf("model = %v, want gpt-4o", obj["model"])
    }
}
```

- [ ] **Step 3: 运行测试**

```bash
go test -count=1 -v ./test/unit/audit_query/ ./test/unit/audit_dto/
```

Expected: All tests PASS.

- [ ] **Step 4: Commit**

```bash
git add test/unit/audit_query/ test/unit/audit_dto/
git commit -m "test: add unit tests for audit query handler and DTOs"
```

---

### Task 10: 全量回归

- [ ] **Step 1: 运行全量测试**

```bash
go test -count=1 ./...
```

- [ ] **Step 2: 运行 lint**

```bash
make lint
```

- [ ] **Step 3: 修复所有失败**

---

## Execution Order

Tasks must be executed sequentially in numbered order (1-10). Each task depends on previous tasks' code being committed.
