package lock

import (
	"context"
	"time"

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

const luaRefresh = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("PEXPIRE", KEYS[1], ARGV[2])
else
    return 0
end
`

const luaUnlock = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end
`

func (l *redisLocker) Lock(ctx context.Context, key, value string, expire time.Duration) (success bool, err error) {
	return l.cache.SetNX(ctx, key, value, expire).Result()
}

func (l *redisLocker) Refresh(ctx context.Context, key, value string, expire time.Duration) (success bool, err error) {
	res, err := l.cache.Eval(ctx, luaRefresh, []string{key}, value, expire.Milliseconds()).Int64()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

func (l *redisLocker) Unlock(ctx context.Context, key, value string) (err error) {
	return l.cache.Eval(ctx, luaUnlock, []string{key}, value).Err()
}
