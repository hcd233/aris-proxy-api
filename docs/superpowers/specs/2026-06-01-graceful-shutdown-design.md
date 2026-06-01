# 强化后台优雅退出逻辑

**日期**: 2026-06-01
**状态**: 已批准

## 1. 背景

当前 `gracefulShutdown` 在收到 SIGINT/SIGTERM 后按序执行 6 步：HTTP shutdown → Logger sync → Pool stop → Cron stop → DB close → Redis close。

存在以下问题：

1. **SSE 流被粗暴切断**：Fiber `ShutdownWithContext` 会关闭活跃连接，正在进行的 SSE 流式响应会被直接断开，客户端收到不完整的响应。
2. **Cron/Pool 与 DB/Redis 的依赖未正确协调**：Cron/Pool 在使用 DB/Redis，但关停顺序中 Cron/Pool 在 HTTP 之后才停，如果 cron job 正在执行，Step 5/6 关闭 DB/Redis 会导致 job 失败。
3. **没有请求追踪机制**：无法知道当前有多少请求正在进行，也无法等待它们完成。
4. **K8s 流量未在退出前排空**：当前 `terminationGracePeriodSeconds: 30` 远不够优雅退出的时间，且没有 `preStop` hook，K8s 删除 Pod 时 SIGTERM 和 Endpoints 移除同时发生，iptables 规则传播有延迟导致 SIGTERM 后仍有新流量到达。
5. **readinessProbe 与 livenessProbe 共用 `/health`**：退出时无法让 `/health` 返回 503（否则 livenessProbe 失败导致 K8s 重启 Pod），也无法单独通知 K8s 不再路由新流量。

## 2. 目标

- 收到退出信号后，等待所有进行中的 API 请求（包括 SSE 流式响应）自然结束
- 等待当前正在执行的 cron job 完成
- 等待协程池中排队任务完成
- 超时后强制退出，避免无限等待
- 新请求在 draining 状态下返回 503
- K8s 滚动更新时，Pod 在退出前从 Service Endpoints 移除，不再接收新流量
- readinessProbe 和 livenessProbe 分离：draining 时 readinessProbe 失败（不再路由新流量），livenessProbe 始终通过（避免 K8s 重启 Pod）

## 3. 方案

采用 `sync.WaitGroup` + 请求追踪中间件方案。

### 3.1 新增 `internal/common/inflight` 包

提供全局请求追踪器，追踪所有进行中的 API 请求（包括 SSE）：

```go
type Tracker struct {
    wg    sync.WaitGroup
    state int32 // 0=running, 1=draining
}
```

核心方法：

| 方法 | 说明 |
|------|------|
| `InitTracker()` | 初始化全局 Tracker |
| `GetTracker()` | 获取全局 Tracker 实例 |
| `Track() bool` | 标记请求开始，draining 状态下返回 false |
| `Untrack()` | 标记请求结束（defer 调用） |
| `Drain(timeout) bool` | 进入 draining 状态，等待所有请求完成或超时，返回是否全部完成 |
| `IsDraining() bool` | 返回是否已进入 draining 状态 |

`Track()` 使用 `sync/atomic` 检查 state，如果已 draining 则返回 `false`。`Untrack()` 调用 `wg.Done()`。`Drain()` 调用 `wg.Wait()`（带 context 超时），确保所有进行中的请求完成。

### 3.2 新增 `InflightMiddleware`

放在 middleware 链中 `RecoverMiddleware` 之后：

```go
func InflightMiddleware() fiber.Handler {
    return func(c fiber.Ctx) error {
        tracker := inflight.GetTracker()
        if !tracker.Track() {
            return c.Status(fiber.StatusServiceUnavailable).JSON(...)
        }
        defer tracker.Untrack()
        return c.Next()
    }
}
```

- 对每个请求 `Track()` 开始、`Untrack()` 结束
- SSE 流式请求也被正确追踪：`defer Untrack()` 在整个 handler（包括 SSE 写循环）完成后才执行
- draining 时新请求返回 503

### 3.3 重新编排关停顺序

新顺序（8 步）：

| 步骤 | 操作 | 说明 |
|------|------|------|
| 1/8 | 停止 Cron | 不再启动新 job，等待当前 job 完成 |
| 2/8 | 停止 Pool | 不再接受新任务，等待排队任务完成 |
| 3/8 | 进入 Draining | 标记 `inflight.IsDraining()`，新请求返回 503 |
| 4/8 | 等待进行中请求完成 | `inflight.Drain(5min)`，等待所有活跃请求（含 SSE）自然结束 |
| 5/8 | 关闭 HTTP Server | `app.ShutdownWithContext()`，关闭监听端口和剩余连接 |
| 6/8 | 同步日志 | flush 日志缓冲 |
| 7/8 | 关闭数据库 | |
| 8/8 | 关闭 Redis | |

关键变化：

- **Cron/Pool 提前到 HTTP 关闭之前**：因为 cron/pool 使用 DB/Redis，需要先停掉这些"任务源"
- **新增 Draining 步骤**：先拒绝新请求，再等已有请求完成
- **等待请求完成后再调用 `ShutdownWithContext`**：确保 SSE 流不被粗暴切断
- **日志同步移到 HTTP 关闭之后**：确保 drain 等待期间的日志也能被 flush

### 3.4 超时层级调整

| 常量 | 值 | 说明 |
|------|-----|------|
| `ShutdownTimeout` | 10min | 整体关停超时（兜底） |
| `CronStopTimeout` | 3min | 等待 cron job 完成的超时 |
| `PoolStopTimeout` | 3min | 等待协程池任务完成的超时 |
| `InflightDrainTimeout` | 5min | 等待进行中请求完成的超时 |
| `FiberShutdownTimeout` | 30s | Fiber shutdown 本身的超时（drain 完成后应该很快） |

整体超时 = CronStopTimeout + PoolStopTimeout + InflightDrainTimeout + FiberShutdownTimeout + 余量 ≈ 10min

### 3.5 Cron/Pool Stop 增加超时保护

当前 `cron.StopCronJobs()` 是同步等待所有 cron 的 `<-ctx.Done()`，`pool.StopPoolManager()` 同理，两者都没有超时保护。需要增加超时：

Cron:

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
    case <-time.After(constant.CronStopTimeout):
        logger.Logger().Warn("[Cron] Cron stop timed out, some jobs may not have completed")
    }
    cronInstances = nil
    logger.Logger().Info("[Cron] All cron jobs stopped")
}
```

Pool:

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
            logger.Logger().Warn("[Pool] Pool stop timed out, some tasks may not have completed")
        }
    }
}
```

超时后 pond 的 Stop goroutine 仍在后台等待，整体 `ShutdownTimeout` 兜底会强制退出进程。

### 3.6 新增 `/ready` 端点（readinessProbe 专用）

当前 `/health` 同时服务 livenessProbe 和 readinessProbe。退出时需要让 K8s 停止路由新流量，但不能让 livenessProbe 失败（否则 K8s 会重启 Pod）。

方案：新增 `/ready` 端点给 readinessProbe，`/health` 只给 livenessProbe。

```go
// HandleReady 就绪检查处理器
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

路由注册：

```go
huma.Register(healthGroup, huma.Operation{
    OperationID: "readinessCheck",
    Method:      http.MethodGet,
    Path:        "/ready",
    Summary:     "ReadinessCheck",
    Description: "Check if the server is ready to accept traffic",
    Tags:        []string{"Health"},
}, pingHandler.HandleReady)
```

注意：`/ready` 和 `/health` 都不受 `InflightMiddleware` 影响（不需要 Track/Untrack），因为它们是基础设施探测端点。`InflightMiddleware` 需要跳过这些路径。

### 3.7 K8s Deployment 联动

当前 `k8s/deployment.yaml` 存在两个问题：

1. **`terminationGracePeriodSeconds: 30`**：远不够 10 分钟的优雅退出时间
2. **没有 `preStop` hook**：K8s 删除 Pod 时 Endpoints 移除和 SIGTERM 同时发生，iptables 规则传播有延迟，SIGTERM 后仍有新流量到达

标准 K8s 优雅退出时序：

```
K8s 删除 Pod
  → 1. 从 Endpoints 移除 Pod（异步，iptables 规则传播需数秒）
  → 2. 执行 preStop hook（阻塞，等待流量排空）
  → 3. preStop 完成后发送 SIGTERM
  → 4. 应用开始 gracefulShutdown
  → 5. terminationGracePeriodSeconds 到期后 SIGKILL
```

变更内容：

1. **添加 `preStop` hook**：`sleep 10`，给 K8s 10 秒时间传播 iptables 规则。期间应用正常运行，只是 K8s 不再路由新流量到此 Pod。

2. **`terminationGracePeriodSeconds` 对齐**：设为 `660`（11 分钟）= preStop sleep 10s + 应用关停 10min + 余量。

3. **readinessProbe 改为 `/ready`**：draining 时返回 503，K8s 不再路由新流量。livenessProbe 仍用 `/health`，始终 200。

修改后的 Deployment 关键部分：

```yaml
spec:
  terminationGracePeriodSeconds: 660
  containers:
  - name: app
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
```

### 3.8 InflightMiddleware 跳过健康检查路径

`/health`、`/ready`、`/ssehealth` 是基础设施探测端点，不应被 `InflightMiddleware` 追踪（否则退出时这些端点也会被 Track 拒绝）。

```go
func InflightMiddleware() fiber.Handler {
    return func(c fiber.Ctx) error {
        path := c.Path()
        if path == constant.RoutePathHealth || path == constant.RoutePathReady || path == constant.RoutePathSSEHealth {
            return c.Next()
        }
        tracker := inflight.GetTracker()
        if !tracker.Track() {
            // ... 503
        }
        defer tracker.Untrack()
        return c.Next()
    }
}
```

### 3.9 基础设施初始化调整

`inflight.InitTracker()` 需要在 `bootstrap.InitInfrastructure()` 中调用，确保在 HTTP 服务启动前就初始化完成。

## 4. 涉及文件

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `internal/common/inflight/tracker.go` | 新增 | 请求追踪器 |
| `internal/middleware/inflight.go` | 新增 | InflightMiddleware（跳过健康检查路径） |
| `internal/common/constant/http.go` | 修改 | 新增超时常量 |
| `internal/common/constant/route.go` | 修改 | 新增 `RoutePathReady` |
| `internal/handler/ping.go` | 修改 | 新增 `HandleReady` 方法 |
| `internal/dto/ping.go` | 修改 | 无变化（复用 PingRsp） |
| `internal/router/health.go` | 修改 | 注册 `/ready` 路由 |
| `internal/bootstrap/container.go` | 修改 | InitInfrastructure 中初始化 Tracker |
| `internal/cron/cron.go` | 修改 | StopCronJobs 增加超时 |
| `internal/infrastructure/pool/pool.go` | 修改 | StopPoolManager 增加超时 |
| `cmd/server.go` | 修改 | 中间件注册 + gracefulShutdown 重编排 |
| `k8s/deployment.yaml` | 修改 | preStop hook + terminationGracePeriodSeconds + readinessProbe 改 /ready |

## 5. 不涉及的变更

- 不修改 SSE handler 逻辑
- 不修改 cron job 的业务逻辑
- 不修改 pool 的 Stop 逻辑（pond.Pool.Stop() 本身会等待）
- 不修改 Fiber 配置
- 不修改 livenessProbe 的行为（始终返回 200）

## 6. 验证标准

1. 退出时日志按 8 步顺序输出
2. SSE 流式请求在退出信号后能正常完成（不被切断）
3. 退出信号后新请求返回 503
4. cron job 在退出时能完成当前执行中的任务
5. 超时后能强制退出
6. `/ready` 在 draining 时返回 503，`/health` 始终返回 200
7. K8s 滚动更新时旧 Pod 在 preStop 期间从 Endpoints 移除，不再接收新流量
8. `make lint` 和 `go test -count=1 ./...` 通过
