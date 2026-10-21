package oauthprovider

import (
	"context"
	"sync"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/hcd233/aris-proxy-api/internal/lock"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// credentialKeyWarnOnce 保证启动期密钥告警只打一次（fx 与 cron 两条装配路径都会调用）。
var credentialKeyWarnOnce sync.Once

// NewDefaultTokenProvider 装配 token provider 的默认组合：
// 凭据仓储 + provider 客户端集合 + access 缓存 + 分布式锁。
//
// 订阅功能在 fx 容器与 cron registry 两条路径上都要构造同一组合，集中在此避免
// 两处手工装配漂移（曾出现同一能力两份装配，其中一份缺少读缓存注入的情况）。
//
// 装配期顺带校验加密密钥：缺配时告警一次，而不是等用户绑定或转发失败时才发现
// （凭据读写 fail-closed，API Key 端点不受影响）。
func NewDefaultTokenProvider(credRepo llmproxy.OAuthCredentialRepository, redisClient *redis.Client) service.OAuthTokenProvider {
	credentialKeyWarnOnce.Do(func() {
		if len(config.OAuthCredentialKey) == 0 {
			logger.WithCtx(context.Background()).Warn(
				"[OAuthProvider] OAUTH_CREDENTIAL_KEY is not configured; subscription endpoints will fail-closed",
				zap.Int("expectedKeyLength", constant.SecretBoxKeyLength))
		}
	})
	return NewOAuthTokenProvider(credRepo, NewClients(), cache.NewOAuthAccessCache(redisClient, config.OAuthCredentialKey), lock.NewLocker(redisClient))
}
