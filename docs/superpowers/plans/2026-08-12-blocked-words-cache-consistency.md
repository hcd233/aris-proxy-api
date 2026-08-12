# 敏感词多副本缓存一致性 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让敏感词增删改后所有 2 个生产副本在秒级内收敛到 PG 最新状态（pub/sub 即时 + 版本轮询兜底 + 低频重建兜底）。

**Architecture:** PG `blocked_words` 保持唯一真源；Redis 仅做"变更信号"（`blocked:changed` pub/sub 频道 + `blocked:version` 计数器）；各 pod 进程内 AC matcher 从 PG 全量幂等重建。写路径：PG 写 → 本进程 rebuildNotify（现状保留）+ `NotifyChanged`（Publish + INCR）。读路径：`BlockedService.StartSync(ctx)` 统一启动订阅 goroutine + 2s 轮询 goroutine。

**Tech Stack:** Go, robfig/cron（仅现有任务，本次不新增 cron 条目）, redis/go-redis v9, miniredis v2（测试）, 现有测试布局 `test/unit/blocked_matcher/`。

## Global Constraints

- 数据真源永远只有 PG 一份；Redis 不存敏感词数据本身（只存 version 计数器和 pub/sub 信号）。
- `Rebuild()` 签名从 `func(ctx)` 改为 `func(ctx) error`；失败时**保持原 matcher 不变**（不清空），游标（`lastSeenVersion`/`lastRebuildAt`）不更新，2s 后自动重试。
- 不新增 cron 条目；`internal/cron/blocked_hit_sync.go` 不动。
- 现有 `rebuildNotify func(ctx)` handler 构造参数签名不变，装配侧（`bootstrap/modules/application.go`）用闭包适配。
- 测试文件放 `test/unit/blocked_matcher/`，包名 `blocked_matcher_test`，用 fake repo + miniredis，无 mock 框架。
- 所有命令在仓库根目录执行；测试命令：`go test -count=1 ./internal/... ./test/...`（Makefile `GO_TEST_PACKAGES`）。

---

### Task 1: BlockedService 核心改造（cache 依赖 + Rebuild 返回 error + NotifyChanged）

**Files:**
- Modify: `internal/application/blocked/service.go`（全文重写，见下方完整代码）
- Modify: `internal/common/constant/string.go`（+2 常量，`BlockedHitKeyPrefix` 附近）
- Test: `test/unit/blocked_matcher/blocked_service_sync_test.go`（新建）
- Modify: `test/unit/blocked_matcher/deny_ids_test.go`（适配签名：`NewBlockedService` 加 nil cache、`Rebuild` 忽略返回值）

**Interfaces:**
- Consumes: `domain.BlockedRepository`（已有）、`port.HitRecorder`（已有）、`*redis.Client`（本次新增）
- Produces:
  - `func NewBlockedService(repo domain.BlockedRepository, hitRecorder port.HitRecorder, cache *redis.Client) *BlockedService`
  - `func (s *BlockedService) Rebuild(ctx context.Context) error`
  - `func (s *BlockedService) NotifyChanged(ctx context.Context)`（Publish + INCR，best-effort）
  - 常量 `constant.BlockedVersionKey = "blocked:version"`、`constant.BlockedChangeChannel = "blocked:changed"`
  - 字段 `lastSeenVersion atomic.Int64`、`lastRebuildAt atomic.Int64`（UnixNano；0=从未成功重建；Task 2 使用）

- [ ] **Step 1: 添加常量**

在 `internal/common/constant/string.go` 的 `BlockedHitKeyScanPattern` 常量（约 190 行）后追加：

```go
	BlockedHitKeyScanPattern          = "blocked:hit:*"
	BlockedVersionKey                 = "blocked:version"
	BlockedChangeChannel              = "blocked:changed"
```

- [ ] **Step 2: 改写 service.go 为完整实现**

将 `internal/application/blocked/service.go` 全文替换为：

```go
package blocked

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	domain "github.com/hcd233/aris-proxy-api/internal/domain/blocked"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/logger"

	"github.com/hcd233/aris-proxy-api/internal/application/blocked/port"
)

type BlockedService struct {
	mu          sync.RWMutex
	matcher     *ACmatcher
	wordIDs     map[string]uint
	wordByID    map[uint]string
	actionByID  map[uint]string
	repo        domain.BlockedRepository
	hitRecorder port.HitRecorder
	cache       *redis.Client

	lastSeenVersion atomic.Int64
	lastRebuildAt   atomic.Int64 // UnixNano；0 表示从未成功重建
}

func NewBlockedService(repo domain.BlockedRepository, hitRecorder port.HitRecorder, cache *redis.Client) *BlockedService {
	return &BlockedService{
		repo: repo, matcher: NewACmatcher(make(map[uint]string)), hitRecorder: hitRecorder, cache: cache,
		actionByID: make(map[uint]string),
	}
}

func (s *BlockedService) rebuild(words map[uint]string) {
	s.matcher = NewACmatcher(words)
	s.wordIDs = lo.Invert(words)
	s.wordByID = words
}

// Rebuild 从 DB 全量重建内存 matcher；失败时保持原 matcher 不变并返回 error。
func (s *BlockedService) Rebuild(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.repo.ListAll(ctx)
	if err != nil {
		logger.WithCtx(ctx).Error("[BlockedService] Rebuild failed, keep old matcher", zap.Error(err))
		return err
	}
	words := lo.SliceToMap(all, func(b *aggregate.Blocked) (uint, string) {
		return b.AggregateID(), b.Word()
	})
	s.rebuild(words)
	s.actionByID = lo.SliceToMap(all, func(b *aggregate.Blocked) (uint, string) {
		return b.AggregateID(), b.Action()
	})
	s.lastRebuildAt.Store(time.Now().UnixNano())
	return nil
}

// NotifyChanged 广播敏感词变更：Publish 即时信号 + INCR 版本计数（best-effort，失败仅记日志）。
func (s *BlockedService) NotifyChanged(ctx context.Context) {
	if s.cache == nil {
		return
	}
	if err := s.cache.Publish(ctx, constant.BlockedChangeChannel, "changed").Err(); err != nil {
		logger.WithCtx(ctx).Warn("[BlockedService] Publish blocked change failed", zap.Error(err))
	}
	if err := s.cache.Incr(ctx, constant.BlockedVersionKey).Err(); err != nil {
		logger.WithCtx(ctx).Warn("[BlockedService] Incr blocked version failed", zap.Error(err))
	}
}

func (s *BlockedService) Check(text string) []uint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.matcher.Match(text)
}

func (s *BlockedService) MatchedWords(ids []uint) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return lo.FilterMap(ids, func(id uint, _ int) (string, bool) {
		w, ok := s.wordByID[id]
		return w, ok
	})
}

// DenyIDs 过滤出 action=deny（命中即拦截）的词 ID，空值按 deny 兜底
func (s *BlockedService) DenyIDs(ids []uint) []uint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return lo.Filter(ids, func(id uint, _ int) bool {
		return s.actionByID[id] == "" || s.actionByID[id] == enum.BlockedActionDeny
	})
}

func (s *BlockedService) IncrementHits(ctx context.Context, ids []uint) error {
	if s.hitRecorder == nil {
		return nil
	}
	return s.hitRecorder.IncrementHits(ctx, ids)
}
```

注意：
- `logger.WithCtx(ctx)` 返回 `*zap.Logger`（`internal/logger/logger.go:38`），错误用 `zap.Error(err)`（`go.uber.org/zap`），上方代码已按此写。
- `b.AggregateID()` 是 `aggregate.Blocked` 的既有方法（`commonagg.Base` 提供），已在原代码使用，无需改动。
- import 需新增 `"go.uber.org/zap"`。

- [ ] **Step 3: 适配现有测试 deny_ids_test.go**

在 `test/unit/blocked_matcher/deny_ids_test.go` 中修改两处（其余不动）：

```go
	svc := blocked.NewBlockedService(&fakeBlockedRepo{items: agg}, nil, nil)
	_ = svc.Rebuild(context.Background())
```

- [ ] **Step 4: 编写新测试 blocked_service_sync_test.go**

新建 `test/unit/blocked_matcher/blocked_service_sync_test.go`：

```go
package blocked_matcher_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/hcd233/aris-proxy-api/internal/application/blocked"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	blockeddomain "github.com/hcd233/aris-proxy-api/internal/domain/blocked"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked/aggregate"
)

type fakeRepo struct {
	items []*aggregate.Blocked
	err   error
}

func (f *fakeRepo) FindByID(ctx context.Context, id uint) (*aggregate.Blocked, error) { return nil, nil }
func (f *fakeRepo) Create(ctx context.Context, w *aggregate.Blocked) (uint, error)     { return 0, nil }
func (f *fakeRepo) Delete(ctx context.Context, id uint) error                          { return nil }
func (f *fakeRepo) DeleteBatch(ctx context.Context, ids []uint) error                  { return nil }
func (f *fakeRepo) UpdateAction(ctx context.Context, id uint, action string) error     { return nil }
func (f *fakeRepo) Paginate(ctx context.Context, p model.CommonParam) ([]*aggregate.Blocked, *model.PageInfo, error) {
	return nil, nil, nil
}
func (f *fakeRepo) ListAll(ctx context.Context) ([]*aggregate.Blocked, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.items, nil
}
func (f *fakeRepo) BatchIncrementHitCount(ctx context.Context, m map[uint]uint) error { return nil }

var _ blockeddomain.BlockedRepository = (*fakeRepo)(nil)

func newRepoWithWord(id uint, word, action string) *fakeRepo {
	b, _ := aggregate.CreateBlocked(id, word, action)
	return &fakeRepo{items: []*aggregate.Blocked{b}}
}

func TestBlockedService_NotifyChanged_PublishAndIncr(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rc.Close()

	svc := blocked.NewBlockedService(&fakeRepo{}, nil, rc)
	svc.NotifyChanged(context.Background())

	if got := mr.Exists(constant.BlockedVersionKey); got != 1 {
		t.Fatalf("expected blocked:version to exist, exists=%d", got)
	}
	if v, _ := mr.Get(constant.BlockedVersionKey); v != "1" {
		t.Fatalf("expected version=1, got %q", v)
	}
}

func TestBlockedService_Rebuild_ErrorKeepsOldMatcher(t *testing.T) {
	t.Parallel()
	repo := newRepoWithWord(1, "hello", enum.BlockedActionDeny)
	svc := blocked.NewBlockedService(repo, nil, nil)
	if err := svc.Rebuild(context.Background()); err != nil {
		t.Fatalf("initial rebuild: %v", err)
	}
	if ids := svc.Check("say hello"); len(ids) != 1 {
		t.Fatalf("expected match before failure, got %v", ids)
	}

	repo.err = fmt.Errorf("db down")
	if err := svc.Rebuild(context.Background()); err == nil {
		t.Fatal("expected error from rebuild")
	}
	// 失败后 matcher 保持原状，仍能命中
	if ids := svc.Check("say hello"); len(ids) != 1 {
		t.Fatalf("expected old matcher retained after failure, got %v", ids)
	}
}

func TestBlockedService_Rebuild_Idempotent(t *testing.T) {
	t.Parallel()
	repo := newRepoWithWord(1, "hello", enum.BlockedActionDeny)
	svc := blocked.NewBlockedService(repo, nil, nil)
	if err := svc.Rebuild(context.Background()); err != nil {
		t.Fatalf("first rebuild: %v", err)
	}
	if err := svc.Rebuild(context.Background()); err != nil {
		t.Fatalf("second rebuild: %v", err)
	}
	if ids := svc.Check("hello"); len(ids) != 1 {
		t.Fatalf("expected stable match after double rebuild, got %v", ids)
	}
}

func TestBlockedService_ConcurrentCheckAndRebuild(t *testing.T) {
	repo := newRepoWithWord(1, "hello", enum.BlockedActionDeny)
	svc := blocked.NewBlockedService(repo, nil, nil)
	_ = svc.Rebuild(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 500; i++ {
			_ = svc.Rebuild(context.Background())
		}
	}()
	for i := 0; i < 2000; i++ {
		_ = svc.Check("hello world")
	}
	<-done
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `go test -count=1 -race ./test/unit/blocked_matcher/...`
Expected: 全部 PASS（含原有 matcher 测试 + 新增 4 个）

- [ ] **Step 6: 编译检查全仓库**

Run: `go build ./...`
Expected: 成功（此时 `bootstrap/modules/application.go` 的 `NewBlockedService` 仍用旧 2 参调用 → **编译失败是预期**，Task 3 修复；如失败提示仅指向 application.go 与 lifecycle.go 即可继续）

- [ ] **Step 7: Commit**

```bash
git add internal/common/constant/string.go internal/application/blocked/service.go test/unit/blocked_matcher/blocked_service_sync_test.go test/unit/blocked_matcher/deny_ids_test.go
git commit -m "feat(blocked): BlockedService 支持 Redis 变更广播（NotifyChanged）与 Rebuild 失败保持原状"
```

---

### Task 2: StartSync 同步循环（订阅 + 版本轮询 + 低频兜底）

**Files:**
- Modify: `internal/application/blocked/service.go`（追加 StartSync / syncLoop / checkVersion / StopSync；复用 Task 1 的字段）
- Test: `test/unit/blocked_matcher/blocked_service_sync_test.go`（追加测试）

**Interfaces:**
- Consumes: `s.cache`（Task 1）、`s.lastSeenVersion` / `s.lastRebuildAt`（Task 1 字段）、`constant.BlockedVersionKey` / `constant.BlockedChangeChannel`（Task 1 常量）、`Rebuild(ctx) error`（Task 1）
- Produces:
  - `func (s *BlockedService) StartSync(ctx context.Context)`
  - `func (s *BlockedService) StopSync()`
  - 内部 `checkVersion(ctx)`、`syncLoop(ctx)`（测试通过导出的 StartSync + 可控 ticker 调用，或直接测 checkVersion——为可测性，将 `syncLoop` 的 ticker 间隔提为私有字段 `syncInterval time.Duration`，默认 2s，测试可注入）

- [ ] **Step 1: 追加同步循环实现**

在 `internal/application/blocked/service.go` 的 `NotifyChanged` 方法之后追加：

```go
// syncInterval 版本轮询间隔；测试可注入短间隔。
var defaultSyncInterval = 2 * time.Second

// StartSync 启动 pub/sub 订阅与版本轮询（每 pod 启动时调用一次）。
func (s *BlockedService) StartSync(ctx context.Context) {
	if s.cache == nil {
		return
	}
	s.pubSub = s.cache.Subscribe(ctx, constant.BlockedChangeChannel)
	go s.syncLoop(ctx)
	go func() {
		for range s.pubSub.Channel() {
			_ = s.Rebuild(ctx) //nolint:errcheck // 失败由版本轮询/低频兜底重试
		}
	}()
	// 订阅建立后立即对比一次，消除"订阅前已发生的变更"竞态
	s.checkVersion(ctx)
}

// StopSync 关闭 pub/sub 订阅（lifecycle OnStop 调用）。
func (s *BlockedService) StopSync() {
	if s.pubSub != nil {
		_ = s.pubSub.Close()
	}
}

func (s *BlockedService) syncLoop(ctx context.Context) {
	t := time.NewTicker(s.syncInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.checkVersion(ctx)
			// 低频兜底：距上次成功 Rebuild ≥ 5min 则无条件重建（失败不更新游标 → 下轮重试）
			if time.Since(time.Unix(0, s.lastRebuildAt.Load())) >= 5*time.Minute {
				_ = s.Rebuild(ctx) //nolint:errcheck
			}
		}
	}
}

func (s *BlockedService) checkVersion(ctx context.Context) {
	v, err := s.cache.Get(ctx, constant.BlockedVersionKey).Int64()
	if err != nil {
		return // 读不到版本号由低频兜底收敛；Redis 故障时保持现状
	}
	if v != s.lastSeenVersion.Load() {
		if err := s.Rebuild(ctx); err == nil {
			s.lastSeenVersion.Store(v)
		}
	}
}
```

并在 `BlockedService` struct 补充 Task 1 未含的 `pubSub`/`syncInterval` 字段（`lastSeenVersion`/`lastRebuildAt` 已在 Task 1 以 atomic 定义，无需重复）：

```go
type BlockedService struct {
	// ...Task 1 既有字段（含 atomic 游标）...
	cache *redis.Client
	pubSub *redis.PubSub
	syncInterval time.Duration
}
```

构造函数补充 `syncInterval` 初始化（其余不变）：

```go
func NewBlockedService(repo domain.BlockedRepository, hitRecorder port.HitRecorder, cache *redis.Client) *BlockedService {
	return &BlockedService{
		repo: repo, matcher: NewACmatcher(make(map[uint]string)), hitRecorder: hitRecorder, cache: cache,
		actionByID:   make(map[uint]string),
		syncInterval: defaultSyncInterval,
	}
}
```

- [ ] **Step 2: 追加测试（版本轮询触发/不触发 + pub/sub 即时重建）**

在 `test/unit/blocked_matcher/blocked_service_sync_test.go` 的 import 块中追加 `"time"`（Task 1 测试未用到；本任务用到 `time.Sleep`/`time.Now`/`time.Duration`）：

```go
import (
	"context"
	"fmt"
	"testing"
	"time"
	...
)
```

在文件末尾追加：

```go
func TestBlockedService_checkVersion_TriggersOnChange(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rc.Close()

	repo := newRepoWithWord(1, "hello", enum.BlockedActionDeny)
	svc := blocked.NewBlockedService(repo, nil, rc)

	// 首次：version 不存在时 checkVersion 不重建（Get 返回 err）
	svc.CheckVersionForTest(context.Background())

	// 发布一次变更 → version=1 → 触发重建并记录游标
	svc.NotifyChanged(context.Background())
	svc.CheckVersionForTest(context.Background())
	if ids := svc.Check("hello"); len(ids) != 1 {
		t.Fatalf("expected match after version change, got %v", ids)
	}

	// version 未变 → 不重复重建（Check 结果应仍一致；通过游标间接断言不 panic）
	svc.CheckVersionForTest(context.Background())
	if v := svc.LastSeenVersionForTest(); v != 1 {
		t.Fatalf("expected lastSeenVersion=1, got %d", v)
	}
}

func TestBlockedService_StartSync_PubSubRebuilds(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rc.Close()

	repo := newRepoWithWord(1, "hello", enum.BlockedActionDeny)
	svc := blocked.NewBlockedService(repo, nil, rc)
	svc.SetSyncIntervalForTest(50 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.StartSync(ctx)
	defer svc.StopSync()

	// 等订阅生效
	time.Sleep(100 * time.Millisecond)

	// 远端变更：repo 增加新词并广播
	repo.items = append(repo.items, mustBlocked(2, "world", enum.BlockedActionDeny))
	svc.NotifyChanged(context.Background())

	// pub/sub 即时重建：1s 内必然命中
	deadline := time.Now().Add(1 * time.Second)
	for {
		if ids := svc.Check("hello world"); len(ids) == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("expected both words matched within 1s")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func mustBlocked(id uint, word, action string) *aggregate.Blocked {
	b, err := aggregate.CreateBlocked(id, word, action)
	if err != nil {
		panic(err)
	}
	return b
}

func TestBlockedService_syncLoop_LowFreqFallback(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rc.Close()

	repo := newRepoWithWord(1, "hello", enum.BlockedActionDeny)
	svc := blocked.NewBlockedService(repo, nil, rc)
	svc.SetSyncIntervalForTest(50 * time.Millisecond)
	_ = svc.Rebuild(context.Background())
	svc.ForceRebuildAtForTest(time.Now().Add(-6 * time.Minute)) // 置旧，模拟超时

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.StartSync(ctx)
	defer svc.StopSync()

	// 低频兜底应在下一个 tick 内重建（这里通过让 repo 换词验证）
	repo.items = []*aggregate.Blocked{mustBlocked(1, "bye", enum.BlockedActionDeny)}
	deadline := time.Now().Add(1 * time.Second)
	for {
		if ids := svc.Check("bye"); len(ids) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("expected low-frequency rebuild to pick up new words")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
```

在 `BlockedService` 增加测试辅助导出（放在 service.go 末尾）：

```go
// LastSeenVersionForTest 仅供测试断言版本游标。
func (s *BlockedService) LastSeenVersionForTest() int64 {
	return s.lastSeenVersion.Load()
}

// CheckVersionForTest 触发一次版本对比（测试用，等价于内部 checkVersion）。
func (s *BlockedService) CheckVersionForTest(ctx context.Context) {
	s.checkVersion(ctx)
}

// SetSyncIntervalForTest 覆盖轮询间隔（测试用，默认 2s）。
func (s *BlockedService) SetSyncIntervalForTest(d time.Duration) {
	s.syncInterval = d
}

// ForceRebuildAtForTest 覆盖上次重建时间（测试用，用于触发低频兜底）。
func (s *BlockedService) ForceRebuildAtForTest(t time.Time) {
	s.lastRebuildAt.Store(t.UnixNano())
}
```

注意：测试包是外部包 `blocked_matcher_test`，不能访问未导出字段/方法，因此所有测试注入统一走导出的 `*ForTest` 辅助方法；`checkVersion`/`syncLoop`/`lastRebuildAt` 均不直接触碰。

- [ ] **Step 3: 运行测试**

Run: `go test -count=1 -race ./test/unit/blocked_matcher/...`
Expected: 全部 PASS（含 Task 1 的 4 个 + 本任务 4 个）

- [ ] **Step 4: Commit**

```bash
git add internal/application/blocked/service.go test/unit/blocked_matcher/blocked_service_sync_test.go
git commit -m "feat(blocked): StartSync 订阅 + 版本轮询 + 低频兜底收敛"
```

---

### Task 3: handler 通知 + DI 装配 + lifecycle 启动

**Files:**
- Modify: `internal/application/blocked/command/create_blocked.go`、`update_blocked.go`、`delete_blocked.go`（+`notifyChanged` 依赖与调用）
- Modify: `internal/bootstrap/modules/application.go`（DI：注入 cache、闭包适配、新增 `NewBlockedService` 3 参）
- Modify: `internal/bootstrap/lifecycle.go`（OnStart 追加 StartSync；OnStop 追加 StopSync；Rebuild 忽略返回值）

**Interfaces:**
- Consumes: `svc.NotifyChanged(ctx)`（Task 1）、`svc.StartSync(ctx)` / `svc.StopSync()`（Task 2）、`svc.Rebuild(ctx) error`（Task 1）
- Produces: 装配后的完整可运行服务

- [ ] **Step 1: 三个 command 增加 notifyChanged 依赖**

`create_blocked.go` 改为：

```go
type createBlockedHandler struct {
	repo          blocked.BlockedRepository
	rebuildNotify func(ctx context.Context)
	notifyChanged func(ctx context.Context)
}

func NewCreateBlockedHandler(repo blocked.BlockedRepository, rebuildNotify func(ctx context.Context), notifyChanged func(ctx context.Context)) port.CreateBlockedHandler {
	return &createBlockedHandler{repo: repo, rebuildNotify: rebuildNotify, notifyChanged: notifyChanged}
}

func (h *createBlockedHandler) Handle(ctx context.Context, cmd port.CreateBlockedCommand) (*port.CreateBlockedResult, error) {
	b, err := aggregate.CreateBlocked(0, cmd.Word, cmd.Action)
	if err != nil {
		return nil, err
	}
	id, err := h.repo.Create(ctx, b)
	if err != nil {
		return nil, err
	}
	h.rebuildNotify(ctx)
	h.notifyChanged(ctx)
	return &port.CreateBlockedResult{BlockedID: id}, nil
}
```

`update_blocked.go` 的 `Handle` 末尾同样追加 `h.notifyChanged(ctx)`（结构体与构造签名同步加第三参 `notifyChanged func(ctx context.Context)`）。

`delete_blocked.go` 的 `Handle` 末尾同样追加 `h.notifyChanged(ctx)`（结构体与构造签名同步加第三参）。

（三个文件 pattern 完全一致：struct 加字段、构造函数加参数、Handle 中 `h.rebuildNotify(ctx)` 后加 `h.notifyChanged(ctx)`。实现时逐文件复制上述模式，不要简写为"类似"。）

`port/handler.go` 无需改动（command 用函数参数注入，不走接口）。

- [ ] **Step 3: 更新 DI 装配 application.go**
`NewBlockedService` 改为 3 参并注入 cache：

```go
func NewBlockedService(repo blockeddomain.BlockedRepository, hitRecorder blockedport.HitRecorder, cache *redis.Client) *blockedapp.BlockedService {
	return blockedapp.NewBlockedService(repo, hitRecorder, cache)
}
```

`NewCreateBlockedHandler` / `NewUpdateBlockedHandler` / `NewDeleteBlockedHandler` 改为闭包适配 + notifyChanged：

```go
func NewCreateBlockedHandler(repo blockeddomain.BlockedRepository, svc *blockedapp.BlockedService) blockedport.CreateBlockedHandler {
	return blockedcommand.NewCreateBlockedHandler(
		repo,
		func(ctx context.Context) { _ = svc.Rebuild(ctx) },
		svc.NotifyChanged,
	)
}

func NewUpdateBlockedHandler(repo blockeddomain.BlockedRepository, svc *blockedapp.BlockedService) blockedport.UpdateBlockedHandler {
	return blockedcommand.NewUpdateBlockedHandler(
		repo,
		func(ctx context.Context) { _ = svc.Rebuild(ctx) },
		svc.NotifyChanged,
	)
}

func NewDeleteBlockedHandler(repo blockeddomain.BlockedRepository, svc *blockedapp.BlockedService) blockedport.DeleteBlockedHandler {
	return blockedcommand.NewDeleteBlockedHandler(
		repo,
		func(ctx context.Context) { _ = svc.Rebuild(ctx) },
		svc.NotifyChanged,
	)
}
```

- [ ] **Step 4: 更新 lifecycle.go**

将既有 Hook（约 59-64 行）替换为：

```go
	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			_ = params.BlockedService.Rebuild(ctx) //nolint:errcheck // 启动失败由 syncLoop 兜底
			params.BlockedService.StartSync(ctx)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			params.BlockedService.StopSync()
			return nil
		},
	})
```

- [ ] **Step 5: 全量编译 + 测试**

Run: `go build ./... && go test -count=1 ./internal/... ./test/...`
Expected: 全部 PASS（Task 1 Step 6 的编译失败此时消除）

- [ ] **Step 6: gofmt + lint 检查**

Run: `gofmt -l internal/application/blocked internal/bootstrap && go vet ./internal/application/blocked/...`
Expected: 无输出（或仅提示修复后无输出）

- [ ] **Step 7: Commit**

```bash
git add internal/application/blocked/command/ internal/bootstrap/modules/application.go internal/bootstrap/lifecycle.go
git commit -m "feat(blocked): 写路径广播变更 + DI 装配 + lifecycle 启动同步循环"
```

---

## 验证与部署检查清单

- [ ] `go test -count=1 -race ./test/unit/blocked_matcher/...` 全绿
- [ ] `go build ./...` 无错
- [ ] `go test -count=1 ./internal/... ./test/...` 全绿（回归）
- [ ] 代码评审：`Rebuild` 失败分支保持 matcher 不清空；`notifyChanged` 在 PG 成功后调用；`StartSync` 仅 OnStart 启动一次
- [ ] 部署后手工 E2E（生产）：pod A 删 deny 敏感词 → 路由到 pod B 的请求 ≤2s 不再命中；Web 后台并发增删改 → 两 pod 行为一致

## 自审记录（plan 与 spec 对照）

- 三通道（pub/sub / 版本轮询 / 低频兜底）→ Task 2（StartSync + syncLoop + checkVersion）
- 写路径三调用（rebuildNotify + Publish + INCR）→ Task 1（NotifyChanged）+ Task 3（command 追加）
- Rebuild 返回 error、失败保持原状、游标不更新 → Task 1（Rebuild 实现 + 测试）
- 签名兼容（rebuildNotify func(ctx) 闭包适配）→ Task 3 Step 3
- 不用 cron 框架、不新增 cron 条目 → 全程未碰 `internal/cron/`
- 低频兜底 5 分钟 → Task 2 syncLoop
- 启动竞态（订阅前变更）→ Task 2 StartSync 末行 checkVersion
- DI：注入 cache + StartSync/StopSync 生命周期 → Task 3
- 既有测试适配（deny_ids_test.go）→ Task 1 Step 3
