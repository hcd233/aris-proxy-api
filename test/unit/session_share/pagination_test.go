package session_share

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
)

func TestRedisShareCacheListUserShares_PaginatesAtRedisLevel(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	server := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer func() { _ = rdb.Close() }()

	shareCache := cache.NewShareCache(rdb)
	now := time.Now()
	userID := uint(7)

	for i := 0; i < 5; i++ {
		seedRedisShare(t, rdb, userID, fmt.Sprintf("share-%d", i), uint(i+1), now.Add(-time.Duration(i)*time.Hour), true)
	}

	page1, pageInfo1, err := shareCache.ListUserShares(ctx, userID, 1, 2)
	if err != nil {
		t.Fatalf("page1 error: %v", err)
	}
	if pageInfo1.Total != 5 {
		t.Fatalf("page1.Total = %d, want 5", pageInfo1.Total)
	}
	if len(page1) != 2 {
		t.Fatalf("page1 count = %d, want 2", len(page1))
	}

	page2, pageInfo2, err := shareCache.ListUserShares(ctx, userID, 2, 2)
	if err != nil {
		t.Fatalf("page2 error: %v", err)
	}
	if pageInfo2.Total != 5 {
		t.Fatalf("page2.Total = %d, want 5", pageInfo2.Total)
	}
	if len(page2) != 2 {
		t.Fatalf("page2 count = %d, want 2", len(page2))
	}

	page3, pageInfo3, err := shareCache.ListUserShares(ctx, userID, 3, 2)
	if err != nil {
		t.Fatalf("page3 error: %v", err)
	}
	if pageInfo3.Total != 5 {
		t.Fatalf("page3.Total = %d, want 5", pageInfo3.Total)
	}
	if len(page3) != 1 {
		t.Fatalf("page3 count = %d, want 1", len(page3))
	}

	page4, _, err := shareCache.ListUserShares(ctx, userID, 4, 2)
	if err != nil {
		t.Fatalf("page4 error: %v", err)
	}
	if len(page4) != 0 {
		t.Fatalf("page4 count = %d, want 0", len(page4))
	}

	seen := make(map[string]bool)
	for _, page := range [][]*dto.ShareItem{page1, page2, page3} {
		for _, s := range page {
			if seen[s.ShareID] {
				t.Fatalf("share %s appeared in multiple pages", s.ShareID)
			}
			seen[s.ShareID] = true
		}
	}
	if len(seen) != 5 {
		t.Fatalf("total unique shares = %d, want 5", len(seen))
	}
}
