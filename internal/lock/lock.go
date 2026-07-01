package lock

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/redis/go-redis/v9"
)

// Locker 分布式锁接口，供测试 mock 使用
//
//	@author centonhuang
//	@update 2026-06-01 10:00:00
type Locker interface {
	Lock(ctx context.Context, key string, value string, expire time.Duration) (success bool, err error)
	Refresh(ctx context.Context, key string, value string, expire time.Duration) (success bool, err error)
	Unlock(ctx context.Context, key string, value string) (err error)
}

// RedisLocker 基于 Redis 的分布式锁
//
//	@author centonhuang
//	@update 2026-06-01 10:00:00
type RedisLocker struct {
	cache *redis.Client
}

// NewLocker 创建基于 Redis 的分布式锁
//
//	@return *RedisLocker
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func NewLocker(cache *redis.Client) *RedisLocker {
	return &RedisLocker{cache: cache}
}

func (l *RedisLocker) Lock(ctx context.Context, key, value string, expire time.Duration) (success bool, err error) {
	return l.cache.SetNX(ctx, key, value, expire).Result()
}

func (l *RedisLocker) Refresh(ctx context.Context, key, value string, expire time.Duration) (success bool, err error) {
	res, err := l.cache.Eval(ctx, constant.LuaRefreshLock, []string{key}, value, expire.Milliseconds()).Int64()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

func (l *RedisLocker) Unlock(ctx context.Context, key, value string) (err error) {
	return l.cache.Eval(ctx, constant.LuaUnlockLock, []string{key}, value).Err()
}
