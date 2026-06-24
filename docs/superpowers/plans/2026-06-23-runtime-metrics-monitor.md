# 运行时与实时业务指标监控 - 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 aris-proxy-api 建立运行时与实时业务指标监控基础设施，包含后端 Prometheus 指标采集/暴露和前端实时监控 dashboard 页面。

**Architecture:** 后端采用 `gofiber/contrib/v3/prometheus` 中间件（内置 HTTP 指标 + Go runtime + process collector），自建 SSE 并发 gauge 和 JSON 转换端点供前端消费。前端复用已有 recharts + shadcn chart 体系，新增 `/web/monitor` 页面以 5s 轮询展示实时指标。

**Tech Stack:** Go 1.25.1, gofiber v3, gofiber/contrib/v3/prometheus, prometheus/client_golang, huma, dig, Next.js 16, React 19, recharts, shadcn/ui

**Spec:** `docs/superpowers/specs/2026-06-23-runtime-metrics-monitor-design.md`

---

## 文件结构

### 后端新增

| 文件 | 职责 |
|------|------|
| `internal/infrastructure/metrics/prometheus.go` | 初始化 prometheus Registry、fiberprometheus 中间件、SSE gauge |
| `internal/infrastructure/metrics/json_exporter.go` | Gatherer → DTO 转换，供 JSON 端点使用 |
| `internal/handler/metrics.go` | MetricsHandler：处理 `/api/v1/metrics/json` 请求 |
| `internal/router/metrics.go` | 注册 metrics 路由 |
| `internal/dto/metrics.go` | MetricsJSONRsp 等 DTO |
| `test/unit/metrics/json_exporter_test.go` | JSON exporter 单元测试 |
| `test/e2e/metrics/metrics_endpoint_test.go` | metrics 端点 E2E 测试 |

### 后端修改

| 文件 | 改动 |
|------|------|
| `internal/common/constant/string.go` | 新增 TagMonitor 常量 |
| `internal/bootstrap/modules/infra.go` | 注册 Registry、SSEGauge、Middleware |
| `internal/bootstrap/container.go` | 挂载 prometheus 中间件 |
| `internal/bootstrap/modules/handler.go` | 注册 MetricsHandler、注入 SSEGauge 到 OpenAI/Anthropic deps |
| `internal/bootstrap/router.go` | routeParams 加 MetricsHandler，调用 initMetricsRouter |
| `internal/router/router.go` | APIRouterDependencies 加 MetricsHandler，调用 initMetricsRouter |
| `internal/handler/openai.go` | 注入 SSEGauge，包装 StreamResponse.Body |
| `internal/handler/anthropic.go` | 注入 SSEGauge，包装 StreamResponse.Body |

### 前端新增

| 文件 | 职责 |
|------|------|
| `web/src/components/charts/runtime-gauge-card.tsx` | 数字卡片组件 |
| `web/src/components/charts/runtime-line-chart.tsx` | 实时折线图组件 |
| `web/src/app/(dashboard)/monitor/page.tsx` | 监控页面 |

### 前端修改

| 文件 | 改动 |
|------|------|
| `web/src/lib/types.ts` | 新增 MetricsJSONRsp 等类型 |
| `web/src/lib/api-client.ts` | 新增 `api.getMetricsJSON()` |
| `web/src/app/(dashboard)/layout.tsx` | navItems 加 Monitor 入口 |

---

## Task 1: 添加依赖 + 创建 metrics 基础设施包

**Files:**
- Modify: `go.mod`
- Create: `internal/infrastructure/metrics/prometheus.go`
- Create: `internal/infrastructure/metrics/json_exporter.go`

- [ ] **Step 1: 添加 prometheus 依赖**

Run:
```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api
go get github.com/gofiber/contrib/v3/prometheus@latest
go get github.com/prometheus/client_golang/prometheus
go get github.com/prometheus/client_model/dto
go mod tidy
```

- [ ] **Step 2: 创建 prometheus.go — Registry + 中间件 + SSEGauge**

Create `internal/infrastructure/metrics/prometheus.go`:

```go
// Package metrics Prometheus 指标采集基础设施
//
//	@author centonhuang
//	@update 2026-06-23 10:00:00
package metrics

import (
	"github.com/gofiber/contrib/v3/prometheus"
	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/prometheus/client_golang/prometheus"
)

// SSEGauge SSE 并发连接数指标接口
//
//	@author centonhuang
//	@update 2026-06-23 10:00:00
type SSEGauge interface {
	Inc(provider string)
	Dec(provider string)
}

type sseGauge struct {
	gauge prometheus.Gauge
}

func (g *sseGauge) Inc(provider string) {
	g.gauge.WithLabelValues(provider).Inc()
}

func (g *sseGauge) Dec(provider string) {
	g.gauge.WithLabelValues(provider).Dec()
}

// NewRegistry 创建 Prometheus Registry
//
//	@return *prometheus.Registry
//	@author centonhuang
//	@update 2026-06-23 10:00:00
func NewRegistry() *prometheus.Registry {
	return prometheus.NewRegistry()
}

// NewSSEGauge 在 Registry 上注册并返回 SSE gauge
//
//	@param registry *prometheus.Registry
//	@return SSEGauge
//	@author centonhuang
//	@update 2026-06-23 10:00:00
func NewSSEGauge(registry *prometheus.Registry) SSEGauge {
	gauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sse_active_connections",
			Help: "Number of active SSE streaming connections",
		},
		[]string{"provider"},
	)
	registry.MustRegister(gauge)
	return &sseGauge{gauge: gauge}
}

// NewMiddleware 创建 fiberprometheus 中间件
//
//	@param registry *prometheus.Registry
//	@return fiber.Handler
//	@author centonhuang
//	@update 2026-06-23 10:00:00
func NewMiddleware(registry *prometheus.Registry) fiber.Handler {
	return prometheus.New(prometheus.Config{
		Service:           "aris-proxy-api",
		Namespace:         "http",
		Registerer:        registry,
		Gatherer:          registry,
		RequestDurationBuckets: []float64{
			0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75,
			1, 2.5, 5, 10, 15, 30, 60, 120, 300, 600, 1800,
		},
		SkipURIs: []string{
			constant.RoutePathHealth,
			constant.RoutePathReady,
			constant.RoutePathSSEHealth,
		},
	})
}
```

- [ ] **Step 3: 创建 json_exporter.go — Gatherer → DTO 转换**

Create `internal/infrastructure/metrics/json_exporter.go`:

```go
package metrics

import (
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/prometheus/client_golang/prometheus"
	dto_pb "github.com/prometheus/client_model/dto"
	"github.com/samber/lo"
)

// GatherMetricFamilies 从 Gatherer 采集指标并转换为 DTO
//
//	@param gatherer prometheus.Gatherer
//	@return []dto.MetricFamilyItem
//	@return error
//	@author centonhuang
//	@update 2026-06-23 10:00:00
func GatherMetricFamilies(gatherer prometheus.Gatherer) ([]dto.MetricFamilyItem, error) {
	families, err := gatherer.Gather()
	if err != nil {
		return nil, err
	}
	return lo.Map(families, func(f *dto_pb.MetricFamily, _ int) dto.MetricFamilyItem {
		return dto.MetricFamilyItem{
			Name:    f.GetName(),
			Type:    f.GetType().String(),
			Help:    f.GetHelp(),
			Samples: convertMetricSamples(f),
		}
	}), nil
}

func convertMetricSamples(f *dto_pb.MetricFamily) []dto.MetricSampleItem {
	metrics := f.GetMetric()
	if len(metrics) == 0 {
		return nil
	}
	return lo.Map(metrics, func(m *dto_pb.Metric, _ int) dto.MetricSampleItem {
		labels := lo.SliceToMap(m.GetLabel(), func(l *dto_pb.LabelPair) (string, string) {
			return l.GetName(), l.GetValue()
		})
		value := getMetricValue(m)
		return dto.MetricSampleItem{
			Labels: labels,
			Value:  value,
		}
	})
}

func getMetricValue(m *dto_pb.Metric) float64 {
	switch {
	case m.GetCounter() != nil:
		return m.GetCounter().GetValue()
	case m.GetGauge() != nil:
		return m.GetGauge().GetValue()
	case m.GetHistogram() != nil:
		return m.GetHistogram().GetSampleSum()
	case m.GetSummary() != nil:
		return m.GetSummary().GetSampleSum()
	case m.GetUntyped() != nil:
		return m.GetUntyped().GetValue()
	default:
		return 0
	}
}
```

- [ ] **Step 4: 验证编译**

Run: `go build ./internal/dto/... ./internal/infrastructure/metrics/...`
Expected: 编译通过（Task 2 创建 DTO 后）

> 注意：Step 4 会因 DTO 尚未创建而失败，这是预期的。DTO 在 Task 2 创建后重新验证。

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/metrics/ go.mod go.sum
git commit -m "feat: add prometheus metrics infrastructure package"
```

---

## Task 2: 创建 metrics DTO

**Files:**
- Create: `internal/dto/metrics.go`
- Modify: `internal/common/constant/string.go` (add TagMonitor)

- [ ] **Step 1: 添加 TagMonitor 常量**

In `internal/common/constant/string.go`, add `TagMonitor` to the tag constants block (near line 142):

```go
	TagMonitor   = "Monitor"
```

- [ ] **Step 2: 创建 dto/metrics.go**

Create `internal/dto/metrics.go`:

```go
package dto

// MetricsJSONRsp Prometheus 指标 JSON 响应
//
//	@author centonhuang
//	@update 2026-06-23 10:00:00
type MetricsJSONRsp struct {
	CommonRsp
	Metrics []MetricFamilyItem `json:"metrics,omitempty" doc:"Metric families"`
}

// MetricFamilyItem 指标族
//
//	@author centonhuang
//	@update 2026-06-23 10:00:00
type MetricFamilyItem struct {
	Name    string             `json:"name" doc:"Metric name"`
	Type    string             `json:"type" doc:"Metric type"`
	Help    string             `json:"help" doc:"Metric help text"`
	Samples []MetricSampleItem `json:"samples,omitempty" doc:"Metric samples"`
}

// MetricSampleItem 指标样本
//
//	@author centonhuang
//	@update 2026-06-23 10:00:00
type MetricSampleItem struct {
	Labels map[string]string `json:"labels,omitempty" doc:"Sample labels"`
	Value  float64           `json:"value" doc:"Sample value"`
}
```

- [ ] **Step 3: 验证编译**

Run: `go build ./internal/dto/... ./internal/infrastructure/metrics/...`
Expected: 编译通过

- [ ] **Step 4: Commit**

```bash
git add internal/dto/metrics.go internal/common/constant/string.go
git commit -m "feat: add metrics DTO and TagMonitor constant"
```

---

## Task 3: JSON exporter 单元测试

**Files:**
- Create: `test/unit/metrics/json_exporter_test.go`

- [ ] **Step 1: 编写失败测试**

Create `test/unit/metrics/json_exporter_test.go`:

```go
package metrics

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/prometheus/client_golang/prometheus"
)

func TestGatherMetricFamilies_Counter(t *testing.T) {
	registry := prometheus.NewRegistry()
	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "test_counter", Help: "test help"},
		[]string{"method"},
	)
	registry.MustRegister(counter)
	counter.WithLabelValues("GET").Add(5)

	families, err := GatherMetricFamilies(registry)
	if err != nil {
		t.Fatalf("GatherMetricFamilies failed: %v", err)
	}

	var found *dto.MetricFamilyItem
	for i := range families {
		if families[i].Name == "test_counter" {
			found = &families[i]
			break
		}
	}
	if found == nil {
		t.Fatal("test_counter not found in gathered families")
	}
	if found.Type != "counter" {
		t.Errorf("expected type 'counter', got '%s'", found.Type)
	}
	if found.Help != "test help" {
		t.Errorf("expected help 'test help', got '%s'", found.Help)
	}
	if len(found.Samples) != 1 {
		t.Fatalf("expected 1 sample, got %d", len(found.Samples))
	}
	if found.Samples[0].Value != 5 {
		t.Errorf("expected value 5, got %f", found.Samples[0].Value)
	}
	if found.Samples[0].Labels["method"] != "GET" {
		t.Errorf("expected label method=GET, got '%s'", found.Samples[0].Labels["method"])
	}
}

func TestGatherMetricFamilies_Gauge(t *testing.T) {
	registry := prometheus.NewRegistry()
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{Name: "test_gauge", Help: "gauge help"})
	registry.MustRegister(gauge)
	gauge.Set(42)

	families, err := GatherMetricFamilies(registry)
	if err != nil {
		t.Fatalf("GatherMetricFamilies failed: %v", err)
	}

	var found *dto.MetricFamilyItem
	for i := range families {
		if families[i].Name == "test_gauge" {
			found = &families[i]
			break
		}
	}
	if found == nil {
		t.Fatal("test_gauge not found")
	}
	if found.Type != "gauge" {
		t.Errorf("expected type 'gauge', got '%s'", found.Type)
	}
	if found.Samples[0].Value != 42 {
		t.Errorf("expected value 42, got %f", found.Samples[0].Value)
	}
}

func TestGatherMetricFamilies_EmptyRegistry(t *testing.T) {
	registry := prometheus.NewRegistry()
	families, err := GatherMetricFamilies(registry)
	if err != nil {
		t.Fatalf("GatherMetricFamilies failed: %v", err)
	}
	// 空 registry 可能返回 0 个或包含 go_* 默认指标（这里用 NewRegistry 不含默认 collector）
	if families == nil {
		t.Fatal("expected non-nil families slice")
	}
}

func TestSSEGauge_IncDec(t *testing.T) {
	registry := prometheus.NewRegistry()
	gaugeVec := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "sse_test", Help: "sse test"},
		[]string{"provider"},
	)
	registry.MustRegister(gaugeVec)

	g := &sseGauge{gauge: gaugeVec}
	g.Inc("openai")
	g.Inc("openai")
	g.Dec("openai")

	families, err := GatherMetricFamilies(registry)
	if err != nil {
		t.Fatalf("GatherMetricFamilies failed: %v", err)
	}

	var found *dto.MetricFamilyItem
	for i := range families {
		if families[i].Name == "sse_test" {
			found = &families[i]
			break
		}
	}
	if found == nil {
		t.Fatal("sse_test not found")
	}
	if found.Samples[0].Value != 1 {
		t.Errorf("expected value 1 (2 inc - 1 dec), got %f", found.Samples[0].Value)
	}
	if found.Samples[0].Labels["provider"] != "openai" {
		t.Errorf("expected label provider=openai, got '%s'", found.Samples[0].Labels["provider"])
	}
}
```

- [ ] **Step 2: 运行测试验证通过**

Run: `go test -v -count=1 ./test/unit/metrics/...`
Expected: PASS（4 个测试全部通过）

- [ ] **Step 3: Commit**

```bash
git add test/unit/metrics/
git commit -m "test: add unit tests for metrics json exporter and sse gauge"
```

---

## Task 4: 注册 metrics 到 DI + 挂载中间件

**Files:**
- Modify: `internal/bootstrap/modules/infra.go`
- Modify: `internal/bootstrap/container.go`

- [ ] **Step 1: 修改 infra.go — 注册 Registry、SSEGauge、Middleware**

In `internal/bootstrap/modules/infra.go`, add to imports:
```go
	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/metrics"
	"github.com/prometheus/client_golang/prometheus"
```

Add to `InfraModule`'s `fx.Provide`:
```go
		metrics.NewRegistry,
		NewSSEGauge,
		NewPrometheusMiddleware,
```

Add provider functions:
```go
// NewSSEGauge 包装 metrics.NewSSEGauge 供 dig 使用
//
//	@param registry *prometheus.Registry
//	@return metrics.SSEGauge
//	@author centonhuang
//	@update 2026-06-23 10:00:00
func NewSSEGauge(registry *prometheus.Registry) metrics.SSEGauge {
	return metrics.NewSSEGauge(registry)
}

// NewPrometheusMiddleware 创建 prometheus fiber 中间件
//
//	@param registry *prometheus.Registry
//	@return fiber.Handler
//	@author centonhuang
//	@update 2026-06-23 10:00:00
func NewPrometheusMiddleware(registry *prometheus.Registry) fiber.Handler {
	return metrics.NewMiddleware(registry)
}
```

- [ ] **Step 2: 修改 container.go — 挂载 prometheus 中间件**

In `internal/bootstrap/container.go`, add to `middlewareParams` struct:
```go
	PrometheusMiddleware fiber.Handler
```

In `registerMiddlewares`, add BEFORE the existing `params.App.Use(...)` chain (prometheus 需要单独挂载到 /metrics 路径):

```go
	params.App.Use("/metrics", params.PrometheusMiddleware)
```

The full `registerMiddlewares` function becomes:

```go
func registerMiddlewares(params middlewareParams) {
	params.App.Use("/metrics", params.PrometheusMiddleware)

	params.App.Use(
		middleware.RecoverMiddleware(),
		middleware.InflightMiddleware(params.InflightTracker),
		middleware.GuardMiddleware(params.Cache, middleware.GuardConfig{
			StrikeThreshold: constant.GuardStrikeThreshold,
			StrikeWindow:    constant.GuardStrikeWindow,
			BanDuration:     constant.GuardBanDuration,
			IgnoredPaths: []string{
				constant.RoutePathRoot,
				constant.RoutePathHealth,
				constant.RoutePathReady,
				constant.RoutePathSSEHealth,
				constant.RoutePathFavicon,
				constant.RoutePathRobots,
				constant.RoutePathAppleTouchIcon,
				constant.RoutePathAppleTouchIconPrecomposed,
				constant.RoutePathWellKnownSecurity,
			},
		}),
		middleware.FgprofMiddleware(),
		middleware.CORSMiddleware(),
		middleware.CompressMiddleware(),
		middleware.TraceMiddleware(),
		middleware.LogMiddleware(middleware.LogMiddlewareConfig{
			SamplingRules: []middleware.LogSamplingRule{
				{Path: constant.RoutePathHealth, Interval: constant.LogMiddlewareSamplingInterval},
				{Path: constant.RoutePathReady, Interval: constant.LogMiddlewareSamplingInterval},
				{Path: constant.RoutePathSSEHealth, Interval: constant.LogMiddlewareSamplingInterval},
			},
		}),
	)
}
```

- [ ] **Step 3: 验证编译**

Run: `go build ./internal/bootstrap/... ./internal/infrastructure/metrics/...`
Expected: 编译通过

- [ ] **Step 4: Commit**

```bash
git add internal/bootstrap/ internal/infrastructure/metrics/
git commit -m "feat: wire prometheus middleware and SSE gauge into DI"
```

---

## Task 5: 创建 MetricsHandler + DTO + Router

**Files:**
- Create: `internal/handler/metrics.go`
- Create: `internal/router/metrics.go`
- Modify: `internal/router/router.go`
- Modify: `internal/bootstrap/modules/handler.go`
- Modify: `internal/bootstrap/router.go`

- [ ] **Step 1: 创建 handler/metrics.go**

Create `internal/handler/metrics.go`:

```go
package handler

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/metrics"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)

// MetricsHandler 指标处理器
//
//	@author centonhuang
//	@update 2026-06-23 10:00:00
type MetricsHandler interface {
	HandleGetMetricsJSON(ctx context.Context, req *dto.EmptyReq) (*dto.HTTPResponse[*dto.MetricsJSONRsp], error)
}

// MetricsDependencies MetricsHandler 依赖项
//
//	@author centonhuang
//	@update 2026-06-23 10:00:00
type MetricsDependencies struct {
	Gatherer prometheus.Gatherer
}

type metricsHandler struct {
	gatherer prometheus.Gatherer
}

// NewMetricsHandler 创建指标处理器
//
//	@param deps MetricsDependencies
//	@return MetricsHandler
//	@author centonhuang
//	@update 2026-06-23 10:00:00
func NewMetricsHandler(deps MetricsDependencies) MetricsHandler {
	return &metricsHandler{gatherer: deps.Gatherer}
}

// HandleGetMetricsJSON 获取 Prometheus 指标的 JSON 表示
//
//	@receiver h *metricsHandler
//	@param ctx context.Context
//	@param req *dto.EmptyReq
//	@return *dto.HTTPResponse[*dto.MetricsJSONRsp]
//	@return error
//	@author centonhuang
//	@update 2026-06-23 10:00:00
func (h *metricsHandler) HandleGetMetricsJSON(ctx context.Context, _ *dto.EmptyReq) (*dto.HTTPResponse[*dto.MetricsJSONRsp], error) {
	families, err := metrics.GatherMetricFamilies(h.gatherer)
	if err != nil {
		logger.WithCtx(ctx).Error("[MetricsHandler] Gather metrics failed", zap.Error(err))
		rsp := &dto.MetricsJSONRsp{}
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	return apiutil.WrapHTTPResponse(&dto.MetricsJSONRsp{Metrics: families}, nil)
}
```

- [ ] **Step 2: 创建 router/metrics.go**

Create `internal/router/metrics.go`:

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

func initMetricsRouter(metricsGroup huma.API, metricsHandler handler.MetricsHandler, db *gorm.DB, cache *redis.Client, accessSigner jwt.TokenSigner) {
	metricsGroup.UseMiddleware(middleware.JwtMiddleware(db, cache, accessSigner))

	huma.Register(metricsGroup, huma.Operation{
		OperationID: "getMetricsJSON",
		Method:      http.MethodGet,
		Path:        "/metrics/json",
		Summary:     "GetMetricsJSON",
		Description: "Get Prometheus metrics in JSON format for dashboard consumption. Admin only.",
		Tags:        []string{constant.TagMonitor},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("getMetricsJSON", enum.PermissionAdmin)},
	}, metricsHandler.HandleGetMetricsJSON)
}
```

- [ ] **Step 3: 修改 router/router.go — 加入 MetricsHandler**

In `internal/router/router.go`, add to `APIRouterDependencies`:
```go
	MetricsHandler    handler.MetricsHandler
```

In `RegisterAPIRouter`, after the `auditGroup` block (around line 97), add:
```go
	metricsGroup := huma.NewGroup(v1Group, "/metrics")
	initMetricsRouter(metricsGroup, deps.MetricsHandler, deps.DB, deps.Cache, deps.AccessSigner)
```

- [ ] **Step 4: 修改 modules/handler.go — 注册 MetricsHandler**

In `internal/bootstrap/modules/handler.go`, add to imports:
```go
	"github.com/prometheus/client_golang/prometheus"
```

Add to `HandlerModule`'s `fx.Provide`:
```go
		NewMetricsDependencies,
		handler.NewMetricsHandler,
```

Add dependency provider:
```go
func NewMetricsDependencies(gatherer prometheus.Gatherer) handler.MetricsDependencies {
	return handler.MetricsDependencies{Gatherer: gatherer}
}
```

> 注意：`prometheus.Gatherer` 是接口，`*prometheus.Registry` 实现了它。dig 会自动将 `*prometheus.Registry` 注入为 `prometheus.Gatherer` 参数。

- [ ] **Step 5: 修改 bootstrap/router.go — 加 MetricsHandler 到 routeParams**

In `internal/bootstrap/router.go`, add to `routeParams`:
```go
	MetricsHandler    handler.MetricsHandler
```

In `registerRoutes`, add to `router.APIRouterDependencies{...}`:
```go
		MetricsHandler:    params.MetricsHandler,
```

- [ ] **Step 6: 验证编译**

Run: `go build ./...`
Expected: 编译通过

- [ ] **Step 7: Commit**

```bash
git add internal/handler/metrics.go internal/router/metrics.go internal/router/router.go internal/bootstrap/modules/handler.go internal/bootstrap/router.go
git commit -m "feat: add metrics JSON endpoint with admin auth"
```

---

## Task 6: SSE gauge 集成到 OpenAI/Anthropic handler

**Files:**
- Modify: `internal/handler/openai.go`
- Modify: `internal/handler/anthropic.go`
- Modify: `internal/bootstrap/modules/handler.go`

- [ ] **Step 1: 修改 handler/openai.go — 注入 SSEGauge + 包装 StreamResponse**

Add import: `"github.com/hcd233/aris-proxy-api/internal/infrastructure/metrics"`

Add `SSEGauge` field to `OpenAIDependencies`:
```go
type OpenAIDependencies struct {
	UseCase  port.OpenAIUseCase
	SSEGauge metrics.SSEGauge
}
```

Add `sseGauge` field to `openAIHandler`:
```go
type openAIHandler struct {
	uc       port.OpenAIUseCase
	sseGauge metrics.SSEGauge
}
```

Update `NewOpenAIHandler`:
```go
func NewOpenAIHandler(deps OpenAIDependencies) OpenAIHandler {
	return &openAIHandler{
		uc:       deps.UseCase,
		sseGauge: deps.SSEGauge,
	}
}
```

Update `HandleChatCompletion` to wrap the stream:
```go
func (h *openAIHandler) HandleChatCompletion(ctx context.Context, req *dto.OpenAIChatCompletionRequest) (*huma.StreamResponse, error) {
	rsp, err := h.uc.CreateChatCompletion(ctx, req)
	if err != nil || rsp == nil || rsp.Body == nil {
		return rsp, err
	}
	originalBody := rsp.Body
	rsp.Body = func(humaCtx huma.Context) {
		h.sseGauge.Inc("openai")
		defer h.sseGauge.Dec("openai")
		originalBody(humaCtx)
	}
	return rsp, nil
}
```

Update `HandleCreateResponse` similarly:
```go
func (h *openAIHandler) HandleCreateResponse(ctx context.Context, req *dto.OpenAICreateResponseRequest) (*huma.StreamResponse, error) {
	rsp, err := h.uc.CreateResponse(ctx, req)
	if err != nil || rsp == nil || rsp.Body == nil {
		return rsp, err
	}
	originalBody := rsp.Body
	rsp.Body = func(humaCtx huma.Context) {
		h.sseGauge.Inc("openai")
		defer h.sseGauge.Dec("openai")
		originalBody(humaCtx)
	}
	return rsp, nil
}
```

- [ ] **Step 2: 修改 handler/anthropic.go — 同样注入 + 包装**

Add import: `"github.com/hcd233/aris-proxy-api/internal/infrastructure/metrics"`

Add `SSEGauge` field to `AnthropicDependencies`:
```go
type AnthropicDependencies struct {
	UseCase  port.AnthropicUseCase
	SSEGauge metrics.SSEGauge
}
```

Add `sseGauge` field to `anthropicHandler`:
```go
type anthropicHandler struct {
	uc       port.AnthropicUseCase
	sseGauge metrics.SSEGauge
}
```

Update `NewAnthropicHandler`:
```go
func NewAnthropicHandler(deps AnthropicDependencies) AnthropicHandler {
	return &anthropicHandler{
		uc:       deps.UseCase,
		sseGauge: deps.SSEGauge,
	}
}
```

Update `HandleCreateMessage`:
```go
func (h *anthropicHandler) HandleCreateMessage(ctx context.Context, req *dto.AnthropicCreateMessageRequest) (*huma.StreamResponse, error) {
	rsp, err := h.uc.CreateMessage(ctx, req)
	if err != nil || rsp == nil || rsp.Body == nil {
		return rsp, err
	}
	originalBody := rsp.Body
	rsp.Body = func(humaCtx huma.Context) {
		h.sseGauge.Inc("anthropic")
		defer h.sseGauge.Dec("anthropic")
		originalBody(humaCtx)
	}
	return rsp, nil
}
```

- [ ] **Step 3: 修改 modules/handler.go — 注入 SSEGauge 到 deps**

Update `NewOpenAIDependencies`:
```go
func NewOpenAIDependencies(useCase usecase.OpenAIUseCase, sseGauge metrics.SSEGauge) handler.OpenAIDependencies {
	return handler.OpenAIDependencies{UseCase: &openAIUseCaseAdapter{inner: useCase}, SSEGauge: sseGauge}
}
```

Update `NewAnthropicDependencies`:
```go
func NewAnthropicDependencies(useCase usecase.AnthropicUseCase, sseGauge metrics.SSEGauge) handler.AnthropicDependencies {
	return handler.AnthropicDependencies{UseCase: &anthropicUseCaseAdapter{inner: useCase}, SSEGauge: sseGauge}
}
```

Add import in handler.go: `"github.com/hcd233/aris-proxy-api/internal/infrastructure/metrics"`

- [ ] **Step 4: 验证编译**

Run: `go build ./...`
Expected: 编译通过

- [ ] **Step 5: Commit**

```bash
git add internal/handler/openai.go internal/handler/anthropic.go internal/bootstrap/modules/handler.go
git commit -m "feat: integrate SSE gauge into OpenAI and Anthropic handlers"
```

---

## Task 7: 全量编译 + lint 验证

**Files:**
- None (verification only)

- [ ] **Step 1: 全量编译**

Run: `go build ./...`
Expected: 编译通过

- [ ] **Step 2: Lint**

Run: `make lint`
Expected: lint 通过

- [ ] **Step 3: 既有测试不受影响**

Run: `go test -count=1 ./test/unit/...`
Expected: 既有测试全部通过（新增的 metrics 测试也通过）

---

## Task 8: E2E 测试 — metrics 端点

**Files:**
- Create: `test/e2e/metrics/metrics_endpoint_test.go`

- [ ] **Step 1: 创建 E2E 测试文件**

Create `test/e2e/metrics/metrics_endpoint_test.go`:

```go
package metrics

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

const e2eHTTPTimeout = 30 * time.Second

type e2eClient struct {
	baseURL string
	http    *http.Client
}

func newE2EClient(t *testing.T) *e2eClient {
	t.Helper()
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		t.Skip("BASE_URL is required for e2e test")
	}
	return &e2eClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: e2eHTTPTimeout},
	}
}

func (c *e2eClient) get(path string) *http.Response {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		panic(err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		panic(err)
	}
	return resp
}

func (c *e2eClient) getWithAuth(path, token string) *http.Response {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.http.Do(req)
	if err != nil {
		panic(err)
	}
	return resp
}

func TestMetricsEndpoint_Returns200(t *testing.T) {
	client := newE2EClient(t)
	resp := client.get("/metrics")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestMetricsJSONEndpoint_RequiresAuth(t *testing.T) {
	client := newE2EClient(t)
	resp := client.get("/api/v1/metrics/json")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401 without token, got %d", resp.StatusCode)
	}
}

func TestMetricsJSONEndpoint_AdminReturnsMetrics(t *testing.T) {
	client := newE2EClient(t)
	adminToken := os.Getenv("ADMIN_TOKEN")
	if adminToken == "" {
		t.Skip("ADMIN_TOKEN is required for admin e2e test")
	}

	resp := client.getWithAuth("/api/v1/metrics/json", adminToken)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 with admin token, got %d", resp.StatusCode)
	}

	var result dto.HTTPResponse[dto.MetricsJSONRsp]
	if err := sonic.Unmarshal([]byte(readBody(resp)), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(result.Body.Metrics) == 0 {
		t.Fatal("expected non-empty metrics list")
	}

	names := make(map[string]bool)
	for _, m := range result.Body.Metrics {
		names[m.Name] = true
	}
	if !names["http_requests_total"] {
		t.Error("expected http_requests_total in metrics")
	}
	if !names["go_goroutines"] {
		t.Error("expected go_goroutines in metrics")
	}
}

func readBody(resp *http.Response) string {
	buf := make([]byte, 0, 1024*1024)
	tmp := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return string(buf)
}
```

- [ ] **Step 2: 运行 E2E 测试**

Run: `BASE_URL=http://localhost:8080 ADMIN_TOKEN=<token> go test -v -count=1 ./test/e2e/metrics/...`
Expected: PASS（无 token 的用例本地可跑，带 admin token 的需提供有效 token）

- [ ] **Step 3: Commit**

```bash
git add test/e2e/metrics/
git commit -m "test: add e2e tests for metrics and metrics/json endpoints"
```

---

## Task 9: 前端类型 + API Client

**Files:**
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/lib/api-client.ts`

- [ ] **Step 1: 在 types.ts 添加 Metrics 类型**

In `web/src/lib/types.ts`, add:

```typescript
export interface MetricSampleItem {
  labels?: Record<string, string>;
  value: number;
}

export interface MetricFamilyItem {
  name: string;
  type: string;
  help: string;
  samples?: MetricSampleItem[];
}

export interface MetricsJSONRsp {
  error?: import("./types").Error;
  metrics?: MetricFamilyItem[];
}
```

> 注意：如果 types.ts 中已有 `Error` 类型定义，直接引用即可。否则检查 `internal/common/model/error.go` 对应的前端类型。

- [ ] **Step 2: 在 api-client.ts 添加 getMetricsJSON 方法**

In `web/src/lib/api-client.ts`, add `MetricsJSONRsp` to the type imports at the top:

```typescript
  MetricsJSONRsp,
```

Add method to the `ApiClient` class (follow existing method patterns):

```typescript
  async getMetricsJSON(): Promise<MetricsJSONRsp> {
    return this.request("/api/v1/metrics/json", "GET");
  }
```

> 注意：`this.request` 是已有的私有方法封装，统一处理 auth 和 401 刷新。确认方法名与现有代码一致（可能是 `this.fetch` 或 `this.request`）。阅读 api-client.ts 的其他方法签名确认。

- [ ] **Step 3: 验证前端类型**

Run: `cd web && npx tsc --noEmit`
Expected: 无类型错误

- [ ] **Step 4: Commit**

```bash
git add web/src/lib/types.ts web/src/lib/api-client.ts
git commit -m "feat: add MetricsJSON types and api client method"
```

---

## Task 10: 前端监控页面 + 图表组件

**Files:**
- Create: `web/src/components/charts/runtime-gauge-card.tsx`
- Create: `web/src/components/charts/runtime-line-chart.tsx`
- Create: `web/src/app/(dashboard)/monitor/page.tsx`
- Modify: `web/src/app/(dashboard)/layout.tsx`

- [ ] **Step 1: 创建 runtime-gauge-card.tsx**

Create `web/src/components/charts/runtime-gauge-card.tsx`:

```tsx
"use client";

import { cn } from "@/lib/utils";

interface RuntimeGaugeCardProps {
  label: string;
  value: string | number;
  unit?: string;
  icon?: React.ReactNode;
}

export function RuntimeGaugeCard({
  label,
  value,
  unit,
  icon,
}: RuntimeGaugeCardProps) {
  return (
    <div className="rounded-xl border border-border bg-card p-4">
      <div className="flex items-center justify-between">
        <span className="text-sm text-muted-foreground">{label}</span>
        {icon && <span className="text-muted-foreground">{icon}</span>}
      </div>
      <div className="mt-2 flex items-baseline gap-1">
        <span className={cn("text-2xl font-semibold tabular-nums")}>
          {value}
        </span>
        {unit && <span className="text-sm text-muted-foreground">{unit}</span>}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: 创建 runtime-line-chart.tsx**

Create `web/src/components/charts/runtime-line-chart.tsx`:

```tsx
"use client";

import {
  CartesianGrid,
  Line,
  LineChart as RechartsLineChart,
  ResponsiveContainer,
  XAxis,
  YAxis,
} from "recharts";

import {
  ChartConfig,
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from "@/components/ui/chart";

interface RuntimeLineChartProps {
  data: { time: string; value: number }[];
  dataKey: string;
  label: string;
  color?: string;
  unit?: string;
}

export function RuntimeLineChart({
  data,
  dataKey,
  label,
  color = "var(--chart-1)",
  unit,
}: RuntimeLineChartProps) {
  const config: ChartConfig = {
    [dataKey]: { label, color },
  };

  return (
    <div className="rounded-xl border border-border bg-card p-4">
      <span className="mb-3 block text-sm font-medium">{label}</span>
      <ChartContainer config={config} className="h-[160px] w-full">
        <RechartsLineChart data={data} margin={{ left: 8, right: 8, top: 8 }}>
          <CartesianGrid vertical={false} strokeDasharray="3 3" />
          <XAxis
            dataKey="time"
            tickLine={false}
            axisLine={false}
            tickMargin={8}
            minTickGap={32}
          />
          <YAxis
            tickLine={false}
            axisLine={false}
            tickMargin={8}
            width={48}
          />
          <ChartTooltip
            content={
              <ChartTooltipContent
                labelKey={dataKey}
                indicator="line"
                formatter={(value) => [
                  `${value}${unit ?? ""}`,
                  label,
                ]}
              />
            }
          />
          <Line
            dataKey={dataKey}
            type="monotone"
            stroke={color}
            strokeWidth={2}
            dot={false}
            isAnimationActive={false}
          />
        </RechartsLineChart>
      </ChartContainer>
    </div>
  );
}
```

- [ ] **Step 3: 创建 monitor/page.tsx**

Create `web/src/app/(dashboard)/monitor/page.tsx`:

```tsx
"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Activity, Cpu, MemoryStick, Zap } from "lucide-react";

import { api } from "@/lib/api-client";
import type { MetricFamilyItem, MetricsJSONRsp } from "@/lib/types";
import { RuntimeGaugeCard } from "@/components/charts/runtime-gauge-card";
import { RuntimeLineChart } from "@/components/charts/runtime-line-chart";

const POLL_INTERVAL_MS = 5000;
const MAX_DATA_POINTS = 60;

interface TimeSeries {
  time: string;
  value: number;
}

interface MonitorState {
  goroutines: TimeSeries[];
  heapMB: TimeSeries[];
  inProgress: TimeSeries[];
  sseActive: TimeSeries[];
  cpuPercent: TimeSeries[];
  qps: TimeSeries[];
  p95Ms: TimeSeries[];
  gcPauseMs: TimeSeries[];
}

function nowLabel(): string {
  return new Date().toLocaleTimeString("en-US", {
    hour12: false,
    minute: "2-digit",
    second: "2-digit",
  });
}

function findMetric(
  families: MetricFamilyItem[],
  name: string,
): MetricFamilyItem | undefined {
  return families.find((f) => f.name === name);
}

function getGaugeValue(
  families: MetricFamilyItem[],
  name: string,
): number {
  const m = findMetric(families, name);
  return m?.samples?.[0]?.value ?? 0;
}

function getHistogramQuantile(
  families: MetricFamilyItem[],
  name: string,
  quantile: number,
): number {
  const m = findMetric(families, name);
  if (!m?.samples) return 0;
  // histogram samples have le=... labels; find the bucket where count >= quantile
  const buckets = m.samples
    .filter((s) => s.labels?.le !== undefined && s.labels?.le !== "+Inf")
    .map((s) => ({ le: parseFloat(s.labels!.le), count: s.value }))
    .sort((a, b) => a.le - b.le);
  const total = m.samples.find((s) => s.labels?.le === "+Inf")?.value ?? 0;
  if (total === 0 || buckets.length === 0) return 0;
  const target = total * quantile;
  for (let i = buckets.length - 1; i >= 0; i--) {
    if (buckets[i].count >= target) {
      return buckets[i].le * 1000; // convert to ms
    }
  }
  return 0;
}

export default function MonitorPage() {
  const [state, setState] = useState<MonitorState>({
    goroutines: [],
    heapMB: [],
    inProgress: [],
    sseActive: [],
    cpuPercent: [],
    qps: [],
    p95Ms: [],
    gcPauseMs: [],
  });
  const [currentValues, setCurrentValues] = useState({
    goroutines: 0,
    heapMB: 0,
    inProgress: 0,
    sseActive: 0,
  });
  const prevCpuRef = useRef<number | null>(null);
  const prevRequestCountRef = useRef<number | null>(null);
  const prevTimeRef = useRef<number | null>(null);

  const pushPoint = useCallback(
    (key: keyof MonitorState, time: string, value: number) => {
      setState((prev) => {
        const arr = [...prev[key], { time, value }];
        if (arr.length > MAX_DATA_POINTS) arr.shift();
        return { ...prev, [key]: arr };
      });
    },
    [],
  );

  useEffect(() => {
    const poll = async () => {
      try {
        const rsp: MetricsJSONRsp = await api.getMetricsJSON();
        const families = rsp.metrics ?? [];
        const time = nowLabel();
        const now = Date.now() / 1000;

        const goroutines = getGaugeValue(families, "go_goroutines");
        const heapBytes = getGaugeValue(families, "go_memstats_alloc_bytes");
        const heapMB = heapBytes / (1024 * 1024);
        const inProgress = getGaugeValue(families, "http_requests_in_progress");
        const sseActive = getGaugeValue(families, "sse_active_connections");
        const cpuTotal = getGaugeValue(families, "process_cpu_seconds_total");
        const gcPauseAvg = getGaugeValue(families, "go_gc_duration_seconds_sum");

        setCurrentValues({
          goroutines,
          heapMB: Math.round(heapMB * 100) / 100,
          inProgress,
          sseActive,
        });

        pushPoint("goroutines", time, goroutines);
        pushPoint("heapMB", time, Math.round(heapMB * 100) / 100);
        pushPoint("inProgress", time, inProgress);
        pushPoint("sseActive", time, sseActive);

        if (prevCpuRef.current !== null && prevTimeRef.current !== null) {
          const cpuDelta = cpuTotal - prevCpuRef.current;
          const timeDelta = now - prevTimeRef.current;
          if (timeDelta > 0) {
            const cpuPercent = (cpuDelta / timeDelta) * 100;
            pushPoint("cpuPercent", time, Math.round(cpuPercent * 100) / 100);
          }
        }
        prevCpuRef.current = cpuTotal;
        prevTimeRef.current = now;

        const requestTotal = getGaugeValue(families, "http_requests_total");
        if (prevRequestCountRef.current !== null && prevTimeRef.current !== null) {
          const reqDelta = requestTotal - prevRequestCountRef.current;
          const timeDelta = now - prevTimeRef.current;
          if (timeDelta > 0) {
            const qps = reqDelta / timeDelta;
            pushPoint("qps", time, Math.round(qps * 100) / 100);
          }
        }
        prevRequestCountRef.current = requestTotal;

        const p95 = getHistogramQuantile(families, "http_request_duration_seconds", 0.95);
        pushPoint("p95Ms", time, Math.round(p95));

        if (gcPauseAvg > 0) {
          pushPoint("gcPauseMs", time, Math.round(gcPauseAvg * 1000));
        }
      } catch {
        // silently ignore polling errors
      }
    };

    const interval = setInterval(poll, POLL_INTERVAL_MS);
    return () => clearInterval(interval);
  }, [pushPoint]);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Monitor</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Real-time runtime and business metrics (5s interval)
        </p>
      </div>

      <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
        <RuntimeGaugeCard
          label="Goroutines"
          value={currentValues.goroutines}
          icon={<Activity className="size-4" />}
        />
        <RuntimeGaugeCard
          label="Heap"
          value={currentValues.heapMB}
          unit="MB"
          icon={<MemoryStick className="size-4" />}
        />
        <RuntimeGaugeCard
          label="In-Progress"
          value={currentValues.inProgress}
          icon={<Zap className="size-4" />}
        />
        <RuntimeGaugeCard
          label="SSE Active"
          value={currentValues.sseActive}
          icon={<Cpu className="size-4" />}
        />
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <RuntimeLineChart
          data={state.cpuPercent}
          dataKey="cpuPercent"
          label="CPU Usage"
          unit="%"
        />
        <RuntimeLineChart
          data={state.heapMB}
          dataKey="heapMB"
          label="Heap Memory"
          unit=" MB"
        />
        <RuntimeLineChart
          data={state.qps}
          dataKey="qps"
          label="Request QPS"
        />
        <RuntimeLineChart
          data={state.p95Ms}
          dataKey="p95Ms"
          label="Latency P95"
          unit=" ms"
          color="var(--chart-2)"
        />
        <RuntimeLineChart
          data={state.goroutines}
          dataKey="goroutines"
          label="Goroutines"
          color="var(--chart-3)"
        />
        <RuntimeLineChart
          data={state.gcPauseMs}
          dataKey="gcPauseMs"
          label="GC Pause (avg)"
          unit=" ms"
          color="var(--chart-4)"
        />
      </div>
    </div>
  );
}
```

- [ ] **Step 4: 在 layout.tsx 添加 Monitor 导航入口**

In `web/src/app/(dashboard)/layout.tsx`, add `Activity` to the lucide-react import (line 38 area):

```typescript
  Activity,
```

Add to `navItems` array (before the Profile entry, around line 98):

```typescript
  {
    label: "Monitor",
    href: "/monitor/",
    icon: <Activity className="size-4" />,
    adminOnly: true,
  },
```

- [ ] **Step 5: 前端 lint + build**

Run:
```bash
cd web && npm run lint && npm run build
```
Expected: lint 和 build 通过

- [ ] **Step 6: Commit**

```bash
git add web/src/components/charts/runtime-gauge-card.tsx web/src/components/charts/runtime-line-chart.tsx web/src/app/\(dashboard\)/monitor/ web/src/app/\(dashboard\)/layout.tsx
git commit -m "feat: add monitor dashboard page with real-time metrics charts"
```

---

## Task 11: 全量验证

**Files:**
- None (verification only)

- [ ] **Step 1: 后端全量编译 + lint**

Run:
```bash
make build
make lint
```
Expected: 编译和 lint 通过

- [ ] **Step 2: 后端全量测试**

Run: `go test -count=1 ./test/unit/...`
Expected: 所有单元测试通过

- [ ] **Step 3: 前端 lint + build**

Run: `cd web && npm run lint && npm run build`
Expected: 通过

- [ ] **Step 4: 本地联调验证**

启动后端：`go run main.go server start --host localhost --port 8080`
启动前端：`cd web && NEXT_PUBLIC_API_BASE_URL=http://localhost:8080 npm run dev`

浏览器访问 `http://localhost:3000/web/`，登录管理员账号，点击侧栏 Monitor：
- 确认 4 个数字卡片显示数值
- 确认 6 个折线图开始渲染
- 确认 5s 后数据更新

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "feat: complete runtime metrics monitoring infrastructure"
```
