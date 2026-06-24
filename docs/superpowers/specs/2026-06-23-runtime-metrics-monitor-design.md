# 运行时与实时业务指标监控设计

> 日期：2026-06-23
> 阶段：CPU/内存优化工程 - 阶段 1（测量基础设施）

## 1. 背景与目标

### 1.1 驱动

项目部署在 K8s，资源配额紧张（requests 50m CPU / 128Mi，limits 300m CPU / 512Mi，2 副本）。
目标是在不扩容的前提下**降本**（压低资源占用，为缩配留空间）并**提升吞吐**（在同等资源下扛更高 QPS）。

### 1.2 路径选择

采用"先建测量再优化"路径：先建立监控基础设施，用数据驱动后续阶段的优化决策与效果验证。

### 1.3 展示方案

不引入 Grafana/Prometheus Server 等外部重型栈，而是复用项目已有的 web 前端（Next.js + recharts），新增监控 dashboard 页面展示实时指标。

## 2. 现状分析

### 2.1 已有能力（不动）

| 能力 | 位置 | 说明 |
|------|------|------|
| 历史业务指标端点 | `internal/router/audit.go` | 6 个 `/stats/*` 端点，基于 DB 审计日志聚合 |
| 前端历史图表 | `web/src/components/charts/` | 6 个图表组件 + shadcn `chart.tsx`（基于 recharts） |
| 前端 dashboard 导航 | `web/src/app/(dashboard)/layout.tsx` | `navItems` 数组，支持 `adminOnly` 权限控制 |
| CPU profile | `internal/middleware/fgprof.go` | fgprof 已在 `container.go:73` 全局挂载，`/debug/fgprof` 可抓 CPU profile |

### 2.2 缺失能力（本次补齐）

| 能力 | 现状 |
|------|------|
| 运行时指标（CPU/内存/goroutine/GC） | 完全缺失，无 pprof、无 metrics、无 automaxprocs |
| 实时业务指标（inflight/QPS/延迟分位/SSE 并发） | 无 |
| /metrics 端点 | 无 |
| 前端实时监控页 | 无 |

### 2.3 关键发现

- `inflight.Tracker`（`internal/common/inflight/tracker.go`）用 `sync.WaitGroup` 实现，仅暴露 running/draining 状态，**无当前计数**。
- 项目无 `go.uber.org/automaxprocs`，容器内 GOMAXPROCS 会取宿主机核数（后续阶段 2 修复）。
- GORM `LogLevel=Info`，生产全量打印 SQL（后续阶段 2 修复）。
- Redis 客户端用默认连接池配置（后续阶段 2 修复）。

## 3. 整体规划

按依赖关系拆分 4 个阶段，每阶段独立 spec → plan → 实现：

| 阶段 | 内容 | 依赖 |
|------|------|------|
| **1. 测量基础设施** | 后端 metrics 采集/暴露 + 前端监控 dashboard | 无 |
| **2. 运行时浪费点修复** | automaxprocs、GORM 日志降级、Redis 连接池、SSE 缓冲等 | 阶段 1 数据 |
| **3. 业务热路径优化** | SSE 转发、消息去重存储、inflight tracker 等 | 阶段 1 数据 |
| **4. K8s 降本调参** | 基于优化后实测调整 requests/limits、副本数 | 阶段 2+3 验证 |

本文档仅覆盖**阶段 1**。

## 4. 阶段 1 详细设计

### 4.1 技术选型

采用 `github.com/gofiber/contrib/v3/prometheus` 中间件（与已有 `fgprof` 同属 contrib 包家族，风格一致）。

该中间件基于 `ansrivas/fiberprometheus`，内置以下能力：

- HTTP 指标：`http_requests_total`、`http_requests_status_class_total`、`http_request_duration_seconds`、`http_requests_in_progress`、`http_request_size_bytes`、`http_response_size_bytes`
- Go runtime 指标：内置 GoCollector（goroutine/heap/GC/mem 等）
- Process 指标：内置 ProcessCollector（CPU 累计时间/RSS）
- `/metrics` 端点：由挂载路径决定，exposition format
- 可配置：`Registerer`/`Gatherer` 注入、`RequestDurationBuckets`、`SkipURIs`、`IgnoreStatusCodes`、`Next` 等

### 4.2 自研部分

中间件不覆盖、仍需自研的部分：

1. **SSE 并发 gauge**：SSE 是 handler 内部行为，中间件无法自动采集。需通过注入的 `Registerer` 自定义注册 `sse_active_connections` gauge，在 OpenAI/Anthropic SSE handler 中 Inc/Dec。
2. **JSON 转换端点**：中间件只暴露 exposition format，web dashboard（recharts）无法直接消费。需新增 `/api/v1/metrics/json` 端点，从 `Gatherer` 采集并序列化为 JSON。

### 4.3 后端架构变更

```
新增 internal/infrastructure/metrics/
  ├── prometheus.go    # 初始化 fiberprometheus 中间件 + 自定义 registry + SSE gauge
  └── json_exporter.go # /api/v1/metrics/json 端点：Gatherer → JSON 序列化

修改 internal/bootstrap/container.go
  └── 中间件链挂载 prometheus 中间件（位置：Fgprof 之后，与 fgprof 同为调试/监控层）

新增 internal/router/metrics.go
  └── 注册 /api/v1/metrics/json huma 路由（admin only，jwtAuth + PermissionAdmin）

修改 SSE handler（OpenAI/Anthropic 代理 handler）
  └── 在流开始时 sseActiveConnections.Inc()，流结束时 Dec()（defer）
```

**不改动**：`internal/common/inflight/tracker.go`（中间件 `http_requests_in_progress` 已覆盖 inflight 采集）。

### 4.4 中间件挂载

中间件链当前顺序（`container.go:52`）：
```
Recover → Inflight → Guard → Fgprof → CORS → Compress → Trace → Log
```

新增 prometheus 中间件，位置在 **Fgprof 之后、CORS 之前**（同为监控/调试层，且需在业务中间件之前才能采集所有请求）：

```
Recover → Inflight → Guard → Fgprof → Prometheus → CORS → Compress → Trace → Log
```

挂载方式：
```go
app.Use("/metrics", prometheusMiddleware)  // /metrics 端点
```

注意：`app.Use("/metrics", handler)` 既注册 `/metrics` 端点，又对全部路由生效采集指标。

### 4.5 采集点列表

#### 运行时指标（中间件内置 GoCollector + ProcessCollector，零开发）

| 指标 | 说明 |
|------|------|
| `go_goroutines` | 当前 goroutine 数 |
| `go_memstats_alloc_bytes` | heap 已分配字节数 |
| `go_memstats_sys_bytes` | 从 OS 获取的字节数 |
| `go_memstats_heap_inuse_bytes` | heap 在用字节 |
| `go_memstats_stack_inuse_bytes` | stack 在用字节 |
| `go_gc_duration_seconds` / `go_gc_duration_seconds_count` | GC 次数与耗时 |
| `process_cpu_seconds_total` | 累计 CPU 时间（前端用 rate 算 CPU%） |
| `process_resident_memory_bytes` | RSS 内存 |

#### HTTP 业务指标（中间件内置，零开发）

| 指标 | 类型 | 标签 | 说明 |
|------|------|------|------|
| `http_requests_total` | counter | method, status, path | 请求总数 |
| `http_requests_status_class_total` | counter | status_class, method, path | 状态码分类计数 |
| `http_request_duration_seconds` | histogram | method, path | 请求延迟 |
| `http_requests_in_progress` | gauge | method, path | 当前在途请求数 |
| `http_request_size_bytes` | histogram | method, path | 请求体大小 |
| `http_response_size_bytes` | histogram | method, path | 响应体大小 |

#### 自定义业务指标（自研）

| 指标 | 类型 | 标签 | 采集点 |
|------|------|------|--------|
| `sse_active_connections` | gauge | provider | OpenAI/Anthropic SSE handler 流开始 Inc / 流结束 Dec |

### 4.6 配置参数

```go
prometheus.Config{
    Service:           "aris-proxy-api",
    Namespace:         "http",
    Registerer:        customRegistry,   // 注入以支持自定义 SSE gauge + JSON 端点
    Gatherer:          customRegistry,   // 同一 registry
    RequestDurationBuckets: []float64{
        0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75,
        1, 2.5, 5, 10, 15, 30, 60, 120, 300, 600, 1800,  // 扩展至 30 分钟，适配 SSE 长连接
    },
    SkipURIs: []string{
        constant.RoutePathHealth,
        constant.RoutePathReady,
        constant.RoutePathSSEHealth,
    },
}
```

### 4.7 端点设计

| 端点 | 格式 | 用途 | 鉴权 |
|------|------|------|------|
| `GET /metrics` | Prometheus exposition | 未来可接 Prometheus/Grafana 抓取 | 内网（K8s 不对外暴露） |
| `GET /api/v1/metrics/json` | JSON | web dashboard 轮询消费 | admin only（jwtAuth + PermissionAdmin） |

#### /api/v1/metrics/json 响应结构

```json
{
  "metrics": [
    {
      "name": "http_requests_total",
      "type": "counter",
      "help": "Total number of HTTP requests",
      "samples": [
        {
          "labels": {"method": "POST", "path": "/api/openai/v1/chat/completions", "status": "200"},
          "value": 1234
        }
      ]
    },
    {
      "name": "http_request_duration_seconds",
      "type": "histogram",
      "help": "...",
      "samples": [
        {
          "labels": {"method": "POST", "path": "...", "le": "0.1"},
          "value": 100
        }
      ]
    }
  ]
}
```

实现方式：调用 `Gatherer.Gather()` 获取 `[]*dto.MetricFamily`，遍历序列化为上述 JSON 结构。

### 4.8 前端 /web/monitor 页面

#### 新增文件

```
web/src/app/(dashboard)/monitor/page.tsx          # 监控页主体
web/src/components/charts/runtime-gauge-card.tsx  # 数字卡片组件（goroutine/heap/inflight/SSE）
web/src/components/charts/runtime-line-chart.tsx  # 实时折线图组件
```

#### 导航入口

在 `web/src/app/(dashboard)/layout.tsx` 的 `navItems` 数组中新增：

```typescript
{
  label: "Monitor",
  href: "/monitor/",
  icon: <Activity className="size-4" />,
  adminOnly: true,
},
```

#### 页面布局

```
┌─────────────────────────────────────────────┐
│  [Goroutines]  [Heap]  [In-Progress]  [SSE]  │  ← 4 个数字卡片（当前值）
├─────────────────────────────────────────────┤
│  CPU 使用率折线图        内存使用折线图         │  ← 时序图（5s 刷新，保留 60 个点）
├─────────────────────────────────────────────┤
│  请求 QPS 折线图          请求延迟 P95 折线图   │
├─────────────────────────────────────────────┤
│  Goroutine 数折线图      GC 耗时折线图          │
└─────────────────────────────────────────────┘
```

#### 数据获取

- 新增 `api.getMetricsJSON()` 方法到 `web/src/lib/api-client.ts`
- 同步在 `web/src/lib/types.ts` 增补 `MetricsJSONRsp` 等类型
- 前端每 5s 轮询 `/api/v1/metrics/json`，在内存维护最近 60 个数据点（5 分钟窗口）画实时折线图
- CPU% 由前端用相邻两次 `process_cpu_seconds_total` 差值 / 时间间隔计算

### 4.9 安全

- `/metrics`：不挂 huma 鉴权，依赖 K8s 网络隔离（Service 不对外暴露），仅供集群内 Prometheus 抓取或本地调试。
- `/api/v1/metrics/json`：走 huma `jwtAuth` + `PermissionAdmin` 中间件，仅管理员可从 web 访问。

## 5. 权衡与风险

| 风险 | 影响 | 缓解 |
|------|------|------|
| `gofiber/contrib/v3/prometheus` 为 pre-release（无 tag 版本） | 依赖不稳定 | 同 contrib 的 fgprof 已稳定（v1.0.6），prometheus 基于成熟 fiberprometheus，可接受；锁定 commit 版本 |
| GoCollector 默认采集 ~60 个 go_* 指标，增大 /metrics 响应 | 内网抓取无性能影响，JSON 端点传输略增 | JSON 端点只返回前端需要的指标子集，过滤无关 go_* |
| SSE 长连接 duration 超出默认 buckets（60s） | 落入 +Inf 桶，延迟统计失真 | 扩展 RequestDurationBuckets 至 1800s |
| 健康检查探针产生指标噪音 | /health /ready 每 10-20s 一次，污染 QPS | 配置 SkipURIs 跳过 |
| 前端 5s 轮询增加 QPS | 单管理员访问时可忽略 | 仅 adminOnly 页面，非全量用户 |

## 6. 测试计划

### 6.1 单元测试（`test/unit/metrics/`）

- `prometheus` 中间件初始化正确性（registry、SSE gauge 注册）
- JSON exporter 序列化正确性（counter/gauge/histogram 各类型覆盖）
- SSE gauge Inc/Dec 并发安全

### 6.2 端到端测试（`test/e2e/metrics/`）

- `GET /metrics` 返回 200 + exposition format
- `GET /api/v1/metrics/json` 鉴权：
  - 无 token → 401
  - 普通用户 → 403
  - 管理员 → 200 + JSON 结构正确
- 发送一个 LLM 代理请求后，`http_requests_total` 递增
- SSE 请求期间 `sse_active_connections` > 0，结束后归 0

### 6.3 前端验证

- `cd web && npm run lint && npm run build` 通过
- 浏览器访问 `/web/monitor/`，确认数字卡片和折线图正常渲染（chrome devtools 验证）

## 7. 不在本次范围

以下留待后续阶段，本次不实现：

- automaxprocs 引入（阶段 2）
- GORM 日志级别降级（阶段 2）
- Redis 连接池配置（阶段 2）
- SSE 转发/消息存储热路径优化（阶段 3）
- K8s requests/limits 调参（阶段 4）
- 告警规则（不在本次目标内）
- 长期历史指标存储（重启丢失可接受，用优化前后两次抓取对比即可）
