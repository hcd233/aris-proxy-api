package cache

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/redis/go-redis/v9"
)

type TriggerHitCache struct {
	client *redis.Client
}

func NewTriggerHitCache(client *redis.Client) *TriggerHitCache {
	return &TriggerHitCache{client: client}
}

func triggerHitKey(id uint) string {
	return fmt.Sprintf(constant.TriggerHitKeyPrefix, id)
}

func (c *TriggerHitCache) IncrementHits(ctx context.Context, ids []uint) error {
	pipe := c.client.Pipeline()
	for _, id := range ids {
		pipe.IncrBy(ctx, triggerHitKey(id), 1)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (c *TriggerHitCache) PopAll(ctx context.Context) (map[uint]uint, error) {
	iter := c.client.Scan(ctx, 0, constant.TriggerHitKeyScanPattern, 0).Iterator()
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
		var n int64
		switch v := vals[i].(type) {
		case int64:
			n = v
		case string:
			parsed, err := strconv.ParseInt(v, constant.DecimalBase, constant.ParseFloat64BitSize)
			if err != nil {
				continue
			}
			n = parsed
		default:
			continue
		}
		if n <= 0 {
			continue
		}
		var id uint
		fmt.Sscanf(key, constant.TriggerHitKeyPrefix, &id) //nolint:errcheck // best-effort parse
		result[id] = uint(n)
	}

	if len(result) > 0 {
		c.client.Del(ctx, keys...)
	}

	return result, nil
}
