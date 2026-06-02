# Model Token Usage 排名表重设计 + API 解耦 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 Dashboard "Model Token Usage" 堆叠柱状图替换为排名表格 + 行内比例条，同时将共享 API 拆为三个专用端点。

**Architecture:** 后端新增两个 usecase handler（TokenRate / TokenUsage），复用现有 `QueryTokenThroughput` 仓库方法，在 usecase 层做聚合转换后返回前端就绪 DTO。前端三个图表组件各调用自己的专用 API。

**Tech Stack:** Go 1.25.1 + dig DI + huma router + Next.js 16 + TypeScript + tailwind

---

## Task 1: 后端 — 新增 DTO

**Files:**
- Modify: `internal/dto/audit_stats.go`

- [ ] **Step 1: 在 TokenThroughputPoint 定义后追加 TokenRate 和 TokenUsage 的 DTO**

```go
// — Token Rate —

type TokenRateReq struct {
	StartTime   time.Time        `query:"startTime" required:"true"`
	EndTime     time.Time        `query:"endTime" required:"true"`
	Granularity enum.Granularity `query:"granularity" required:"true" enum:"minute,hour,day,week"`
}

type TokenRateRsp struct {
	CommonRsp
	Data []*TokenRateItem `json:"data,omitempty" doc:"各模型的输出 Token 速率"`
}

type TokenRateItem struct {
	Model  string           `json:"model" doc:"模型名"`
	Points []*TokenRatePoint `json:"points" doc:"时间序列点"`
}

type TokenRatePoint struct {
	Time                  time.Time `json:"time" doc:"时间桶"`
	OutputTokensPerSecond float64   `json:"outputTokensPerSecond" doc:"输出 Token 速率 (tokens/s)"`
}

// — Token Usage —

type TokenUsageReq struct {
	StartTime   time.Time        `query:"startTime" required:"true"`
	EndTime     time.Time        `query:"endTime" required:"true"`
	Granularity enum.Granularity `query:"granularity" required:"true" enum:"minute,hour,day,week"`
}

type TokenUsageRsp struct {
	CommonRsp
	Data []*TokenUsageItem `json:"data,omitempty" doc:"各模型的 Token 聚合用量"`
}

type TokenUsageItem struct {
	Model               string `json:"model" doc:"模型名"`
	InputTokens         int    `json:"inputTokens" doc:"输入 Token 总数"`
	OutputTokens        int    `json:"outputTokens" doc:"输出 Token 总数"`
	CacheReadTokens     int    `json:"cacheReadTokens" doc:"缓存读取 Token 总数"`
	CacheCreationTokens int    `json:"cacheCreationTokens" doc:"缓存创建 Token 总数"`
}
```

- [ ] **Step 2: 编译验证**

```bash
go build ./internal/dto/
```

- [ ] **Step 3: 提交**

```bash
git add internal/dto/audit_stats.go
git commit -m "feat: add TokenRate and TokenUsage DTOs"
```

---

## Task 2: 后端 — 新增 TokenRate usecase handler

**Files:**
- Create: `internal/application/audit/query/token_rate.go`

- [ ] **Step 1: 创建 token_rate.go**

```go
package query

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/audit/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

type TokenRateQuery struct {
	StartTime   time.Time
	EndTime     time.Time
	Granularity enum.Granularity
}

type TokenRateByUserQuery struct {
	UserID      uint
	StartTime   time.Time
	EndTime     time.Time
	Granularity enum.Granularity
}

type TokenRateHandler interface {
	Handle(ctx context.Context, q TokenRateQuery) ([]*dto.TokenRateItem, error)
}

type TokenRateByUserHandler interface {
	Handle(ctx context.Context, q TokenRateByUserQuery) ([]*dto.TokenRateItem, error)
}

type tokenRateHandler struct {
	repo modelcall.AuditRepository
}

type tokenRateByUserHandler struct {
	repo      modelcall.AuditRepository
	apiKeyIDs port.APIKeyIDLookup
}

func NewTokenRateHandler(repo modelcall.AuditRepository) TokenRateHandler {
	return &tokenRateHandler{repo: repo}
}

func NewTokenRateByUserHandler(repo modelcall.AuditRepository, apiKeyIDs port.APIKeyIDLookup) TokenRateByUserHandler {
	return &tokenRateByUserHandler{repo: repo, apiKeyIDs: apiKeyIDs}
}

func (h *tokenRateHandler) Handle(ctx context.Context, q TokenRateQuery) ([]*dto.TokenRateItem, error) {
	points, err := h.repo.QueryTokenThroughput(ctx, nil, q.StartTime, q.EndTime, q.Granularity)
	if err != nil {
		return nil, err
	}
	return fillTokenRateSeries(points, q.StartTime, q.EndTime, q.Granularity), nil
}

func (h *tokenRateByUserHandler) Handle(ctx context.Context, q TokenRateByUserQuery) ([]*dto.TokenRateItem, error) {
	keyIDs, err := h.apiKeyIDs.LookupIDsByUserID(ctx, q.UserID)
	if err != nil {
		return nil, err
	}
	points, err := h.repo.QueryTokenThroughput(ctx, keyIDs, q.StartTime, q.EndTime, q.Granularity)
	if err != nil {
		return nil, err
	}
	return fillTokenRateSeries(points, q.StartTime, q.EndTime, q.Granularity), nil
}

func fillTokenRateSeries(points []*modelcall.TokenThroughputPoint, start, end time.Time, granularity enum.Granularity) []*dto.TokenRateItem {
	type rateSlot struct{ outputTokensPerSec float64 }
	modelOrder, byModel, timeSet := indexSeries(points,
		func(p *modelcall.TokenThroughputPoint) string { return p.Model },
		func(p *modelcall.TokenThroughputPoint) time.Time { return p.Time.UTC() },
		func(p *modelcall.TokenThroughputPoint) rateSlot {
			return rateSlot{outputTokensPerSec: p.OutputTokensPerSecond}
		},
	)
	buckets := buildBuckets(start.UTC(), end.UTC(), granularity, timeSet)
	items := make([]*dto.TokenRateItem, 0, len(modelOrder))
	for _, m := range modelOrder {
		pts := make([]*dto.TokenRatePoint, 0, len(buckets))
		for _, t := range buckets {
			s := byModel[m][t]
			tp := &dto.TokenRatePoint{Time: t, OutputTokensPerSecond: s.outputTokensPerSec}
			pts = append(pts, tp)
		}
		items = append(items, &dto.TokenRateItem{Model: m, Points: pts})
	}
	return items
}
```

- [ ] **Step 2: 编译验证**

```bash
go build ./internal/application/audit/query/
```

- [ ] **Step 3: 提交**

```bash
git add internal/application/audit/query/token_rate.go
git commit -m "feat: add TokenRate usecase handler"
```

---

## Task 3: 后端 — 新增 TokenUsage usecase handler

**Files:**
- Create: `internal/application/audit/query/token_usage.go`

- [ ] **Step 1: 创建 token_usage.go**

```go
package query

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/audit/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

type TokenUsageQuery struct {
	StartTime   time.Time
	EndTime     time.Time
	Granularity enum.Granularity
}

type TokenUsageByUserQuery struct {
	UserID      uint
	StartTime   time.Time
	EndTime     time.Time
	Granularity enum.Granularity
}

type TokenUsageHandler interface {
	Handle(ctx context.Context, q TokenUsageQuery) ([]*dto.TokenUsageItem, error)
}

type TokenUsageByUserHandler interface {
	Handle(ctx context.Context, q TokenUsageByUserQuery) ([]*dto.TokenUsageItem, error)
}

type tokenUsageHandler struct {
	repo modelcall.AuditRepository
}

type tokenUsageByUserHandler struct {
	repo      modelcall.AuditRepository
	apiKeyIDs port.APIKeyIDLookup
}

func NewTokenUsageHandler(repo modelcall.AuditRepository) TokenUsageHandler {
	return &tokenUsageHandler{repo: repo}
}

func NewTokenUsageByUserHandler(repo modelcall.AuditRepository, apiKeyIDs port.APIKeyIDLookup) TokenUsageByUserHandler {
	return &tokenUsageByUserHandler{repo: repo, apiKeyIDs: apiKeyIDs}
}

func (h *tokenUsageHandler) Handle(ctx context.Context, q TokenUsageQuery) ([]*dto.TokenUsageItem, error) {
	points, err := h.repo.QueryTokenThroughput(ctx, nil, q.StartTime, q.EndTime, q.Granularity)
	if err != nil {
		return nil, err
	}
	return aggregateTokenUsage(points), nil
}

func (h *tokenUsageByUserHandler) Handle(ctx context.Context, q TokenUsageByUserQuery) ([]*dto.TokenUsageItem, error) {
	keyIDs, err := h.apiKeyIDs.LookupIDsByUserID(ctx, q.UserID)
	if err != nil {
		return nil, err
	}
	points, err := h.repo.QueryTokenThroughput(ctx, keyIDs, q.StartTime, q.EndTime, q.Granularity)
	if err != nil {
		return nil, err
	}
	return aggregateTokenUsage(points), nil
}

func aggregateTokenUsage(points []*modelcall.TokenThroughputPoint) []*dto.TokenUsageItem {
	totals := make(map[string]*dto.TokenUsageItem)
	order := make([]string, 0)
	for _, p := range points {
		if _, ok := totals[p.Model]; !ok {
			order = append(order, p.Model)
			totals[p.Model] = &dto.TokenUsageItem{Model: p.Model}
		}
		t := totals[p.Model]
		t.InputTokens += p.InputTokens
		t.OutputTokens += p.OutputTokens
		t.CacheReadTokens += p.CacheReadTokens
		t.CacheCreationTokens += p.CacheCreationTokens
	}
	items := make([]*dto.TokenUsageItem, 0, len(order))
	for _, m := range order {
		items = append(items, totals[m])
	}
	return items
}
```

- [ ] **Step 2: 编译验证**

```bash
go build ./internal/application/audit/query/
```

- [ ] **Step 3: 提交**

```bash
git add internal/application/audit/query/token_usage.go
git commit -m "feat: add TokenUsage usecase handler"
```

---

## Task 4: 后端 — 更新 AuditService 接口和实现

**Files:**
- Modify: `internal/application/audit/query/service.go`

- [ ] **Step 1: AuditService 接口新增两个方法，auditService 结构体新增字段和构造参数**

将 `service.go` 做以下修改：

在 `AuditService` 接口的 `TokenThroughput` 方法后追加：

```go
	TokenRate(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*dto.TokenRateItem, error)
	TokenUsage(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*dto.TokenUsageItem, error)
```

在 `import` 块追加 `"github.com/hcd233/aris-proxy-api/internal/dto"`（如果还没有）。

在 `auditService` 结构体追加两个字段（在 `tokenThroughputByUser` 后）：

```go
	tokenRate           TokenRateHandler
	tokenRateByUser     TokenRateByUserHandler
	tokenUsage          TokenUsageHandler
	tokenUsageByUser    TokenUsageByUserHandler
```

`NewAuditService` 函数签名追加四个参数（在 `tokenThroughputByUser` 后）：

```go
	tokenRate TokenRateHandler,
	tokenRateByUser TokenRateByUserHandler,
	tokenUsage TokenUsageHandler,
	tokenUsageByUser TokenUsageByUserHandler,
```

构造体中追加赋值：

```go
		tokenRate:           tokenRate,
		tokenRateByUser:     tokenRateByUser,
		tokenUsage:          tokenUsage,
		tokenUsageByUser:    tokenUsageByUser,
```

在文件末尾追加两个服务方法实现（在 `TokenThroughput` 方法之后）：

```go
func (s *auditService) TokenRate(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*dto.TokenRateItem, error) {
	switch permission {
	case enum.PermissionAdmin:
		return s.tokenRate.Handle(ctx, TokenRateQuery{StartTime: startTime, EndTime: endTime, Granularity: granularity})
	case enum.PermissionUser:
		return s.tokenRateByUser.Handle(ctx, TokenRateByUserQuery{UserID: userID, StartTime: startTime, EndTime: endTime, Granularity: granularity})
	default:
		return nil, ierr.ErrUnauthorized
	}
}

func (s *auditService) TokenUsage(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*dto.TokenUsageItem, error) {
	switch permission {
	case enum.PermissionAdmin:
		return s.tokenUsage.Handle(ctx, TokenUsageQuery{StartTime: startTime, EndTime: endTime, Granularity: granularity})
	case enum.PermissionUser:
		return s.tokenUsageByUser.Handle(ctx, TokenUsageByUserQuery{UserID: userID, StartTime: startTime, EndTime: endTime, Granularity: granularity})
	default:
		return nil, ierr.ErrUnauthorized
	}
}
```

- [ ] **Step 2: 编译验证**

```bash
go build ./...
```

- [ ] **Step 3: 提交**

```bash
git add internal/application/audit/query/service.go
git commit -m "feat: add TokenRate and TokenUsage to AuditService"
```

---

## Task 5: 后端 — 更新 AuditHandler

**Files:**
- Modify: `internal/handler/audit.go`

- [ ] **Step 1: AuditHandler 接口新增两个方法，auditHandler 实现**

在 `AuditHandler` 接口的 `HandleTokenThroughput` 后追加：

```go
	HandleTokenRate(ctx context.Context, req *dto.TokenRateReq) (*dto.HTTPResponse[*dto.TokenRateRsp], error)
	HandleTokenUsage(ctx context.Context, req *dto.TokenUsageReq) (*dto.HTTPResponse[*dto.TokenUsageRsp], error)
```

在文件末尾追加两个 handler 方法：

```go
func (h *auditHandler) HandleTokenRate(ctx context.Context, req *dto.TokenRateReq) (*dto.HTTPResponse[*dto.TokenRateRsp], error) {
	rsp := &dto.TokenRateRsp{}
	items, err := h.svc.TokenRate(ctx,
		util.CtxValuePermission(ctx),
		util.CtxValueUint(ctx, constant.CtxKeyUserID),
		req.StartTime, req.EndTime, req.Granularity,
	)
	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] Token rate failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	rsp.Data = items
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *auditHandler) HandleTokenUsage(ctx context.Context, req *dto.TokenUsageReq) (*dto.HTTPResponse[*dto.TokenUsageRsp], error) {
	rsp := &dto.TokenUsageRsp{}
	items, err := h.svc.TokenUsage(ctx,
		util.CtxValuePermission(ctx),
		util.CtxValueUint(ctx, constant.CtxKeyUserID),
		req.StartTime, req.EndTime, req.Granularity,
	)
	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] Token usage failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	rsp.Data = items
	return apiutil.WrapHTTPResponse(rsp, nil)
}
```

- [ ] **Step 2: 编译验证**

```bash
go build ./...
```

- [ ] **Step 3: 提交**

```bash
git add internal/handler/audit.go
git commit -m "feat: add HandleTokenRate and HandleTokenUsage to AuditHandler"
```

---

## Task 6: 后端 — 注册新路由

**Files:**
- Modify: `internal/router/audit.go`

- [ ] **Step 1: 在 audit.go 末尾（现有 HandleTokenThroughput 注册之后）追加两个路由注册**

```go
	huma.Register(auditGroup, huma.Operation{
		OperationID: "queryTokenRate",
		Method:      http.MethodGet,
		Path:        "/stats/token/rate",
		Summary:     "QueryTokenRate",
		Description: "Query output token rate grouped by model and time bucket. Admin sees all; user sees only their own keys.",
		Tags:        []string{"Audit"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("queryTokenRate", enum.PermissionUser)},
	}, auditHandler.HandleTokenRate)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "queryTokenUsage",
		Method:      http.MethodGet,
		Path:        "/stats/token/usage",
		Summary:     "QueryTokenUsage",
		Description: "Query aggregated token usage per model. Admin sees all; user sees only their own keys.",
		Tags:        []string{"Audit"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("queryTokenUsage", enum.PermissionUser)},
	}, auditHandler.HandleTokenUsage)
```

- [ ] **Step 2: 编译验证**

```bash
go build ./...
```

- [ ] **Step 3: 提交**

```bash
git add internal/router/audit.go
git commit -m "feat: register TokenRate and TokenUsage routes"
```

---

## Task 7: 后端 — 更新 DI 容器注册

**Files:**
- Modify: `internal/bootstrap/container.go`

- [ ] **Step 1: 在 `newAuditService` 函数签名中追加四个新参数**

修改 `newAuditService` 函数：

在参数列表末尾（`tokenThroughputByUser` 之后，`auditquery.AuditService` 返回类型之前）追加：

```go
	tokenRate auditquery.TokenRateHandler,
	tokenRateByUser auditquery.TokenRateByUserHandler,
	tokenUsage auditquery.TokenUsageHandler,
	tokenUsageByUser auditquery.TokenUsageByUserHandler,
```

在 `auditquery.NewAuditService(...)` 调用中对应位置追加参数：

```go
	return auditquery.NewAuditService(listAll, listByUser, modelTrend, modelTrendByUser, requestRate, requestRateByUser, tokenThroughput, tokenThroughputByUser, tokenRate, tokenRateByUser, tokenUsage, tokenUsageByUser)
```

- [ ] **Step 2: 编译全量验证**

```bash
go build ./...
```

dig 会自动发现 `NewTokenRateHandler`、`NewTokenRateByUserHandler`、`NewTokenUsageHandler`、`NewTokenUsageByUserHandler` 构造函数并注入，无需额外注册。

- [ ] **Step 3: 提交**

```bash
git add internal/bootstrap/container.go
git commit -m "feat: wire TokenRate and TokenUsage into DI container"
```

---

## Task 8: 后端 — 单元测试

**Files:**
- Modify: `test/unit/audit_query/audit_query_test.go`

- [ ] **Step 1: 更新 fakeAuditRepo，添加 TokenRate 和 TokenUsage 的测试不直接调用 repo 方法（它们通过 TokenThroughput 复用），但需要在 AuditService 派发测试中更新构造函数调用**

修改 `TestAuditService_DispatchesByPermission` 函数中的 `NewAuditService` 调用，追加四个参数：

```go
	svc := auditquery.NewAuditService(
		auditquery.NewListAllAuditLogsHandler(repo),
		auditquery.NewListAuditLogsByUserHandler(repo, &fakeAPIKeyIDLookup{}),
		auditquery.NewModelTrendHandler(repo),
		auditquery.NewModelTrendByUserHandler(repo, &fakeAPIKeyIDLookup{}),
		auditquery.NewRequestRateHandler(repo),
		auditquery.NewRequestRateByUserHandler(repo, &fakeAPIKeyIDLookup{}),
		auditquery.NewTokenThroughputHandler(repo),
		auditquery.NewTokenThroughputByUserHandler(repo, &fakeAPIKeyIDLookup{}),
		auditquery.NewTokenRateHandler(repo),
		auditquery.NewTokenRateByUserHandler(repo, &fakeAPIKeyIDLookup{}),
		auditquery.NewTokenUsageHandler(repo),
		auditquery.NewTokenUsageByUserHandler(repo, &fakeAPIKeyIDLookup{}),
	)
```

- [ ] **Step 2: 在文件末尾追加 token_usage 聚合测试**

```go
func TestAggregateTokenUsage_SumsPerModel(t *testing.T) {
	t1 := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	repo := &fakeAuditRepo{
		queryTokenThroughputFn: func(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.TokenThroughputPoint, error) {
			return []*modelcall.TokenThroughputPoint{
				{Model: "gpt-4", Time: t1, InputTokens: 100, OutputTokens: 50, CacheReadTokens: 30, CacheCreationTokens: 10},
				{Model: "gpt-4", Time: t2, InputTokens: 200, OutputTokens: 150, CacheReadTokens: 20, CacheCreationTokens: 5},
				{Model: "claude", Time: t1, InputTokens: 300, OutputTokens: 250, CacheReadTokens: 50, CacheCreationTokens: 15},
			}, nil
		},
	}
	h := auditquery.NewTokenUsageHandler(repo)
	items, err := h.Handle(context.Background(), auditquery.TokenUsageQuery{
		StartTime: t1, EndTime: t2, Granularity: enum.GranularityHour,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	gpt := items[0]
	if gpt.InputTokens != 300 || gpt.OutputTokens != 200 || gpt.CacheReadTokens != 50 || gpt.CacheCreationTokens != 15 {
		t.Errorf("gpt-4 totals mismatch: %+v", gpt)
	}
	claude := items[1]
	if claude.InputTokens != 300 || claude.OutputTokens != 250 || claude.CacheReadTokens != 50 || claude.CacheCreationTokens != 15 {
		t.Errorf("claude totals mismatch: %+v", claude)
	}
}
```

fakeAuditRepo 需要支持可注入的 `queryTokenThroughputFn`，将其追加为结构体字段：

```go
type fakeAuditRepo struct {
	// ... 现有字段 ...
	queryTokenThroughputFn func(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.TokenThroughputPoint, error)
}
```

并修改 `QueryTokenThroughput` 方法：

```go
func (f *fakeAuditRepo) QueryTokenThroughput(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.TokenThroughputPoint, error) {
	if f.queryTokenThroughputFn != nil {
		return f.queryTokenThroughputFn(ctx, apiKeyIDs, startTime, endTime, granularity)
	}
	return nil, nil
}
```

- [ ] **Step 3: 运行测试**

```bash
go test -v -count=1 -run TestAggregateTokenUsage_SumsPerModel ./test/unit/audit_query/
go test -v -count=1 -run TestAuditService_DispatchesByPermission ./test/unit/audit_query/
```

- [ ] **Step 4: 全量测试**

```bash
go test -count=1 ./test/unit/audit_query/
```

- [ ] **Step 5: 提交**

```bash
git add test/unit/audit_query/audit_query_test.go
git commit -m "test: add TokenUsage aggregation test"
```

---

## Task 9: 前端 — 新增 TypeScript 类型

**Files:**
- Modify: `web/src/lib/types.ts`

- [ ] **Step 1: 在文件末尾追加 TokenRate 和 TokenUsage 类型**

```typescript
export interface TokenRatePoint {
  time: string;
  outputTokensPerSecond: number;
}

export interface TokenRateItem {
  model: string;
  points: TokenRatePoint[];
}

export interface TokenRateRsp extends CommonRsp {
  data?: TokenRateItem[];
}

export interface TokenUsageItem {
  model: string;
  inputTokens: number;
  outputTokens: number;
  cacheReadTokens: number;
  cacheCreationTokens: number;
}

export interface TokenUsageRsp extends CommonRsp {
  data?: TokenUsageItem[];
}
```

- [ ] **Step 2: 编译验证**

```bash
cd web && npx tsc --noEmit
```

- [ ] **Step 3: 提交**

```bash
git add web/src/lib/types.ts
git commit -m "feat: add TokenRate and TokenUsage frontend types"
```

---

## Task 10: 前端 — 新增 API 方法

**Files:**
- Modify: `web/src/lib/api-client.ts`

- [ ] **Step 1: 找到现有 `fetchTokenThroughput` 方法，在其后追加两个新方法**

```typescript
  async fetchTokenRate(params: {
    startTime: string;
    endTime: string;
    granularity: Granularity;
  }): Promise<TokenRateRsp> {
    const sp = new URLSearchParams(params);
    return this.request<TokenRateRsp>(`/api/v1/audit/stats/token/rate?${sp}`);
  }

  async fetchTokenUsage(params: {
    startTime: string;
    endTime: string;
    granularity: Granularity;
  }): Promise<TokenUsageRsp> {
    const sp = new URLSearchParams(params);
    return this.request<TokenUsageRsp>(`/api/v1/audit/stats/token/usage?${sp}`);
  }
```

确保 types.ts 中的 `TokenRateRsp` 和 `TokenUsageRsp` 已在 import 中引入。

- [ ] **Step 2: 编译验证**

```bash
cd web && npx tsc --noEmit
```

- [ ] **Step 3: 提交**

```bash
git add web/src/lib/api-client.ts
git commit -m "feat: add fetchTokenRate and fetchTokenUsage API methods"
```

---

## Task 11: 前端 — Token Rate 图表改用新 API

**Files:**
- Modify: `web/src/components/charts/token-rate-chart.tsx`

- [ ] **Step 1: 修改 fetchData 调用，从 `fetchTokenThroughput` 改为 `fetchTokenRate`**

将第 49 行：
```typescript
const rsp = await api.fetchTokenThroughput({ startTime, endTime, granularity });
```
改为：
```typescript
const rsp = await api.fetchTokenRate({ startTime, endTime, granularity });
```

同时删去不再需要的 `TokenThroughputItem` 类型 import（第 5 行），改为 import `TokenRateItem`：

```typescript
import type { TokenRateItem } from "@/lib/types";
```

将第 39 行 state 类型从 `TokenThroughputItem[]` 改为 `TokenRateItem[]`：

```typescript
const [data, setData] = useState<TokenRateItem[]>([]);
```

数据转换逻辑（第 58-70 行）中，`p.outputTokensPerSecond` 保持不变（`TokenRatePoint` 也包含此字段），无需修改。

- [ ] **Step 2: 编译验证**

```bash
cd web && npx tsc --noEmit
```

- [ ] **Step 3: 提交**

```bash
git add web/src/components/charts/token-rate-chart.tsx
git commit -m "refactor: switch TokenRateChart to dedicated API"
```

---

## Task 12: 前端 — Token Volume 图表改名

**Files:**
- Modify: `web/src/components/charts/token-volume-chart.tsx`

- [ ] **Step 1: 仅修改 CardTitle**

将第 89 行：
```tsx
<CardTitle className="font-display">Token Volume</CardTitle>
```
改为：
```tsx
<CardTitle className="font-display">Token Throughput</CardTitle>
```

- [ ] **Step 2: 编译验证**

```bash
cd web && npx tsc --noEmit
```

- [ ] **Step 3: 提交**

```bash
git add web/src/components/charts/token-volume-chart.tsx
git commit -m "refactor: rename Token Volume to Token Throughput"
```

---

## Task 13: 前端 — 重写 Model Token Usage 为排名表格

**Files:**
- Modify: `web/src/components/charts/model-token-bar-chart.tsx`

- [ ] **Step 1: 完全重写组件**

将整个文件替换为：

```tsx
"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api-client";
import type { TokenUsageItem } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import type { TimeRangeKey } from "@/lib/time-range";
import { computeRange } from "@/lib/time-range";

type SortField = "total" | "inputTokens" | "outputTokens" | "cacheReadTokens" | "cacheCreationTokens";

function formatTokenCount(v: number): string {
  if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(1)}M`;
  if (v >= 1_000) return `${(v / 1_000).toFixed(1)}K`;
  return String(v);
}

function tokenTotal(item: TokenUsageItem): number {
  return item.inputTokens + item.outputTokens + item.cacheReadTokens + item.cacheCreationTokens;
}

export function ModelTokenBarChart() {
  const [timeRange, setTimeRange] = useState<TimeRangeKey>("7d");
  const [customStart, setCustomStart] = useState("");
  const [customEnd, setCustomEnd] = useState("");
  const [data, setData] = useState<TokenUsageItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [sortField, setSortField] = useState<SortField>("total");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(false);
    try {
      const { startTime, endTime, granularity } = computeRange(timeRange, customStart, customEnd);
      const rsp = await api.fetchTokenUsage({ startTime, endTime, granularity });
      setData(rsp.data ?? []);
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  }, [timeRange, customStart, customEnd]);

  /* eslint-disable react-hooks/set-state-in-effect */
  useEffect(() => {
    fetchData();
  }, [fetchData]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const sorted = [...data].sort((a, b) => {
    let va: number, vb: number;
    if (sortField === "total") {
      va = tokenTotal(a);
      vb = tokenTotal(b);
    } else {
      va = a[sortField];
      vb = b[sortField];
    }
    return sortDir === "desc" ? vb - va : va - vb;
  });

  function handleSort(field: SortField) {
    if (sortField === field) {
      setSortDir((d) => (d === "desc" ? "asc" : "desc"));
    } else {
      setSortField(field);
      setSortDir("desc");
    }
  }

  function sortIndicator(field: SortField) {
    if (sortField !== field) return "";
    return sortDir === "desc" ? " ▼" : " ▲";
  }

  function renderBar(
    leftLabel: string,
    leftValue: number,
    leftColor: string,
    rightLabel: string,
    rightValue: number,
    rightColor: string,
    total: number,
  ) {
    const leftPct = total > 0 ? (leftValue / total) * 100 : 0;
    const rightPct = total > 0 ? (rightValue / total) * 100 : 0;
    return (
      <div>
        <div
          className="flex h-3 overflow-hidden rounded-md bg-muted"
          title={`${leftLabel}: ${formatTokenCount(leftValue)} / ${rightLabel}: ${formatTokenCount(rightValue)}`}
        >
          {leftPct > 0 && (
            <div
              style={{ width: `${leftPct}%`, backgroundColor: leftColor }}
              className="transition-all duration-200"
            />
          )}
          {rightPct > 0 && (
            <div
              style={{ width: `${rightPct}%`, backgroundColor: rightColor }}
              className="transition-all duration-200"
            />
          )}
        </div>
        <div className="mt-1 flex justify-between text-[10px] text-muted-foreground">
          <span style={{ color: leftColor }}>{leftLabel} {formatTokenCount(leftValue)}</span>
          <span style={{ color: rightColor }}>{rightLabel} {formatTokenCount(rightValue)}</span>
        </div>
      </div>
    );
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="font-display">Model Usage</CardTitle>
        <TimeRangePicker
          value={timeRange}
          customStart={customStart}
          customEnd={customEnd}
          onChange={(key, cs, ce) => {
            setTimeRange(key);
            setCustomStart(cs);
            setCustomEnd(ce);
          }}
        />
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
        ) : sorted.length === 0 ? (
          <div className="flex h-64 items-center justify-center text-sm text-muted-foreground">
            No data for this period
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm tabular-nums">
              <thead>
                <tr className="border-b border-border text-muted-foreground">
                  <th className="w-8 py-2 text-left font-medium">#</th>
                  <th className="py-2 text-left font-medium">Model</th>
                  <th
                    className="cursor-pointer py-2 text-right font-medium hover:text-foreground"
                    onClick={() => handleSort("total")}
                  >
                    Total{sortIndicator("total")}
                  </th>
                  <th className="w-[220px] py-2 text-left font-medium">Input</th>
                  <th className="w-[220px] py-2 text-left font-medium">Output</th>
                </tr>
              </thead>
              <tbody>
                {sorted.map((item, i) => {
                  const total = tokenTotal(item);
                  const inputTotal = item.inputTokens + item.cacheReadTokens;
                  const outputTotal = item.outputTokens + item.cacheCreationTokens;
                  return (
                    <tr
                      key={item.model}
                      className="border-b border-border transition-colors hover:bg-muted/50"
                    >
                      <td className="py-3 pr-2 text-muted-foreground">{i + 1}</td>
                      <td className="py-3 pr-4 font-medium">{item.model}</td>
                      <td className="py-3 pr-4 text-right font-semibold">
                        {formatTokenCount(total)}
                      </td>
                      <td className="py-3 pr-4">
                        {renderBar(
                          "Cache Read",
                          item.cacheReadTokens,
                          "#7C6BA5",
                          "Input",
                          item.inputTokens,
                          "#D97757",
                          inputTotal,
                        )}
                      </td>
                      <td className="py-3">
                        {renderBar(
                          "Cache Created",
                          item.cacheCreationTokens,
                          "#4A9E7D",
                          "Output",
                          item.outputTokens,
                          "#5B8DB8",
                          outputTotal,
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
```

- [ ] **Step 2: 编译验证**

```bash
cd web && npx tsc --noEmit
```

- [ ] **Step 3: 提交**

```bash
git add web/src/components/charts/model-token-bar-chart.tsx
git commit -m "feat: replace Model Token Usage bar chart with ranked table"
```

---

## Task 14: 全量验证

- [ ] **Step 1: 后端全量编译 + 测试 + lint**

```bash
go build ./...
go test -count=1 ./...
make lint
```

- [ ] **Step 2: 前端编译 + lint**

```bash
cd web && npx tsc --noEmit && npm run lint
```

- [ ] **Step 3: 提交（如有遗留改动）**

```bash
git status
git add ...
git commit -m "chore: full verification pass"
```
