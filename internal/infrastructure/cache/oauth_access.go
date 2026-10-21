package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/util"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
)

// oauthAccessCache OAuthAccessCache 的 Redis 实现
type oauthAccessCache struct {
	rdb *redis.Client
	key []byte
}

// storedOAuthAccess 缓存载荷
//
// TokenCiphertext 存的是 AES-256-GCM 密文（与数据库凭据同一把密钥、同一套格式）：
// Redis 会落盘（RDB/AOF、备份、从库），明文 token 等于绕过数据库里的加密存储。
type storedOAuthAccess struct {
	TokenCiphertext string    `json:"token"`
	AccountRef      string    `json:"account_ref"`
	ExpiresAt       time.Time `json:"expires_at"`
}

// NewOAuthAccessCache 构造订阅账号 access token 缓存（读写失败一律按未命中处理）。
//
// key 为凭据加密密钥（config.OAuthCredentialKey）：为空/非法时缓存只读不写，
// 绝不在密钥异常时降级为明文存储。
func NewOAuthAccessCache(rdb *redis.Client, key []byte) service.OAuthAccessCache {
	return &oauthAccessCache{rdb: rdb, key: key}
}

// Get 读取缓存；未命中/解析失败/解密失败/已不足安全余量返回 false。
//
// 余量判定不依赖 Redis TTL 单一防线：TTL 只保证 key 过期，若 TTL 计算出现偏差
// （如 ExpiresAt 被上游少报），这里能进步一道。
// 解密失败按未命中处理：历史明文条目、密钥轮换后的旧条目都会自然回落到 DB 重新写入。
func (c *oauthAccessCache) Get(ctx context.Context, endpointID uint) (service.OAuthAccess, bool) {
	raw, err := c.rdb.Get(ctx, fmt.Sprintf(constant.OAuthAccessKeyTemplate, endpointID)).Bytes()
	if err != nil {
		return service.OAuthAccess{}, false
	}
	var stored storedOAuthAccess
	if err := sonic.Unmarshal(raw, &stored); err != nil {
		return service.OAuthAccess{}, false
	}
	if !stored.ExpiresAt.After(time.Now().Add(constant.OAuthAccessCacheGuard)) {
		return service.OAuthAccess{}, false
	}
	token, err := util.DecryptSecret(c.key, stored.TokenCiphertext)
	if err != nil {
		return service.OAuthAccess{}, false
	}
	return service.OAuthAccess{Token: token, AccountRef: stored.AccountRef, ExpiresAt: stored.ExpiresAt}, true
}

// Put 写入缓存（ttl 由调用方按剩余有效期计算）；失败仅降级
func (c *oauthAccessCache) Put(ctx context.Context, endpointID uint, access service.OAuthAccess, ttl time.Duration) {
	encrypted, err := util.EncryptSecret(c.key, access.Token)
	if err != nil {
		return
	}
	payload, err := sonic.Marshal(storedOAuthAccess{TokenCiphertext: encrypted, AccountRef: access.AccountRef, ExpiresAt: access.ExpiresAt})
	if err != nil {
		return
	}
	_ = c.rdb.Set(ctx, fmt.Sprintf(constant.OAuthAccessKeyTemplate, endpointID), payload, ttl).Err() //nolint:errcheck // 缓存写失败仅降级到 DB 路径
}

// Delete 失效缓存（重新授权后调用）；失败仅降级（该端点下一次请求会重新走 DB）
func (c *oauthAccessCache) Delete(ctx context.Context, endpointID uint) {
	_ = c.rdb.Del(ctx, fmt.Sprintf(constant.OAuthAccessKeyTemplate, endpointID)).Err() //nolint:errcheck // 删除失败仅降级
}
