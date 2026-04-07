package lock

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/redis/go-redis/v9"
)

// Locker 锁接口
//
//	@param ctx context.Context
//	@param key string
//	@param value string
//	@return err error
//	@author centonhuang
//	@update 2025-11-11 16:54:41
type Locker interface {
	Lock(ctx context.Context, key string, value string, expire time.Duration) (success bool, err error)
	Unlock(ctx context.Context, key string, value string) (err error)
}

// NewLocker 创建锁
//
//	@return Locker
//	@author centonhuang
//	@update 2025-11-11 17:49:18
func NewLocker() Locker {
	return &redisLocker{
		rdb: cache.GetRedisClient(),
	}
}

type redisLocker struct {
	rdb *redis.Client
}

func (l *redisLocker) Lock(ctx context.Context, key string, value string, expire time.Duration) (success bool, err error) {
	return l.rdb.SetNX(ctx, key, value, expire).Result()
}

func (l *redisLocker) Unlock(ctx context.Context, key string, value string) (err error) {
	luaScript := `
			if redis.call("get", KEYS[1]) == ARGV[1] then
				return redis.call("del", KEYS[1])
			else
				return 0
			end
		`
	return l.rdb.Eval(ctx, luaScript, []string{key}, value).Err()
}
