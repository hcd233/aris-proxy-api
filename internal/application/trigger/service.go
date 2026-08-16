package trigger

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
	domain "github.com/hcd233/aris-proxy-api/internal/domain/trigger"
	"github.com/hcd233/aris-proxy-api/internal/domain/trigger/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/logger"

	"github.com/hcd233/aris-proxy-api/internal/application/trigger/port"
)

type TriggerService struct {
	mu           sync.RWMutex
	matcher      *ACmatcher
	wordByID     map[uint]string
	actionByID   map[uint]enum.TriggerAction
	repo         domain.TriggerRepository
	hitRecorder  port.HitRecorder
	cache        *redis.Client
	pubSub       *redis.PubSub
	syncInterval time.Duration
	syncCancel   context.CancelFunc

	lastSeenVersion atomic.Int64
	lastRebuildAt   atomic.Int64 // UnixNano；0 表示从未成功重建
}

func NewTriggerService(repo domain.TriggerRepository, hitRecorder port.HitRecorder, cache *redis.Client) *TriggerService {
	return &TriggerService{
		repo: repo, matcher: NewACmatcher(make(map[uint]string)), hitRecorder: hitRecorder, cache: cache,
		actionByID:   make(map[uint]enum.TriggerAction),
		syncInterval: defaultSyncInterval,
	}
}

func (s *TriggerService) rebuild(words map[uint]string) {
	s.matcher = NewACmatcher(words)
	s.wordByID = words
}

// Rebuild 从 DB 全量重建内存 matcher；失败时保持原 matcher 不变并返回 error。
func (s *TriggerService) Rebuild(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.repo.ListAll(ctx)
	if err != nil {
		logger.WithCtx(ctx).Error("[TriggerService] Rebuild failed, keep old matcher", zap.Error(err))
		return err
	}
	words := lo.SliceToMap(all, func(b *aggregate.Trigger) (uint, string) {
		return b.AggregateID(), b.Word()
	})
	s.rebuild(words)
	s.actionByID = lo.SliceToMap(all, func(b *aggregate.Trigger) (uint, enum.TriggerAction) {
		return b.AggregateID(), b.Action()
	})
	s.lastRebuildAt.Store(time.Now().UnixNano())
	return nil
}

// NotifyChanged 广播触发词变更：Publish 即时信号 + INCR 版本计数（best-effort，失败仅记日志）。
func (s *TriggerService) NotifyChanged(ctx context.Context) {
	if s.cache == nil {
		return
	}
	if err := s.cache.Publish(ctx, constant.TriggerChangeChannel, constant.TriggerChangeMessage).Err(); err != nil {
		logger.WithCtx(ctx).Warn("[TriggerService] Publish trigger change failed", zap.Error(err))
	}
	if err := s.cache.Incr(ctx, constant.TriggerVersionKey).Err(); err != nil {
		logger.WithCtx(ctx).Warn("[TriggerService] Incr trigger version failed", zap.Error(err))
	}
}

// syncInterval 版本轮询间隔；测试可注入短间隔。
var defaultSyncInterval = constant.TriggerVersionPollInterval

// StartSync 启动 pub/sub 订阅与版本轮询（每 pod 启动时调用一次）。
func (s *TriggerService) StartSync(ctx context.Context) {
	if s.cache == nil {
		return
	}
	ctx, s.syncCancel = context.WithCancel(ctx)
	s.pubSub = s.cache.Subscribe(ctx, constant.TriggerChangeChannel)
	go s.syncLoop(ctx)
	go func() {
		for range s.pubSub.Channel() {
			_ = s.Rebuild(ctx) //nolint:errcheck // 失败由版本轮询/低频兜底重试
		}
	}()
	// 订阅建立后立即对比一次，消除"订阅前已发生的变更"竞态
	s.checkVersion(ctx)
}

// StopSync 停止版本轮询与 pub/sub 订阅（lifecycle OnStop 调用）。
func (s *TriggerService) StopSync() {
	if s.syncCancel != nil {
		s.syncCancel()
		s.syncCancel = nil
	}
	if s.pubSub != nil {
		_ = s.pubSub.Close() //nolint:errcheck // 关闭失败无副作用，goroutine 随 ctx 结束
	}
}

func (s *TriggerService) syncLoop(ctx context.Context) {
	t := time.NewTicker(s.syncInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.checkVersion(ctx)
			// 低频兜底：距上次成功 Rebuild ≥ 5min 则无条件重建（失败不更新游标 → 下轮重试）
			if time.Since(time.Unix(0, s.lastRebuildAt.Load())) >= constant.TriggerLowFreqRebuildInterval {
				_ = s.Rebuild(ctx) //nolint:errcheck // 失败不更新 lastRebuildAt，由下轮 tick 重试
			}
		}
	}
}

func (s *TriggerService) checkVersion(ctx context.Context) {
	v, err := s.cache.Get(ctx, constant.TriggerVersionKey).Int64()
	if err != nil {
		return // 读不到版本号由低频兜底收敛；Redis 故障时保持现状
	}
	if v != s.lastSeenVersion.Load() {
		if err := s.Rebuild(ctx); err == nil {
			s.lastSeenVersion.Store(v)
		}
	}
}

// LastSeenVersionForTest 仅供测试断言版本游标。
func (s *TriggerService) LastSeenVersionForTest() int64 {
	return s.lastSeenVersion.Load()
}

// CheckVersionForTest 触发一次版本对比（测试用，等价于内部 checkVersion）。
func (s *TriggerService) CheckVersionForTest(ctx context.Context) {
	s.checkVersion(ctx)
}

// SetSyncIntervalForTest 覆盖轮询间隔（测试用，默认 2s）。
func (s *TriggerService) SetSyncIntervalForTest(d time.Duration) {
	s.syncInterval = d
}

// ForceRebuildAtForTest 覆盖上次重建时间（测试用，用于触发低频兜底）。
func (s *TriggerService) ForceRebuildAtForTest(t time.Time) {
	s.lastRebuildAt.Store(t.UnixNano())
}

func (s *TriggerService) Check(text string) []uint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.matcher.Match(text)
}

func (s *TriggerService) MatchedWords(ids []uint) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return lo.FilterMap(ids, func(id uint, _ int) (string, bool) {
		w, ok := s.wordByID[id]
		return w, ok
	})
}

// DenyIDs 过滤出 action=deny（命中即拦截）的词 ID，空值按 deny 兜底
func (s *TriggerService) DenyIDs(ids []uint) []uint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return lo.Filter(ids, func(id uint, _ int) bool {
		return s.actionByID[id] == "" || s.actionByID[id] == enum.TriggerActionDeny
	})
}

// CaptureIDs 过滤出 action=capture（命中捕获上下文）的词 ID
func (s *TriggerService) CaptureIDs(ids []uint) []uint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return lo.Filter(ids, func(id uint, _ int) bool {
		return s.actionByID[id] == enum.TriggerActionCapture
	})
}

// OmitIDs 过滤出 action=omit（命中放行但跳过存储）的词 ID
func (s *TriggerService) OmitIDs(ids []uint) []uint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return lo.Filter(ids, func(id uint, _ int) bool {
		return s.actionByID[id] == enum.TriggerActionOmit
	})
}

func (s *TriggerService) IncrementHits(ctx context.Context, ids []uint) error {
	if s.hitRecorder == nil {
		return nil
	}
	return s.hitRecorder.IncrementHits(ctx, ids)
}
