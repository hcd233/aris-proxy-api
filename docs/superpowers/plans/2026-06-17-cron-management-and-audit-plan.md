# Implementation Plan: Cron Management & Cron Call Audit

**2026-06-17**

## Overview

Add cron job management (list + toggle enabled/disabled) and cron call audit (list with type/status filter) to both API and Web. Rename existing "Audit" nav tab to "Model Call Audit", move its path to `/model-call-audit/`, add "Cron" and "Cron Call Audit" tabs.

---

## Step 1 — Constants & Field Names

### `internal/common/constant/string.go`

Add tags and field names:

```go
TagCron      = "Cron"
TagCronAudit = "CronAudit"

FieldName     = "name"
FieldSpec     = "spec"
FieldEnabled  = "enabled"
FieldCronName = "cron_name"
FieldTraceID  = "trace_id"
FieldDurationMs = "duration_ms"
FieldErrorMessage = "message"
FieldStartedAt = "started_at"
FieldEndedAt   = "ended_at"
```

### `internal/common/constant/cron.go`

Add cron job/audit constants:

```go
CronModuleSessionDeduplicate = "SessionDeduplicateCron"
CronSpecSessionDeduplicate  = "*/5 * * * *"
CronModuleNameCronManagement = "cron_cronjob"

CronAuditFilterFieldType   = "type"
CronAuditFilterFieldStatus = "status"

CronCallAuditStatusSuccess = "success"
CronCallAuditStatusFailed  = "failed"
CronCallAuditStatusPanic   = "panic"
CronCallAuditStatusSkipped = "skipped"

CronAuditFilterTypeSQLColumn   = "cron_name"
CronAuditFilterStatusSQLColumn = "status"
```

### `internal/common/constant/sql.go` (new file, or use existing)

Add field names for GORM:

```go
FieldTableCronJob       = "cron_jobs"
FieldTableCronCallAudit = "cron_call_audits"
```

---

## Step 2 — Database Models

### `internal/infrastructure/database/model/cron.go` (new)

```go
package model

import "time"

type CronJob struct {
    Name        string    `gorm:"column:name;primaryKey"`
    Spec        string    `gorm:"column:spec;not null"`
    Description string    `gorm:"column:description"`
    Enabled     bool      `gorm:"column:enabled;default:true"`
    CreatedAt   time.Time `gorm:"autoCreateTime"`
    UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

func (CronJob) TableName() string { return "cron_jobs" }

type CronCallAudit struct {
    BaseModel
    CronName   string    `gorm:"column:cron_name;not null;index"`
    TraceID    string    `gorm:"column:trace_id"`
    StartedAt  time.Time `gorm:"column:started_at;not null"`
    EndedAt    time.Time `gorm:"column:ended_at"`
    DurationMs int64     `gorm:"column:duration_ms"`
    Status     string    `gorm:"column:status;not null"`
    Message    string    `gorm:"column:message"`
}

func (CronCallAudit) TableName() string { return "cron_call_audits" }
```

### DAO

`internal/infrastructure/database/dao/cron.go` (new):

```go
package dao

import dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"

type CronJobDAO struct{ baseDAO[dbmodel.CronJob] }
type CronCallAuditDAO struct{ baseDAO[dbmodel.CronCallAudit] }
```

In `internal/infrastructure/database/dao/singleton.go`, add singletons and getters.

---

## Step 3 — Application Ports

### `internal/application/cronmgmt/port/handler.go` (new)

```go
package port

import (
    "context"
    "time"
    "github.com/hcd233/aris-proxy-api/internal/common/model"
)

type CronJobView struct {
    Name        string
    Spec        string
    Description string
    Enabled     bool
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type ListCronJobsHandler interface {
    Handle(ctx context.Context, param model.CommonParam) ([]*CronJobView, *model.PageInfo, error)
}

type UpdateCronJobHandler interface {
    Handle(ctx context.Context, name string, enabled bool) error
}

type CronJobRepository interface {
    Sync(ctx context.Context, jobs []*CronJobView) error
    List(ctx context.Context, param dao.CommonParam) ([]*CronJobView, *model.PageInfo, error)
    Update(ctx context.Context, name string, enabled bool) error
    Get(ctx context.Context, name string) (*CronJobView, error)
}
```

### `internal/application/cronaudit/port/handler.go` (new)

```go
package port

import (
    "context"
    "time"
    "github.com/hcd233/aris-proxy-api/internal/common/model"
)

type CronCallAuditView struct {
    ID         uint
    CronName   string
    TraceID    string
    StartedAt  time.Time
    EndedAt    time.Time
    DurationMs int64
    Status     string
    Message    string
    CreatedAt  time.Time
}

type ListCronCallAuditsHandler interface {
    Handle(ctx context.Context, param model.CommonParam, startTime, endTime time.Time, filter string) ([]*CronCallAuditView, *model.PageInfo, error)
}

type ListCronCallAuditOptionsHandler interface {
    Handle(ctx context.Context, field, keyword string, startTime, endTime time.Time) ([]string, error)
}

type CronCallAuditRepository interface {
    Save(ctx context.Context, audit *CronCallAuditView) error
    List(ctx context.Context, param dao.CommonParam, startTime, endTime time.Time, filterExp string) ([]*CronCallAuditView, *model.PageInfo, error)
    ListDistinctTypes(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error)
}
```

---

## Step 4 — Application Handlers

### `internal/application/cronmgmt/query/list_cron_jobs.go` (new)

```go
package query

import (
    "context"
    "github.com/hcd233/aris-proxy-api/internal/application/cronmgmt/port"
    "github.com/hcd233/aris-proxy-api/internal/common/model"
)

type listCronJobsHandler struct{ repo port.CronJobRepository }

func NewListCronJobsHandler(repo port.CronJobRepository) port.ListCronJobsHandler {
    return &listCronJobsHandler{repo}
}

func (h *listCronJobsHandler) Handle(ctx context.Context, param model.CommonParam) ([]*port.CronJobView, *model.PageInfo, error) {
    daoParam := dao.CommonParam{
        PageParam:   dao.PageParam{Page: param.Page, PageSize: param.PageSize},
        QueryParam:  dao.QueryParam{Query: param.Query, QueryFields: []string{constant.FieldName, constant.FieldSpec}},
        SortParam:   dao.SortParam{Sort: param.Sort, SortField: param.SortField},
    }
    return h.repo.List(ctx, daoParam)
}
```

### `internal/application/cronmgmt/command/update_cron_job.go` (new)

```go
package command

import (
    "context"
    "github.com/hcd233/aris-proxy-api/internal/application/cronmgmt/port"
)

type updateCronJobHandler struct{ repo port.CronJobRepository }

func NewUpdateCronJobHandler(repo port.CronJobRepository) port.UpdateCronJobHandler {
    return &updateCronJobHandler{repo}
}

func (h *updateCronJobHandler) Handle(ctx context.Context, name string, enabled bool) error {
    return h.repo.Update(ctx, name, enabled)
}
```

### `internal/application/cronaudit/query/list_cron_call_audits.go` (new)

```go
package query

import (
    "context"
    "time"
    "github.com/hcd233/aris-proxy-api/internal/application/cronaudit/port"
    "github.com/hcd233/aris-proxy-api/internal/common/constant"
    "github.com/hcd233/aris-proxy-api/internal/common/filter"
    "github.com/hcd233/aris-proxy-api/internal/common/model"
    "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
)

var cronAuditFieldConfigs = map[string]filter.FieldConfig{
    constant.CronAuditFilterFieldType:   {SQLColumn: constant.CronAuditFilterTypeSQLColumn},
    constant.CronAuditFilterFieldStatus: {SQLColumn: constant.CronAuditFilterStatusSQLColumn},
}

type listCronCallAuditsHandler struct{ repo port.CronCallAuditRepository }

func NewListCronCallAuditsHandler(repo port.CronCallAuditRepository) port.ListCronCallAuditsHandler {
    return &listCronCallAuditsHandler{repo}
}

func (h *listCronCallAuditsHandler) Handle(ctx context.Context, param model.CommonParam, startTime, endTime time.Time, filterStr string) ([]*port.CronCallAuditView, *model.PageInfo, error) {
    daoParam := dao.CommonParam{
        PageParam:  dao.PageParam{Page: param.Page, PageSize: param.PageSize},
        SortParam:  dao.SortParam{Sort: param.Sort, SortField: param.SortField},
        QueryParam: dao.QueryParam{Query: param.Query, QueryFields: []string{constant.FieldCronName, constant.FieldTraceID}},
    }
    return h.repo.List(ctx, daoParam, startTime, endTime, filterStr)
}
```

### `internal/application/cronaudit/query/option_list.go` (new)

```go
package query

import (
    "context"
    "time"
    "github.com/hcd233/aris-proxy-api/internal/application/cronaudit/port"
    "github.com/hcd233/aris-proxy-api/internal/common/constant"
)

type listCronCallAuditOptionsHandler struct{ repo port.CronCallAuditRepository }

func NewListCronCallAuditOptionsHandler(repo port.CronCallAuditRepository) port.ListCronCallAuditOptionsHandler {
    return &listCronCallAuditOptionsHandler{repo}
}

func (h *listCronCallAuditOptionsHandler) Handle(ctx context.Context, field, keyword string, startTime, endTime time.Time) ([]string, error) {
    switch field {
    case constant.CronAuditFilterFieldType:
        return h.repo.ListDistinctTypes(ctx, keyword, startTime, endTime)
    default:
        return []string{}, nil
    }
}
```

---

## Step 5 — Repository Implementation

### `internal/infrastructure/repository/cron_repository.go` (new)

```go
package repository

import (
    "context"
    "errors"
    "time"
    "github.com/hcd233/aris-proxy-api/internal/application/cronmgmt/port"
    "github.com/hcd233/aris-proxy-api/internal/common/constant"
    "github.com/hcd233/aris-proxy-api/internal/common/filter"
    "github.com/hcd233/aris-proxy-api/internal/common/ierr"
    "github.com/hcd233/aris-proxy-api/internal/common/model"
    "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
    dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
    "gorm.io/gorm"
    "gorm.io/gorm/clause"
)

type cronRepository struct {
    db  *gorm.DB
    dao *dao.CronJobDAO
}

func NewCronRepository(db *gorm.DB) port.CronJobRepository {
    return &cronRepository{db: db, dao: dao.NewCronJobDAO()}
}

func (r *cronRepository) Sync(ctx context.Context, jobs []*port.CronJobView) error {
    return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        for _, job := range jobs {
            existing := dbmodel.CronJob{}
            err := tx.Where("name = ?", job.Name).First(&existing).Error
            if err != nil {
                if !errors.Is(err, gorm.ErrRecordNotFound) {
                    return ierr.Wrap(ierr.ErrDBQuery, err, "query cron job")
                }
                if err := tx.Create(&dbmodel.CronJob{
                    Name: job.Name, Spec: job.Spec, Description: job.Description, Enabled: true,
                }).Error; err != nil {
                    return ierr.Wrap(ierr.ErrDBCreate, err, "create cron job")
                }
                continue
            }
            if existing.Spec != job.Spec || existing.Description != job.Description {
                if err := tx.Model(&existing).Updates(map[string]any{
                    constant.FieldSpec: job.Spec, constant.FieldDescription: job.Description,
                }).Error; err != nil {
                    return ierr.Wrap(ierr.ErrDBQuery, err, "update cron job spec")
                }
            }
        }
        return nil
    })
}

func (r *cronRepository) List(ctx context.Context, param dao.CommonParam) ([]*port.CronJobView, *model.PageInfo, error) {
    var rows []dbmodel.CronJob
    pageInfo, err := r.dao.Paginate(r.db.WithContext(ctx), param, &rows)
    if err != nil {
        return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "list cron jobs")
    }
    views := lo.Map(rows, func(row dbmodel.CronJob, _ int) *port.CronJobView {
        return &port.CronJobView{
            Name: row.Name, Spec: row.Spec, Description: row.Description,
            Enabled: row.Enabled, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
        }
    })
    return views, pageInfo, nil
}

func (r *cronRepository) Update(ctx context.Context, name string, enabled bool) error {
    result := r.db.WithContext(ctx).Model(&dbmodel.CronJob{}).
        Where("name = ?", name).
        Update(constant.FieldEnabled, enabled)
    if result.Error != nil {
        return ierr.Wrap(ierr.ErrDBQuery, result.Error, "update cron job")
    }
    if result.RowsAffected == 0 {
        return ierr.New(ierr.ErrNotFound, "cron job not found: "+name)
    }
    return nil
}

func (r *cronRepository) Get(ctx context.Context, name string) (*port.CronJobView, error) {
    row := dbmodel.CronJob{}
    err := r.db.WithContext(ctx).Where("name = ?", name).First(&row).Error
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, ierr.New(ierr.ErrNotFound, "cron job not found: "+name)
        }
        return nil, ierr.Wrap(ierr.ErrDBQuery, err, "get cron job")
    }
    return &port.CronJobView{
        Name: row.Name, Spec: row.Spec, Description: row.Description,
        Enabled: row.Enabled, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
    }, nil
}
```

### `internal/infrastructure/repository/cron_audit_repository.go` (new)

Similar to `audit_repository.go`, implementing `CronCallAuditRepository`:

- `Save`: Insert row with all fields using DAO.
- `List`: Join pagination with filter parsing, time range, keyword search.
- `ListDistinctTypes`: `SELECT DISTINCT cron_name FROM cron_call_audits WHERE deleted_at = 0 AND ... ORDER BY cron_name LIMIT 50`.

---

## Step 6 — Cron Runtime Integration

### Modify `internal/cron/cron.go`

1. Add `Spec` and `Description` fields to `CronRegistryEntry`.
2. Change `InitCronJobs` to accept stores, sync cron jobs on startup, write audit on each run.

```go
type CronJobStore interface {
    Sync(ctx context.Context, jobs []*port.CronJobView) error
    IsEnabled(ctx context.Context, name string) (bool, error)
}

type CronCallAuditStore interface {
    Save(ctx context.Context, audit *port.CronCallAuditView) error
}
```

Add `WrapCronFunc(db, store) func()` that:
- Queries `cron_jobs.enabled` before running
- Records start/end/duration/status/message after run
- Writes audit via store

Modify `InitCronJobs` to call store.Sync before starting crons, and wrap each cron's factory with `WrapCronFunc`.

---

## Step 7 — DTOs

### `internal/dto/cron.go` (new)

```go
package dto

import (
    "time"
    "github.com/hcd233/aris-proxy-api/internal/common/enum"
    "github.com/hcd233/aris-proxy-api/internal/common/model"
)

type ListCronJobsReq struct {
    Page      int       `query:"page" required:"true" minimum:"1"`
    PageSize  int       `query:"pageSize" required:"true" minimum:"1" maximum:"100"`
    Query     string    `query:"query" maxLength:"100"`
    Sort      enum.Sort `query:"sort" enum:"asc,desc"`
    SortField string    `query:"sortField" maxLength:"50"`
}

type ListCronJobsRsp struct {
    CommonRsp
    Jobs     []*CronJobItem `json:"jobs,omitempty"`
    PageInfo *model.PageInfo `json:"pageInfo,omitempty"`
}

type CronJobItem struct {
    Name        string    `json:"name"`
    Spec        string    `json:"spec"`
    Description string    `json:"description"`
    Enabled     bool      `json:"enabled"`
    CreatedAt   time.Time `json:"createdAt"`
    UpdatedAt   time.Time `json:"updatedAt"`
}

type UpdateCronJobReq struct {
    Name    string `json:"name" required:"true" maxLength:"100"`
    Enabled bool   `json:"enabled"`
}

type UpdateCronJobRsp struct {
    CommonRsp
}

type ListCronCallAuditsReq struct {
    Page      int       `query:"page" required:"true" minimum:"1"`
    PageSize  int       `query:"pageSize" required:"true" minimum:"1" maximum:"100"`
    Query     string    `query:"query" maxLength:"100"`
    Sort      enum.Sort `query:"sort" enum:"asc,desc"`
    SortField string    `query:"sortField" maxLength:"50"`
    StartTime time.Time `query:"startTime"`
    EndTime   time.Time `query:"endTime"`
    Filter    string    `query:"filter" maxLength:"500"`
}

type ListCronCallAuditsRsp struct {
    CommonRsp
    Logs     []*CronCallAuditItem `json:"logs,omitempty"`
    PageInfo *model.PageInfo      `json:"pageInfo,omitempty"`
}

type CronCallAuditItem struct {
    ID         uint      `json:"id"`
    CronName   string    `json:"cronName"`
    TraceID    string    `json:"traceId"`
    StartedAt  time.Time `json:"startedAt"`
    EndedAt    time.Time `json:"endedAt"`
    DurationMs int64     `json:"durationMs"`
    Status     string    `json:"status"`
    Message    string    `json:"message"`
    CreatedAt  time.Time `json:"createdAt"`
}

type CronCallAuditOptionListReq struct {
    Field      string    `query:"field" required:"true" enum:"type"` // only type for now
    Keyword    string    `query:"keyword" maxLength:"100"`
    StartTime  time.Time `query:"startTime"`
    EndTime    time.Time `query:"endTime"`
}

type CronCallAuditOptionListRsp struct {
    CommonRsp
    Items []string `json:"items,omitempty"`
}
```

---

## Step 8 — Handlers

### `internal/handler/cron.go` (new)

```go
package handler

import (
    "context"
    "github.com/samber/lo"
    "go.uber.org/zap"
    apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
    cronmgmtcommand "github.com/hcd233/aris-proxy-api/internal/application/cronmgmt/command"
    cronmgmtport "github.com/hcd233/aris-proxy-api/internal/application/cronmgmt/port"
    cronauditport "github.com/hcd233/aris-proxy-api/internal/application/cronaudit/port"
    "github.com/hcd233/aris-proxy-api/internal/common/ierr"
    "github.com/hcd233/aris-proxy-api/internal/dto"
    "github.com/hcd233/aris-proxy-api/internal/logger"
)

type CronHandler interface {
    HandleListCronJobs(ctx context.Context, req *dto.ListCronJobsReq) (*dto.HTTPResponse[*dto.ListCronJobsRsp], error)
    HandleUpdateCronJob(ctx context.Context, req *dto.UpdateCronJobReq) (*dto.HTTPResponse[*dto.UpdateCronJobRsp], error)
    HandleListCronCallAudits(ctx context.Context, req *dto.ListCronCallAuditsReq) (*dto.HTTPResponse[*dto.ListCronCallAuditsRsp], error)
    HandleListCronCallAuditOptions(ctx context.Context, req *dto.CronCallAuditOptionListReq) (*dto.HTTPResponse[*dto.CronCallAuditOptionListRsp], error)
}

type CronDependencies struct {
    ListCronJobs    cronmgmtport.ListCronJobsHandler
    UpdateCronJob   cronmgmtport.UpdateCronJobHandler
    ListCronCallAudits      cronauditport.ListCronCallAuditsHandler
    ListCronCallAuditOptions cronauditport.ListCronCallAuditOptionsHandler
}

type cronHandler struct {
    listCronJobs           cronmgmtport.ListCronJobsHandler
    updateCronJob          cronmgmtport.UpdateCronJobHandler
    listCronCallAudits     cronauditport.ListCronCallAuditsHandler
    listCronCallAuditOpts  cronauditport.ListCronCallAuditOptionsHandler
}

func NewCronHandler(deps CronDependencies) CronHandler {
    return &cronHandler{
        listCronJobs:          deps.ListCronJobs,
        updateCronJob:         deps.UpdateCronJob,
        listCronCallAudits:    deps.ListCronCallAudits,
        listCronCallAuditOpts: deps.ListCronCallAuditOptions,
    }
}

func (h *cronHandler) HandleListCronJobs(ctx context.Context, req *dto.ListCronJobsReq) (*dto.HTTPResponse[*dto.ListCronJobsRsp], error) {
    // standard pattern...
}

func (h *cronHandler) HandleUpdateCronJob(ctx context.Context, req *dto.UpdateCronJobReq) (*dto.HTTPResponse[*dto.UpdateCronJobRsp], error) {
    // standard pattern...
}

func (h *cronHandler) HandleListCronCallAudits(ctx context.Context, req *dto.ListCronCallAuditsReq) (*dto.HTTPResponse[*dto.ListCronCallAuditsRsp], error) {
    // uses model.CommonParam, calls usecase...
}

func (h *cronHandler) HandleListCronCallAuditOptions(ctx context.Context, req *dto.CronCallAuditOptionListReq) (*dto.HTTPResponse[*dto.CronCallAuditOptionListRsp], error) {
    // delegate to usecase...
}
```

Full handler implementations mirror the `auditHandler` pattern exactly.

---

## Step 9 — Router

### `internal/router/cron.go` (new)

```go
package router

import (
    "net/http"
    "github.com/danielgtaylor/huma/v2"
    "github.com/hcd233/aris-proxy-api/internal/common/constant"
    "github.com/hcd233/aris-proxy-api/internal/common/enum"
    "github.com/hcd233/aris-proxy-api/internal/handler"
    "github.com/hcd233/aris-proxy-api/internal/infrastructure/jwt"
    "github.com/hcd233/aris-proxy-api/internal/middleware"
    "github.com/redis/go-redis/v9"
    "gorm.io/gorm"
)

func initCronRouter(cronGroup huma.API, cronHandler handler.CronHandler, db *gorm.DB, cache *redis.Client, accessSigner jwt.TokenSigner) {
    cronGroup.UseMiddleware(middleware.JwtMiddleware(db, cache, accessSigner))

    huma.Register(cronGroup, huma.Operation{
        OperationID: "listCronJobs",
        Method:      http.MethodGet,
        Path:        "/list",
        Summary:     "ListCronJobs",
        Description: "List all cron jobs with their enabled status",
        Tags:        []string{constant.TagCron},
        Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
        Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listCronJobs", enum.PermissionAdmin)},
    }, cronHandler.HandleListCronJobs)

    huma.Register(cronGroup, huma.Operation{
        OperationID: "updateCronJob",
        Method:      http.MethodPatch,
        Path:        "/{name}",
        Summary:     "UpdateCronJob",
        Description: "Enable or disable a cron job",
        Tags:        []string{constant.TagCron},
        Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
        Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("updateCronJob", enum.PermissionAdmin)},
    }, cronHandler.HandleUpdateCronJob)
}

func initCronAuditRouter(auditGroup huma.API, cronHandler handler.CronHandler, db *gorm.DB, cache *redis.Client, accessSigner jwt.TokenSigner) {
    auditGroup.UseMiddleware(middleware.JwtMiddleware(db, cache, accessSigner))

    huma.Register(auditGroup, huma.Operation{
        OperationID: "listCronCallAudits",
        Method:      http.MethodGet,
        Path:        "/log/list",
        Summary:     "ListCronCallAudits",
        Description: "Paginate cron call audit records",
        Tags:        []string{constant.TagCronAudit},
        Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
        Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listCronCallAudits", enum.PermissionAdmin)},
    }, cronHandler.HandleListCronCallAudits)

    huma.Register(auditGroup, huma.Operation{
        OperationID: "listCronCallAuditOptions",
        Method:      http.MethodGet,
        Path:        "/option/list",
        Summary:     "ListCronCallAuditOptions",
        Description: "Get available filter options for cron call audit (cron type)",
        Tags:        []string{constant.TagCronAudit},
        Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
        Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listCronCallAuditOptions", enum.PermissionAdmin)},
    }, cronHandler.HandleListCronCallAuditOptions)
}
```

### Modify `internal/router/router.go`

Add to `APIRouterDependencies`:
```go
CronHandler     handler.CronHandler
```

In `RegisterAPIRouter`:
```go
cronGroup := huma.NewGroup(v1Group, "/cron")
initCronRouter(cronGroup, deps.CronHandler, deps.DB, deps.Cache, deps.AccessSigner)

cronAuditGroup := huma.NewGroup(v1Group, "/cron-audit")
initCronAuditRouter(cronAuditGroup, deps.CronHandler, deps.DB, deps.Cache, deps.AccessSigner)
```

---

## Step 10 — DI Wiring

### `internal/bootstrap/modules/repository.go`

Add:
```go
func NewCronRepository(db *gorm.DB) cronmgmtport.CronJobRepository {
    return repository.NewCronRepository(db)
}
func NewCronCallAuditRepository(db *gorm.DB) cronauditport.CronCallAuditRepository {
    return repository.NewCronCallAuditRepository(db)
}
```
Register in `RepositoryModule` Provide.

### `internal/bootstrap/modules/application.go`

Add `NewListCronJobsHandler`, `NewUpdateCronJobHandler`, `NewListCronCallAuditsHandler`, `NewListCronCallAuditOptionsHandler` connector functions. Register in `ApplicationModule`.

### `internal/bootstrap/modules/handler.go`

Add:
```go
func NewCronDependencies(
    listJobs cronmgmtport.ListCronJobsHandler,
    updateJob cronmgmtport.UpdateCronJobHandler,
    listAudits cronauditport.ListCronCallAuditsHandler,
    listAuditOpts cronauditport.ListCronCallAuditOptionsHandler,
) handler.CronDependencies {
    return handler.CronDependencies{...}
}
```
Register in `HandlerModule`.

---

## Step 11 — Model Call Audit Path Migration (API)

Add a Fiber 308 redirect route in `router/registerRoutes` (in `internal/bootstrap/container.go` or a dedicated router file):

```go
app.Get("/api/v1/audit/redirect", func(c fiber.Ctx) error {
    return c.Redirect().To("/api/v1/model-call-audit/")
})
```

We don't need API-level redirect for `/api/v1/audit/` → the web frontend just changes its path. The old API endpoints remain at `/api/v1/audit/`. The path migration is purely at the web routing level.

---

## Step 12 — Web Types & API Client

### `web/src/lib/types.ts`

Add types:
```ts
// ─── Cron ───────────────────────────────────────────────────
export interface CronJobItem {
  name: string;
  spec: string;
  description: string;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface ListCronJobsRsp extends CommonRsp {
  jobs?: CronJobItem[];
  pageInfo?: PageInfo;
}

export interface UpdateCronJobReqBody {
  name: string;
  enabled: boolean;
}

// ─── Cron Call Audit ────────────────────────────────────────
export interface CronCallAuditItem {
  id: number;
  cronName: string;
  traceId: string;
  startedAt: string;
  endedAt: string;
  durationMs: number;
  status: string;
  message: string;
  createdAt: string;
}

export interface ListCronCallAuditsRsp extends CommonRsp {
  logs?: CronCallAuditItem[];
  pageInfo?: PageInfo;
}

export interface CronCallAuditOptionListReq {
  field: "type";
  keyword?: string;
  startTime?: string;
  endTime?: string;
}

export interface CronCallAuditOptionListRsp extends CommonRsp {
  items: string[];
}
```

### `web/src/lib/api-client.ts`

Add methods:
```ts
async listCronJobs(params: { page: number; pageSize: number; query?: string; sort?: string; sortField?: string }): Promise<ListCronJobsRsp> { ... }
async updateCronJob(body: UpdateCronJobReqBody): Promise<CommonRsp> { ... }
async listCronCallAudits(params: { page: number; pageSize: number; query?: string; sort?: string; sortField?: string; startTime?: string; endTime?: string; filter?: string }): Promise<ListCronCallAuditsRsp> { ... }
async listCronCallAuditOptions(params: CronCallAuditOptionListReq): Promise<CronCallAuditOptionListRsp> { ... }
```

---

## Step 13 — Web Pages

### 13.1 Rename Audit → Model Call Audit, add Cron & Cron Audit tabs

#### `web/src/app/(dashboard)/layout.tsx`

Change navItems:
- Rename `label: "Audit"` → `label: "Model Call Audit"`, `href: "/audit/"` → `href: "/model-call-audit/"`
- Add before Profile:
```ts
  {
    label: "Cron",
    href: "/cron/",
    icon: <Timer className="size-4" />,
    adminOnly: true,
  },
  {
    label: "Cron Audit",
    href: "/cron-audit/",
    icon: <ScrollText className="size-4" />,
    adminOnly: true,
  },
```

Add `Timer` (from `lucide-react`) and `Clock` imports. `Timer` already in lucide.

#### 13.2 Move audit page to `/model-call-audit/`

Copy `web/src/app/(dashboard)/audit/` → `web/src/app/(dashboard)/model-call-audit/`. Add a redirect component at the old path.

Create `web/src/app/(dashboard)/audit/page.tsx` that does client-side redirect:
```tsx
"use client";
import { useEffect } from "react";
import { useRouter } from "next/navigation";

export default function AuditRedirectPage() {
  const router = useRouter();
  useEffect(() => { router.replace("/web/model-call-audit/"); }, [router]);
  return null;
}
```

#### 13.3 Create Cron page

`web/src/app/(dashboard)/cron/page.tsx`:

Simple table showing cron jobs with name, spec, description, enabled toggle switch. Use `api.listCronJobs` and `api.updateCronJob`. Admin-only.

#### 13.4 Create Cron Audit page

`web/src/app/(dashboard)/cron-audit/page.tsx`:

Similar structure to the model call audit page but with columns: Time, Cron Name, Trace ID, Status (badge), Duration (ms), Error Message. Paginated with same time range picker and type/status filter.

---

## Step 14 — Database Migration

Add a migration script or manual SQL. Since the project doesn't have an auto-migrate, add to `cmd/server.go` startup or `container.go`:

```go
if err := db.AutoMigrate(&model.CronJob{}, &model.CronCallAudit{}); err != nil {
    log.Fatal("failed to migrate cron tables")
}
```

Add to the existing migration block.

---

## Step 15 — Tests

### `test/unit/cron/cron_repository_test.go`
Test Sync, List, Update, Get with in-memory SQLite.

### `test/unit/cron/cron_audit_repository_test.go`
Test Save, List with filter/time range, ListDistinctTypes.

### `test/unit/cron/application_cronmgmt_test.go`
Test list/update handlers with mock repo.

### `test/unit/cron/application_cronaudit_test.go`
Test list/option handlers with mock repo.

### `test/e2e/cron/cron_management_test.go`
E2E test calling API with JWT token to list/update cron jobs.

### Update existing cron tests
`test/unit/cron/cron_test.go` needs `Spec` and `Description` fields added to `CronRegistryEntry` in test registries.

---

## Verification Plan

1. `make lint` - No new lint errors.
2. `make build` - Binary compiles.
3. `go test -count=1 ./test/unit/cron/...` - New unit tests pass.
4. `cd web && npm run build` - Frontend builds without errors.
5. (Optional) Start server, call APIs via curl to verify.
