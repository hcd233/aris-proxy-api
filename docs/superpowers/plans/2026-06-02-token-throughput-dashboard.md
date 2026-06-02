# Token Throughput Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a token throughput dashboard with two charts (token volume stacked area + output rate line) to the existing Dashboard page, backed by a new `GET /api/v1/audit/stats/token/throughput` API.

**Architecture:** Follow the exact same CQRS pattern as existing `ModelTrend`/`RequestRate`: domain point struct → repository interface + SQL impl → application handler pair (admin + user) → service dispatch → DTO → handler → router. Frontend follows existing chart components using recharts + shadcn chart wrapper + `useChartLegendHighlight` hook.

**Tech Stack:** Go (huma, GORM, dig), TypeScript (Next.js, recharts, shadcn/ui)

---

## File Structure

| Action | File | Responsibility |
|--------|------|---------------|
| Modify | `internal/domain/modelcall/repository.go` | Add `TokenThroughputPoint` struct + `QueryTokenThroughput` method to interface |
| Modify | `internal/infrastructure/repository/audit_repository.go` | Add `QueryTokenThroughput` SQL implementation |
| Create | `internal/application/audit/query/token_throughput.go` | Admin + user query handlers |
| Modify | `internal/application/audit/query/service.go` | Add `TokenThroughput` method to interface + service |
| Modify | `internal/application/audit/query/fill_series.go` | Add `FillTokenThroughputSeries` helper |
| Modify | `internal/dto/audit_stats.go` | Add `TokenThroughputReq/Rsp/Item/Point` DTOs |
| Modify | `internal/handler/audit.go` | Add `HandleTokenThroughput` method |
| Modify | `internal/router/audit.go` | Register new endpoint |
| Modify | `internal/bootstrap/container.go` | Wire new handlers + update service constructor |
| Modify | `web/src/lib/types.ts` | Add TS types |
| Modify | `web/src/lib/api-client.ts` | Add `fetchTokenThroughput` method |
| Create | `web/src/components/charts/token-volume-chart.tsx` | Token volume stacked area chart |
| Create | `web/src/components/charts/token-rate-chart.tsx` | Token output rate line chart |
| Modify | `web/src/app/(dashboard)/page.tsx` | Add new chart row |

---

### Task 1: Domain — Add `TokenThroughputPoint` and repository method

**Files:**
- Modify: `internal/domain/modelcall/repository.go`

- [ ] **Step 1: Add `TokenThroughputPoint` struct after `RequestRatePoint`**

```go
type TokenThroughputPoint struct {
	Model                 string
	Time                  time.Time
	InputTokens           int
	OutputTokens          int
	CacheCreationTokens   int
	CacheReadTokens       int
	OutputTokensPerSecond float64
}
```

- [ ] **Step 2: Add `QueryTokenThroughput` method to `AuditRepository` interface**

Add after the `QueryRequestRate` method:

```go
QueryTokenThroughput(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*TokenThroughputPoint, error)
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./internal/domain/...`
Expected: PASS (repository impl doesn't exist yet, but interface compiles)

---

### Task 2: Repository — Implement `QueryTokenThroughput`

**Files:**
- Modify: `internal/infrastructure/repository/audit_repository.go`

- [ ] **Step 1: Add `QueryTokenThroughput` method to `auditRepository`**

Add after the `QueryRequestRate` method:

```go
func (r *auditRepository) QueryTokenThroughput(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.TokenThroughputPoint, error) {
	db := r.db.WithContext(ctx).Model(&dbmodel.ModelCallAudit{}).
		Where(constant.FieldCreatedAt+" >= ? AND "+constant.FieldCreatedAt+" <= ?", startTime, endTime).
		Where(constant.DBConditionDeletedAtZero)

	if len(apiKeyIDs) > 0 {
		db = db.Where(constant.FieldAPIKeyID+" IN ?", apiKeyIDs)
	}

	timeBucketExpr := dateTruncSQL(granularity)
	selectFields := constant.FieldModel + ", " + timeBucketExpr + " AS time, " +
		"SUM(" + constant.FieldInputTokens + ") AS input_tokens, " +
		"SUM(" + constant.FieldOutputTokens + ") AS output_tokens, " +
		"SUM(" + constant.FieldCacheCreationInputTokens + ") AS cache_creation_tokens, " +
		"SUM(" + constant.FieldCacheReadInputTokens + ") AS cache_read_tokens, " +
		"SUM(" + constant.FieldOutputTokens + ") * 1000.0 / NULLIF(SUM(" + constant.FieldStreamDurationMs + "), 0) AS output_tokens_per_second"

	var results []*modelcall.TokenThroughputPoint
	if err := db.Select(selectFields).
		Group(constant.FieldModel + ", time").
		Order(constant.FieldModel + ", time").
		Scan(&results).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "query token throughput")
	}
	return results, nil
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/infrastructure/repository/...`
Expected: PASS

---

### Task 3: Application — Add token throughput query handlers

**Files:**
- Create: `internal/application/audit/query/token_throughput.go`
- Modify: `internal/application/audit/query/service.go`

- [ ] **Step 1: Create `internal/application/audit/query/token_throughput.go`**

```go
package query

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/audit/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
)

type TokenThroughputQuery struct {
	StartTime   time.Time
	EndTime     time.Time
	Granularity enum.Granularity
}

type TokenThroughputByUserQuery struct {
	UserID      uint
	StartTime   time.Time
	EndTime     time.Time
	Granularity enum.Granularity
}

type TokenThroughputHandler interface {
	Handle(ctx context.Context, q TokenThroughputQuery) ([]*modelcall.TokenThroughputPoint, error)
}

type TokenThroughputByUserHandler interface {
	Handle(ctx context.Context, q TokenThroughputByUserQuery) ([]*modelcall.TokenThroughputPoint, error)
}

type tokenThroughputHandler struct {
	repo modelcall.AuditRepository
}

type tokenThroughputByUserHandler struct {
	repo      modelcall.AuditRepository
	apiKeyIDs port.APIKeyIDLookup
}

func NewTokenThroughputHandler(repo modelcall.AuditRepository) TokenThroughputHandler {
	return &tokenThroughputHandler{repo: repo}
}

func NewTokenThroughputByUserHandler(repo modelcall.AuditRepository, apiKeyIDs port.APIKeyIDLookup) TokenThroughputByUserHandler {
	return &tokenThroughputByUserHandler{repo: repo, apiKeyIDs: apiKeyIDs}
}

func (h *tokenThroughputHandler) Handle(ctx context.Context, q TokenThroughputQuery) ([]*modelcall.TokenThroughputPoint, error) {
	return h.repo.QueryTokenThroughput(ctx, nil, q.StartTime, q.EndTime, q.Granularity)
}

func (h *tokenThroughputByUserHandler) Handle(ctx context.Context, q TokenThroughputByUserQuery) ([]*modelcall.TokenThroughputPoint, error) {
	keyIDs, err := h.apiKeyIDs.LookupIDsByUserID(ctx, q.UserID)
	if err != nil {
		return nil, err
	}
	return h.repo.QueryTokenThroughput(ctx, keyIDs, q.StartTime, q.EndTime, q.Granularity)
}
```

- [ ] **Step 2: Add `TokenThroughput` method to `AuditService` interface in `service.go`**

Add after the `RequestRate` method:

```go
TokenThroughput(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.TokenThroughputPoint, error)
```

- [ ] **Step 3: Add handler fields and constructor params to `auditService` struct**

Add fields:

```go
tokenThroughput       TokenThroughputHandler
tokenThroughputByUser TokenThroughputByUserHandler
```

Update `NewAuditService` signature to add two params:

```go
func NewAuditService(
	listAll ListAllAuditLogsHandler,
	listByUser ListAuditLogsByUserHandler,
	modelTrend ModelTrendHandler,
	modelTrendByUser ModelTrendByUserHandler,
	requestRate RequestRateHandler,
	requestRateByUser RequestRateByUserHandler,
	tokenThroughput TokenThroughputHandler,
	tokenThroughputByUser TokenThroughputByUserHandler,
) AuditService {
	return &auditService{
		listAll:              listAll,
		listByUser:           listByUser,
		modelTrend:           modelTrend,
		modelTrendByUser:     modelTrendByUser,
		requestRate:          requestRate,
		requestRateByUser:    requestRateByUser,
		tokenThroughput:      tokenThroughput,
		tokenThroughputByUser: tokenThroughputByUser,
	}
}
```

- [ ] **Step 4: Implement `TokenThroughput` dispatch method**

```go
func (s *auditService) TokenThroughput(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.TokenThroughputPoint, error) {
	switch permission {
	case enum.PermissionAdmin:
		return s.tokenThroughput.Handle(ctx, TokenThroughputQuery{StartTime: startTime, EndTime: endTime, Granularity: granularity})
	case enum.PermissionUser:
		return s.tokenThroughputByUser.Handle(ctx, TokenThroughputByUserQuery{UserID: userID, StartTime: startTime, EndTime: endTime, Granularity: granularity})
	default:
		return nil, ierr.ErrUnauthorized
	}
}
```

- [ ] **Step 5: Verify compilation**

Run: `go build ./internal/application/audit/...`
Expected: FAIL (container.go not updated yet) — this is expected, will fix in Task 6.

---

### Task 4: Application — Add `FillTokenThroughputSeries` helper

**Files:**
- Modify: `internal/application/audit/query/fill_series.go`

- [ ] **Step 1: Add `FillTokenThroughputSeries` function after `FillRateSeries`**

```go
type throughputSlot struct {
	inputTokens         int
	outputTokens        int
	cacheCreationTokens int
	cacheReadTokens     int
	outputTokensPerSec  float64
}

func FillTokenThroughputSeries(points []*modelcall.TokenThroughputPoint) []*dto.TokenThroughputItem {
	modelOrder, byModel, buckets := indexSeries(points,
		func(p *modelcall.TokenThroughputPoint) string { return p.Model },
		func(p *modelcall.TokenThroughputPoint) time.Time { return p.Time },
		func(p *modelcall.TokenThroughputPoint) throughputSlot {
			return throughputSlot{
				inputTokens:         p.InputTokens,
				outputTokens:        p.OutputTokens,
				cacheCreationTokens: p.CacheCreationTokens,
				cacheReadTokens:     p.CacheReadTokens,
				outputTokensPerSec:  p.OutputTokensPerSecond,
			}
		},
	)
	items := make([]*dto.TokenThroughputItem, 0, len(modelOrder))
	for _, m := range modelOrder {
		pts := make([]*dto.TokenThroughputPoint, 0, len(buckets))
		for _, t := range buckets {
			s, ok := byModel[m][t]
			tp := &dto.TokenThroughputPoint{
				Time:                  t,
				InputTokens:           0,
				OutputTokens:          0,
				CacheCreationTokens:   0,
				CacheReadTokens:       0,
				OutputTokensPerSecond: 0,
			}
			if ok {
				tp.InputTokens = s.inputTokens
				tp.OutputTokens = s.outputTokens
				tp.CacheCreationTokens = s.cacheCreationTokens
				tp.CacheReadTokens = s.cacheReadTokens
				tp.OutputTokensPerSecond = s.outputTokensPerSec
			}
			pts = append(pts, tp)
		}
		items = append(items, &dto.TokenThroughputItem{Model: m, Points: pts})
	}
	return items
}
```

Note: zero-value `throughputSlot` fills missing time buckets with 0 tokens and 0 rate, which is the correct behavior for sparse series.

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/application/audit/query/...`
Expected: FAIL (dto not updated yet) — expected, will fix in Task 5.

---

### Task 5: DTO — Add token throughput request/response types

**Files:**
- Modify: `internal/dto/audit_stats.go`

- [ ] **Step 1: Add DTOs after `RatePoint`**

```go
type TokenThroughputReq struct {
	StartTime   time.Time        `query:"startTime" required:"true"`
	EndTime     time.Time        `query:"endTime" required:"true"`
	Granularity enum.Granularity `query:"granularity" required:"true" enum:"minute,hour,day,week"`
}

type TokenThroughputRsp struct {
	CommonRsp
	Data []*TokenThroughputItem `json:"data,omitempty" doc:"各模型的 Token 吞吐量"`
}

type TokenThroughputItem struct {
	Model  string                  `json:"model" doc:"模型名"`
	Points []*TokenThroughputPoint `json:"points" doc:"时间序列点"`
}

type TokenThroughputPoint struct {
	Time                  time.Time `json:"time" doc:"时间桶"`
	InputTokens           int       `json:"inputTokens" doc:"输入 Token 数"`
	OutputTokens          int       `json:"outputTokens" doc:"输出 Token 数"`
	CacheCreationTokens   int       `json:"cacheCreationTokens" doc:"缓存创建 Token 数"`
	CacheReadTokens       int       `json:"cacheReadTokens" doc:"缓存读取 Token 数"`
	OutputTokensPerSecond float64   `json:"outputTokensPerSecond" doc:"输出 Token 速率 (tokens/s)"`
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/dto/...`
Expected: PASS

---

### Task 6: DI Container — Wire new handlers

**Files:**
- Modify: `internal/bootstrap/container.go`

- [ ] **Step 1: Register new handlers in `provideUseCases`**

After line 272 (`newRequestRateByUserHandler`), add:

```go
if err := container.Provide(auditquery.NewTokenThroughputHandler); err != nil {
	return err
}
if err := container.Provide(newTokenThroughputByUserHandler); err != nil {
	return err
}
```

- [ ] **Step 2: Add constructor function**

After `newRequestRateByUserHandler` function (around line 557), add:

```go
func newTokenThroughputByUserHandler(repo modelcall.AuditRepository, apiKeyRepo apikey.APIKeyRepository) auditquery.TokenThroughputByUserHandler {
	return auditquery.NewTokenThroughputByUserHandler(repo, apiKeyRepo)
}
```

- [ ] **Step 3: Update `newAuditService` signature and call**

Update the function at line ~537:

```go
func newAuditService(
	listAll auditquery.ListAllAuditLogsHandler,
	listByUser auditquery.ListAuditLogsByUserHandler,
	modelTrend auditquery.ModelTrendHandler,
	modelTrendByUser auditquery.ModelTrendByUserHandler,
	requestRate auditquery.RequestRateHandler,
	requestRateByUser auditquery.RequestRateByUserHandler,
	tokenThroughput auditquery.TokenThroughputHandler,
	tokenThroughputByUser auditquery.TokenThroughputByUserHandler,
) auditquery.AuditService {
	return auditquery.NewAuditService(listAll, listByUser, modelTrend, modelTrendByUser, requestRate, requestRateByUser, tokenThroughput, tokenThroughputByUser)
}
```

- [ ] **Step 4: Verify full backend compilation**

Run: `go build ./...`
Expected: PASS

---

### Task 7: Handler + Router — Add endpoint

**Files:**
- Modify: `internal/handler/audit.go`
- Modify: `internal/router/audit.go`

- [ ] **Step 1: Add `HandleTokenThroughput` to `AuditHandler` interface**

```go
HandleTokenThroughput(ctx context.Context, req *dto.TokenThroughputReq) (*dto.HTTPResponse[*dto.TokenThroughputRsp], error)
```

- [ ] **Step 2: Implement `HandleTokenThroughput` on `auditHandler`**

```go
func (h *auditHandler) HandleTokenThroughput(ctx context.Context, req *dto.TokenThroughputReq) (*dto.HTTPResponse[*dto.TokenThroughputRsp], error) {
	rsp := &dto.TokenThroughputRsp{}
	points, err := h.svc.TokenThroughput(ctx,
		util.CtxValuePermission(ctx),
		util.CtxValueUint(ctx, constant.CtxKeyUserID),
		req.StartTime, req.EndTime, req.Granularity,
	)
	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] Token throughput failed", zap.Error(err))
		rsp.Error = bizErrorFrom(err)
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	rsp.Data = auditquery.FillTokenThroughputSeries(points)
	return apiutil.WrapHTTPResponse(rsp, nil)
}
```

- [ ] **Step 3: Register route in `internal/router/audit.go`**

After the `queryRequestRate` registration, add:

```go
huma.Register(auditGroup, huma.Operation{
	OperationID: "queryTokenThroughput",
	Method:      http.MethodGet,
	Path:        "/stats/token/throughput",
	Summary:     "QueryTokenThroughput",
	Description: "Query token throughput (volume + output rate) grouped by model and time bucket. Admin sees all; user sees only their own keys.",
	Tags:        []string{"Audit"},
	Security:    []map[string][]string{{"jwtAuth": {}}},
	Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("queryTokenThroughput", enum.PermissionUser)},
}, auditHandler.HandleTokenThroughput)
```

- [ ] **Step 4: Verify compilation**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 5: Run linter**

Run: `make lint`
Expected: PASS

---

### Task 8: Frontend — Add TypeScript types and API client method

**Files:**
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/lib/api-client.ts`

- [ ] **Step 1: Add types in `types.ts` after `RequestRateRsp`**

```ts
export interface TokenThroughputPoint {
  time: string;
  inputTokens: number;
  outputTokens: number;
  cacheCreationTokens: number;
  cacheReadTokens: number;
  outputTokensPerSecond: number;
}

export interface TokenThroughputItem {
  model: string;
  points: TokenThroughputPoint[];
}

export interface TokenThroughputRsp extends CommonRsp {
  data?: TokenThroughputItem[];
}
```

- [ ] **Step 2: Add `fetchTokenThroughput` method to `ApiClient` in `api-client.ts`**

Add import `TokenThroughputRsp` to the import block, then add method after `fetchRequestRate`:

```ts
async fetchTokenThroughput(params: {
  startTime: string;
  endTime: string;
  granularity: Granularity;
}): Promise<TokenThroughputRsp> {
  const sp = new URLSearchParams(params);
  return this.request<TokenThroughputRsp>(`/api/v1/audit/stats/token/throughput?${sp}`);
}
```

- [ ] **Step 3: Verify TypeScript compilation**

Run: `cd web && npx tsc --noEmit`
Expected: PASS

---

### Task 9: Frontend — Create TokenVolumeChart component

**Files:**
- Create: `web/src/components/charts/token-volume-chart.tsx`

- [ ] **Step 1: Create the component**

```tsx
"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api-client";
import type { TokenThroughputItem } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  ChartLegend,
  ChartLegendContent,
} from "@/components/ui/chart";
import { Area, AreaChart, XAxis, YAxis, CartesianGrid } from "recharts";
import { useChartLegendHighlight } from "@/hooks/use-chart-legend-highlight";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import type { TimeRangeKey } from "@/lib/time-range";
import { computeRange, formatChartTime } from "@/lib/time-range";

const TOKEN_LAYERS = [
  { key: "inputTokens", label: "Input", color: "#D97757" },
  { key: "outputTokens", label: "Output", color: "#5B8DB8" },
  { key: "cacheReadTokens", label: "Cache Read", color: "#7C6BA5" },
  { key: "cacheCreationTokens", label: "Cache Creation", color: "#4A9E7D" },
] as const;

const MODEL_COLORS = ["#D97757", "#5B8DB8", "#7C6BA5", "#4A9E7D", "#C76B8A", "#8B7355", "#6B8BA4", "#A0522D"];

function formatTokenCount(v: number): string {
  if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(1)}M`;
  if (v >= 1_000) return `${(v / 1_000).toFixed(1)}K`;
  return String(v);
}

export function TokenVolumeChart() {
  const [timeRange, setTimeRange] = useState<TimeRangeKey>("7d");
  const [customStart, setCustomStart] = useState("");
  const [customEnd, setCustomEnd] = useState("");
  const [data, setData] = useState<TokenThroughputItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const { activeLegend, onLegendHover, getStrokeOpacity } = useChartLegendHighlight();

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(false);
    try {
      const { startTime, endTime, granularity } = computeRange(timeRange, customStart, customEnd);
      const rsp = await api.fetchTokenThroughput({ startTime, endTime, granularity });
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

  const models = [...new Set(data.map((d) => d.model))];
  const chartConfig = Object.fromEntries(
    TOKEN_LAYERS.map((l) => [l.key, { label: l.label, color: l.color }])
  );

  const timeSet = new Set<string>();
  const pointMap = new Map<string, Record<string, number>>();
  for (const item of data) {
    for (const p of item.points) {
      timeSet.add(p.time);
      if (!pointMap.has(p.time)) pointMap.set(p.time, {});
      const entry = pointMap.get(p.time)!;
      entry.inputTokens = (entry.inputTokens ?? 0) + p.inputTokens;
      entry.outputTokens = (entry.outputTokens ?? 0) + p.outputTokens;
      entry.cacheReadTokens = (entry.cacheReadTokens ?? 0) + p.cacheReadTokens;
      entry.cacheCreationTokens = (entry.cacheCreationTokens ?? 0) + p.cacheCreationTokens;
    }
  }
  const flatData = Array.from(timeSet).sort().map((time) => ({
    time,
    ...pointMap.get(time),
  }));

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="font-display">Token Volume</CardTitle>
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
        ) : flatData.length === 0 ? (
          <div className="flex h-64 items-center justify-center text-sm text-muted-foreground">
            No data for this period
          </div>
        ) : (
          <ChartContainer config={chartConfig} className="h-64 w-full">
            <AreaChart data={flatData}>
              <CartesianGrid strokeDasharray="3 3" vertical={false} />
              <XAxis
                dataKey="time"
                tickFormatter={(v) => formatChartTime(v, timeRange, customStart, customEnd)}
                fontSize={12}
              />
              <YAxis fontSize={12} tickFormatter={formatTokenCount} domain={[0, "auto"]} allowDataOverflow={false} />
              <ChartTooltip content={<ChartTooltipContent />} />
              <ChartLegend content={<ChartLegendContent activeLegend={activeLegend} onLegendHover={onLegendHover} />} />
              {TOKEN_LAYERS.map((layer) => (
                <Area
                  key={layer.key}
                  type="monotone"
                  dataKey={layer.key}
                  stackId="1"
                  stroke={layer.color}
                  fill={layer.color}
                  strokeOpacity={getStrokeOpacity(layer.key)}
                  fillOpacity={0.6}
                />
              ))}
            </AreaChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  );
}
```

Note: The TokenVolumeChart aggregates across all models into one stacked area showing the 4 token type layers. This differs from ModelTrendChart which shows one line per model. The stacked area by token type is more informative for volume because showing per-model stacked areas would be visually cluttered. If per-model breakdown is needed, the rate chart below provides that.

---

### Task 10: Frontend — Create TokenRateChart component

**Files:**
- Create: `web/src/components/charts/token-rate-chart.tsx`

- [ ] **Step 1: Create the component**

```tsx
"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api-client";
import type { TokenThroughputItem } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
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
import { useChartLegendHighlight } from "@/hooks/use-chart-legend-highlight";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import type { TimeRangeKey } from "@/lib/time-range";
import { computeRange, formatChartTime } from "@/lib/time-range";

const CHART_COLORS = ["#D97757", "#5B8DB8", "#7C6BA5", "#4A9E7D", "#C76B8A", "#8B7355", "#6B8BA4", "#A0522D"];

export function TokenRateChart() {
  const [timeRange, setTimeRange] = useState<TimeRangeKey>("7d");
  const [customStart, setCustomStart] = useState("");
  const [customEnd, setCustomEnd] = useState("");
  const [data, setData] = useState<TokenThroughputItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const { activeLegend, onLegendHover, getStrokeOpacity } = useChartLegendHighlight();

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(false);
    try {
      const { startTime, endTime, granularity } = computeRange(timeRange, customStart, customEnd);
      const rsp = await api.fetchTokenThroughput({ startTime, endTime, granularity });
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

  const models = [...new Set(data.map((d) => d.model))];
  const chartConfig = Object.fromEntries(
    models.map((m, i) => [m, { label: m, color: CHART_COLORS[i % CHART_COLORS.length] }])
  );

  const timeSet = new Set<string>();
  const pointMap = new Map<string, Record<string, number | null>>();
  for (const item of data) {
    for (const p of item.points) {
      timeSet.add(p.time);
      if (!pointMap.has(p.time)) pointMap.set(p.time, {});
      pointMap.get(p.time)![item.model] = p.outputTokensPerSecond || null;
    }
  }
  const flatData = Array.from(timeSet).sort().map((time) => ({
    time,
    ...pointMap.get(time),
  }));

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="font-display">Output Token Rate</CardTitle>
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
                tickFormatter={(v) => formatChartTime(v, timeRange, customStart, customEnd)}
                fontSize={12}
              />
              <YAxis fontSize={12} domain={[0, "auto"]} allowDataOverflow={false} />
              <ChartTooltip content={<ChartTooltipContent />} />
              <ChartLegend content={<ChartLegendContent activeLegend={activeLegend} onLegendHover={onLegendHover} />} />
              {models.map((m) => (
                <Line
                  key={m}
                  type="monotone"
                  dataKey={m}
                  stroke={chartConfig[m]?.color ?? "#888"}
                  strokeWidth={2}
                  strokeOpacity={getStrokeOpacity(m)}
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

### Task 11: Frontend — Add charts to Dashboard page

**Files:**
- Modify: `web/src/app/(dashboard)/page.tsx`

- [ ] **Step 1: Add imports**

Add to the existing import block:

```ts
import { TokenVolumeChart } from "@/components/charts/token-volume-chart";
import { TokenRateChart } from "@/components/charts/token-rate-chart";
```

- [ ] **Step 2: Add new chart row**

After the existing chart grid (`<ModelTrendChart />` + `<RequestRateChart />`), add:

```tsx
<div className="grid gap-4 lg:grid-cols-2">
  <TokenVolumeChart />
  <TokenRateChart />
</div>
```

- [ ] **Step 3: Verify frontend build**

Run: `cd web && npm run lint && npm run build`
Expected: PASS

---

### Task 12: Full integration verification

- [ ] **Step 1: Run Go linter**

Run: `make lint`
Expected: PASS

- [ ] **Step 2: Run Go tests**

Run: `go test -count=1 ./internal/domain/... ./internal/application/audit/... ./internal/infrastructure/repository/... ./internal/handler/... ./internal/router/...`
Expected: PASS

- [ ] **Step 3: Run full Go build**

Run: `make build`
Expected: PASS

- [ ] **Step 4: Run frontend lint + build**

Run: `cd web && npm run lint && npm run build`
Expected: PASS
