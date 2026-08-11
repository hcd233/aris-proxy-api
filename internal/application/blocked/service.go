package blocked

import (
	"context"
	"sync"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	domain "github.com/hcd233/aris-proxy-api/internal/domain/blocked"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked/aggregate"

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
}

func NewBlockedService(repo domain.BlockedRepository, hitRecorder port.HitRecorder) *BlockedService {
	return &BlockedService{
		repo: repo, matcher: NewACmatcher(make(map[uint]string)), hitRecorder: hitRecorder,
		actionByID: make(map[uint]string),
	}
}

func (s *BlockedService) rebuild(words map[uint]string) {
	s.matcher = NewACmatcher(words)
	s.wordIDs = lo.Invert(words)
	s.wordByID = words
}

func (s *BlockedService) Rebuild(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.repo.ListAll(ctx)
	if err != nil {
		s.rebuild(make(map[uint]string))
		s.actionByID = make(map[uint]string)
		return
	}
	words := lo.SliceToMap(all, func(b *aggregate.Blocked) (uint, string) {
		return b.AggregateID(), b.Word()
	})
	s.rebuild(words)
	s.actionByID = lo.SliceToMap(all, func(b *aggregate.Blocked) (uint, string) {
		return b.AggregateID(), b.Action()
	})
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
