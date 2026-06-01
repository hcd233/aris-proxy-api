package lock

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/redis/go-redis/v9"
)

// Locker 锁接口
//
//	@author centonhuang
//	@update 2026-06-01 10:00:00
type Locker interface {
	Lock(ctx context.Context, key string, value string, expire time.Duration) (success bool, err error)
	Refresh(ctx context.Context, key string, value string, expire time.Duration) (success bool, err error)
	Unlock(ctx context.Context, key string, value string) (err error)
}

// NewLocker 创建锁
//
//	@return Locker
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func NewLocker(cache *redis.Client) Locker {
	return &redisLocker{cache: cache}
}

type redisLocker struct {
	cache *redis.Client
}

func (l *redisLocker) Lock(ctx context.Context, key, value string, expire time.Duration) (success bool, err error) {
	return l.cache.SetNX(ctx, key, value, expire).Result()
}

func (l *redisLocker) Refresh(ctx context.Context, key, value string, expire time.Duration) (success bool, err error) {
	res, err := l.cache.Eval(ctx, constant.LuaRefreshLock, []string{key}, value, expire.Milliseconds()).Int64()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

func (l *redisLocker) Unlock(ctx context.Context, key, value string) (err error) {
	return l.cache.Eval(ctx, constant.LuaUnlockLock, []string{key}, value).Err()
}
