# 强化后台优雅退出 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 收到退出信号后等待所有进行中的 API 请求（含 SSE）和定时任务自然完成，K8s 滚动更新时 Pod 退出前排空流量，超时后强制退出。

**Architecture:** 新增 `internal/common/inflight` 包提供 `sync.WaitGroup` + `atomic` 状态的全局追踪器；新增 `InflightMiddleware` 在请求生命周期内 Track/Untrack（跳过健康检查路径）；新增 `/ready` 端点给 readinessProbe（draining 时 503）；重新编排 `gracefulShutdown` 关停顺序为 Cron → Pool → Draining → Drain → HTTP shutdown → Logger → DB → Redis；K8s Deployment 增加 preStop hook 和对齐 terminationGracePeriodSeconds。

**Tech Stack:** Go 1.25, sync.WaitGroup, sync/atomic, Fiber v3, Huma, robfig/cron/v3, alitto/pond/v2, K8s

---

### Task 1: 新增超时常量

**Files:**
- Modify: `internal/common/constant/http.go:67-69`

- [ ] **Step 1: 修改 `internal/common/constant/http.go`，新增超时常量**

```go
	IdleTimeout             = 2 * time.Minute
	ShutdownTimeout         = 10 * time.Minute
	CronStopTimeout         = 3 * time.Minute
	PoolStopTimeout         = 3 * time.Minute
	InflightDrainTimeout    = 5 * time.Minute
	FiberShutdownTimeout    = 30 * time.Second
```

将 `ShutdownTimeout` 从 `60 * time.Second` 改为 `10 * time.Minute`，新增 `CronStopTimeout`、`PoolStopTimeout`、`InflightDrainTimeout`。

- [ ] **Step 2: 验证编译通过**

Run: `go build ./...`
Expected: 编译成功

- [ ] **Step 3: Commit**

```bash
git add internal/common/constant/http.go
git commit -m "feat(shutdown): add cron/pool/drain timeout constants"
```

---

### Task 2: 新增 inflight 追踪器

**Files:**
- Create: `internal/common/inflight/tracker.go`
- Test: `test/unit/inflight/inflight_test.go`

- [ ] **Step 1: 创建 `internal/common/inflight/tracker.go`**

```go
package inflight

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)

const (
	stateRunning  int32 = 0
	stateDraining int32 = 1
)

type Tracker struct {
	wg    sync.WaitGroup
	state atomic.Int32
}

var globalTracker *Tracker

func InitTracker() *Tracker {
	t := &Tracker{}
	t.state.Store(stateRunning)
	globalTracker = t
	return t
}

func GetTracker() *Tracker {
	return globalTracker
}

func (t *Tracker) Track() bool {
	if t.state.Load() == stateDraining {
		return false
	}
	t.wg.Add(1)
	if t.state.Load() == stateDraining {
		t.wg.Done()
		return false
	}
	return true
}

func (t *Tracker) Untrack() {
	t.wg.Done()
}

func (t *Tracker) Drain(timeout time.Duration) bool {
	t.state.Store(stateDraining)

	done := make(chan struct{})
	go func() {
		defer close(done)
		t.wg.Wait()
	}()

	select {
	case <-done:
		logger.Logger().Info("[Inflight] All inflight requests completed")
		return true
	case <-time.After(timeout):
		logger.Logger().Warn("[Inflight] Drain timed out, some requests may not have completed",
			zap.Duration("timeout", timeout))
		return false
	}
}

func (t *Tracker) IsDraining() bool {
	return t.state.Load() == stateDraining
}
```

- [ ] **Step 2: 编写测试 `test/unit/inflight/inflight_test.go`**

```go
package inflight_test

import (
	"sync"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/inflight"
)

func TestTracker_TrackAndUntrack(t *testing.T) {
	tracker := inflight.InitTracker()

	if !tracker.Track() {
		t.Fatal("Track should succeed when running")
	}

	tracker.Untrack()

	done := make(chan struct{})
	go func() {
		defer close(done)
		tracker.Drain(time.Second)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Drain should complete quickly when no inflight requests")
	}
}

func TestTracker_TrackReturnsFalseDuringDraining(t *testing.T) {
	tracker := inflight.InitTracker()

	tracker.Track()

	go func() {
		time.Sleep(50 * time.Millisecond)
		tracker.Untrack()
	}()

	drained := make(chan bool, 1)
	go func() {
		drained <- tracker.Drain(2 * time.Second)
	}()

	time.Sleep(100 * time.Millisecond)

	if tracker.Track() {
		t.Fatal("Track should return false during draining")
	}

	<-drained
}

func TestTracker_DrainTimeout(t *testing.T) {
	tracker := inflight.InitTracker()

	tracker.Track()

	drained := make(chan bool, 1)
	go func() {
		drained <- tracker.Drain(100 * time.Millisecond)
	}()

	result := <-drained
	if result {
		t.Fatal("Drain should return false on timeout")
	}

	tracker.Untrack()
}

func TestTracker_ConcurrentTrackUntrack(t *testing.T) {
	tracker := inflight.InitTracker()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if tracker.Track() {
				tracker.Untrack()
			}
		}()
	}
	wg.Wait()

	done := make(chan struct{})
	go func() {
		defer close(done)
		tracker.Drain(time.Second)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Drain should complete after all Track/Untrack pairs resolve")
	}
}
```

- [ ] **Step 3: 运行测试验证通过**

Run: `go test -v -count=1 -run TestTracker ./test/unit/inflight/`
Expected: 4 个测试全部 PASS

- [ ] **Step 4: Commit**

```bash
git add internal/common/inflight/tracker.go test/unit/inflight/inflight_test.go
git commit -m "feat(shutdown): add inflight request tracker with WaitGroup"
```

---

### Task 3: 新增 `/ready` 端点 + 路由常量

**Files:**
- Modify: `internal/common/constant/route.go:6`
- Modify: `internal/handler/ping.go:25-27`
- Modify: `internal/router/health.go:15-33`

- [ ] **Step 1: 修改 `internal/common/constant/route.go`，新增 `RoutePathReady`**

```go
	RoutePathHealth                       = "/health"
	RoutePathReady                        = "/ready"
	RoutePathSSEHealth                    = "/ssehealth"
```

- [ ] **Step 2: 修改 `internal/handler/ping.go`，新增 `HandleReady`**

在 `import` 中新增 `"github.com/hcd233/aris-proxy-api/internal/common/inflight"` 和 `"github.com/danielgtaylor/huma/v2"`（huma 已导入）。

修改 `PingHandler` 接口，新增 `HandleReady`：

```go
type PingHandler interface {
	HandlePing(ctx context.Context, req *dto.EmptyReq) (rsp *dto.HTTPResponse[*dto.PingRsp], err error)
	HandleReady(ctx context.Context, req *dto.EmptyReq) (rsp *dto.HTTPResponse[*dto.PingRsp], err error)
	HandleSSEPing(ctx context.Context, req *dto.EmptyReq) (rsp *huma.StreamResponse, err error)
}
```

新增 `HandleReady` 方法实现：

```go
func (h *pingHandler) HandleReady(_ context.Context, _ *dto.EmptyReq) (*dto.HTTPResponse[*dto.PingRsp], error) {
	tracker := inflight.GetTracker()
	if tracker.IsDraining() {
		return nil, huma.Error503ServiceUnavailable("server is shutting down")
	}
	rsp := &dto.PingRsp{
		Status: constant.PingStatusOK,
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}
```

- [ ] **Step 3: 修改 `internal/router/health.go`，注册 `/ready` 路由**

在 `import` 中新增 `"github.com/hcd233/aris-proxy-api/internal/common/constant"`。

在 `initHealthRouter` 函数中，在 `/ssehealth` 注册之前新增：

```go
	huma.Register(healthGroup, huma.Operation{
		OperationID: "readinessCheck",
		Method:      http.MethodGet,
		Path:        constant.RoutePathReady,
		Summary:     "ReadinessCheck",
		Description: "Check if the server is ready to accept traffic",
		Tags:        []string{"Health"},
	}, pingHandler.HandleReady)
```

- [ ] **Step 4: 验证编译通过**

Run: `go build ./...`
Expected: 编译成功

- [ ] **Step 5: Commit**

```bash
git add internal/common/constant/route.go internal/handler/ping.go internal/router/health.go
git commit -m "feat(shutdown): add /ready endpoint for readinessProbe"
```

---

### Task 4: 新增 InflightMiddleware（跳过健康检查路径）

**Files:**
- Create: `internal/middleware/inflight.go`

- [ ] **Step 1: 创建 `internal/middleware/inflight.go`**

```go
package middleware

import (
	"github.com/bytedance/sonic"
	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/inflight"
)

type serviceUnavailableResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

var healthCheckPaths = map[string]struct{}{
	constant.RoutePathHealth:    {},
	constant.RoutePathReady:     {},
	constant.RoutePathSSEHealth: {},
}

func InflightMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		if _, skip := healthCheckPaths[c.Path()]; skip {
			return c.Next()
		}

		tracker := inflight.GetTracker()
		if !tracker.Track() {
			c.Set("Content-Type", "application/json")
			c.Status(fiber.StatusServiceUnavailable)

			resp := serviceUnavailableResponse{}
			resp.Error.Message = "server is shutting down"
			resp.Error.Type = "server_error"

			body, _ := sonic.Marshal(resp)
			return c.Send(body)
		}
		defer tracker.Untrack()
		return c.Next()
	}
}
```

- [ ] **Step 2: 验证编译通过**

Run: `go build ./...`
Expected: 编译成功

- [ ] **Step 3: Commit**

```bash
git add internal/middleware/inflight.go
git commit -m "feat(shutdown): add InflightMiddleware to track requests, skip health paths"
```

---

### Task 5: 初始化 Tracker + 注册中间件

**Files:**
- Modify: `internal/bootstrap/container.go:72-78`
- Modify: `cmd/server.go:66-91`

- [ ] **Step 1: 修改 `internal/bootstrap/container.go`，在 `InitInfrastructure` 中初始化 Tracker**

在 `import` 中新增 `"github.com/hcd233/aris-proxy-api/internal/common/inflight"`。

```go
func InitInfrastructure() *Infrastructure {
	db := database.InitDatabase()
	cache := cache.InitCache()
	httpclient.InitHTTPClient()
	inflight.InitTracker()
	poolManager := pool.InitPoolManager(db)
	cron.InitCronJobs(db, poolManager)
	return &Infrastructure{DB: db, Cache: cache, PoolManager: poolManager}
}
```

- [ ] **Step 2: 修改 `cmd/server.go`，在中间件链中注册 `InflightMiddleware`**

在 `RecoverMiddleware()` 之后加入 `InflightMiddleware()`：

```go
	app.Use(
		middleware.RecoverMiddleware(),
		middleware.InflightMiddleware(),
		middleware.GuardMiddleware(infra.Cache, middleware.GuardConfig{
```

- [ ] **Step 3: 验证编译通过**

Run: `go build ./...`
Expected: 编译成功

- [ ] **Step 4: Commit**

```bash
git add internal/bootstrap/container.go cmd/server.go
git commit -m "feat(shutdown): init Tracker and register InflightMiddleware"
```

---

### Task 6: 给 StopCronJobs 增加超时保护

**Files:**
- Modify: `internal/cron/cron.go:88-98`

- [ ] **Step 1: 修改 `internal/cron/cron.go` 中的 `StopCronJobs`**

确认 `import` 中已有 `"time"` 和 `"github.com/hcd233/aris-proxy-api/internal/common/constant"`。

```go
func StopCronJobs() {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, c := range cronInstances {
			c.Stop()
		}
	}()

	select {
	case <-done:
		logger.Logger().Info("[Cron] All cron jobs stopped")
	case <-time.After(constant.CronStopTimeout):
		logger.Logger().Warn("[Cron] Cron stop timed out, some jobs may not have completed",
			zap.Duration("timeout", constant.CronStopTimeout))
	}
	cronInstances = nil
}
```

- [ ] **Step 2: 运行现有 cron 测试**

Run: `go test -v -count=1 -run TestInitCronJobs ./test/unit/cron/`
Expected: 全部 PASS

- [ ] **Step 3: Commit**

```bash
git add internal/cron/cron.go
git commit -m "feat(shutdown): add timeout protection to StopCronJobs"
```

---

### Task 7: 给 StopPoolManager 增加超时保护

**Files:**
- Modify: `internal/infrastructure/pool/pool.go:62-70`

- [ ] **Step 1: 修改 `internal/infrastructure/pool/pool.go` 中的 `StopPoolManager`**

在 `import` 中新增 `"time"` 和 `"github.com/hcd233/aris-proxy-api/internal/logger"`。

```go
func StopPoolManager() {
	if poolManager != nil {
		done := make(chan struct{})
		go func() {
			defer close(done)
			poolManager.Stop()
		}()

		select {
		case <-done:
			logger.Logger().Info("[Pool] Pool manager stopped")
		case <-time.After(constant.PoolStopTimeout):
			logger.Logger().Warn("[Pool] Pool stop timed out, some tasks may not have completed",
				zap.Duration("timeout", constant.PoolStopTimeout))
		}
	}
}
```

- [ ] **Step 2: 运行现有 pool 测试**

Run: `go test -v -count=1 ./test/unit/pool_manager/`
Expected: 全部 PASS

- [ ] **Step 3: Commit**

```bash
git add internal/infrastructure/pool/pool.go
git commit -m "feat(shutdown): add timeout protection to StopPoolManager"
```

---

### Task 8: 重新编排 gracefulShutdown

**Files:**
- Modify: `cmd/server.go:122-181`

- [ ] **Step 1: 修改 `cmd/server.go` 中的 `gracefulShutdown` 函数**

更新 `import`：新增 `"github.com/hcd233/aris-proxy-api/internal/common/inflight"` 和 `"time"`。

```go
func gracefulShutdown(app *fiber.App, infra *bootstrap.Infrastructure) {
	ctx, cancel := context.WithTimeout(context.Background(), constant.ShutdownTimeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)

		// Step 1: 停止定时任务（等待当前 job 完成）
		logger.Logger().Info("[Server] Step 1/8: Stopping cron jobs...")
		cron.StopCronJobs()

		// Step 2: 停止协程池（等待排队任务完成）
		logger.Logger().Info("[Server] Step 2/8: Stopping pool manager...")
		pool.StopPoolManager()

		// Step 3: 进入 draining 状态，拒绝新请求
		logger.Logger().Info("[Server] Step 3/8: Entering draining state...")
		tracker := inflight.GetTracker()

		// Step 4: 等待进行中请求完成（含 SSE 流）
		logger.Logger().Info("[Server] Step 4/8: Waiting for inflight requests to complete...")
		tracker.Drain(constant.InflightDrainTimeout)

		// Step 5: 关闭 HTTP Server
		logger.Logger().Info("[Server] Step 5/8: Shutting down HTTP server...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), constant.FiberShutdownTimeout)
		defer shutdownCancel()
		if err := app.ShutdownWithContext(shutdownCtx); err != nil {
			logger.Logger().Error("[Server] HTTP server shutdown error", zap.Error(err))
		}

		// Step 6: 同步日志（flush CLS 等外部日志缓冲）
		logger.Logger().Info("[Server] Step 6/8: Syncing logger...")
		if err := logger.Logger().Sync(); err != nil {
			logger.Logger().Error("[Server] Logger sync error", zap.Error(err))
		}

		// Step 7: 关闭数据库连接池
		logger.Logger().Info("[Server] Step 7/8: Closing database connection...")
		if err := database.CloseDatabase(infra.DB); err != nil {
			logger.Logger().Error("[Server] Database close error", zap.Error(err))
		}

		// Step 8: 关闭 Redis 连接
		logger.Logger().Info("[Server] Step 8/8: Closing Redis connection...")
		if err := cache.CloseCache(infra.Cache); err != nil {
			logger.Logger().Error("[Server] Redis close error", zap.Error(err))
		}

		logger.Logger().Info("[Server] Graceful shutdown completed")
	}()

	select {
	case <-done:
	case <-ctx.Done():
		logger.Logger().Error("[Server] Graceful shutdown timed out, forcing exit", zap.Duration("timeout", constant.ShutdownTimeout))
	}
}
```

- [ ] **Step 2: 验证编译通过**

Run: `go build ./...`
Expected: 编译成功

- [ ] **Step 3: Commit**

```bash
git add cmd/server.go
git commit -m "feat(shutdown): reorder graceful shutdown to drain inflight requests before HTTP close"
```

---

### Task 9: K8s Deployment 联动

**Files:**
- Modify: `k8s/deployment.yaml`

- [ ] **Step 1: 修改 `k8s/deployment.yaml`**

关键变更：`terminationGracePeriodSeconds: 30` → `660`，新增 `preStop` hook，readinessProbe 路径 `/health` → `/ready`。

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: aris-proxy-api
  namespace: aris-proxy-api
  labels:
    app: aris-proxy-api
spec:
  replicas: 2
  revisionHistoryLimit: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 0
      maxSurge: 1
  selector:
    matchLabels:
      app: aris-proxy-api
  template:
    metadata:
      labels:
        app: aris-proxy-api
    spec:
      terminationGracePeriodSeconds: 660
      containers:
      - name: app
        image: ghcr.io/hcd233/aris-proxy-api:PLACEHOLDER
        imagePullPolicy: IfNotPresent
        command: ["/app/aris-proxy-api", "server", "start", "--host", "0.0.0.0", "--port", "8080"]
        ports:
        - name: http
          containerPort: 8080
        envFrom:
        - configMapRef:
            name: aris-proxy-api-config
        - secretRef:
            name: aris-proxy-api-secret
        volumeMounts:
        - name: logs
          mountPath: /app/logs
        lifecycle:
          preStop:
            exec:
              command: ["/bin/sh", "-c", "sleep 10"]
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 15
          periodSeconds: 20
          timeoutSeconds: 3
          failureThreshold: 3
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
          timeoutSeconds: 3
          failureThreshold: 6
        resources:
          requests:
            cpu: 50m
            memory: 128Mi
          limits:
            cpu: 300m
            memory: 512Mi
      volumes:
      - name: logs
        emptyDir:
          sizeLimit: 512Mi
```

- [ ] **Step 2: Commit**

```bash
git add k8s/deployment.yaml
git commit -m "feat(shutdown): add preStop hook, align terminationGracePeriodSeconds, use /ready for readinessProbe"
```

---

### Task 10: 全量验证

- [ ] **Step 1: 运行 lint**

Run: `make lint`
Expected: 无错误

- [ ] **Step 2: 运行全量测试**

Run: `go test -count=1 ./...`
Expected: 全部 PASS

- [ ] **Step 3: 运行 inflight 单元测试**

Run: `go test -v -count=1 ./test/unit/inflight/`
Expected: 全部 PASS
