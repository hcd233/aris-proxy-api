package cache

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"
	"github.com/samber/mo"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// ReadCache 通用 cache-aside 读缓存
//
// 面向读多写少的目录类查询（模型别名、模型详情、模型列表），
// 三类缓存风险的防护：
//   - 穿透：loader 返回 mo.None 视为「查无此物」，写空值标记（短 TTL）后直接返回零值
//   - 雪崩：TTL 叠加随机抖动，同批写入/同批失效重建的条目不会同时过期
//   - 击穿：singleflight 合并同 key 并发回源，只有一个 goroutine 查库
//
// Redis 故障一律 fail-open：读失败按未命中处理、写失败忽略，缓存永不影响正确性。
type ReadCache struct {
	client *redis.Client
	group  singleflight.Group
}

// NewReadCache 创建通用读缓存
//
//	@param client *redis.Client
//	@return *ReadCache
//	@author centonhuang
//	@update 2026-10-10 10:00:00
func NewReadCache(client *redis.Client) *ReadCache {
	return &ReadCache{client: client}
}

// GetOrLoad 读缓存主入口
//
// 命中缓存直接返回；未命中经 singleflight 回源后写回。
// loader 返回 mo.None 表示「查无此物」，写入空值标记（短 TTL）防缓存穿透，
// 后续同 key 请求直接返回零值，不再回源。
//
//	@param ctx context.Context
//	@param c *ReadCache 为 nil 时退化为直接回源（无缓存装配）
//	@param key string 缓存键（constant.ReadCache*KeyTemplate）
//	@param loader func(context.Context) (mo.Option[T], error) 回源加载器，空结果返回 mo.None
//	@return mo.Option[T]
//	@return error
//	@author centonhuang
//	@update 2026-10-10 10:00:00
func GetOrLoad[T any](ctx context.Context, c *ReadCache, key string, loader func(context.Context) (mo.Option[T], error)) (mo.Option[T], error) {
	if c == nil || c.client == nil {
		return loader(ctx)
	}

	if cached, ok := c.get(ctx, key); ok {
		if cached == constant.ReadCacheEmptyValue {
			return mo.None[T](), nil
		}
		var out T
		if err := sonic.UnmarshalString(cached, &out); err == nil {
			return mo.Some(out), nil
		}
		logger.WithCtx(ctx).Warn("[ReadCache] Corrupted cache payload, fallback to load", zap.String("key", key))
	}

	shared, err, _ := c.group.Do(key, func() (any, error) {
		loadCtx := context.WithoutCancel(ctx)
		value, loadErr := loader(loadCtx)
		if loadErr != nil {
			return mo.None[T](), loadErr
		}
		if payload, ttl, encodeErr := encodeCached(value); encodeErr != nil {
			logger.WithCtx(loadCtx).Warn("[ReadCache] Encode cache payload failed", zap.Error(encodeErr))
		} else {
			c.set(loadCtx, key, payload, ttl)
		}
		return value, nil
	})
	if err != nil {
		return mo.None[T](), err
	}
	value, ok := shared.(mo.Option[T])
	if !ok {
		// singleflight 共享结果类型不符（理论上不可达），回源兜底
		return loader(ctx)
	}
	return value, nil
}

// InvalidateAll 清空读缓存命名空间（写路径成功后调用）
//
// 写低频读高频，SCAN + DEL 的全量失效成本可接受；相比按 owner 记账，
// 不存在「admin 删他人数据漏失效」的记账漏洞。失效后的新鲜数据由
// TTL 抖动 + singleflight 平滑回源。失败仅记日志，由 TTL 兜底收敛。
//
//	@receiver c *ReadCache
//	@param ctx context.Context
//	@author centonhuang
//	@update 2026-10-10 10:00:00
func (c *ReadCache) InvalidateAll(ctx context.Context) {
	if c == nil || c.client == nil {
		return
	}
	var cursor uint64
	for {
		keys, next, err := c.client.Scan(ctx, cursor, constant.ReadCacheKeyScanPattern, constant.ReadCacheScanBatch).Result()
		if err != nil {
			logger.WithCtx(ctx).Warn("[ReadCache] Scan failed during invalidation", zap.Error(err))
			return
		}
		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				logger.WithCtx(ctx).Warn("[ReadCache] Delete failed during invalidation", zap.Error(err))
				return
			}
		}
		if next == 0 {
			return
		}
		cursor = next
	}
}

// get 读缓存载荷；Redis 故障按未命中处理（fail-open）
func (c *ReadCache) get(ctx context.Context, key string) (string, bool) {
	payload, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			logger.WithCtx(ctx).Warn("[ReadCache] Get failed, fallback to load", zap.String("key", key), zap.Error(err))
		}
		return "", false
	}
	return payload, true
}

// set 写缓存载荷；失败忽略（fail-open）
func (c *ReadCache) set(ctx context.Context, key, payload string, ttl time.Duration) {
	if err := c.client.Set(ctx, key, payload, ttl).Err(); err != nil {
		logger.WithCtx(ctx).Warn("[ReadCache] Set failed, skip caching", zap.String("key", key), zap.Error(err))
	}
}

// encodeCached 编码缓存载荷
//
// mo.None 编码为空值标记（短 TTL，防穿透）；mo.Some 编码为 JSON 载荷，
// TTL 叠加随机抖动（防雪崩）。
func encodeCached[T any](value mo.Option[T]) (string, time.Duration, error) {
	v, ok := value.Get()
	if !ok {
		return constant.ReadCacheEmptyValue, jitteredTTL(constant.ReadCacheEmptyTTL, constant.ReadCacheEmptyTTLJitter), nil
	}
	payload, err := sonic.MarshalString(v)
	if err != nil {
		return "", 0, err
	}
	return payload, jitteredTTL(constant.ReadCacheTTL, constant.ReadCacheTTLJitter), nil
}

// jitteredTTL 返回叠加随机抖动的 TTL，打散同批条目的过期时刻（防缓存雪崩）
func jitteredTTL(base, jitter time.Duration) time.Duration {
	if jitter <= 0 {
		return base
	}
	return base + time.Duration(rand.Int64N(int64(jitter))) //nolint:gosec // G404 TTL 抖动无需密码学随机
}
