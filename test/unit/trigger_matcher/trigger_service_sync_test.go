package trigger_matcher_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/hcd233/aris-proxy-api/internal/application/trigger"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	triggerdomain "github.com/hcd233/aris-proxy-api/internal/domain/trigger"
	"github.com/hcd233/aris-proxy-api/internal/domain/trigger/aggregate"
)

type fakeRepo struct {
	mu    sync.RWMutex
	items []*aggregate.Trigger
	err   error
}

func (f *fakeRepo) FindByID(ctx context.Context, id uint) (*aggregate.Trigger, error) {
	return nil, nil
}
func (f *fakeRepo) Create(ctx context.Context, w *aggregate.Trigger) (uint, error) { return 0, nil }
func (f *fakeRepo) Delete(ctx context.Context, id uint) error                      { return nil }
func (f *fakeRepo) DeleteBatch(ctx context.Context, ids []uint) error              { return nil }
func (f *fakeRepo) UpdateAction(ctx context.Context, id uint, action string) error { return nil }
func (f *fakeRepo) Paginate(ctx context.Context, p model.CommonParam) ([]*aggregate.Trigger, *model.PageInfo, error) {
	return nil, nil, nil
}
func (f *fakeRepo) ListAll(ctx context.Context) ([]*aggregate.Trigger, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.err != nil {
		return nil, f.err
	}
	return f.items, nil
}
func (f *fakeRepo) BatchIncrementHitCount(ctx context.Context, m map[uint]uint) error { return nil }

// setItems 测试辅助：并发安全地替换词条。
func (f *fakeRepo) setItems(items []*aggregate.Trigger) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.items = items
}

// setErr 测试辅助：并发安全地设置 ListAll 错误。
func (f *fakeRepo) setErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// appendItems 测试辅助：并发安全地在末尾追加词条。
func (f *fakeRepo) appendItems(items ...*aggregate.Trigger) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.items = append(f.items, items...)
}

var _ triggerdomain.TriggerRepository = (*fakeRepo)(nil)

func newHelloRepo() *fakeRepo {
	b, _ := aggregate.CreateTrigger(1, "hello", enum.TriggerActionDeny)
	return &fakeRepo{items: []*aggregate.Trigger{b}}
}

func TestTriggerService_NotifyChanged_PublishAndIncr(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rc.Close()

	svc := trigger.NewTriggerService(&fakeRepo{}, nil, rc)
	svc.NotifyChanged(context.Background())

	if got := mr.Exists(constant.TriggerVersionKey); !got {
		t.Fatalf("expected trigger:version to exist")
	}
	if v, _ := mr.Get(constant.TriggerVersionKey); v != "1" {
		t.Fatalf("expected version=1, got %q", v)
	}
}

func TestTriggerService_Rebuild_ErrorKeepsOldMatcher(t *testing.T) {
	t.Parallel()
	repo := newHelloRepo()
	svc := trigger.NewTriggerService(repo, nil, nil)
	if err := svc.Rebuild(context.Background()); err != nil {
		t.Fatalf("initial rebuild: %v", err)
	}
	if ids := svc.Check("say hello"); len(ids) != 1 {
		t.Fatalf("expected match before failure, got %v", ids)
	}

	repo.setErr(ierr.New(ierr.ErrInternal, "db down"))
	if err := svc.Rebuild(context.Background()); err == nil {
		t.Fatal("expected error from rebuild")
	}
	// 失败后 matcher 保持原状，仍能命中
	if ids := svc.Check("say hello"); len(ids) != 1 {
		t.Fatalf("expected old matcher retained after failure, got %v", ids)
	}
}

func TestTriggerService_Rebuild_Idempotent(t *testing.T) {
	t.Parallel()
	repo := newHelloRepo()
	svc := trigger.NewTriggerService(repo, nil, nil)
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

func TestTriggerService_ConcurrentCheckAndRebuild(t *testing.T) {
	t.Parallel()
	repo := newHelloRepo()
	svc := trigger.NewTriggerService(repo, nil, nil)
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

func TestTriggerService_checkVersion_TriggersOnChange(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rc.Close()

	repo := newHelloRepo()
	svc := trigger.NewTriggerService(repo, nil, rc)

	// 首次：version 不存在时 checkVersion 不重建（Get 返回 err）
	svc.CheckVersionForTest(context.Background())

	// 发布一次变更 → version=1 → 触发重建并记录游标
	svc.NotifyChanged(context.Background())
	svc.CheckVersionForTest(context.Background())
	if ids := svc.Check("hello"); len(ids) != 1 {
		t.Fatalf("expected match after version change, got %v", ids)
	}

	// version 未变 → 不重复重建（通过游标断言已记录）
	svc.CheckVersionForTest(context.Background())
	if v := svc.LastSeenVersionForTest(); v != 1 {
		t.Fatalf("expected lastSeenVersion=1, got %d", v)
	}
}

func TestTriggerService_StartSync_PubSubRebuilds(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rc.Close()

	repo := newHelloRepo()
	svc := trigger.NewTriggerService(repo, nil, rc)
	svc.SetSyncIntervalForTest(50 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.StartSync(ctx)
	defer svc.StopSync()

	// 远端变更：repo 增加新词并广播
	repo.appendItems(mustTrigger(2, "world", enum.TriggerActionDeny))
	svc.NotifyChanged(context.Background())

	// pub/sub 即时重建：1s 内必然命中（ticker 轮询等待，避免固定 sleep）
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.Now().Add(1 * time.Second)
	for {
		if ids := svc.Check("hello world"); len(ids) == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("expected both words matched within 1s")
		}
		<-ticker.C
	}
}

func mustTrigger(id uint, word, action string) *aggregate.Trigger {
	b, err := aggregate.CreateTrigger(id, word, action)
	if err != nil {
		panic(err)
	}
	return b
}

func TestTriggerService_syncLoop_LowFreqFallback(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rc.Close()

	repo := newHelloRepo()
	svc := trigger.NewTriggerService(repo, nil, rc)
	svc.SetSyncIntervalForTest(50 * time.Millisecond)
	_ = svc.Rebuild(context.Background())
	svc.ForceRebuildAtForTest(time.Now().Add(-6 * time.Minute)) // 置旧，模拟超时

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.StartSync(ctx)
	defer svc.StopSync()

	// 低频兜底应在下一个 tick 内重建（通过让 repo 换词验证）
	repo.setItems([]*aggregate.Trigger{mustTrigger(1, "bye", enum.TriggerActionDeny)})
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.Now().Add(1 * time.Second)
	for {
		if ids := svc.Check("bye"); len(ids) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("expected low-frequency rebuild to pick up new words")
		}
		<-ticker.C
	}
}
