# Cron 分布式锁（带续期）设计

## 1. 背景与目标

**现状问题：**
- `internal/cron/` 下 4 个定时任务（`SessionDeduplicateCron`、`SessionSummarizeCron`、`SessionScoreCron`、`SoftDeletePurgeCron`）由 `robfig/cron/v3` 调度，每实例各自触发。
- 在多实例部署下，同一任务的 fn 会被多机同时执行，可能产生：
  - 数据库重复删除/更新（去重任务最敏感）
  - LLM 总结/评分的重复调用与配额浪费
  - 软删除清理的非预期并发
- 现有 `internal/lock/lock.go` 仅有 `SetNX + Lua unlock` 的简单互斥，**不支持续期**；而 cron 任务（特别是 summarize/score 调用 LLM）执行时间可能远超默认 TTL。

**目标：**
- 同一时刻同一定时任务只能由一个实例执行。
- 持锁实例执行期间，锁 TTL 自动续期，避免长任务被中途释放。
- 续期失败时不让基础设施抖动直接中断业务 fn。
- 改动聚焦在 cron 调度层，4 个 fn 业务逻辑零侵入。

**非目标：**
- 不实现跨任务依赖、leader 选举、锁重试/排队等待。
- 不引入新的第三方锁库。

## 2. 架构设计

### 2.1 总体流程

```
       robfig/cron 触发 ──► 包装 fn = RunWithLock(parentCtx, log, locker, key, opts, realFn)
                                  │
                                  ├─ childCtx, cancel := context.WithCancel(parentCtx)
                                  ├─ value := uuid.New().String()
                                  ├─ SETNX key value TTL
                                  │     ├─ err!=nil  → log.Error, return
                                  │     └─ success=false → log.Info "Lock held by another instance, skip", return
                                  ├─ defer cancel(); Unlock(key, value)  // 失败仅 log
                                  ├─ 启 goroutine: ticker 每 RenewInterval 调 Refresh(Lua PEXPIRE)
                                  └─ realFn(childCtx)  // 业务用 childCtx 替换原 background
```

### 2.2 模块划分

| 文件 | 变更类型 | 说明 |
|------|---------|------|
| `internal/lock/lock.go` | 扩展 | `Locker` 接口加 `Refresh`；`redisLocker` 实现 Lua PEXPIRE |
| `internal/cron/lock_runner.go` | 新增 | `RunWithLock`、`renewLoop`、`LockOptions`、默认常量 |
| `internal/cron/cron.go` | 修改 | `CronRegistryEntry` 加 `LockTTL`/`LockRenewInterval`；`InitCronJobs` 接 `*redis.Client` |
| `internal/cron/session_dedup.go` | 修改 | 注入 locker；真实 fn 改为接收 ctx 形参；Start 包装 RunWithLock |
| `internal/cron/session_summarize.go` | 修改 | 同上 |
| `internal/cron/session_score.go` | 修改 | 同上 |
| `internal/cron/soft_delete_purge.go` | 修改 | 同上 |
| `internal/common/constant/rediskey.go` | 修改 | 新增 `CronLockKeyTemplate` |
| `internal/bootstrap/container.go` | 修改 | `InitInfrastructure` 调 `cron.InitCronJobs` 传入 cache |
| `cmd/server.go` | 修改 | 同步调用点 |
| `test/unit/cron/cron_test.go` | 修改 | `InitCronJobs` 新签名 |
| `test/unit/cron/lock_runner_test.go` | 新增 | 6 个 RunWithLock 用例（miniredis） |

## 3. 关键组件

### 3.1 `internal/lock/lock.go` 扩展

```go
type Locker interface {
    Lock(ctx context.Context, key string, value string, expire time.Duration) (success bool, err error)
    Refresh(ctx context.Context, key string, value string, expire time.Duration) (success bool, err error) // 新增
    Unlock(ctx context.Context, key string, value string) error
}
```

`redisLocker.Refresh` Lua 脚本（仅 owner 可续期）：
```lua
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("PEXPIRE", KEYS[1], ARGV[2])
else
    return 0
end
```

`success=false` 表示锁已丢失（被 TTL 过期或被抢占），`err!=nil` 表示网络/脚本错误。两者语义不同，调用方需区分处理。

### 3.2 `internal/cron/lock_runner.go` 新文件

常量放 `internal/common/constant/cron.go`：
```go
const (
    CronLockDefaultTTL             = 5 * time.Minute
    CronLockDefaultRenewInterval   = 1 * time.Minute
    CronLockDefaultRenewDivisor    = 3 // 当 LockOptions.RenewInterval=0 时回退到 TTL/Divisor
    CronLockMaxConsecutiveRenewFailures = 3
)
```

Redis key 模板放 `internal/common/constant/rediskey.go`：
```go
CronLockKeyTemplate = "cron:lock:%s"
```

Lua 脚本放 `internal/common/constant/lock.go`：
```go
const (
    LuaRefreshLock = `...`  // 仅持有者可 PEXPIRE
    LuaUnlockLock  = `...`  // 仅持有者可 DEL
)
```

`RunWithLock` 签名（不含 `*zap.Logger` 参数，从 ctx 派生）：
```go
func RunWithLock(
    parentCtx context.Context,
    locker lock.Locker,
    key string,
    opts LockOptions,
    fn func(ctx context.Context),
) {
    ttl := opts.TTL
    if ttl <= 0 {
        ttl = constant.CronLockDefaultTTL
    }
    renew := opts.RenewInterval
    if renew <= 0 {
        renew = ttl / constant.CronLockDefaultRenewDivisor
    }

    childCtx, cancel := context.WithCancel(parentCtx)
    defer cancel()

    value := uuid.New().String()
    locked, err := locker.Lock(childCtx, key, value, ttl)
    if err != nil {
        log.Error("[CronLock] Lock acquire error", zap.String("key", key), zap.Error(err))
        return
    }
    if !locked {
        log.Info("[CronLock] Lock held by another instance, skip this run", zap.String("key", key))
        return
    }
    defer func() {
        if err := locker.Unlock(childCtx, key, value); err != nil {
            log.Error("[CronLock] Unlock error", zap.String("key", key), zap.Error(err))
        }
    }()

    go renewLoop(childCtx, log, locker, key, value, ttl, renew)
    fn(childCtx)
}

func renewLoop(ctx context.Context, locker lock.Locker, key, value string, ttl, renew time.Duration) {
    log := logger.WithCtx(ctx)
    t := time.NewTicker(renew)
    defer t.Stop()
    failCount := 0
    for {
        select {
        case <-ctx.Done():
            return
        case <-t.C:
            ok, err := locker.Refresh(ctx, key, value, ttl)
            switch {
            case err != nil:
                failCount++
                log.Warn("[CronLock] Refresh error",
                    zap.String("key", key),
                    zap.Int("consecutiveFailures", failCount),
                    zap.Error(err))
                if failCount >= constant.CronLockMaxConsecutiveRenewFailures {
                    log.Warn("[CronLock] Too many refresh failures, stop renewing",
                        zap.String("key", key), zap.Int("failures", failCount))
                    return
                }
            case !ok:
                log.Warn("[CronLock] Lock lost, stop renewing", zap.String("key", key))
                return
            default:
                failCount = 0
            }
        }
    }
}
```

设计要点：
- **`childCtx` 派生自 `parentCtx`**：cron 触发时的 ctx 来自 robfig（无 traceID），fn 内部原本用 `context.WithValue(background, traceID, ...)` 自行注入。这部分保持不变——`RunWithLock` 不污染 traceID。
- **续期失败不 cancel childCtx**：fn 继续跑，让它自然完成 defer 释放。这避免了网络抖动打断长任务。
- **失败计数清零**：只要一次成功就重置 `failCount`，避免偶发抖动累积。
- **常量放 `internal/common/constant/cron.go`**：`CronLockDefaultTTL`、`CronLockDefaultRenewInterval` 遵循项目"业务包禁止本地 const"规则。

### 3.3 `internal/cron/cron.go` 注册表

```go
type CronRegistryEntry struct {
    Name              string
    Enabled           func() bool
    Factory           func(db *gorm.DB, poolManager *pool.PoolManager) Cron
    LockTTL           time.Duration // 新增，0 → constant.CronLockDefaultTTL
    LockRenewInterval time.Duration // 新增，0 → constant.CronLockDefaultRenewInterval
}
```

`InitCronJobs` 签名变化：
```go
func InitCronJobs(db *gorm.DB, poolManager *pool.PoolManager, cache *redis.Client)
```

改法：将 `*redis.Client` 透传给每个 cron 实例，由各 cron 自行 `lock.NewLocker(cache)` 构造并持有 `locker` 字段。

为减少 4 个 cron 文件重复样板，在 `lock_runner.go` 暴露同包辅助函数：

```go
// wrapCronFunc 把 cron 实际 fn 包成"注入 traceID + RunWithLock"的整体
func wrapCronFunc(locker lock.Locker, key string, opts LockOptions, fn func(ctx context.Context)) func() {
    return func() {
        ctx := context.WithValue(context.Background(), constant.CtxKeyTraceID, uuid.New().String())
        log := logger.WithCtx(ctx)
        RunWithLock(ctx, log, locker, key, opts, fn)
    }
}
```

各 cron struct 增加 `locker lock.Locker` 字段，`Start()` 模板：

```go
func (c *SessionDeduplicateCron) Start() error {
    opts := LockOptions{TTL: c.lockTTL, RenewInterval: c.lockRenewInterval}
    key := fmt.Sprintf(constant.CronLockKeyTemplate, constant.CronModuleSessionDeduplicate)
    entryID, err := c.cron.AddFunc(constant.CronSpecSessionDeduplicate, wrapCronFunc(c.locker, key, opts, c.deduplicate))
    if err != nil {
        logger.Logger().Error("[SessionDeduplicateCron] Add func error", zap.Error(err))
        return err
    }
    logger.Logger().Info("[SessionDeduplicateCron] Add func success", zap.Int("entryID", int(entryID)))
    c.cron.Start()
    return nil
}
```

注：traceID 由 `wrapCronFunc` 注入到 `parentCtx`；`RunWithLock` 内部 `childCtx` 派生自 `parentCtx`，链路上 `logger.WithCtx` 与 `c.db.WithContext(childCtx)` 都能取到同一 traceID。

### 3.4 4 个 cron fn 的 ctx 改造

每个 `deduplicate/summarize/score/purge` 当前签名是 `func (c *XxxCron) xxx()`，内部 `ctx := context.WithValue(context.Background(), ...)`。改造：

```go
func (c *SessionDeduplicateCron) deduplicate(ctx context.Context) {
    // 直接使用传入的 ctx（来自 RunWithLock 的 childCtx）
    log := logger.WithCtx(ctx)
    db := c.db.WithContext(ctx)
    // ... 业务不变
}
```

注意：`session_summarize` 和 `session_score` 内提交到协程池的 task 用 `ctx` 字段（`dto.SummarizeTask.Ctx`），过去是从 cron 自己创建的 ctx 传入，改造后用 `RunWithLock` 提供的 childCtx，语义等价（childCtx 是 parentCtx 的派生，parentCtx 由 cron fn 注入 traceID）。

### 3.5 Redis Key 模板

`internal/common/constant/rediskey.go` 新增：

```go
CronLockKeyTemplate = "cron:lock:%s"
```

例：`cron:lock:SessionDeduplicateCron`、`cron:lock:SoftDeletePurgeCron`。

## 4. 错误处理

| 场景 | 行为 |
|------|------|
| `Lock` 返回 `err != nil`（Redis 故障） | log.Error，return，fn 不执行 |
| `Lock` 返回 `success=false`（被其他实例持有） | log.Info "Lock held by another instance, skip this run"，return，fn 不执行 |
| `Refresh` 返回 `err != nil` | log.Warn，累加 `failCount`；未达上限继续续期；达到上限停止续期，fn 继续 |
| `Refresh` 返回 `success=false`（锁已丢） | log.Warn "Lock lost, stop renewing"，停止续期，**fn 不中断** |
| `Unlock` 返回 `err` | log.Error，不影响 fn 返回结果 |
| `parentCtx` 被 cancel | renewLoop 退出；fn 内部使用 `childCtx` 的协程池任务可能仍跑（与现状一致，不在本次设计范围） |

**续期失败容忍度决策**：基础设施抖动（Redis 瞬时不可用）不应打断业务长任务；去重/打分/清理任务都是幂等的，即使锁中途丢失也不影响数据正确性。

## 5. 测试

### 5.1 单元测试（`test/unit/cron/lock_runner_test.go` 新增）

使用 `miniredis` 真实交互 + 注入真实 `redisLocker`：

| 用例 | 场景 | 断言 |
|------|------|------|
| `TestRunWithLock_LockFailed_SkipsFn` | 预占 key，RunWithLock | log info 出现"skip"，fn 未调用，key 仍为预占者持有 |
| `TestRunWithLock_LockSuccess_RunsFnAndUnlocks` | 正常路径 | fn 跑完，defer Unlock 后 key 消失 |
| `TestRunWithLock_RefreshesLock` | fn 内 `time.Sleep` > TTL | miniredis `FastForward` 验证 PEXPIRE 被调用，fn 不被中断 |
| `TestRunWithLock_RenewFailure_StopsRenewal_KeepsFnRunning` | 注入 mock locker，Refresh 持续返回 `err` | 3 次后 stop renewing，fn 继续跑完 |
| `TestRunWithLock_LockLost_StopsRenewal_KeepsFnRunning` | mock locker Refresh 返回 `success=false` | 立即 stop renewing，fn 继续跑完 |
| `TestRunWithLock_DeferUnlockOnFnReturn` | fn panic | （可选）用 recover 验证 defer Unlock 仍执行 |

### 5.2 注册表测试（`test/unit/cron/cron_test.go` 扩展）

- 验证 `InitCronJobs(db, poolManager, cache)` 接受 cache 后构造的 cron 实例持有 locker。
- 验证 `LockTTL=0` 走默认值（通过 `*MockCron` 暴露构造时的入参）。
- 验证 `LockTTL=2*time.Minute` 时被透传。

### 5.3 E2E

本次不强制增加多实例 E2E（受本地单实例环境限制）。后续若部署到多实例环境，可补一个 2 进程的互斥验证：两个进程同时启 SessionDeduplicateCron，断言 Redis 上锁 key 同一时刻只被一方持有。

## 6. 兼容性 & 迁移

- **`internal/lock.Locker` 接口变更**：`+1` 方法。唯一已知实现是 `redisLocker`（同文件内），同步补实现；middleware `RedisLockMiddleware` 走的是 `redisLocker`，但不需要 `Refresh`，无影响。
- **`InitCronJobs` 签名**：从 `(db, poolManager)` 变为 `(db, poolManager, cache)`。同步修改 `cmd/server.go` 的调用点、`container.go` 的 `InitInfrastructure`、所有单测。
- **`CronRegistryEntry` 新增字段**：未填则走默认，向后兼容。
- **4 个 cron fn 签名变化**：从无参方法改为 `(ctx context.Context)`，每个调用点已重构。

## 7. 风险 & 缓解

| 风险 | 缓解 |
|------|------|
| 续期失败但 fn 继续跑，锁已丢时可能与新持锁实例并发 | 4 个任务（去重/打分/清理/总结）均为幂等操作；最多浪费少量 LLM 配额，无脏数据风险 |
| TTL=5min 偏短，长任务（千级 session 总结）可能不够 | 续期间隔 1min 最多 4 次续期窗口；如真实需要，可在 `CronRegistryEntry` 单任务覆盖 `LockTTL` |
| `miniredis` 对 Lua `PEXPIRE` 毫秒参数的处理 | miniredis 支持 PEXPIRE（毫秒），但 unit test 需传入整毫秒值，避免浮点截断 |
| cron fn panic 时 defer 仍需 Unlock | `RunWithLock` 的 defer 写在 `childCtx`/`cancel()` 之后、fn 调用之前，符合 Go defer 语义；`defer` 会随函数返回执行（包括 panic），需用 `recover` 包住 fn 避免进程崩溃（不在本次设计范围） |

## 8. 关键决策记录

| 决策 | 理由 |
|------|------|
| 续期失败不中断 fn | 基础设施抖动不应导致业务中断；4 任务均幂等 |
| 拿锁失败不重试 | cron 定时触发，丢一次等下一轮即可，无需重试逻辑 |
| 续期间隔 1min（5min TTL 下） | 至少 4 次续期窗口，语义可预测；保留 TTL/3 回退 |
| 不引入第三方锁库 | `go-redis` 自身 + Lua 已足够，避免依赖膨胀 |
| `childCtx` 派生自 parentCtx | 沿用 cron fn 内部 traceID 注入习惯，零侵入 |
| 各 cron fn 改为接收 ctx 形参 | 替换原 `context.WithValue(background, ...)` 写法，traceID 由调用方注入 |
