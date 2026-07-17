# 项目模型

> **使用场景**：需要理解项目架构、启动链路、请求链路、依赖注入、LLM 代理分层、优雅关闭时加载。

- Go `1.25.1` 后端，提供 LLM 代理网关、用户、API Key、会话管理。
- 服务端入口：`cmd/server/main.go` → `execute()` → `cmd/server/server.go` 的 `server start`；客户端入口位于 `cmd/client/`。
- 启动链路：database、Redis、共享 HTTP Client、Pond 协程池 → `inflight.InitTracker()` → cron（5 个模块，CronRegistryEntry 模式，含 think-extract）→ Fiber 中间件链（Recover → Inflight → Guard → Fgprof → CORS → Compress → Trace → Log 采样）→ 可选 `/docs` → API 路由。
- 请求链路：Fiber 中间件 → Huma 路由 → handler → application usecase/command → domain service → infrastructure repository/transport。
- 依赖注入：`go.uber.org/dig`，全部在 `internal/bootstrap/container.go` 中注册。
- LLM 代理分层：`application/llmproxy/usecase` 编排端点查找、转换、代理和存储；`infrastructure/transport` 做 HTTP/SSE 传输；`application/llmproxy/converter` 做 DTO 映射。
- 模型路由和代理 Key 由数据库驱动；运行配置来自 Viper 和 `env/api.env`。
- 优雅关闭：接收 SIGINT/SIGTERM → 8 步顺序关闭（停止 cron → 停止协程池 → draining 拒绝新请求 → 等待进行中请求 → 关闭 HTTP Server → 同步日志 → 关闭 DB → 关闭 Redis）。K8s 部署用 `terminationGracePeriodSeconds: 660` + `preStop: sleep 10` 配合 `/ready` 探针实现无损下线。
