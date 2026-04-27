# Dig 依赖注入实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**目标:** 引入 `go.uber.org/dig`，把 HTTP API 请求链路的对象构造集中到组合根，保持现有业务行为、启动顺序和关闭顺序不变。

**架构:** 新增 `internal/bootstrap` 作为唯一使用 `dig` 的组合根，负责提供 Fiber、Huma、repository、transport、domain service、application command/query/usecase、handler 和路由注册入口。`cmd/server.go` 只调用 bootstrap 获取 app 并注册路由，router 只负责 Huma 路由声明，handler 只接收 application 接口并处理 DTO 适配。

**Tech Stack:** Go 1.25.1、Fiber、Huma、`go.uber.org/dig`、标准库 `testing`。

---

## 执行摘要

本计划按 TDD 执行：先补强 `test/unit/bootstrap/bootstrap_test.go`，验证容器可解析 HTTP server、路由注册成功、组合根不暴露容器、bootstrap 不新增 `any` provider 列表；随后实现 bootstrap 容器、API 构造函数、application/usecase 注入、handler 注入、router 注入改造和 server 接入，并完成全量验证。

## 文件职责

- `internal/bootstrap/container.go`：HTTP 请求链路唯一组合根，注册 provider 并解析 `Server`。
- `internal/bootstrap/router.go`：通过未导出的容器解析 route registrar，注册 docs/API 路由。
- `internal/handler/*.go`：handler 构造函数接收 application 层接口，不再创建 command/query/usecase。
- `internal/api/fiber.go`、`internal/api/huma.go`：只提供 Fiber/Huma 构造函数，不维护可变全局实例。
- `cmd/server.go`：保持基础设施显式初始化和关闭，只调用 bootstrap 构建 app 与注册路由。
- `test/unit/bootstrap/bootstrap_test.go`：验证组合根和路由注册，不启动网络监听。

## 验证命令

- `go test -v -count=1 ./test/unit/bootstrap/`
- `go test -count=1 ./test/unit/...`
- `go test -count=1 ./...`
- `make lint-conv`

## 实现要点

- `internal/bootstrap` 是唯一直接使用 `go.uber.org/dig` 的业务内包。
- `Server` 不导出 `*dig.Container`，避免运行时 service locator。
- `bootstrap` 新增代码不使用 `any` 或 `interface{}` provider 列表。
- `cmd/server.go` 保留原有基础设施初始化和优雅关闭顺序。
- `internal/router` 只接收 handler 并注册 Huma 路由，不再创建 repository、proxy 或 handler。
- `internal/handler` 只接收 application command/query/usecase 接口，不再创建 repository、transport、domain service 或 application 对象。
- OAuth2 的对象存储目录创建器保持可选语义；未配置对象存储时 bootstrap 注入 nil，避免启动期或单测期 panic。

## 任务清单

- [ ] 更新 bootstrap 单测，覆盖 `Server` 容器封装、核心路由存在、bootstrap 禁止新增 `any` provider 列表。
- [ ] 移除 `api.SetFiberApp`、`api.SetHumaAPI` 和可变全局 app/api 用法。
- [ ] 拆分 bootstrap provider 注册函数，逐个注册 provider，避免 `[]any`。
- [ ] 在 bootstrap 中注册 application command/query/usecase 和 domain service provider。
- [ ] 修改 handler 构造函数，让 handler 接收 application 接口。
- [ ] 收窄 `Server` 的容器字段为未导出，并调整 `RegisterRoutes`。
- [ ] 运行 `go test -v -count=1 ./test/unit/bootstrap/`。
- [ ] 运行 `make lint-conv`。
- [ ] 运行 `go test -count=1 ./...`。
