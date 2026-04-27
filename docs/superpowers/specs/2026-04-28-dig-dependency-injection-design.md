# Dig 依赖注入设计

## 目标

引入 `go.uber.org/dig` 作为应用启动阶段的依赖注入机制。迁移后，对象构造集中在组合根中完成，减少路由层分散的手动依赖创建，并保持当前运行行为和优雅关闭顺序不变。

## 范围

本次变更覆盖 HTTP API 启动链路的依赖组装：

- 在模块依赖中加入 `go.uber.org/dig`。
- 新增组合根包，负责创建和持有 `dig.Container`。
- 通过容器注册 Fiber、Huma、repository、transport、handler 和路由注册入口。
- 将 router 层的 `NewXxx()` 依赖构造迁移到 provider 注册中。
- 保持请求处理、业务逻辑、DTO 和公开 API 行为不变。

本次变更不引入运行时 service locator。容器只在启动阶段使用。

## 非目标

- 不重写 repository 和 database 内部逻辑，也不在本次迁移中移除现有数据库、Redis 单例访问方式。
- 不改变请求或响应协议。
- 不引入新的生命周期框架，继续沿用当前显式初始化和显式关闭流程。
- 不做无关业务逻辑重构。

## 架构

新增组合根包，暂定为 `internal/bootstrap`，提供类似以下函数：

- `NewContainer() *dig.Container`
- `Provide(container *dig.Container) error`
- `RegisterRoutes(container *dig.Container) error`
- `InvokeServer(container *dig.Container) (*fiber.App, error)`，或返回一个包含 app 的轻量 server 结构体。

`internal/bootstrap` 是唯一直接导入 `go.uber.org/dig` 的业务内包。handler、service、repository 等业务包继续通过构造函数接收强类型依赖，不感知容器存在。

`cmd/server.go` 继续负责命令参数、启动顺序、中间件注册、监听循环、信号处理和优雅关闭。对象图构建和路由注册交给 bootstrap 层完成。

## 依赖图

容器需要提供以下对象：

- `*fiber.App`
- `huma.API`
- `internal/infrastructure/repository` 中的 repositories
- `internal/infrastructure/transport` 中的 transports/proxies
- handler 依赖结构体或 handler 接口
- `handler.OpenAIHandler`、`handler.UserHandler` 等路由 handler 接口
- 路由注册函数，或一个统一的路由注册入口

handler 构造函数第一阶段可以保留现有 `XxxDependencies` 结构体。这样能减小迁移范围，并避免影响直接实例化 handler 的既有测试。如果某些 handler 内部已经构造 domain service 或 usecase，本次迁移可以先保持不变，除非该依赖原本已经暴露在 router 层。

## 启动流程

服务启动流程保持当前语义：

1. 记录环境和运行时配置。
2. 显式初始化 database、Redis、共享 HTTP client、pond 协程池和 cron jobs。
3. 创建 `dig` container。
4. 注册 providers。
5. 解析 Fiber app 和 Huma API。
6. 注册全局中间件。
7. 非生产环境注册 docs 路由。
8. 通过注入的 handlers 注册 API 路由。
9. 启动 `app.Listen()`，等待关闭信号。

这样既保留现有运维顺序，也让依赖创建集中且可追踪。

## 路由调整

router 函数不再在内部构造依赖。例如 `initOpenAIRouter` 应接收 `handler.OpenAIHandler`，而不是自行创建 repository 和 proxy。

`RegisterAPIRouter` 可以直接接收全部 handler 接口，也可以由 bootstrap 层包装一个统一 registrar 后调用更细粒度的路由注册函数。推荐做法是让 router 只负责 Huma 路由注册，把对象构造完全移到 bootstrap 层。

## 错误处理

provider 注册和容器 invoke 错误都发生在启动阶段。错误应返回给 `cmd/server.go`，使用 `[Server]` 或 `[Bootstrap]` 前缀记录日志，并让启动快速失败。

业务错误处理不变。handler 和 service 继续沿用现有 `ierr` 与 response wrapping 规范。

## 测试

新增聚焦 bootstrap 测试，验证容器可成功构建并完成足够的路由注册，从而捕获缺失 provider。测试不应依赖外部服务，也不应开启网络监听。

实现后运行现有验证命令：

- `go test -count=1 ./test/unit/...`
- `go test -count=1 ./...`
- `make lint-conv`

如果部署后需要生产链路验证，使用既有 E2E 模式，并通过 `BASE_URL` 和 `API_KEY` 注入运行参数；禁止把密钥写入仓库。

## 迁移策略

按小步迁移：

1. 添加依赖和 bootstrap 包。
2. 在 bootstrap 中提供 Fiber 和 Huma，并保持当前配置。
3. 在 bootstrap 中注册 infrastructure constructors。
4. 在 bootstrap 中使用现有依赖结构体注册 handlers。
5. 修改 router 函数，让它们接收 handlers，而不是内部构造依赖。
6. 更新 `cmd/server.go`，用 bootstrap 创建 app 并注册路由。
7. 运行聚焦和全量验证。

## 风险

主要风险是改变初始化顺序，或意外把路由注册到不同的 Fiber/Huma 实例上。为避免这个问题，bootstrap provider 必须先构造 Fiber，再基于同一个 Fiber 构造 Huma；所有 router 都必须使用同一个容器注入的 Huma 实例。

另一个风险是滥用 `dig`。容器不得传入 handler、service、middleware 或 repository；它只能存在于启动组合根中。
