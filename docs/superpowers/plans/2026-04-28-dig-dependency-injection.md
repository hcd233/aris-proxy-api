# Dig 依赖注入实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**目标:** 引入 `go.uber.org/dig`，把 HTTP API 启动阶段的对象构造集中到组合根，保持现有业务行为、启动顺序和关闭顺序不变。

**架构:** 新增 `internal/bootstrap` 作为唯一使用 `dig` 的组合根，负责提供 Fiber、Huma、repository、transport、handler 和路由注册入口。`cmd/server.go` 只调用 bootstrap 获取 app 并注册路由，router 只负责 Huma 路由声明，不再创建 handler 依赖。

**Tech Stack:** Go 1.25.1、Fiber、Huma、`go.uber.org/dig`、标准库 `testing`。

---

## 执行摘要

本计划已按 TDD 执行：先新增 `test/unit/bootstrap/bootstrap_test.go`，验证 `internal/bootstrap` 缺失导致 RED；随后实现 bootstrap 容器、API 构造函数、router 注入改造和 server 接入，并完成全量验证。

## 验证命令

- `go test -v -count=1 ./test/unit/bootstrap/`
- `go test -count=1 ./test/unit/...`
- `go test -count=1 ./...`
- `make lint-conv`

## 实现要点

- `internal/bootstrap` 是唯一直接使用 `go.uber.org/dig` 的业务内包。
- `cmd/server.go` 保留原有基础设施初始化和优雅关闭顺序。
- `internal/router` 只接收 handler 并注册 Huma 路由，不再创建 repository、proxy 或 handler。
- OAuth2 的对象存储目录创建器保持可选语义；未配置对象存储时 bootstrap 注入 nil，避免启动期或单测期 panic。
