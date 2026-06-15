package blocked

import (
	"context"
	"sync"

	"github.com/samber/lo"

	domain "github.com/hcd233/aris-proxy-api/internal/domain/blocked"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked/aggregate"

	"github.com/hcd233/aris-proxy-api/internal/application/blocked/port"
)

type BlockedService struct {
	mu          sync.RWMutex
	matcher     *ACmatcher
	wordIDs     map[string]uint
	wordByID    map[uint]string
	repo        domain.BlockedRepository
	hitRecorder port.HitRecorder
}

func NewBlockedService(repo domain.BlockedRepository, hitRecorder port.HitRecorder) *BlockedService {
	return &BlockedService{repo: repo, matcher: NewACmatcher(make(map[uint]string)), hitRecorder: hitRecorder}
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
		return
	}
	words := lo.SliceToMap(all, func(b *aggregate.Blocked) (uint, string) {
		return b.AggregateID(), b.Word()
	})
	s.rebuild(words)
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

func (s *BlockedService) IncrementHits(ctx context.Context, ids []uint) error {
	if s.hitRecorder == nil {
		return nil
	}
	return s.hitRecorder.IncrementHits(ctx, ids)
}
