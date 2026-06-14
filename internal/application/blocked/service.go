package blocked

import (
	"context"
	"sync"

	"github.com/hcd233/aris-proxy-api/internal/domain/blocked"
)

type BlockedService struct {
	mu      sync.RWMutex
	matcher *ACmatcher
	wordIDs map[string]uint
	repo    blocked.BlockedRepository
}

func NewBlockedService(repo blocked.BlockedRepository) *BlockedService {
	return &BlockedService{repo: repo, matcher: NewACmatcher(make(map[uint]string))}
}

func (s *BlockedService) rebuild(words map[uint]string) {
	ids := make(map[string]uint, len(words))
	for id, word := range words {
		ids[word] = id
	}
	s.matcher = NewACmatcher(words)
	s.wordIDs = ids
}

func (s *BlockedService) Rebuild(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.repo.ListAll(ctx)
	if err != nil {
		s.rebuild(make(map[uint]string))
		return
	}
	words := make(map[uint]string, len(all))
	for _, b := range all {
		words[b.AggregateID()] = b.Word()
	}
	s.rebuild(words)
}

func (s *BlockedService) Check(text string) []uint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.matcher.Match(text)
}
