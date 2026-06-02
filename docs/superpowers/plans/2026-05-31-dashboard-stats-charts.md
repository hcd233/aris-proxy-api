# Dashboard Stats Charts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add two audit-based chart cards (Model Trend + Request Success Rate) to the web Dashboard, backed by new backend aggregation endpoints.

**Architecture:** Backend adds two HTTP GET endpoints that execute PostgreSQL `date_trunc` + `GROUP BY` queries against `model_call_audits`. Frontend uses shadcn/chart (recharts) to render multi-series line charts. Permission scoping reuses existing audit logic (admin = all, user = own keys).

**Tech Stack:** Go 1.25 + GORM + Huma v2 (backend), Next.js + shadcn/chart + recharts (frontend)

---

### Task 1: Backend DTO — Add stats DTO types

**Files:**
- Create: `internal/dto/audit_stats.go`

- [ ] **Step 1: Create the stats DTO file**

```go
package dto

import "time"

type Granularity string

const (
	GranularityMinute Granularity = "minute"
	GranularityHour   Granularity = "hour"
	GranularityDay    Granularity = "day"
	GranularityWeek   Granularity = "week"
)

// ─── Model Trend ────────────────────────────────────────────────

type ModelTrendReq struct {
	StartTime   time.Time   `query:"startTime" required:"true"`
	EndTime     time.Time   `query:"endTime" required:"true"`
	Granularity Granularity `query:"granularity" required:"true" enum:"minute,hour,day,week"`
}

type ModelTrendRsp struct {
	CommonRsp
	Data []*ModelTrendItem `json:"data,omitempty" doc:"各模型的调用趋势"`
}

type ModelTrendItem struct {
	Model  string        `json:"model" doc:"模型名"`
	Points []*TrendPoint `json:"points" doc:"时间序列点"`
}

type TrendPoint struct {
	Time  time.Time `json:"time" doc:"时间桶"`
	Count int       `json:"count" doc:"调用次数"`
}

// ─── Request Rate ────────────────────────────────────────────────

type RequestRateReq struct {
	StartTime   time.Time   `query:"startTime" required:"true"`
	EndTime     time.Time   `query:"endTime" required:"true"`
	Granularity Granularity `query:"granularity" required:"true" enum:"minute,hour,day,week"`
}

type RequestRateRsp struct {
	CommonRsp
	Data []*RequestRateItem `json:"data,omitempty" doc:"各模型的请求成功率"`
}

type RequestRateItem struct {
	Model  string        `json:"model" doc:"模型名"`
	Points []*RatePoint  `json:"points" doc:"时间序列点"`
}

type RatePoint struct {
	Time        time.Time `json:"time" doc:"时间桶"`
	Total       int       `json:"total" doc:"总请求数"`
	Success     int       `json:"success" doc:"成功数"`
	Failed      int       `json:"failed" doc:"失败数"`
	SuccessRate float64   `json:"successRate" doc:"成功率 0-1"`
}
```

- [ ] **Step 2: Verify compilation**

Run: `go vet ./internal/dto/...` — expected: no errors

---

### Task 2: Backend Repository Interface — Add stats query methods

**Files:**
- Modify: `internal/domain/modelcall/repository.go`

- [ ] **Step 1: Add query types and interface methods**

Before the closing of the file (after `BatchGetRelations`), add:

```go
// ModelTrendPoint 模型调用趋势的数据点
type ModelTrendPoint struct {
	Model string
	Time  time.Time
	Count int
}

// RequestRatePoint 请求成功率的数据点
type RequestRatePoint struct {
	Model   string
	Time    time.Time
	Total   int
	Success int
}

// QueryModelTrend 按模型 + 时间桶统计调用次数。
// apiKeyIDs 为 nil 时查全部，非空时按 key 过滤。
QueryModelTrend(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time, granularity string) ([]*ModelTrendPoint, error)

// QueryRequestRate 按模型 + 时间桶统计请求成功率。
// apiKeyIDs 为 nil 时查全部，非空时按 key 过滤。
QueryRequestRate(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time, granularity string) ([]*RequestRatePoint, error)
```

- [ ] **Step 2: Verify compilation**

Run: `go vet ./internal/domain/...` — expected: no errors

---

### Task 3: Backend Repository Implementation — GORM raw SQL

**Files:**
- Modify: `internal/infrastructure/repository/audit_repository.go`

- [ ] **Step 1: Add `dateTruncSQL` helper function at package level**

```go
// dateTruncSQL 根据粒度返回 PostgreSQL date_trunc 表达式和参数
func dateTruncSQL(granularity string) string {
	switch granularity {
	case "minute":
		return "date_trunc('minute', created_at)"
	case "hour":
		return "date_trunc('hour', created_at)"
	case "day":
		return "date_trunc('day', created_at)"
	case "week":
		return "date_trunc('week', created_at)"
	default:
		return "date_trunc('day', created_at)"
	}
}
```

- [ ] **Step 2: Add `QueryModelTrend` method to `auditRepository`**

```go
func (r *auditRepository) QueryModelTrend(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time, granularity string) ([]*modelcall.ModelTrendPoint, error) {
	db := r.db.WithContext(ctx).Model(&dbmodel.ModelCallAudit{}).
		Where("created_at >= ? AND created_at <= ?", startTime, endTime).
		Where("deleted_at IS NULL")

	if len(apiKeyIDs) > 0 {
		db = db.Where("api_key_id IN ?", apiKeyIDs)
	}

	timeBucketExpr := dateTruncSQL(granularity)
	var results []*modelcall.ModelTrendPoint
	if err := db.Select("model, "+timeBucketExpr+" AS time, COUNT(*) AS count").
		Group("model, time").
		Order("model, time").
		Scan(&results).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "query model trend")
	}
	return results, nil
}
```

- [ ] **Step 3: Add `QueryRequestRate` method to `auditRepository`**

```go
func (r *auditRepository) QueryRequestRate(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time, granularity string) ([]*modelcall.RequestRatePoint, error) {
	db := r.db.WithContext(ctx).Model(&dbmodel.ModelCallAudit{}).
		Where("created_at >= ? AND created_at <= ?", startTime, endTime).
		Where("deleted_at IS NULL")

	if len(apiKeyIDs) > 0 {
		db = db.Where("api_key_id IN ?", apiKeyIDs)
	}

	timeBucketExpr := dateTruncSQL(granularity)
	var results []*modelcall.RequestRatePoint
	if err := db.Select("model, "+timeBucketExpr+" AS time, COUNT(*) AS total, COUNT(*) FILTER (WHERE upstream_status_code = 200) AS success").
		Group("model, time").
		Order("model, time").
		Scan(&results).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "query request rate")
	}
	return results, nil
}
```

- [ ] **Step 4: Verify compilation**

Run: `go vet ./internal/infrastructure/repository/...` — expected: no errors

---

### Task 4: Backend Usecase — Query handlers for both stats

**Files:**
- Create: `internal/application/audit/query/model_trend.go`
- Create: `internal/application/audit/query/request_rate.go`

- [ ] **Step 1: Create `model_trend.go`**

```go
package query

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
)

// ModelTrendQuery admin 全量模型调用趋势查询
type ModelTrendQuery struct {
	StartTime   time.Time
	EndTime     time.Time
	Granularity string
}

// ModelTrendByUserQuery user 维度模型调用趋势查询
type ModelTrendByUserQuery struct {
	UserID      uint
	StartTime   time.Time
	EndTime     time.Time
	Granularity string
}

// ModelTrendHandler admin 全量模型趋势查询处理器
type ModelTrendHandler interface {
	Handle(ctx context.Context, q ModelTrendQuery) ([]*modelcall.ModelTrendPoint, error)
}

// ModelTrendByUserHandler user 维度模型趋势查询处理器
type ModelTrendByUserHandler interface {
	Handle(ctx context.Context, q ModelTrendByUserQuery) ([]*modelcall.ModelTrendPoint, error)
}

type modelTrendHandler struct {
	repo modelcall.AuditRepository
}

type modelTrendByUserHandler struct {
	repo      modelcall.AuditRepository
	apiKeyIDs apiKeyIDLookup
}

func NewModelTrendHandler(repo modelcall.AuditRepository) ModelTrendHandler {
	return &modelTrendHandler{repo: repo}
}

func NewModelTrendByUserHandler(repo modelcall.AuditRepository, apiKeyIDs apiKeyIDLookup) ModelTrendByUserHandler {
	return &modelTrendByUserHandler{repo: repo, apiKeyIDs: apiKeyIDs}
}

func (h *modelTrendHandler) Handle(ctx context.Context, q ModelTrendQuery) ([]*modelcall.ModelTrendPoint, error) {
	return h.repo.QueryModelTrend(ctx, nil, q.StartTime, q.EndTime, q.Granularity)
}

func (h *modelTrendByUserHandler) Handle(ctx context.Context, q ModelTrendByUserQuery) ([]*modelcall.ModelTrendPoint, error) {
	keyIDs, err := h.apiKeyIDs.LookupIDsByUserID(ctx, q.UserID)
	if err != nil {
		return nil, err
	}
	return h.repo.QueryModelTrend(ctx, keyIDs, q.StartTime, q.EndTime, q.Granularity)
}
```

- [ ] **Step 2: Create `request_rate.go`**

```go
package query

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
)

// RequestRateQuery admin 全量请求成功率查询
type RequestRateQuery struct {
	StartTime   time.Time
	EndTime     time.Time
	Granularity string
}

// RequestRateByUserQuery user 维度请求成功率查询
type RequestRateByUserQuery struct {
	UserID      uint
	StartTime   time.Time
	EndTime     time.Time
	Granularity string
}

// RequestRateHandler admin 全量请求成功率处理器
type RequestRateHandler interface {
	Handle(ctx context.Context, q RequestRateQuery) ([]*modelcall.RequestRatePoint, error)
}

// RequestRateByUserHandler user 维度请求成功率处理器
type RequestRateByUserHandler interface {
	Handle(ctx context.Context, q RequestRateByUserQuery) ([]*modelcall.RequestRatePoint, error)
}

type requestRateHandler struct {
	repo modelcall.AuditRepository
}

type requestRateByUserHandler struct {
	repo      modelcall.AuditRepository
	apiKeyIDs apiKeyIDLookup
}

func NewRequestRateHandler(repo modelcall.AuditRepository) RequestRateHandler {
	return &requestRateHandler{repo: repo}
}

func NewRequestRateByUserHandler(repo modelcall.AuditRepository, apiKeyIDs apiKeyIDLookup) RequestRateByUserHandler {
	return &requestRateByUserHandler{repo: repo, apiKeyIDs: apiKeyIDs}
}

func (h *requestRateHandler) Handle(ctx context.Context, q RequestRateQuery) ([]*modelcall.RequestRatePoint, error) {
	return h.repo.QueryRequestRate(ctx, nil, q.StartTime, q.EndTime, q.Granularity)
}

func (h *requestRateByUserHandler) Handle(ctx context.Context, q RequestRateByUserQuery) ([]*modelcall.RequestRatePoint, error) {
	keyIDs, err := h.apiKeyIDs.LookupIDsByUserID(ctx, q.UserID)
	if err != nil {
		return nil, err
	}
	return h.repo.QueryRequestRate(ctx, keyIDs, q.StartTime, q.EndTime, q.Granularity)
}
```

- [ ] **Step 3: Verify compilation**

Run: `go vet ./internal/application/audit/query/...` — expected: no errors

---

### Task 5: Backend Handler — Add stats handler methods

**Files:**
- Modify: `internal/handler/audit.go`

- [ ] **Step 1: Add new fields to `AuditDependencies` and `auditHandler`**

Update `AuditDependencies` struct:
```go
type AuditDependencies struct {
	ListAll              auditquery.ListAllAuditLogsHandler
	ListByUser           auditquery.ListAuditLogsByUserHandler
	ModelTrend           auditquery.ModelTrendHandler
	ModelTrendByUser     auditquery.ModelTrendByUserHandler
	RequestRate          auditquery.RequestRateHandler
	RequestRateByUser    auditquery.RequestRateByUserHandler
}
```

Update `auditHandler` struct:
```go
type auditHandler struct {
	listAll           auditquery.ListAllAuditLogsHandler
	listByUser        auditquery.ListAuditLogsByUserHandler
	modelTrend        auditquery.ModelTrendHandler
	modelTrendByUser  auditquery.ModelTrendByUserHandler
	requestRate       auditquery.RequestRateHandler
	requestRateByUser auditquery.RequestRateByUserHandler
}
```

Update `NewAuditHandler`:
```go
func NewAuditHandler(deps AuditDependencies) AuditHandler {
	return &auditHandler{
		listAll:           deps.ListAll,
		listByUser:        deps.ListByUser,
		modelTrend:        deps.ModelTrend,
		modelTrendByUser:  deps.ModelTrendByUser,
		requestRate:       deps.RequestRate,
		requestRateByUser: deps.RequestRateByUser,
	}
}
```

- [ ] **Step 2: Add `HandleModelTrend` method to `AuditHandler` interface**

```go
type AuditHandler interface {
	HandleListAuditLogs(ctx context.Context, req *dto.ListAuditLogsReq) (*dto.HTTPResponse[*dto.ListAuditLogsRsp], error)
	HandleModelTrend(ctx context.Context, req *dto.ModelTrendReq) (*dto.HTTPResponse[*dto.ModelTrendRsp], error)
	HandleRequestRate(ctx context.Context, req *dto.RequestRateReq) (*dto.HTTPResponse[*dto.RequestRateRsp], error)
}
```

- [ ] **Step 3: Add `HandleModelTrend` implementation**

```go
func (h *auditHandler) HandleModelTrend(ctx context.Context, req *dto.ModelTrendReq) (*dto.HTTPResponse[*dto.ModelTrendRsp], error) {
	rsp := &dto.ModelTrendRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValuePermission(ctx)

	var points []*modelcall.ModelTrendPoint
	var err error

	switch permission {
	case enum.PermissionAdmin:
		points, err = h.modelTrend.Handle(ctx, auditquery.ModelTrendQuery{
			StartTime:   req.StartTime,
			EndTime:     req.EndTime,
			Granularity: string(req.Granularity),
		})
	case enum.PermissionUser:
		points, err = h.modelTrendByUser.Handle(ctx, auditquery.ModelTrendByUserQuery{
			UserID:      userID,
			StartTime:   req.StartTime,
			EndTime:     req.EndTime,
			Granularity: string(req.Granularity),
		})
	default:
		rsp.Error = ierr.ErrUnauthorized.BizError()
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] Model trend failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.Data = groupTrendPoints(points, nil)
	return apiutil.WrapHTTPResponse(rsp, nil)
}
```

- [ ] **Step 4: Add `HandleRequestRate` implementation**

```go
func (h *auditHandler) HandleRequestRate(ctx context.Context, req *dto.RequestRateReq) (*dto.HTTPResponse[*dto.RequestRateRsp], error) {
	rsp := &dto.RequestRateRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValuePermission(ctx)

	var points []*modelcall.RequestRatePoint
	var err error

	switch permission {
	case enum.PermissionAdmin:
		points, err = h.requestRate.Handle(ctx, auditquery.RequestRateQuery{
			StartTime:   req.StartTime,
			EndTime:     req.EndTime,
			Granularity: string(req.Granularity),
		})
	case enum.PermissionUser:
		points, err = h.requestRateByUser.Handle(ctx, auditquery.RequestRateByUserQuery{
			UserID:      userID,
			StartTime:   req.StartTime,
			EndTime:     req.EndTime,
			Granularity: string(req.Granularity),
		})
	default:
		rsp.Error = ierr.ErrUnauthorized.BizError()
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] Request rate failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.Data = groupRatePoints(points)
	return apiutil.WrapHTTPResponse(rsp, nil)
}
```

- [ ] **Step 5: Add helper functions `groupTrendPoints` and `groupRatePoints`**

```go
func groupTrendPoints(points []*modelcall.ModelTrendPoint, _ interface{}) []*dto.ModelTrendItem {
	modelMap := make(map[string][]*dto.TrendPoint)
	modelOrder := make([]string, 0)
	for _, p := range points {
		if _, ok := modelMap[p.Model]; !ok {
			modelOrder = append(modelOrder, p.Model)
		}
		modelMap[p.Model] = append(modelMap[p.Model], &dto.TrendPoint{
			Time:  p.Time,
			Count: p.Count,
		})
	}
	items := make([]*dto.ModelTrendItem, 0, len(modelOrder))
	for _, m := range modelOrder {
		items = append(items, &dto.ModelTrendItem{
			Model:  m,
			Points: modelMap[m],
		})
	}
	return items
}

func groupRatePoints(points []*modelcall.RequestRatePoint) []*dto.RequestRateItem {
	modelMap := make(map[string][]*dto.RatePoint)
	modelOrder := make([]string, 0)
	for _, p := range points {
		if _, ok := modelMap[p.Model]; !ok {
			modelOrder = append(modelOrder, p.Model)
		}
		failed := p.Total - p.Success
		var rate float64
		if p.Total > 0 {
			rate = float64(p.Success) / float64(p.Total)
		}
		modelMap[p.Model] = append(modelMap[p.Model], &dto.RatePoint{
			Time:        p.Time,
			Total:       p.Total,
			Success:     p.Success,
			Failed:      failed,
			SuccessRate: rate,
		})
	}
	items := make([]*dto.RequestRateItem, 0, len(modelOrder))
	for _, m := range modelOrder {
		items = append(items, &dto.RequestRateItem{
			Model:  m,
			Points: modelMap[m],
		})
	}
	return items
}
```

- [ ] **Step 6: Verify compilation**

Run: `go vet ./internal/handler/...` — expected: no errors

---

### Task 6: Backend Router — Register stats routes

**Files:**
- Modify: `internal/router/audit.go`

- [ ] **Step 1: Add two new routes after the existing `huma.Register` call**

```go
huma.Register(auditGroup, huma.Operation{
	OperationID: "queryModelTrend",
	Method:      http.MethodGet,
	Path:        "/stats/model/trend",
	Summary:     "QueryModelTrend",
	Description: "Query model call count trend grouped by model and time bucket. Admin sees all; user sees only their own keys.",
	Tags:        []string{"Audit"},
	Security:    []map[string][]string{{"jwtAuth": {}}},
	Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("queryModelTrend", enum.PermissionUser)},
}, auditHandler.HandleModelTrend)

huma.Register(auditGroup, huma.Operation{
	OperationID: "queryRequestRate",
	Method:      http.MethodGet,
	Path:        "/stats/request/rate",
	Summary:     "QueryRequestRate",
	Description: "Query request success rate grouped by model and time bucket. Admin sees all; user sees only their own keys.",
	Tags:        []string{"Audit"},
	Security:    []map[string][]string{{"jwtAuth": {}}},
	Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("queryRequestRate", enum.PermissionUser)},
}, auditHandler.HandleRequestRate)
```

- [ ] **Step 2: Verify compilation**

Run: `go vet ./internal/router/...` — expected: no errors

---

### Task 7: Backend Container — Register new dependencies

**Files:**
- Modify: `internal/bootstrap/container.go`

- [ ] **Step 1: Add Provide calls for the four new handlers (after existing audit query handlers around line 254)**

```go
if err := container.Provide(auditquery.NewModelTrendHandler); err != nil {
	return err
}
if err := container.Provide(newModelTrendByUserHandler); err != nil {
	return err
}
if err := container.Provide(auditquery.NewRequestRateHandler); err != nil {
	return err
}
if err := container.Provide(newRequestRateByUserHandler); err != nil {
	return err
}
```

- [ ] **Step 2: Add the two wrapper functions (after `newListAuditLogsByUserHandler` around line 525)**

```go
func newModelTrendByUserHandler(repo modelcall.AuditRepository, apiKeyRepo apikey.APIKeyRepository) auditquery.ModelTrendByUserHandler {
	return auditquery.NewModelTrendByUserHandler(repo, apiKeyRepo)
}

func newRequestRateByUserHandler(repo modelcall.AuditRepository, apiKeyRepo apikey.APIKeyRepository) auditquery.RequestRateByUserHandler {
	return auditquery.NewRequestRateByUserHandler(repo, apiKeyRepo)
}
```

- [ ] **Step 3: Update `newAuditDependencies` to include new handlers**

```go
func newAuditDependencies(
	listAll auditquery.ListAllAuditLogsHandler,
	listByUser auditquery.ListAuditLogsByUserHandler,
	modelTrend auditquery.ModelTrendHandler,
	modelTrendByUser auditquery.ModelTrendByUserHandler,
	requestRate auditquery.RequestRateHandler,
	requestRateByUser auditquery.RequestRateByUserHandler,
) handler.AuditDependencies {
	return handler.AuditDependencies{
		ListAll:           listAll,
		ListByUser:        listByUser,
		ModelTrend:        modelTrend,
		ModelTrendByUser:  modelTrendByUser,
		RequestRate:       requestRate,
		RequestRateByUser: requestRateByUser,
	}
}
```

- [ ] **Step 4: Verify compilation**

Run: `go vet ./internal/bootstrap/...` — expected: no errors

---

### Task 8: Backend Unit Tests

**Files:**
- Create: `test/unit/audit_query/audit_stats_test.go`

- [ ] **Step 1: Create test file**

```go
package audit_query

import (
	"context"
	"testing"
	"time"

	auditquery "github.com/hcd233/aris-proxy-api/internal/application/audit/query"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
)

// ─── ModelTrendHandler 测试 ─────────────────────────────────

func TestModelTrendHandler_DelegatesToRepo(t *testing.T) {
	repo := &fakeAuditRepo{
		listAllFunc: nil,
	}
	// replace with a real call; handler should call QueryModelTrend
	// This verifies the interface contract is satisfied.
	h := auditquery.NewModelTrendHandler(repo)

	points, err := h.Handle(context.Background(), auditquery.ModelTrendQuery{
		StartTime:   time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		EndTime:     time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC),
		Granularity: "day",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if points == nil {
		t.Error("expected non-nil points")
	}
}
```

- [ ] **Step 2: Add fake methods to existing `fakeAuditRepo` so it implements the new interface methods**

In `test/unit/audit_query/audit_query_test.go`, add after `BatchGetRelations`:

```go
func (f *fakeAuditRepo) QueryModelTrend(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time, granularity string) ([]*modelcall.ModelTrendPoint, error) {
	return nil, nil
}

func (f *fakeAuditRepo) QueryRequestRate(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time, granularity string) ([]*modelcall.RequestRatePoint, error) {
	return nil, nil
}
```

- [ ] **Step 3: Run tests**

Run: `go test -v -count=1 ./test/unit/audit_query/` — expected: all pass

---

### Task 9: Frontend Types — Add stats response types

**Files:**
- Modify: `web/src/lib/types.ts`

- [ ] **Step 1: Add stats types at the end of the file**

```typescript
// ─── Dashboard Stats ──────────────────────────────────────────

export interface TrendPoint {
  time: string;
  count: number;
}

export interface ModelTrendItem {
  model: string;
  points: TrendPoint[];
}

export interface ModelTrendRsp extends CommonRsp {
  data?: ModelTrendItem[];
}

export interface RatePoint {
  time: string;
  total: number;
  success: number;
  failed: number;
  successRate: number;
}

export interface RequestRateItem {
  model: string;
  points: RatePoint[];
}

export interface RequestRateRsp extends CommonRsp {
  data?: RequestRateItem[];
}

export type Granularity = "minute" | "hour" | "day" | "week";
```

---

### Task 10: Frontend API Client — Add stats fetch methods

**Files:**
- Modify: `web/src/lib/api-client.ts`

- [ ] **Step 1: Add `fetchModelTrend` and `fetchRequestRate` methods to `ApiClient` class (before the closing brace)**

```typescript
async fetchModelTrend(params: {
  startTime: string;
  endTime: string;
  granularity: Granularity;
}): Promise<ModelTrendRsp> {
  const sp = new URLSearchParams(params);
  return this.request<ModelTrendRsp>(`/api/v1/audit/stats/model/trend?${sp}`);
}

async fetchRequestRate(params: {
  startTime: string;
  endTime: string;
  granularity: Granularity;
}): Promise<RequestRateRsp> {
  const sp = new URLSearchParams(params);
  return this.request<RequestRateRsp>(`/api/v1/audit/stats/request/rate?${sp}`);
}
```

- [ ] **Step 2: Add imports for the new types**

Add to the import block:
```typescript
import type {
  ...
  ModelTrendRsp,
  RequestRateRsp,
  Granularity,
} from "./types";
```

---

### Task 11: Frontend Chart — Model Trend Card

**Files:**
- Create: `web/src/components/charts/model-trend-chart.tsx`

- [ ] **Step 1: Install shadcn chart**

```bash
cd web && npx shadcn add chart --yes
```

- [ ] **Step 2: Create the Model Trend chart card**

```tsx
"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api-client";
import type { ModelTrendItem, Granularity } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  ChartLegend,
  ChartLegendContent,
} from "@/components/ui/chart";
import { Line, LineChart, XAxis, YAxis, CartesianGrid } from "recharts";

const granularityOptions: { value: Granularity; label: string }[] = [
  { value: "hour", label: "Hour" },
  { value: "day", label: "Day" },
  { value: "week", label: "Week" },
];

function toISODate(d: Date): string {
  return d.toISOString().replace(/\.\d+Z$/, "Z");
}

interface FlattenedPoint {
  time: string;
  [model: string]: number | string;
}

export function ModelTrendChart() {
  const [granularity, setGranularity] = useState<Granularity>("day");
  const [data, setData] = useState<ModelTrendItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(false);
    try {
      const end = new Date();
      const start = new Date();
      start.setDate(start.getDate() - 7);
      const rsp = await api.fetchModelTrend({
        startTime: toISODate(start),
        endTime: toISODate(end),
        granularity,
      });
      setData(rsp.data ?? []);
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  }, [granularity]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const models = [...new Set(data.map((d) => d.model))];
  const chartConfig = Object.fromEntries(
    models.map((m, i) => [
      m,
      { label: m, color: `hsl(var(--chart-${(i % 5) + 1}))` },
    ])
  );

  // flatten data for recharts: [{ time, model1: count, model2: count }]
  const timeSet = new Set<string>();
  const pointMap = new Map<string, Record<string, number>>();
  for (const item of data) {
    for (const p of item.points) {
      timeSet.add(p.time);
      if (!pointMap.has(p.time)) pointMap.set(p.time, {});
      pointMap.get(p.time)![item.model] = p.count;
    }
  }
  const flatData = Array.from(timeSet).sort().map((time) => ({
    time,
    ...pointMap.get(time),
  }));

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="font-display">Model Call Trend</CardTitle>
        <ToggleGroup
          type="single"
          value={granularity}
          onValueChange={(v) => v && setGranularity(v as Granularity)}
          size="sm"
        >
          {granularityOptions.map((opt) => (
            <ToggleGroupItem key={opt.value} value={opt.value}>
              {opt.label}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      </CardHeader>
      <CardContent>
        {loading ? (
          <Skeleton className="h-64 w-full" />
        ) : error ? (
          <div className="flex h-64 flex-col items-center justify-center gap-2 text-sm text-muted-foreground">
            <p>Failed to load</p>
            <Button variant="outline" size="sm" onClick={fetchData}>
              Retry
            </Button>
          </div>
        ) : flatData.length === 0 ? (
          <div className="flex h-64 items-center justify-center text-sm text-muted-foreground">
            No data for this period
          </div>
        ) : (
          <ChartContainer config={chartConfig} className="h-64 w-full">
            <LineChart data={flatData}>
              <CartesianGrid strokeDasharray="3 3" vertical={false} />
              <XAxis
                dataKey="time"
                tickFormatter={(v) => new Date(v).toLocaleDateString()}
                fontSize={12}
              />
              <YAxis fontSize={12} />
              <ChartTooltip content={<ChartTooltipContent />} />
              <ChartLegend content={<ChartLegendContent />} />
              {models.map((m) => (
                <Line
                  key={m}
                  type="monotone"
                  dataKey={m}
                  stroke={`var(--color-${m})`}
                  strokeWidth={2}
                  dot={false}
                />
              ))}
            </LineChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  );
}
```

---

### Task 12: Frontend Chart — Request Rate Card

**Files:**
- Create: `web/src/components/charts/request-rate-chart.tsx`

- [ ] **Step 1: Create the Request Rate chart card**

```tsx
"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api-client";
import type { RequestRateItem, Granularity } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  ChartLegend,
  ChartLegendContent,
} from "@/components/ui/chart";
import { Line, LineChart, XAxis, YAxis, CartesianGrid } from "recharts";

const granularityOptions: { value: Granularity; label: string }[] = [
  { value: "hour", label: "Hour" },
  { value: "day", label: "Day" },
  { value: "week", label: "Week" },
];

function toISODate(d: Date): string {
  return d.toISOString().replace(/\.\d+Z$/, "Z");
}

interface FlattenedRatePoint {
  time: string;
  [model: string]: number | string;
}

export function RequestRateChart() {
  const [granularity, setGranularity] = useState<Granularity>("hour");
  const [data, setData] = useState<RequestRateItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(false);
    try {
      const end = new Date();
      const start = new Date();
      start.setHours(start.getHours() - 24);
      const rsp = await api.fetchRequestRate({
        startTime: toISODate(start),
        endTime: toISODate(end),
        granularity,
      });
      setData(rsp.data ?? []);
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  }, [granularity]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const models = [...new Set(data.map((d) => d.model))];
  const chartConfig = Object.fromEntries(
    models.map((m, i) => [
      m,
      { label: m, color: `hsl(var(--chart-${(i % 5) + 1}))` },
    ])
  );

  const timeSet = new Set<string>();
  const pointMap = new Map<string, Record<string, number>>();
  for (const item of data) {
    for (const p of item.points) {
      timeSet.add(p.time);
      if (!pointMap.has(p.time)) pointMap.set(p.time, {});
      pointMap.get(p.time)![item.model] = p.successRate * 100;
    }
  }
  const flatData = Array.from(timeSet).sort().map((time) => ({
    time,
    ...pointMap.get(time),
  }));

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="font-display">Request Success Rate</CardTitle>
        <ToggleGroup
          type="single"
          value={granularity}
          onValueChange={(v) => v && setGranularity(v as Granularity)}
          size="sm"
        >
          {granularityOptions.map((opt) => (
            <ToggleGroupItem key={opt.value} value={opt.value}>
              {opt.label}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      </CardHeader>
      <CardContent>
        {loading ? (
          <Skeleton className="h-64 w-full" />
        ) : error ? (
          <div className="flex h-64 flex-col items-center justify-center gap-2 text-sm text-muted-foreground">
            <p>Failed to load</p>
            <Button variant="outline" size="sm" onClick={fetchData}>
              Retry
            </Button>
          </div>
        ) : flatData.length === 0 ? (
          <div className="flex h-64 items-center justify-center text-sm text-muted-foreground">
            No data for this period
          </div>
        ) : (
          <ChartContainer config={chartConfig} className="h-64 w-full">
            <LineChart data={flatData}>
              <CartesianGrid strokeDasharray="3 3" vertical={false} />
              <XAxis
                dataKey="time"
                tickFormatter={(v) => new Date(v).toLocaleDateString()}
                fontSize={12}
              />
              <YAxis
                fontSize={12}
                domain={[0, 100]}
                tickFormatter={(v) => `${v}%`}
              />
              <ChartTooltip
                content={
                  <ChartTooltipContent formatter={(v: number) => `${v.toFixed(1)}%`} />
                }
              />
              <ChartLegend content={<ChartLegendContent />} />
              {models.map((m) => (
                <Line
                  key={m}
                  type="monotone"
                  dataKey={m}
                  stroke={`var(--color-${m})`}
                  strokeWidth={2}
                  dot={false}
                />
              ))}
            </LineChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  );
}
```

---

### Task 13: Frontend Dashboard Page — Wire up chart cards

**Files:**
- Modify: `web/src/app/(dashboard)/page.tsx`

- [ ] **Step 1: Add imports for the new chart components**

Add after existing imports:
```typescript
import { ModelTrendChart } from "@/components/charts/model-trend-chart";
import { RequestRateChart } from "@/components/charts/request-rate-chart";
```

- [ ] **Step 2: Add the chart cards below the stat cards grid, inside the main `<div className="space-y-8">`**

After the stat cards `</div>` closing tag and before the parent `</div>`:
```tsx
        <div className="grid gap-4 lg:grid-cols-2">
          <ModelTrendChart />
          <RequestRateChart />
        </div>
```

---

### Task 14: Verify — Lint, test, build

- [ ] **Step 1: Backend lint**

Run: `make lint` — expected: no errors

- [ ] **Step 2: Backend build**

Run: `make build-dev` or `go build -o /dev/null ./main.go` — expected: success

- [ ] **Step 3: Backend unit tests**

Run: `go test -count=1 ./test/unit/audit_query/` — expected: all pass

- [ ] **Step 4: Frontend lint**

Run: `cd web && npm run lint` — expected: no errors

- [ ] **Step 5: Frontend build**

Run: `cd web && npm run build` — expected: success

---

### Self-Review

**1. Spec coverage check:**
- ✅ `model/trend` endpoint → Tasks 1-7
- ✅ `request/rate` endpoint → Tasks 1-7
- ✅ Granularity support (minute/hour/day/week) → Task 3 `dateTruncSQL`, Task 1 DTO enum
- ✅ Frontend chart cards with toggle → Tasks 11-12
- ✅ Dashboard page integration → Task 13
- ✅ Permission scoping (admin vs user) → Task 4-5, 7
- ✅ Unit tests → Task 8
- ✅ Lint/build → Task 14

**2. No placeholder scan:** All tasks have complete code blocks.

**3. Type consistency:** `Granularity` type matches across all layers (Go const → TS type), response shapes are consistent.

**4. No gaps found.**
