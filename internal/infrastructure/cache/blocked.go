package cache

import (
	"context"
	"fmt"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/redis/go-redis/v9"
)

type BlockedHitCache struct {
	client *redis.Client
}

func NewBlockedHitCache(client *redis.Client) *BlockedHitCache {
	return &BlockedHitCache{client: client}
}

func blockedHitKey(id uint) string {
	return fmt.Sprintf(constant.BlockedHitKeyPrefix, id)
}

func (c *BlockedHitCache) IncrementHits(ctx context.Context, ids []uint) error {
	pipe := c.client.Pipeline()
	for _, id := range ids {
		pipe.IncrBy(ctx, blockedHitKey(id), 1)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (c *BlockedHitCache) PopAll(ctx context.Context) (map[uint]uint, error) {
	iter := c.client.Scan(ctx, 0, constant.BlockedHitKeyScanPattern, 0).Iterator()
	result := make(map[uint]uint)
	var keys []string

	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return result, nil
	}

	vals, err := c.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	for i, key := range keys {
		if vals[i] == nil {
			continue
		}
		val, ok := vals[i].(int64)
		if !ok || val <= 0 {
			continue
		}
		var id uint
		fmt.Sscanf(key, constant.BlockedHitKeyPrefix, &id) //nolint:errcheck // best-effort parse
		result[id] = uint(val)
	}

	if len(result) > 0 {
		c.client.Del(ctx, keys...)
	}

	return result, nil
}
