package service

import (
	"context"
	"time"
)

// OAuthAccess 一次取用的有效订阅凭据（access token 从不出现在日志/响应）
type OAuthAccess struct {
	// Token 明文 access token（仅限转发链路注入 Authorization 头使用）
	Token string
	// AccountRef 账号标识（Claude organization uuid / Codex chatgpt account id）
	AccountRef string
	// ExpiresAt access token 到期时间（缓存判定用）
	ExpiresAt time.Time
}

// OAuthTokenProvider 订阅账号 access token 提供者（请求路径与 cron 维护共用）。
//
// GetAccessToken 在 token 临期时自动刷新（锁 + 双检）；刷新失败返回
// *model.OAuthCredentialError（可切换错误，由 runWithFallback 切备选端点）。
type OAuthTokenProvider interface {
	// GetAccessToken 返回可用 access token（必要时刷新）
	GetAccessToken(ctx context.Context, endpointID uint) (OAuthAccess, error)
	// RefreshIfNeeded 临期（提前量大于请求路径）则主动刷新；未临期时 no-op（cron 用）
	RefreshIfNeeded(ctx context.Context, endpointID uint) error
}

// OAuthAccessCache access token 缓存（实现于 infrastructure/cache）。
//
// 读写失败一律按未命中处理（fail-open 到 DB + 刷新路径），绝不因缓存故障阻断转发。
type OAuthAccessCache interface {
	// Get 读取缓存；未命中或解析失败返回 (零值, false)
	Get(ctx context.Context, endpointID uint) (OAuthAccess, bool)
	// Put 写入缓存（ttl 由调用方按剩余有效期计算）
	Put(ctx context.Context, endpointID uint, access OAuthAccess, ttl time.Duration)
}
