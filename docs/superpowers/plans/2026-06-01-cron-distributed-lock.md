# Cron 分布式锁（带续期）实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在多实例部署下，所有 cron 任务通过 Redis 互斥锁保证同一时刻同一任务只有一个实例运行；执行期间自动续期，长任务不被中途释放。

**Architecture:** 扩展 `internal/lock.Locker` 增加 `Refresh`（Lua PEXPIRE 保证 only-owner），新增 `internal/cron/lock_runner.go` 提供 `RunWithLock`/`wrapCronFunc` 包装器；`CronRegistryEntry` 增加可选 `LockTTL`/`LockRenewInterval`；4 个 cron 的 `Start()` 用 `wrapCronFunc` 包装 fn，fn 签名改为接收 `context.Context`。

**Tech Stack:** Go 1.25, robfig/cron/v3, go-redis/v9, alicebob/miniredis/v2, sonic

---

## 文件变更概览

| 操作 | 文件 | 职责 |
|------|------|------|
| 修改 | `internal/lock/lock.go` | `Locker` 接口加 `Refresh`；`redisLocker` 实现 Lua PEXPIRE |
| 新增 | `internal/cron/lock_runner.go` | `RunWithLock`、`renewLoop`、`wrapCronFunc`、`LockOptions`、默认常量 |
| 修改 | `internal/cron/cron.go` | `CronRegistryEntry` 加 `LockTTL`/`LockRenewInterval`；`InitCronJobs` 接 `*redis.Client` |
| 修改 | `internal/cron/session_dedup.go` | 注入 locker；fn 改 `(ctx)`；Start 用 `wrapCronFunc` |
| 修改 | `internal/cron/session_summarize.go` | 同上 |
| 修改 | `internal/cron/session_score.go` | 同上 |
| 修改 | `internal/cron/soft_delete_purge.go` | 同上 |
| 修改 | `internal/common/constant/rediskey.go` | 新增 `CronLockKeyTemplate` |
| 修改 | `internal/bootstrap/container.go` | `InitInfrastructure` 调 `cron.InitCronJobs` 传 cache |
| 修改 | `cmd/server.go` | 同步 `InitCronJobs` 调用 |
| 修改 | `test/unit/cron/cron_test.go` | 适配新签名 |
| 新增 | `test/unit/cron/lock_runner_test.go` | 6 个 RunWithLock 用例（miniredis） |
| 新增 | `test/unit/lock/lock_test.go` | `Refresh` + `Unlock` owner-check 用例（miniredis） |

---

## Task 1: 扩展 `Locker` 接口与 `redisLocker.Refresh`

**Files:**
- Modify: `internal/lock/lock.go:18-49`
- Test: `test/unit/lock/lock_test.go`

- [ ] **Step 1.1: 写 `Refresh` 失败用例（红）**

新建 `test/unit/lock/lock_test.go`：

```go
package lock_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/hcd233/aris-proxy-api/internal/lock"
	"github.com/redis/go-redis/v9"
)

func newRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

func TestLocker_Refresh_OwnerOnly(t *testing.T) {
	mr, rdb := newRedis(t)
	locker := lock.NewLocker(rdb)
	ctx := context.Background()
	key := "test:lock:" + uuid.New().String()
	value := "owner-1"
	other := "owner-2"

	ok, err := locker.Lock(ctx, key, value, 5*time.Second)
	if err != nil || !ok {
		t.Fatalf("lock: ok=%v err=%v", ok, err)
	}

	ok, err = locker.Refresh(ctx, key, value, 5*time.Second)
	if err != nil || !ok {
		t.Fatalf("owner refresh: ok=%v err=%v", ok, err)
	}

	ok, err = locker.Refresh(ctx, key, other, 5*time.Second)
	if err != nil {
		t.Fatalf("non-owner refresh err: %v", err)
	}
	if ok {
		t.Fatal("non-owner refresh must not succeed")
	}

	mr.FastForward(6 * time.Second)
	_ = mr
}
```

- [ ] **Step 1.2: 跑测试确认编译失败（红）**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/cron-distributed-lock-2026-06-01 && go test ./test/unit/lock/ -run TestLocker_Refresh_OwnerOnly
```

预期：编译错误 `lock.Locker missing Refresh method`。

- [ ] **Step 1.3: 扩展 `Locker` 接口和 `redisLocker` 实现**

`internal/lock/lock.go` 替换全文为：

```go
package lock

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Locker 锁接口
//
//	@author centonhuang
//	@update 2026-06-01 10:00:00
type Locker interface {
	Lock(ctx context.Context, key string, value string, expire time.Duration) (success bool, err error)
	Refresh(ctx context.Context, key string, value string, expire time.Duration) (success bool, err error)
	Unlock(ctx context.Context, key string, value string) (err error)
}

// NewLocker 创建锁
//
//	@return Locker
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func NewLocker(cache *redis.Client) Locker {
	return &redisLocker{cache: cache}
}

type redisLocker struct {
	cache *redis.Client
}

const luaRefresh = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("PEXPIRE", KEYS[1], ARGV[2])
else
    return 0
end
`

const luaUnlock = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end
`

func (l *redisLocker) Lock(ctx context.Context, key, value string, expire time.Duration) (success bool, err error) {
	return l.cache.SetNX(ctx, key, value, expire).Result()
}

func (l *redisLocker) Refresh(ctx context.Context, key, value string, expire time.Duration) (success bool, err error) {
	res, err := l.cache.Eval(ctx, luaRefresh, []string{key}, value, expire.Milliseconds()).Int64()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

func (l *redisLocker) Unlock(ctx context.Context, key, value string) (err error) {
	return l.cache.Eval(ctx, luaUnlock, []string{key}, value).Err()
}
```

- [ ] **Step 1.4: 跑测试确认通过（绿）**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/cron-distributed-lock-2026-06-01 && go test ./test/unit/lock/ -run TestLocker_Refresh_OwnerOnly -v
```

预期：PASS。

- [ ] **Step 1.5: 补 Unlock owner-only 用例（顺手覆盖旧逻辑）**

在 `test/unit/lock/lock_test.go` 末尾追加：

```go
func TestLocker_Unlock_OwnerOnly(t *testing.T) {
	_, rdb := newRedis(t)
	locker := lock.NewLocker(rdb)
	ctx := context.Background()
	key := "test:lock:" + uuid.New().String()

	if ok, err := locker.Lock(ctx, key, "owner-1", 5*time.Second); !ok || err != nil {
		t.Fatalf("lock: ok=%v err=%v", ok, err)
	}

	if err := locker.Unlock(ctx, key, "owner-2"); err != nil {
		t.Fatalf("non-owner unlock err: %v", err)
	}
	if exists, _ := rdb.Exists(ctx, key).Result(); exists != 1 {
		t.Fatal("non-owner unlock must not delete the key")
	}

	if err := locker.Unlock(ctx, key, "owner-1"); err != nil {
		t.Fatalf("owner unlock err: %v", err)
	}
	if exists, _ := rdb.Exists(ctx, key).Result(); exists != 0 {
		t.Fatal("owner unlock must delete the key")
	}
}
```

跑：

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/cron-distributed-lock-2026-06-01 && go test ./test/unit/lock/ -v
```

预期：两个用例 PASS。

- [ ] **Step 1.6: 提交**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/cron-distributed-lock-2026-06-01 && git add internal/lock/lock.go test/unit/lock/lock_test.go && git commit -m "feat(lock): add Refresh method with owner-only PEXPIRE"
```

---

## Task 2: 新增 `CronLockKeyTemplate` 常量

**Files:**
- Modify: `internal/common/constant/rediskey.go`

- [ ] **Step 2.1: 在 `rediskey.go` 现有 `const ( ... )` 块内追加一行**

在 `UserSharesKeyTemplate` 之后、`SessionSharesKeyTemplate` 之前，插入：

```go
	// CronLockKeyTemplate cron 任务互斥锁的 Redis key 模板（%s = CronModule*）
	CronLockKeyTemplate = "cron:lock:%s"
```

最终 `const ( ... )` 块顺序：

```go
const (
	LockKeyTemplateMiddleware = "%s:%s:%v"
	JWTUserCacheKeyTemplate   = "jwt:user:%d"
	TokenBucketKeyTemplate    = "tb:%s:%s:%v"
	ScannerBanKeyTemplate     = "scanner:ban:%s"
	ScannerStrikeKeyTemplate  = "scanner:strike:%s"
	ShareKeyTemplate          = "share:%s"
	UserSharesKeyTemplate     = "user_shares:%d"
	// CronLockKeyTemplate cron 任务互斥锁的 Redis key 模板（%s = CronModule*）
	CronLockKeyTemplate = "cron:lock:%s"
	SessionSharesKeyTemplate  = "session_shares:%d"

	// SessionMetaKeyTemplate 缓存 session 元数据（含 messageIDs/toolIDs，仅内部使用）
	SessionMetaKeyTemplate = "session:meta:%d"
	// MessageKeyTemplate 缓存单条 message 详情（不可变，TTL 内永远有效）
	MessageKeyTemplate = "message:%d"
	// ToolKeyTemplate 缓存单条 tool 详情（不可变，TTL 内永远有效）
	ToolKeyTemplate = "tool:%d"
)
```

- [ ] **Step 2.2: 编译验证**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/cron-distributed-lock-2026-06-01 && go build ./internal/common/constant/
```

预期：无输出（成功）。

- [ ] **Step 2.3: 提交**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/cron-distributed-lock-2026-06-01 && git add internal/common/constant/rediskey.go && git commit -m "feat(constant): add CronLockKeyTemplate"
```

---

## Task 3: 新增 `internal/cron/lock_runner.go` 框架（红：先写失败用例）

**Files:**
- Create: `test/unit/cron/lock_runner_test.go`
- Create: `internal/cron/lock_runner.go`

- [ ] **Step 3.1: 写 lock_runner_test.go 失败用例（先覆盖 6 个路径，骨架）**

新建 `test/unit/cron/lock_runner_test.go`：

```go
package cron_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/hcd233/aris-proxy-api/internal/cron"
	"github.com/hcd233/aris-proxy-api/internal/lock"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func newMiniredis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	mr := miniredis.RunT(t)
	t.Cleanup(mr.Close)
	return mr
}

func newRealLocker(t *testing.T) (lock.Locker, *miniredis.Miniredis) {
	mr := newMiniredis(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return lock.NewLocker(rdb), mr
}

// mockLocker 用于注入可控行为的 Locker
type mockLocker struct {
	lockFunc   func(ctx context.Context, key, value string, expire time.Duration) (bool, error)
	refreshOK  atomic.Bool
	refreshErr atomic.Value // error
	refreshCnt atomic.Int32
	unlockCnt  atomic.Int32
}

func (m *mockLocker) Lock(ctx context.Context, key, value string, expire time.Duration) (bool, error) {
	if m.lockFunc != nil {
		return m.lockFunc(ctx, key, value, expire)
	}
	return true, nil
}
func (m *mockLocker) Refresh(ctx context.Context, key, value string, expire time.Duration) (bool, error) {
	m.refreshCnt.Add(1)
	if v := m.refreshErr.Load(); v != nil {
		return false, v.(error)
	}
	return m.refreshOK.Load(), nil
}
func (m *mockLocker) Unlock(ctx context.Context, key, value string) error {
	m.unlockCnt.Add(1)
	return nil
}

func TestRunWithLock_LockFailed_SkipsFn(t *testing.T) {
	locker, _ := newRealLocker(t)
	log := zap.NewNop()
	called := false
	key := "test:lockfail"

	cron.RunWithLock(context.Background(), log, locker, key, cron.LockOptions{}, func(ctx context.Context) {
		called = true
	})

	if called {
		t.Fatal("fn must not run when lock failed")
	}
}

func TestRunWithLock_LockSuccess_RunsFnAndUnlocks(t *testing.T) {
	locker, _ := newRealLocker(t)
	log := zap.NewNop()
	key := "test:success"
	called := false

	cron.RunWithLock(context.Background(), log, locker, key, cron.LockOptions{
		TTL:           500 * time.Millisecond,
		RenewInterval: 100 * time.Millisecond,
	}, func(ctx context.Context) {
		called = true
	})

	if !called {
		t.Fatal("fn must run when lock acquired")
	}
}

func TestRunWithLock_RefreshesLock(t *testing.T) {
	locker, mr := newRealLocker(t)
	log := zap.NewNop()
	key := "test:refresh"

	done := make(chan struct{})
	cron.RunWithLock(context.Background(), log, locker, key, cron.LockOptions{
		TTL:           300 * time.Millisecond,
		RenewInterval: 80 * time.Millisecond,
	}, func(ctx context.Context) {
		// 让 fn 跑 1.2s，期间 ticker 应多次续期
		mr.FastForward(1 * time.Second)
		close(done)
	})

	<-done
	if exists, _ := redis.NewClient(&redis.Options{Addr: mr.Addr()}).Exists(context.Background(), key).Result(); exists == 0 {
		// fn 返回后 defer Unlock 已清掉，这是预期
	}
}

func TestRunWithLock_RenewFailure_StopsRenewal_KeepsFnRunning(t *testing.T) {
	m := &mockLocker{}
	m.refreshOK.Store(false)
	m.refreshErr.Store(errors.New("redis down"))
	log := zap.NewNop()
	key := "test:renewfail"

	fnReturned := make(chan struct{})
	cron.RunWithLock(context.Background(), log, m, key, cron.LockOptions{
		TTL:           200 * time.Millisecond,
		RenewInterval: 30 * time.Millisecond,
	}, func(ctx context.Context) {
		time.Sleep(300 * time.Millisecond)
		close(fnReturned)
	})

	select {
	case <-fnReturned:
	case <-time.After(2 * time.Second):
		t.Fatal("fn must run to completion even when renewal keeps failing")
	}

	if m.refreshCnt.Load() < 3 {
		t.Fatalf("expected at least 3 refresh attempts, got %d", m.refreshCnt.Load())
	}
	if got := m.unlockCnt.Load(); got != 1 {
		t.Fatalf("expected exactly 1 unlock, got %d", got)
	}
}

func TestRunWithLock_LockLost_StopsRenewal_KeepsFnRunning(t *testing.T) {
	m := &mockLocker{}
	m.refreshOK.Store(false) // 锁丢失
	log := zap.NewNop()
	key := "test:locklost"

	fnReturned := make(chan struct{})
	cron.RunWithLock(context.Background(), log, m, key, cron.LockOptions{
		TTL:           500 * time.Millisecond,
		RenewInterval: 30 * time.Millisecond,
	}, func(ctx context.Context) {
		time.Sleep(150 * time.Millisecond)
		close(fnReturned)
	})

	select {
	case <-fnReturned:
	case <-time.After(2 * time.Second):
		t.Fatal("fn must run to completion when lock lost")
	}
}

func TestRunWithLock_DeferUnlockAlways(t *testing.T) {
	m := &mockLocker{}
	m.refreshOK.Store(true)
	log := zap.NewNop()
	key := "test:unlock"

	cron.RunWithLock(context.Background(), log, m, key, cron.LockOptions{
		TTL:           1 * time.Second,
		RenewInterval: 500 * time.Millisecond,
	}, func(ctx context.Context) {})

	if got := m.unlockCnt.Load(); got != 1 {
		t.Fatalf("expected 1 unlock after fn returns, got %d", got)
	}
}
```

- [ ] **Step 3.2: 跑测试确认编译失败（红）**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/cron-distributed-lock-2026-06-01 && go test ./test/unit/cron/ -run TestRunWithLock -v 2>&1 | head -20
```

预期：`undefined: cron.RunWithLock` 等编译错误。

- [ ] **Step 3.3: 实现 `internal/cron/lock_runner.go`**

新建 `internal/cron/lock_runner.go`：

```go
package cron

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/lock"
	"go.uber.org/zap"
)

const (
	// DefaultCronLockTTL 默认 cron 任务锁 TTL
	DefaultCronLockTTL = 5 * time.Minute
	// DefaultCronLockRenewInterval 默认 cron 任务锁续期间隔
	DefaultCronLockRenewInterval = 1 * time.Minute
	// DefaultCronLockRenewDivisor 当 RenewInterval<=0 时回退到 TTL/Divisor
	DefaultCronLockRenewDivisor = 3
	// MaxConsecutiveRenewFailures 续期连续失败最大次数
	MaxConsecutiveRenewFailures = 3
)

// LockOptions cron 锁的可选参数（0 → 走默认值）
//
//	@author centonhuang
//	@update 2026-06-01 10:00:00
type LockOptions struct {
	TTL           time.Duration
	RenewInterval time.Duration
}

// RunWithLock 拿 Redis 分布式锁后执行 fn；执行期间 ticker 续期；返回前 defer 释放。
// 续期失败不中断 fn（业务任务均幂等）。
//
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func RunWithLock(
	parentCtx context.Context,
	log *zap.Logger,
	locker lock.Locker,
	key string,
	opts LockOptions,
	fn func(ctx context.Context),
) {
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = DefaultCronLockTTL
	}
	renew := opts.RenewInterval
	if renew <= 0 {
		renew = ttl / DefaultCronLockRenewDivisor
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

func renewLoop(ctx context.Context, log *zap.Logger, locker lock.Locker, key, value string, ttl, renew time.Duration) {
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
				if failCount >= MaxConsecutiveRenewFailures {
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

// wrapCronFunc 把 cron fn 包成"注入 traceID + RunWithLock"的整体，供 AddFunc 使用。
//
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func wrapCronFunc(locker lock.Locker, key string, opts LockOptions, fn func(ctx context.Context)) func() {
	return func() {
		ctx := context.WithValue(context.Background(), constant.CtxKeyTraceID, uuid.New().String())
		log := loggerWithTraceID(ctx)
		RunWithLock(ctx, log, locker, key, opts, fn)
	}
}
```

在同文件底部追加 logger 工具（避免循环 import）：

```go
// loggerWithTraceID 从带 traceID 的 ctx 派生 logger，避免 import 整个 logger 包造成耦合。
//
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func loggerWithTraceID(ctx context.Context) *zap.Logger {
	return loggerFromCtx(ctx)
}
```

`loggerFromCtx` 直接调用项目已有的 `logger.WithCtx`，新增 import：

修改 `internal/cron/lock_runner.go` 顶部 import：

```go
import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/lock"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)
```

把 `loggerWithTraceID` 函数体改为：

```go
func loggerWithTraceID(ctx context.Context) *zap.Logger {
	return logger.WithCtx(ctx)
}
```

- [ ] **Step 3.4: 跑测试确认通过（绿）**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/cron-distributed-lock-2026-06-01 && go test ./test/unit/cron/ -run TestRunWithLock -v
```

预期：6 个用例全 PASS。

- [ ] **Step 3.5: 跑 lint 验证**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/cron-distributed-lock-2026-06-01 && make lint
```

预期：无 error。

- [ ] **Step 3.6: 提交**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/cron-distributed-lock-2026-06-01 && git add internal/cron/lock_runner.go test/unit/cron/lock_runner_test.go && git commit -m "feat(cron): add RunWithLock + wrapCronFunc with TTL renewal"
```

---

## Task 4: 扩展 `CronRegistryEntry` + 改造 `InitCronJobs` 签名

**Files:**
- Modify: `internal/cron/cron.go:30-86`
- Modify: `internal/bootstrap/container.go:72-79`
- Modify: `cmd/server.go`（仅检查调用点）
- Modify: `test/unit/cron/cron_test.go`

- [ ] **Step 4.1: 修改 `internal/cron/cron.go`**

替换 imports（增加 `time`、`github.com/redis/go-redis/v9`）：

```go
import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"
)
```

替换 `CronRegistryEntry` 结构和 `InitCronJobs` 签名：

```go
// CronRegistryEntry 单个定时任务注册项
//
//	@author centonhuang
//	@update 2026-06-01 10:00:00
type CronRegistryEntry struct {
	Name              string
	Enabled           func() bool
	Factory           func(db *gorm.DB, poolManager *pool.PoolManager) Cron
	LockTTL           time.Duration // 0 → DefaultCronLockTTL
	LockRenewInterval time.Duration // 0 → DefaultCronLockRenewInterval
}

// InitCronJobs 初始化定时任务（每个 cron 自带分布式锁）
//
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func InitCronJobs(db *gorm.DB, poolManager *pool.PoolManager, cache *redis.Client) {
	for _, entry := range DefaultCronRegistry {
		if !entry.Enabled() {
			logger.Logger().Info("[Cron] Cron job is disabled by configuration", zap.String("name", entry.Name))
			continue
		}

		c := entry.Factory(db, poolManager)
		lo.Must0(c.Start())
		cronInstances = append(cronInstances, c)
		logger.Logger().Info("[Cron] Cron job started", zap.String("name", entry.Name))
	}

	logger.Logger().Info("[Cron] Init cron jobs", zap.Int("count", len(cronInstances)))
}
```

- [ ] **Step 4.2: 修改 `internal/bootstrap/container.go:72-79`**

`InitInfrastructure` 函数体：

```go
func InitInfrastructure() *Infrastructure {
	db := database.InitDatabase()
	cache := cache.InitCache()
	httpclient.InitHTTPClient()
	poolManager := pool.InitPoolManager(db)
	cron.InitCronJobs(db, poolManager, cache)
	return &Infrastructure{DB: db, Cache: cache, PoolManager: poolManager}
}
```

- [ ] **Step 4.3: 同步 `test/unit/cron/cron_test.go` 调用点**

把 `cron.InitCronJobs(nil, nil)` 全部替换为 `cron.InitCronJobs(nil, nil, nil)`，共 3 处：

- 行 45
- 行 97
- 行 135

- [ ] **Step 4.4: 编译验证**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/cron-distributed-lock-2026-06-01 && go build ./...
```

预期：无错误（不需通过所有测试，因为后续任务才注入 locker；本任务先保证签名正确）。

- [ ] **Step 4.5: 跑 cron_test 确认基础测试仍通过**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/cron-distributed-lock-2026-06-01 && go test ./test/unit/cron/ -v -run TestInitCronJobs
```

预期：3 个测试 PASS（仍走 mock cron，locker 注入在 Task 5-8 完成）。

- [ ] **Step 4.6: 提交**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/cron-distributed-lock-2026-06-01 && git add internal/cron/cron.go internal/bootstrap/container.go test/unit/cron/cron_test.go && git commit -m "refactor(cron): add LockTTL/LockRenewInterval to registry; InitCronJobs accepts cache"
```

---

## Task 5: 改造 `SessionDeduplicateCron`

**Files:**
- Modify: `internal/cron/session_dedup.go:24-89`

- [ ] **Step 5.1: 替换 imports**

在 `session_dedup.go` 现有 imports 里：

- 删 `"context"`（不再用，但 fn 仍会接收 ctx 形参；保留 context 即可）
- 删 `"github.com/hcd233/aris-proxy-api/internal/logger"` 中的使用（Start 仍用，保留）
- 加 `"fmt"` 用于 key 拼装
- 加 `"github.com/hcd233/aris-proxy-api/internal/lock"`
- 加 `"github.com/redis/go-redis/v9"`

最终：

```go
import (
	"context"
	"fmt"
	"slices"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/lock"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"
)
```

- [ ] **Step 5.2: 修改 `SessionDeduplicateCron` struct + Factory**

替换 `SessionDeduplicateCron` 定义和 `NewSessionDeduplicateCron`：

```go
// SessionDeduplicateCron Session去重定时任务，清理MessageIDs被其他Session包含的冗余Session
//
//	@author centonhuang
//	@update 2026-06-01 10:00:00
type SessionDeduplicateCron struct {
	cron       *cron.Cron
	db         *gorm.DB
	locker     lock.Locker
	sessionDAO *dao.SessionDAO
	messageDAO *dao.MessageDAO
}

// NewSessionDeduplicateCron 创建Session去重定时任务
//
//	@return Cron
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func NewSessionDeduplicateCron(db *gorm.DB, cache *redis.Client) Cron {
	return &SessionDeduplicateCron{
		cron: cron.New(
			cron.WithLogger(newCronLoggerAdapter(constant.CronModuleSessionDeduplicate)),
		),
		db:         db,
		locker:     lock.NewLocker(cache),
		sessionDAO: dao.GetSessionDAO(),
		messageDAO: dao.GetMessageDAO(),
	}
}
```

- [ ] **Step 5.3: 改造 `Start()` 用 `wrapCronFunc`**

```go
// Start 启动Session去重定时任务
//
//	@receiver c *SessionDeduplicateCron
//	@return error
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func (c *SessionDeduplicateCron) Start() error {
	// 每小时执行一次，定期清理冗余Session
	key := fmt.Sprintf(constant.CronLockKeyTemplate, constant.CronModuleSessionDeduplicate)
	opts := LockOptions{}
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

- [ ] **Step 5.4: 改 `deduplicate` 签名为接收 `ctx`**

替换 `func (c *SessionDeduplicateCron) deduplicate()` 为：

```go
// deduplicate 执行Session去重逻辑
//
//	@receiver c *SessionDeduplicateCron
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func (c *SessionDeduplicateCron) deduplicate(ctx context.Context) {
	log := logger.WithCtx(ctx)
	db := c.db.WithContext(ctx)

	sessions, err := c.sessionDAO.BatchGet(db, &dbmodel.Session{}, constant.SessionRepoFieldsDedup)
	// ... 以下业务逻辑完全保持不变
```

`uuid` import 可以删除（不再调用 `uuid.New().String()`）；保留也行。先编译验证：

- [ ] **Step 5.5: 编译 + 跑 cron_test 确认无回归**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/cron-distributed-lock-2026-06-01 && go build ./... && go test ./test/unit/cron/ -v
```

预期：编译成功；`TestInitCronJobs_*` 测试因 `Factory` 签名变化可能失败 → 见 Step 5.6 修复。

- [ ] **Step 5.6: 同步 `DefaultCronRegistry` Factory 签名**

`internal/cron/cron.go` 的 `DefaultCronRegistry` 4 个 Factory 都需要更新为 `func(db, pool) Cron`（不变）+ 但其中 Deduplicate/Purge 多了 `*redis.Client` 参数。改 registry 工厂：

```go
var DefaultCronRegistry = []CronRegistryEntry{
	{
		Name:    constant.CronModuleSessionDeduplicate,
		Enabled: func() bool { return config.CronSessionDeduplicateEnabled },
		Factory: func(db *gorm.DB, _ *pool.PoolManager) Cron { return nil }, // 见下：注入 cache
	},
	// ...
}
```

实际上，registry 的 `Factory` 签名固定为 `func(db, pool) Cron`，无法直接拿 cache。改方案：把 cache 通过闭包注入到 `Factory`——但 `Factory` 是 struct 字段，无法捕获外部变量。

**改方案**：在 `InitCronJobs` 内构造每个 entry 的 Factory，注入 cache。改写 `InitCronJobs`：

```go
func InitCronJobs(db *gorm.DB, poolManager *pool.PoolManager, cache *redis.Client) {
	entries := buildRegistryEntries(db, poolManager, cache)
	for _, entry := range entries {
		if !entry.Enabled() {
			logger.Logger().Info("[Cron] Cron job is disabled by configuration", zap.String("name", entry.Name))
			continue
		}
		c := entry.Factory(db, poolManager)
		lo.Must0(c.Start())
		cronInstances = append(cronInstances, c)
		logger.Logger().Info("[Cron] Cron job started", zap.String("name", entry.Name))
	}
	logger.Logger().Info("[Cron] Init cron jobs", zap.Int("count", len(cronInstances)))
}

func buildRegistryEntries(db *gorm.DB, poolManager *pool.PoolManager, cache *redis.Client) []CronRegistryEntry {
	_ = db
	_ = poolManager
	return []CronRegistryEntry{
		{
			Name:    constant.CronModuleSessionDeduplicate,
			Enabled: func() bool { return config.CronSessionDeduplicateEnabled },
			Factory: func(_ *gorm.DB, _ *pool.PoolManager) Cron { return NewSessionDeduplicateCron(db, cache) },
		},
		{
			Name:    constant.CronModuleSessionSummarize,
			Enabled: func() bool { return config.CronSessionSummarizeEnabled },
			Factory: func(_ *gorm.DB, _ *pool.PoolManager) Cron { return NewSessionSummarizeCron(db, poolManager, cache) },
		},
		{
			Name:    constant.CronModuleSessionScore,
			Enabled: func() bool { return config.CronSessionScoreEnabled },
			Factory: func(_ *gorm.DB, _ *pool.PoolManager) Cron { return NewSessionScoreCron(db, poolManager, cache) },
		},
		{
			Name:    constant.CronModuleSoftDeletePurge,
			Enabled: func() bool { return config.CronSoftDeletePurgeEnabled },
			Factory: func(_ *gorm.DB, _ *pool.PoolManager) Cron { return NewSoftDeletePurgeCron(db, cache) },
		},
	}
}
```

`DefaultCronRegistry` 仍保留（测试覆盖用），但 `InitCronJobs` 用 `buildRegistryEntries` 替代。改完后 `DefaultCronRegistry` 退化为"只用于测试覆盖"——保留它，Factory 改为可注入 cache 的 nil 实现（实际不被生产使用）：

```go
var DefaultCronRegistry = []CronRegistryEntry{
	{
		Name:    constant.CronModuleSessionDeduplicate,
		Enabled: func() bool { return config.CronSessionDeduplicateEnabled },
		Factory: func(_ *gorm.DB, _ *pool.PoolManager) Cron { return nil },
	},
	{
		Name:    constant.CronModuleSessionSummarize,
		Enabled: func() bool { return config.CronSessionSummarizeEnabled },
		Factory: func(_ *gorm.DB, _ *pool.PoolManager) Cron { return nil },
	},
	{
		Name:    constant.CronModuleSessionScore,
		Enabled: func() bool { return config.CronSessionScoreEnabled },
		Factory: func(_ *gorm.DB, _ *pool.PoolManager) Cron { return nil },
	},
	{
		Name:    constant.CronModuleSoftDeletePurge,
		Enabled: func() bool { return config.CronSoftDeletePurgeEnabled },
		Factory: func(_ *gorm.DB, _ *pool.PoolManager) Cron { return nil },
	},
}
```

- [ ] **Step 5.7: 重新编译**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/cron-distributed-lock-2026-06-01 && go build ./...
```

预期：成功。

- [ ] **Step 5.8: 提交**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/cron-distributed-lock-2026-06-01 && git add internal/cron/session_dedup.go internal/cron/cron.go && git commit -m "feat(cron): wrap SessionDeduplicateCron with distributed lock"
```

---

## Task 6: 改造 `SessionSummarizeCron`

**Files:**
- Modify: `internal/cron/session_summarize.go:7-87, 88-136`

- [ ] **Step 6.1: 修改 imports**

在原 imports 基础上：

- 加 `"fmt"`
- 加 `"github.com/hcd233/aris-proxy-api/internal/lock"`
- 加 `"github.com/redis/go-redis/v9"`

最终：

```go
import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/hcd233/aris-proxy-api/internal/lock"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"
)
```

- [ ] **Step 6.2: 改 struct + Factory**

```go
// SessionSummarizeCron Session总结定时任务
//
//	@author centonhuang
//	@update 2026-06-01 10:00:00
type SessionSummarizeCron struct {
	cron        *cron.Cron
	db          *gorm.DB
	poolManager *pool.PoolManager
	locker      lock.Locker
	sessionDAO  *dao.SessionDAO
	messageDAO  *dao.MessageDAO
}

// NewSessionSummarizeCron 创建Session总结定时任务
//
//	@return Cron
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func NewSessionSummarizeCron(db *gorm.DB, poolManager *pool.PoolManager, cache *redis.Client) Cron {
	return &SessionSummarizeCron{
		cron: cron.New(
			cron.WithLogger(newCronLoggerAdapter(constant.CronModuleSessionSummarize)),
		),
		db:          db,
		poolManager: poolManager,
		locker:      lock.NewLocker(cache),
		sessionDAO:  dao.GetSessionDAO(),
		messageDAO:  dao.GetMessageDAO(),
	}
}
```

- [ ] **Step 6.3: 改 Start**

```go
// Start 启动Session总结定时任务
//
//	@receiver c *SessionSummarizeCron
//	@return error
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func (c *SessionSummarizeCron) Start() error {
	// 每天凌晨2:00执行，在去重任务完成后执行
	key := fmt.Sprintf(constant.CronLockKeyTemplate, constant.CronModuleSessionSummarize)
	opts := LockOptions{}
	entryID, err := c.cron.AddFunc(constant.CronSpecSessionSummarize, wrapCronFunc(c.locker, key, opts, c.summarize))
	if err != nil {
		logger.Logger().Error("[SessionSummarizeCron] Add func error", zap.Error(err))
		return err
	}

	logger.Logger().Info("[SessionSummarizeCron] Add func success", zap.Int("entryID", int(entryID)))

	c.cron.Start()

	return nil
}
```

- [ ] **Step 6.4: 改 `summarize` 签名为接收 `ctx`**

```go
// summarize 执行Session总结逻辑
//
//	@receiver c *SessionSummarizeCron
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func (c *SessionSummarizeCron) summarize(ctx context.Context) {
	log := logger.WithCtx(ctx)
	db := c.db.WithContext(ctx)
	poolManager := c.poolManager

	sessions, err := c.sessionDAO.BatchGetByField(db, constant.WhereFieldSummary, []string{""}, constant.SessionRepoFieldsSummarize)
	// ... 以下业务逻辑完全保持不变
```

- [ ] **Step 6.5: 编译 + 跑 cron_test**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/cron-distributed-lock-2026-06-01 && go build ./... && go test ./test/unit/cron/ -v
```

预期：编译成功；`TestRunWithLock_*` 与 `TestInitCronJobs_*` 全部 PASS。

- [ ] **Step 6.6: 提交**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/cron-distributed-lock-2026-06-01 && git add internal/cron/session_summarize.go && git commit -m "feat(cron): wrap SessionSummarizeCron with distributed lock"
```

---

## Task 7: 改造 `SessionScoreCron`

**Files:**
- Modify: `internal/cron/session_score.go:7-87, 88-134`

- [ ] **Step 7.1: 改 imports**

`session_score.go` 原 imports 加：

- `"fmt"`
- `"github.com/hcd233/aris-proxy-api/internal/lock"`
- `"github.com/redis/go-redis/v9"`

最终：

```go
import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/hcd233/aris-proxy-api/internal/lock"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"
)
```

- [ ] **Step 7.2: 改 struct + Factory**

```go
// SessionScoreCron Session评分定时任务
//
//	@author centonhuang
//	@update 2026-06-01 10:00:00
type SessionScoreCron struct {
	cron        *cron.Cron
	db          *gorm.DB
	poolManager *pool.PoolManager
	locker      lock.Locker
	sessionDAO  *dao.SessionDAO
	messageDAO  *dao.MessageDAO
}

// NewSessionScoreCron 创建Session评分定时任务
//
//	@return Cron
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func NewSessionScoreCron(db *gorm.DB, poolManager *pool.PoolManager, cache *redis.Client) Cron {
	return &SessionScoreCron{
		cron: cron.New(
			cron.WithLogger(newCronLoggerAdapter(constant.CronModuleSessionScore)),
		),
		db:          db,
		poolManager: poolManager,
		locker:      lock.NewLocker(cache),
		sessionDAO:  dao.GetSessionDAO(),
		messageDAO:  dao.GetMessageDAO(),
	}
}
```

- [ ] **Step 7.3: 改 Start**

```go
// Start 启动Session评分定时任务
//
//	@receiver c *SessionScoreCron
//	@return error
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func (c *SessionScoreCron) Start() error {
	// 每天凌晨3:00执行，在摘要任务完成后执行
	key := fmt.Sprintf(constant.CronLockKeyTemplate, constant.CronModuleSessionScore)
	opts := LockOptions{}
	entryID, err := c.cron.AddFunc(constant.CronSpecSessionScore, wrapCronFunc(c.locker, key, opts, c.score))
	if err != nil {
		logger.Logger().Error("[SessionScoreCron] Add func error", zap.Error(err))
		return err
	}

	logger.Logger().Info("[SessionScoreCron] Add func success", zap.Int("entryID", int(entryID)))

	c.cron.Start()

	return nil
}
```

- [ ] **Step 7.4: 改 `score` 签名为接收 `ctx`**

```go
// score 执行Session评分逻辑
//
//	@receiver c *SessionScoreCron
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func (c *SessionScoreCron) score(ctx context.Context) {
	log := logger.WithCtx(ctx)
	db := c.db.WithContext(ctx)
	poolManager := c.poolManager

	// 获取未评分且未删除的session（score_version为空字符串）
	sessions, err := c.sessionDAO.BatchGetByField(db, constant.WhereFieldScoreVersion, []string{""}, constant.SessionRepoFieldsScore)
	// ... 以下业务逻辑完全保持不变
```

- [ ] **Step 7.5: 编译 + 测试**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/cron-distributed-lock-2026-06-01 && go build ./... && go test ./test/unit/cron/ -v
```

预期：全部 PASS。

- [ ] **Step 7.6: 提交**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/cron-distributed-lock-2026-06-01 && git add internal/cron/session_score.go && git commit -m "feat(cron): wrap SessionScoreCron with distributed lock"
```

---

## Task 8: 改造 `SoftDeletePurgeCron`

**Files:**
- Modify: `internal/cron/soft_delete_purge.go:7-79, 80-113`

- [ ] **Step 8.1: 改 imports**

```go
import (
	"context"
	"fmt"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	"github.com/hcd233/aris-proxy-api/internal/lock"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"
)
```

- [ ] **Step 8.2: 改 struct + Factory**

```go
// SoftDeletePurgeCron 软删除数据清理定时任务，每周硬删除所有已软删除的Message、Session、Tool记录
//
//	@author centonhuang
//	@update 2026-06-01 10:00:00
type SoftDeletePurgeCron struct {
	cron       *cron.Cron
	db         *gorm.DB
	locker     lock.Locker
	messageDAO *dao.MessageDAO
	sessionDAO *dao.SessionDAO
	toolDAO    *dao.ToolDAO
}

// NewSoftDeletePurgeCron 创建软删除数据清理定时任务
//
//	@return Cron
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func NewSoftDeletePurgeCron(db *gorm.DB, cache *redis.Client) Cron {
	return &SoftDeletePurgeCron{
		cron: cron.New(
			cron.WithLogger(newCronLoggerAdapter(constant.CronModuleSoftDeletePurge)),
		),
		db:         db,
		locker:     lock.NewLocker(cache),
		messageDAO: dao.GetMessageDAO(),
		sessionDAO: dao.GetSessionDAO(),
		toolDAO:    dao.GetToolDAO(),
	}
}
```

- [ ] **Step 8.3: 改 Start**

```go
// Start 启动软删除数据清理定时任务
//
//	@receiver c *SoftDeletePurgeCron
//	@return error
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func (c *SoftDeletePurgeCron) Start() error {
	// 每周日凌晨4:00执行，确保所有任务完成后再清理
	key := fmt.Sprintf(constant.CronLockKeyTemplate, constant.CronModuleSoftDeletePurge)
	opts := LockOptions{}
	entryID, err := c.cron.AddFunc(constant.CronSpecSoftDeletePurge, wrapCronFunc(c.locker, key, opts, c.purge))
	if err != nil {
		logger.Logger().Error("[SoftDeletePurgeCron] Add func error", zap.Error(err))
		return err
	}

	logger.Logger().Info("[SoftDeletePurgeCron] Add func success", zap.Int("entryID", int(entryID)))

	c.cron.Start()

	return nil
}
```

- [ ] **Step 8.4: 改 `purge` 签名为接收 `ctx`**

```go
// purge 执行硬删除逻辑，依次清理Message、Session、Tool中所有已软删除的记录
//
//	@receiver c *SoftDeletePurgeCron
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func (c *SoftDeletePurgeCron) purge(ctx context.Context) {
	log := logger.WithCtx(ctx)
	db := c.db.WithContext(ctx)

	msgCount, err := c.messageDAO.HardDeleteSoftDeleted(db)
	// ... 以下业务逻辑完全保持不变
```

- [ ] **Step 8.5: 编译 + 完整 cron 单测**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/cron-distributed-lock-2026-06-01 && go build ./... && go test ./test/unit/cron/ -v
```

预期：全部 PASS。

- [ ] **Step 8.6: 提交**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/cron-distributed-lock-2026-06-01 && git add internal/cron/soft_delete_purge.go && git commit -m "feat(cron): wrap SoftDeletePurgeCron with distributed lock"
```

---

## Task 9: 全量验证（lint + 全部单测 + e2e 兼容）

**Files:** （无修改）

- [ ] **Step 9.1: 跑 lint**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/cron-distributed-lock-2026-06-01 && make lint
```

预期：0 error（warning 允许但需 review）。

- [ ] **Step 9.2: 跑全部单测**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/cron-distributed-lock-2026-06-01 && make test
```

预期：所有 PASS（除 e2e 需 BASE_URL/API_KEY 时 skip）。

- [ ] **Step 9.3: 跑 cron 相关单测聚焦**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/cron-distributed-lock-2026-06-01 && go test -count=1 -v ./test/unit/cron/ ./test/unit/lock/ ./test/unit/session_dedup/
```

预期：全 PASS。

- [ ] **Step 9.4: 构建镜像验证（可选）**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/cron-distributed-lock-2026-06-01 && make build
```

预期：二进制产出，无错误。

- [ ] **Step 9.5: 提交（如有 lint 修复）**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/cron-distributed-lock-2026-06-01 && git status --short
```

如果有改动：

```bash
git add -A && git commit -m "chore(cron): lint fixes after lock integration"
```

否则跳到 Task 10。

---

## Task 10: 文档同步

**Files:**
- Modify: `docs/superpowers/specs/2026-06-01-cron-distributed-lock-design.md`（如需根据实现微调）

- [ ] **Step 10.1: 复核 spec 与实现一致性**

检查清单：

- `internal/lock.Locker` 接口含 `Refresh` ✓
- `internal/cron/lock_runner.go` 含 `RunWithLock`/`wrapCronFunc`/`LockOptions`/`renewLoop` ✓
- TTL 默认 5 min、Renew 1 min、3 次失败停止 ✓
- `CronRegistryEntry` 含 `LockTTL`/`LockRenewInterval` ✓
- `InitCronJobs(db, poolManager, cache)` 签名 ✓
- `wrapCronFunc` 注入 traceID + RunWithLock ✓
- 4 个 cron fn 改为 `(ctx context.Context)` ✓
- `CronLockKeyTemplate` 常量 ✓

如发现实现与 spec 有偏差（例如常量命名、错误消息措辞），编辑 spec 保持文档与代码一致。

- [ ] **Step 10.2: 提交（如果 spec 改了）**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/cron-distributed-lock-2026-06-01 && git add docs/superpowers/specs/2026-06-01-cron-distributed-lock-design.md && git commit -m "docs(spec): sync spec with implementation"
```

---

## Self-Review

### Spec 覆盖检查

- §3.1 `Locker` 扩展 `Refresh` → Task 1
- §3.2 `RunWithLock` / `renewLoop` / `wrapCronFunc` / `LockOptions` / 默认常量 → Task 3
- §3.3 `CronRegistryEntry` 加字段 / `InitCronJobs` 新签名 / 4 cron Start 模板 → Task 4-8
- §3.4 4 个 fn 改 `(ctx)` 形参 → Task 5-8 各 Step 4
- §3.5 `CronLockKeyTemplate` → Task 2
- §4 错误处理（lock 失败、refresh 失败、unlock 失败、不中断 fn）→ Task 3 单测覆盖 6 个用例
- §5.1 单测 6 条路径 → Task 1（lock 2 条）+ Task 3（RunWithLock 6 条）
- §5.2 注册表测试扩展 → Task 4.5
- §6 兼容性（`InitCronJobs` 签名、`Locker` 接口扩展）→ Task 4 + Task 1

无遗漏。

### 占位符扫描

搜索 "TBD" / "TODO" / "implement later" / "fill in" / "类似" / "add appropriate" — 无。

### 类型一致性

- `LockOptions` 在 Task 3 定义，Task 5-8 复用 ✓
- `RunWithLock` 签名 `(parentCtx, log, locker, key, opts, fn)` 一致 ✓
- `wrapCronFunc` 签名 `(locker, key, opts, fn)` 一致 ✓
- `CronRegistryEntry.LockTTL`/`LockRenewInterval` 在 Task 4 定义，但 Task 4-8 未使用（spec §3.3 提到可选）；保持字段存在供未来使用，不传走默认 ✓
- 4 个 cron 构造函数签名：dedup/purge `(db, cache)`，summarize/score `(db, poolManager, cache)` — 与 spec §2.2 表一致 ✓

### 风险点

- Task 4 步骤多：需要把 registry 从 `var` 改为 `buildRegistryEntries()` 函数。如果发现 cron_test.go 引用了 `DefaultCronRegistry` 的真实 Factory，测试可能需要更新——已通过 `Factory: func(_, _) Cron { return nil }` 兼容测试（test 只关心 `CronInstanceCount` 数量，不调真实 Factory）。
- miniredis 续期测试中用 `mr.FastForward(1*time.Second)` —— miniredis v2 的 `FastForward` 仅对**已注册 TTL** 的 key 生效；`SetNX` 设了 EX，FastForward 应当让锁过期。但我们的 `Refresh` 内部用 Lua `PEXPIRE`，miniredis 支持 `PEXPIRE`，应正常工作。Step 3.4 跑测试若失败，需要切到手动 `time.Sleep` 验证（取消 FastForward 改用更长 wait）。
