package vo

import "github.com/hcd233/aris-proxy-api/internal/common/util"

// UpstreamCreds 上游接入凭证值对象
//
// 封装调用上游 LLM 所需的连接信息（BaseURL + APIKey + 真实模型名），
// 与客户端暴露的 EndpointAlias 区分。
//
//	@author centonhuang
//	@update 2026-04-22 16:30:00
type UpstreamCreds struct {
	// BaseURL 上游基础 URL，如 https://api.openai.com/v1
	BaseURL string
	// APIKey 上游 API 密钥
	APIKey string
	// Model 上游真实模型名（区别于对外暴露的 alias）
	Model string
}

// IsValid 判断凭证是否完整
//
//	@receiver c UpstreamCreds
//	@return bool
//	@author centonhuang
//	@update 2026-04-22 16:30:00
func (c UpstreamCreds) IsValid() bool {
	return c.BaseURL != "" && c.APIKey != "" && c.Model != ""
}

// MaskedAPIKey 返回 APIKey 的脱敏形式用于日志输出
//
//	@receiver c UpstreamCreds
//	@return string
//	@author centonhuang
//	@update 2026-04-24 20:00:00
func (c UpstreamCreds) MaskedAPIKey() string {
	return util.MaskSecret(c.APIKey)
}
