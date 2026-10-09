package cache

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

type endpointAffinityCache struct {
	rdb *redis.Client
}

// NewEndpointAffinityCache 构造端点亲和映射（Redis，TTL 见 constant.AffinityTTL；读写失败 fail-open）
//
//	@param rdb *redis.Client
//	@return service.EndpointAffinity
//	@author centonhuang
//	@update 2026-10-09 10:00:00
func NewEndpointAffinityCache(rdb *redis.Client) service.EndpointAffinity {
	return &endpointAffinityCache{rdb: rdb}
}

// Get 读取亲和端点 ID；不存在或 Redis 失败返回 false（fail-open）。
//
//	@receiver c *endpointAffinityCache
//	@param ctx context.Context
//	@param userID uint
//	@param alias string
//	@param key string
//	@return uint 端点 ID
//	@return bool false=未命中
//	@author centonhuang
//	@update 2026-10-09 10:00:00
func (c *endpointAffinityCache) Get(ctx context.Context, userID uint, alias, key string) (uint, bool) {
	val, err := c.rdb.Get(ctx, fmt.Sprintf(constant.AffinityKeyTemplate, userID, alias, key)).Uint64()
	if err != nil {
		return 0, false
	}
	return uint(val), true
}

// Put 写入/刷新亲和映射（TTL 随写入刷新）；失败仅告警。
//
//	@receiver c *endpointAffinityCache
//	@param ctx context.Context
//	@param userID uint
//	@param alias string
//	@param key string
//	@param endpointID uint
//	@author centonhuang
//	@update 2026-10-09 10:00:00
func (c *endpointAffinityCache) Put(ctx context.Context, userID uint, alias, key string, endpointID uint) {
	if err := c.rdb.Set(ctx, fmt.Sprintf(constant.AffinityKeyTemplate, userID, alias, key), endpointID, constant.AffinityTTL).Err(); err != nil {
		logger.WithCtx(ctx).Warn("[EndpointAffinityCache] Put affinity failed", zap.Error(err))
	}
}
