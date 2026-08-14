# SSE 长连接优雅排空：两阶段 Drain + 礼貌断流

**日期**: 2026-08-15
**状态**: 待评审
**前置设计**: [2026-06-01-graceful-shutdown-design.md](2026-06-01-graceful-shutdown-design.md)（8 步顺序关闭与 readiness 联动，已实现）

## 1. 背景

前序设计实现了 8 步顺序优雅退出，`InflightDrainTimeout=5min` 内纯等待存量请求完成。上线后发现新的矛盾：

1. **发布慢且时长不可控**：SSE 流常见 3~10 分钟（上限 30 分钟，`WRITE_TIMEOUT=30m`）。`maxUnavailable: 0, maxSurge: 1` 下滚动更新串行等待每个旧 pod 完全终止，发布最坏 ≈ 12 分钟。
2. **超窗长流被"裸掐"**：drain 5 分钟超时后，`FiberShutdownTimeout=30s` 到点返回错误，但 SSE 连接仍挂着；进程最终退出时由内核 RST 断开。SSE 读循环（`transport/openai.go ReadChatCompletionStream` 等）是纯阻塞读、不感知任何取消信号。客户端只看到 `connection closed`，**收不到任何协议内错误帧**，无法区分"服务故障"和"服务在发布"。
3. **drain 期间存储链路已死**（附带发现）：停止顺序为 Pool → Inflight（`bootstrap/lifecycle.go`），drain 等待期间协程池已停，此时完成/被截断的流走 `recordModelCall`/`storeOpenAIChat`（经 pool 异步提交）会丢失计量与存储。

### 本质矛盾

TCP 连接绑定进程，进程退出连接必断——"长流完整交付"与"发布快速完成"不可兼得。本设计在两个旋钮上取舍：**等待自然结束的时间上限（soft window）** 与 **到点后断流的方式（裸掐 → 协议内礼貌断开）**。

## 2. 已确认的决策

| 决策点 | 结论 |
|--------|------|
| soft window | 保持 5 分钟（不牺牲流的自然完成率） |
| 截断重试语义 | 接受"重新生成整段回答"（LLM 流式协议无断点续传），客户端收到明确错误帧后自动重试进新 pod |
| Pool/Inflight 停止顺序 | 一并修复（Inflight 先停、Pool 后停） |
| K8s 参数 | `terminationGracePeriodSeconds` 660 → 480；`maxSurge` 1 → 2 |

放弃的备选：延长等待窗口覆盖 P99（发布最坏 1h+，超窗仍裸掐）；后台完成 + 幂等回放（复杂度高且需客户端配合断点语义）。

## 3. 方案

### 3.1 核心机制：两阶段 Drain + 断流广播

```
SIGTERM
  ├─ Cron 停止（现状）
  ├─ Inflight Drain 开始（顺序调整后提前于 Pool）
  │    ├─ 进入 draining（现状保留：/ready→503，新请求→503）
  │    ├─ 阶段1 soft window（5min，不变）：纯等待，短请求与多数 SSE 自然完成
  │    ├─ soft 到点 → close 广播 channel（tracker 新增 cancel 广播）
  │    │    ├─ transport 层：上游 HTTP 请求 ctx 被 cancel
  │    │    │     → 阻塞读返回 context canceled → SSE 读循环退出（复用现有错误路径）
  │    │    └─ usecase 层：WriteUpstreamSSEError 识别 context.Canceled
  │    │          → 写协议内 shutdown 错误帧（客户端可识别并重试）
  │    └─ 阶段2 hard window（30s，新增）：等被截断的流写完错误帧、计量、Untrack
  ├─ Pool 停止（顺序调整：drain 期间存储/计量链路存活）
  ├─ HTTP Shutdown（连接已关，秒级完成，不再依赖 RST/SIGKILL 收尾）
  └─ Logger → DB → Redis → 干净退出
```

### 3.2 关键技术前提（已验证）

- huma v2 handler 的 ctx 包装自 fiber `c.Context()`（`humafiber/humafiber.go:209` `contextWrapper{Context: c.Context()}`），middleware 层无法整体替换请求 ctx；但 `ReadChatCompletionStream` 等读循环消费的是上游 `resp.Body`，其生命周期由**发起上游请求时的 ctx** 决定 → **在 transport 层上游请求入口融合 drain 广播即可让阻塞读返回**，无需改造读循环为 select。
- `UpstreamConnectionError`（`common/model/error.go:106`）**未实现 `Unwrap()`**，`errors.Is(err, context.Canceled)` 无法穿透，需补一行。
- 客户端断开时 fasthttp `RequestCtx.Done()` 会级联取消上游请求（ctx 父子链），此为现状已有的正确行为；drain cancel 是对称的服务端侧取消。

### 3.3 改动明细

#### (1) `internal/common/inflight/tracker.go` — 两阶段 Drain

```go
type Tracker struct {
    wg         sync.WaitGroup
    state      atomic.Int32
    cancelCh   chan struct{}   // soft 到点时 close，一次性
    cancelOnce sync.Once
}

// Drain soft 到点广播取消，再等 hard 窗口让被截断请求收尾
func (t *Tracker) Drain(soft, hard time.Duration) bool

// DrainCanceled 返回广播 channel；未进入 soft 到点前阻塞
func (t *Tracker) DrainCanceled() <-chan struct{}
```

语义：全部请求在 soft 内自然完成 → 返回 true 且**不广播**；soft 到点 → 广播 → hard 内完成 → 返回 true；hard 超时 → warn 并返回 false（HTTP shutdown 兜底）。

#### (2) `internal/common/constant/http.go` — 常量拆分

```go
InflightDrainSoftTimeout = 5 * time.Minute  // 原 InflightDrainTimeout 语义
InflightDrainHardTimeout = 30 * time.Second // 新增
```

#### (3) `internal/infrastructure/transport/{openai,anthropic}.go` — 融合 drain 广播

`openAIProxy`/`anthropicProxy` 构造函数注入 `*inflight.Tracker`（dig 已注册），在 `doUpstreamRequest`/`sendRequest` 内：

```go
upCtx, cancel := context.WithCancel(ctx)
stop := context.AfterFunc(tracker.DrainCanceled(), cancel) // Go 1.21+，无 goroutine 泄漏
defer stop()
// 用 upCtx 发请求
```

unary 与 streaming 均经由这两个入口，一处生效。注意重试逻辑（`doUpstreamRequest` 内自动重试）不受影响：draining 状态下重试没有意义，cancel 后重试路径自然终止。

#### (4) `internal/common/model/error.go` — 补 Unwrap

```go
func (e *UpstreamConnectionError) Unwrap() error { return e.Cause }
```

#### (5) `internal/application/llmproxy/util/sse.go` — shutdown 错误帧

`WriteUpstreamSSEError` 增加 `errors.Is(err, context.Canceled)` 分支，写协议内专用帧：

- OpenAI（data-only）：`data: {"error":{"message":"server is restarting, please retry","type":"server_error","code":"server_shutting_down"}}`
- Anthropic（event）：`event: error` + `data: {"type":"error","error":{"type":"overloaded_error","message":"server is restarting, please retry"}}`

不引入 tracker 依赖做 draining 判断：客户端主动断开也会产生 context.Canceled，但该场景下错误帧写入必然失败（连接已断），实际可写出的 canceled 场景只有 drain 取消，语义无损。

#### (6) `internal/bootstrap/lifecycle.go` — 停止顺序修复

交换 Inflight 与 Pool 两个 hook 的注册顺序，使停止顺序变为：

```
Cron → Inflight(drain) → Pool → Metrics → HTTP → Logger → Blocked → DB → Redis
```

drain 期间 pool/metrics 存活，被截断流的 `recordModelCall`/`storeOpenAIChat` 不丢。

#### (7) `k8s/deployment.yaml` — 参数调整

```yaml
terminationGracePeriodSeconds: 480   # 660 → 480
strategy:
  rollingUpdate:
    maxUnavailable: 0
    maxSurge: 2                      # 1 → 2，新副本提前就绪
```

grace 推导：preStop 10s + soft 300s + hard 30s + HTTP/DB/Redis 收尾 ≈ 400s，480 留余量。maxSurge 2 让 2 副本的新 pod 并行就绪，进一步压缩发布窗口。

### 3.4 预期效果

| 指标 | 现状 | 改后 |
|------|------|------|
| 单 pod 终止最坏时长 | ~6min（可能 660s SIGKILL 兜底） | ~6.5min，确定性收敛且干净退出 |
| 2 副本发布最坏时长 | ~12min（maxSurge 1 串行） | ~7min（maxSurge 2 并行就绪 + 删除阶段确定性） |
| 超窗长流客户端体验 | `connection closed`（像故障） | 协议内 `server_shutting_down` 帧 → 自动重试进新 pod |
| 被截断流的计量/存储 | 丢失（pool 已停） | 保留 |

注：soft 保持 5min 意味着发布最坏时长改善主要来自 maxSurge 2 与确定性收尾，而非缩短 drain；本设计的主要收益是**消除裸掐**与**修复存储丢失**。

## 4. 涉及文件

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `internal/common/inflight/tracker.go` | 修改 | 两阶段 Drain + DrainCanceled 广播 |
| `internal/common/constant/http.go` | 修改 | 常量拆分 soft/hard |
| `internal/infrastructure/transport/openai.go` | 修改 | doUpstreamRequest 融合 drain cancel，注入 tracker |
| `internal/infrastructure/transport/anthropic.go` | 修改 | sendRequest 同上 |
| `internal/common/model/error.go` | 修改 | UpstreamConnectionError 补 Unwrap |
| `internal/application/llmproxy/util/sse.go` | 修改 | context.Canceled → shutdown 错误帧 |
| `internal/bootstrap/container.go` | 修改 | transport 构造函数注入 tracker |
| `internal/bootstrap/lifecycle.go` | 修改 | Inflight/Pool 停止顺序交换 |
| `k8s/deployment.yaml` | 修改 | grace 480 + maxSurge 2 |
| `test/unit/inflight/inflight_test.go` | 修改 | 两阶段 Drain 用例 |

## 5. 不涉及的变更

- 不改造 SSE 读循环为 select 化（经 transport ctx cancel 间接中断，无需动 4 个读循环）
- 不修改 `InflightMiddleware`（请求 ctx 不动，取消落点在 transport 层）
- 不修改 readiness/liveness 行为（`/ready` draining→503 已实现）
- 不修改 cron、pool 内部 Stop 逻辑
- 不引入 SSE 断点续传 / Last-Event-ID / 后台完成机制
- dataset.go 的内部生成流不接入 drain cancel（无上游请求，依赖 hard 窗口 + HTTP shutdown 兜底）

## 6. 验证标准

1. 单测：`Tracker.Drain(soft, hard)` 三种路径——soft 内全部完成（返回 true、不广播）；soft 到点广播后被截断请求在 hard 内完成（返回 true）；hard 超时（返回 false）
2. 单测：`UpstreamConnectionError.Unwrap` 使 `errors.Is(err, context.Canceled)` 穿透
3. 单测：`WriteUpstreamSSEError` 对 context.Canceled 写出 `server_shutting_down` 帧（双协议格式）
4. 单测：httptest 模拟阻塞上游 + tracker 广播 → `doUpstreamRequest`/`sendRequest` 返回 context canceled
5. 单测：lifecycle 停止顺序（如可行；否则以代码评审确认注册顺序）
6. `make lint` 与 `go test -count=1 ./...` 通过
7. 部署后 e2e：发起 >5min 的流式请求 → `kubectl rollout restart` → 客户端收到 `server_shutting_down` 帧；CLS 检索 drain 广播日志与 pod 终止时长 ≤ ~6.5min；被截断流的 model call audit 记录存在（验证 pool 顺序修复）
