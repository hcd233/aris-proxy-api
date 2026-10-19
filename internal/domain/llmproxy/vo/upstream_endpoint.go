package vo

import "github.com/hcd233/aris-proxy-api/internal/common/enum"

// UpstreamEndpoint 上游端点信息（与数据库模型解耦）
// 原定义位于 infrastructure/transport，移至 domain 层以消除 application → infrastructure 依赖。
//
// APIKey 字段承载最终鉴权凭据：api_key 端点 = 上游密钥；
// OAuth 端点 = 从 token provider 取（必要时刷新）的 access token。
type UpstreamEndpoint struct {
	Model   string
	APIKey  string
	BaseURL string
	// AuthType 认证类型（api_key / oauth_anthropic / oauth_openai）；
	// transport 层据此决定 headers/body 特化分支
	AuthType enum.EndpointAuthType
	// OAuthAccountID OAuth 账号标识（Codex 需 chatgpt-account-id 头；Claude 留空）
	OAuthAccountID string
	// StableKey 端点稳定身份（OAuth 端点为 oauth:{endpointID}）。
	// 熔断/并发隔离键与指标 label 优先使用它：access token 会轮换，
	// 直接用 token 摘要会让熔断状态每次刷新归零、指标序列无限增长。
	StableKey string
}

// NewUpstreamEndpointFromCredential 从字段值构建 UpstreamEndpoint
func NewUpstreamEndpointFromCredential(model, apiKey, baseURL string, authType enum.EndpointAuthType, oauthAccountID, stableKey string) UpstreamEndpoint {
	return UpstreamEndpoint{
		Model:          model,
		APIKey:         apiKey,
		BaseURL:        baseURL,
		AuthType:       authType,
		OAuthAccountID: oauthAccountID,
		StableKey:      stableKey,
	}
}
