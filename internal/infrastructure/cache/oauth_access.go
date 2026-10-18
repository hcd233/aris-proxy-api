package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
)

// oauthAccessCache OAuthAccessCache 的 Redis 实现
type oauthAccessCache struct {
	rdb *redis.Client
}

// storedOAuthAccess 缓存载荷
type storedOAuthAccess struct {
	Token      string    `json:"token"`
	AccountRef string    `json:"account_ref"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// NewOAuthAccessCache 构造订阅账号 access token 缓存（读写失败一律按未命中处理）
func NewOAuthAccessCache(rdb *redis.Client) service.OAuthAccessCache {
	return &oauthAccessCache{rdb: rdb}
}

// Get 读取缓存；未命中/解析失败/已过安全余量返回 false
func (c *oauthAccessCache) Get(ctx context.Context, endpointID uint) (service.OAuthAccess, bool) {
	raw, err := c.rdb.Get(ctx, fmt.Sprintf(constant.OAuthAccessKeyTemplate, endpointID)).Bytes()
	if err != nil {
		return service.OAuthAccess{}, false
	}
	var stored storedOAuthAccess
	if err := sonic.Unmarshal(raw, &stored); err != nil {
		return service.OAuthAccess{}, false
	}
	return service.OAuthAccess{Token: stored.Token, AccountRef: stored.AccountRef, ExpiresAt: stored.ExpiresAt}, true
}

// Put 写入缓存（ttl 由调用方按剩余有效期计算）；失败仅降级
func (c *oauthAccessCache) Put(ctx context.Context, endpointID uint, access service.OAuthAccess, ttl time.Duration) {
	payload, err := sonic.Marshal(storedOAuthAccess{Token: access.Token, AccountRef: access.AccountRef, ExpiresAt: access.ExpiresAt})
	if err != nil {
		return
	}
	_ = c.rdb.Set(ctx, fmt.Sprintf(constant.OAuthAccessKeyTemplate, endpointID), payload, ttl).Err() //nolint:errcheck // 缓存写失败仅降级到 DB 路径
}
