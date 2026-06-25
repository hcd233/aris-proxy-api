# 用 Redis ZSET 自建运行时指标时序，而非部署 Prometheus

---
Status: accepted
---

## 决策

Runtime Metrics（goroutine/heap/cpu/QPS/在途请求/P95/SSE 并发）的采集、跨 pod 聚合与时序留存，**用 Redis 有序集合（ZSET）自建一个迷你时序层**实现，而不引入 Prometheus/Grafana 等独立监控栈。每个 pod 在内存用 `prometheus/client_golang` 维护计数器，由一个 fx 托管的 5s ticker goroutine 把自身 Snapshot 写入 Redis（按 instance 分 key）；一个 Admin-only 聚合 API 读取窗口内所有 instance 的 Snapshot，做跨 pod 聚合后返回已分桶的时序，前端纯渲染。

## 背景与动机

- 集群是 `replicas: 2` 多 pod，且**当前零监控栈**。旧方案让前端轮询 `/api/v1/metrics/json`，每次被负载均衡路由到随机 pod，只能拿到单 pod 的内存快照，数据跳变。
- 旧的 `gofiber/contrib/prometheus` 中间件被错误地挂成 `app.Use("/metrics", mw)`（只对 `/metrics` 路径生效），导致 QPS / In-Progress / P95 全为 0。
- 业务指标（token/cost）已有一套"后端从审计库聚合 → 前端渲染"的成熟范式；运行时指标希望复用同一展示范式以统一样式。

## 考虑过的方案

1. **部署 Prometheus 抓取各 pod `/metrics`**（业界标准）：最稳，但要新增 Prometheus 部署 + 运维，pod 资源配额很紧（300m/512Mi），为一个内部 admin 大盘引入独立监控栈成本过高。**保留 `/metrics` 文本端点为将来接入留口，但当下不依赖它。**
2. **Redis ZSET 自建时序（选中）**：复用现成 Redis、复用业务图表的后端聚合范式、零新基础设施；运行时指标只需短期留存（24h），ZSET + `ZREMRANGEBYSCORE` 滚动裁剪天然契合。
3. **落库持久化**：高频写库浪费 IO，运行时瞬时指标无长存价值，过重。

## 关键约束（写给未来的人）

- **聚合顺序不可颠倒**：counter 必须"先按各 pod 求速率（含 reset 清零）、再跨 pod 求和"（等同 `sum(rate(...))`）；histogram 必须跨 pod 合并 bucket 再求分位。绝不能先跨 pod 求和累计值——某 pod 重启会塌陷全局累计、污染整段曲线。详见 `CONTEXT.md` 的 Aggregation 条目。
- **flusher 必须每个 pod 各跑各的**：不能走 cron（cron 用 `cron:lock` 分布式锁、全集群单例），也不塞进 `pond` 协程池（常驻周期任务会squat 一个 worker，且会耦合到业务写入背压）。用专属 fx 生命周期 ticker goroutine。
- **这是"够用就好"的迷你时序，不是 Prometheus 替代品**：无 PromQL、无告警、无长期留存。若监控需求增长，应转向方案 1，而非继续在 Redis 上堆功能。
