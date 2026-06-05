# dig → fx 迁移设计文档

## 元数据

- **日期**: 2026-06-05
- **类型**: 重构
- **范围**: full（DI 容器替换 + 生命周期下沉 + 包级单例消除）

## 目标

将项目依赖注入从 `go.uber.org/dig` 迁移到 `go.uber.org/fx`，消除手动生命周期管理代码和包级全局变量，提高可维护性与可测试性。

## 当前架构

```
cmd/server.go (191 lines)
  ├── cobra.Command.Run
  ├── InitInfrastructure() ─── 初始化 DB/Cache/HTTPClient/Inflight/Pool/Cron
  ├── BuildServer() ─── dig.New() + provide() + Invoke()
  ├── app.Use(中间件链条)
  ├── RegisterRoutes() ─── Invoke 提取 handler, 注册路由
  ├── app.Listen() (goroutine)
  ├── signal.Notify(SIGINT/SIGTERM)
  └── gracefulShutdown() ─── 8步顺序关闭 + 10min 总超时
```

核心文件：
- `internal/bootstrap/container.go`（886 行）：dig 容器构建，~70 个 provider 函数
- `cmd/server.go`（191 行）：启动、信号监听、8 步优雅关闭

## 目标架构

```
cmd/server.go (~30 lines)
  ├── cobra.Command.Run
  └── bootstrap.BuildFxApp().Start() ─── 单行，阻塞等待，自动 graceful shutdown

internal/bootstrap/
  ├── container.go ─── BuildFxApp(): fx.New(modules...)
  ├── lifecycle.go ─── 集中注册所有 OnStart/OnStop hooks，保证 8 步顺序
  └── modules/
      ├── infra.go        ─── fx.Module("infrastructure", DB/Cache/Pool/Inflight)
      ├── cron.go         ─── fx.Module("cron", cron 启动 + 停止)
      ├── repository.go   ─── fx.Module("repositories", 所有 repository)
      ├── application.go  ─── fx.Module("application", 所有 usecase/command/query)
      └── router.go       ─── fx.Module("router", handlers + route registration)
```

## api 对比

| dig API | fx 等价 |
|---------|---------|
| `dig.New()` | `fx.New(...)` |
| `container.Provide(fn)` | `fx.Provide(fn)` |
| `container.Invoke(fn)` | `fx.Invoke(fn)` |
| `dig.In` struct tag | `fx.In`（完全兼容） |
| `dig.Out` struct tag | `fx.Out`（完全兼容） |
| `dig.Name("xxx")` | `fx.ResultTags(`name:"xxx"`)` |
| `dig.As(...)` | `fx.As(...)`（完全兼容） |

## 生命周期 Hook 时序

关闭顺序复用现有 8 步逻辑，通过集中式 `fx.Invoke` 手动注册 hook：

```
注册顺序: Redis → DB → Logger → HTTP → Inflight → Pool → Cron
         ↓
OnStart 顺序: Redis → DB → Logger → HTTP → Inflight → Pool → Cron
OnStop 逆序:  Cron → Pool → Inflight → HTTP → Logger → DB → Redis
```

即：
1. 停止 Cron（timeout: 3min）
2. 停止 Pool（timeout: 3min）
3. Inflight Drain + /ready 返回 503（timeout: 5min）
4. 关闭 Fiber HTTP（timeout: 30s）
5. Sync Logger
6. 关闭 DB
7. 关闭 Redis
总超时: fx.StopTimeout(11min)

各步骤内部使用 `context.WithTimeout` 保留逐步超时。K8s `preStop: sleep 10` + `/ready` 探针配合保持不变。

## 包级 Singleton 消除

| 变量 | 调用方 | 策略 |
|------|--------|------|
| `pool.poolManager` | 19 处 | 改为 fx provider，`GetPoolManager()` 删除 |
| `inflight.tracker` | 3 处 | 改为 fx provider，`GetTracker()` 删除 |
| `jwt.*Signer` | 已通过 dig.Name 注入 | 删除 Get 函数，改为直接构造 |
| `cron.cronInstances` | 2 处 | fx 管理 cron 实例切片 |
| `config.*` | 全项目 | **不动**，静态配置不属 DI 范畴 |

## 文件改动清单

| 文件 | 改动 | 净行数 |
|------|------|--------|
| `cmd/server.go` | 大幅精简 | -150 / +20 |
| `internal/bootstrap/container.go` | 重写为 fx | -886 / +150 |
| `internal/bootstrap/lifecycle.go` | **新建** | +80 |
| `internal/bootstrap/modules/infra.go` | **新建** | +80 |
| `internal/bootstrap/modules/cron.go` | **新建** | +50 |
| `internal/bootstrap/modules/repository.go` | **新建** | +60 |
| `internal/bootstrap/modules/application.go` | **新建** | +60 |
| `internal/bootstrap/modules/router.go` | **新建** | +50 |
| `internal/infrastructure/pool/pool.go` | 删单例 + StopWithContext | ±15 |
| `internal/infrastructure/pool/agent_pool.go` | 构造函数注入 | ±5 |
| `internal/infrastructure/pool/store_pool.go` | 构造函数注入 | ±3 |
| `internal/common/inflight/tracker.go` | 删 GetTracker/InitTracker | -10 / +5 |
| `internal/infrastructure/jwt/*.go` | 删 Get*Signer | -15 |
| `internal/cron/cron.go` | 删 InitCronJobs/StopCronJobs | -40 / +15 |
| `internal/middleware/inflight.go` | Tracker 注入 | ±3 |
| `internal/router/*.go` | 依赖传递 | ±5 |
| `test/e2e/` | 适配新启动 | ±20 |
| `test/unit/` | 适配新注入 | ±30 |

总净改动：~+600 / -1000，净删除 ~400 行。

## 风险与缓解

| 风险 | 缓解 |
|------|------|
| Inflight 与 HTTP 关闭时序错乱 | 集中式 lifecycle.go 中手工编排 hook，保留逐步超时 |
| `dig.Name` → `fx.ResultTags` 遗漏 | 仅 2 处命名注入，编译期即可验证 |
| 测试断裂 | `fxtest` 可完全控制每个测试的依赖图 |
| 部署不兼容 | K8s preStop + /ready 探针逻辑不变 |

## 验证标准

1. `make build` 编译通过
2. `make lint` 零告警
3. `go test -count=1 ./...` 全量测试通过
4. `test/e2e/` 全部通过
5. 手动验证：`go run main.go server start` 启动成功，Ctrl+C 触发 8 步关闭日志正确输出
