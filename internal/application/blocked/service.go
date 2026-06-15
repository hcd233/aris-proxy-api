package blocked

import (
	"context"
	"sync"

	"github.com/hcd233/aris-proxy-api/internal/application/blocked/port"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked"
)

type BlockedService struct {
	mu          sync.RWMutex
	matcher     *ACmatcher
	wordIDs     map[string]uint
	wordByID    map[uint]string
	repo        blocked.BlockedRepository
	hitRecorder port.HitRecorder
}

func NewBlockedService(repo blocked.BlockedRepository, hitRecorder port.HitRecorder) *BlockedService {
	return &BlockedService{repo: repo, matcher: NewACmatcher(make(map[uint]string)), hitRecorder: hitRecorder}
}

func (s *BlockedService) rebuild(words map[uint]string) {
	ids := make(map[string]uint, len(words))
	byID := make(map[uint]string, len(words))
	for id, word := range words {
		ids[word] = id
		byID[id] = word
	}
	s.matcher = NewACmatcher(words)
	s.wordIDs = ids
	s.wordByID = byID
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

func (s *BlockedService) MatchedWords(ids []uint) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	words := make([]string, 0, len(ids))
	for _, id := range ids {
		if w, ok := s.wordByID[id]; ok {
			words = append(words, w)
		}
	}
	return words
}

func (s *BlockedService) IncrementHits(ctx context.Context, ids []uint) error {
	if s.hitRecorder == nil {
		return nil
	}
	return s.hitRecorder.IncrementHits(ctx, ids)
}
