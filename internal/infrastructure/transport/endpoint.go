// Package transport 上游传输层，负责与上游 LLM 提供者的 HTTP/SSE 通信
package transport

import "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"

// UpstreamEndpoint 上游端点信息（与数据库模型解耦）
//
//	@author centonhuang
//	@update 2026-04-26 14:00:00
type UpstreamEndpoint struct {
	Model   string
	APIKey  string
	BaseURL string
}

// NewUpstreamEndpoint 从 EndpointReadRepository 的凭据投影构建 UpstreamEndpoint
//
//	@param creds *llmproxy.EndpointCredentialProjection
//	@return UpstreamEndpoint
//	@author centonhuang
//	@update 2026-04-26 14:00:00
func NewUpstreamEndpointFromCredential(creds *llmproxy.EndpointCredentialProjection) UpstreamEndpoint {
	return UpstreamEndpoint{
		Model:   creds.Model,
		APIKey:  creds.APIKey,
		BaseURL: creds.BaseURL,
	}
}
