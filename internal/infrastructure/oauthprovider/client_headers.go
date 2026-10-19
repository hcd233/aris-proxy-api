package oauthprovider

import (
	"net/http"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// setClaudeClientHeaders 补 Claude 客户端身份头（token 与 usage 端点共用）。
//
// 官方客户端在 token/usage 请求上一致携带 CLI 形态 UA 与 x-app；缺头时上游
// 可能按未知客户端处理（风控/拒绝），故统一在此补齐，避免逐调用点漂移。
func setClaudeClientHeaders(header http.Header) {
	header.Set(constant.HTTPHeaderUserAgent, constant.OAuthClaudeUserAgent)
	header.Set(constant.HTTPHeaderXApp, constant.OAuthClaudeXApp)
}

// setCodexClientHeaders 补 Codex 客户端身份头（token 与 usage 端点共用）。
func setCodexClientHeaders(header http.Header) {
	header.Set(constant.HTTPHeaderUserAgent, constant.OAuthCodexUserAgent)
	header.Set(constant.HTTPHeaderOriginator, constant.OAuthCodexOriginator)
}
