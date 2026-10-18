package enum

// EndpointAuthType Endpoint 认证类型
//
// api_key（默认，全部存量端点）使用 Endpoint.apiKey 直连上游；
// oauth_anthropic / oauth_openai 为订阅账号 OAuth 凭据（密文存 endpoint_oauth_credentials 表），
// 连接字段（baseURL/能力标记）由服务端固定写入，Update 路径不可修改。
type EndpointAuthType string

const (
	// EndpointAuthAPIKey API Key 直连（默认）
	EndpointAuthAPIKey EndpointAuthType = "api_key"

	// EndpointAuthOAuthAnthropic Anthropic 订阅账号（Claude Code OAuth）
	EndpointAuthOAuthAnthropic EndpointAuthType = "oauth_anthropic"

	// EndpointAuthOAuthOpenAI OpenAI 订阅账号（Codex OAuth）
	EndpointAuthOAuthOpenAI EndpointAuthType = "oauth_openai"
)

// IsOAuth 是否为 OAuth 订阅账号类型
func (t EndpointAuthType) IsOAuth() bool {
	return t == EndpointAuthOAuthAnthropic || t == EndpointAuthOAuthOpenAI
}

// Normalize 空值归一为 api_key（调用方漏传时保持默认语义，不产生脏数据）
func (t EndpointAuthType) Normalize() EndpointAuthType {
	if t == "" {
		return EndpointAuthAPIKey
	}
	return t
}
