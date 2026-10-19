package service

import (
	"context"
	"strconv"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// OAuthTokenSet 一次 OAuth token 交换/刷新的结果（明文，落库前由 repository 加密）。
type OAuthTokenSet struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	AccountRef   string
	AccountEmail string
	PlanType     string
}

// OAuthProviderClient 单 provider（Claude Code / Codex）的 OAuth 协议客户端。
//
// 协议事实（端点 URL、client_id、scope、请求形态）集中在实现与 constant/oauth.go；
// 本接口只定义领域需要的能力，不暴露 provider 细节。
type OAuthProviderClient interface {
	// Provider 返回提供方标识
	Provider() enum.OAuthProvider
	// AuthorizeURL 生成授权链接（PKCE challenge 与 state 由调用方生成）
	AuthorizeURL(challenge, state string) string
	// ExchangeCode 用授权码换 token（verifier/state 为 PKCE 流配套参数，provider 按需使用）
	ExchangeCode(ctx context.Context, code, verifier, state string) (*OAuthTokenSet, error)
	// Refresh 刷新 access token
	Refresh(ctx context.Context, refreshToken string) (*OAuthTokenSet, error)
	// ProbeQuota 探测订阅窗口用量（Claude 5h/7d；Codex primary/secondary）
	ProbeQuota(ctx context.Context, accessToken, accountRef string) (*vo.QuotaSnapshot, error)
}

// OAuthProtocolError provider 端点返回的协议级错误（非 2xx）。
//
// Task 之外的上层（token provider）据此判定凭据是否需要重新授权：
// Reason 为上游 error 字段（如 invalid_grant）；网络/解析错误走 ierr 而非本类型。
type OAuthProtocolError struct {
	StatusCode int
	Reason     string
}

// Error 实现 error 接口
func (e *OAuthProtocolError) Error() string {
	return "oauth provider rejected request: status " + strconv.Itoa(e.StatusCode) + " reason " + e.Reason
}
