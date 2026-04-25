package vo

import "github.com/hcd233/aris-proxy-api/internal/common/util"

// UpstreamCreds 上游接入凭证值对象
//
// 封装调用上游 LLM 所需的连接信息（BaseURL + APIKey + 真实模型名），
// 与客户端暴露的 EndpointAlias 区分。不可变，构造后仅通过 getter 读取。
//
//	@author centonhuang
//	@update 2026-04-22 16:30:00
type UpstreamCreds struct {
	baseURL string
	apiKey  string
	model   string
}

// NewUpstreamCreds 构造上游凭证值对象
//
//	@param baseURL string 上游基础 URL
//	@param apiKey string 上游 API 密钥
//	@param model string 上游真实模型名
//	@return UpstreamCreds
//	@author centonhuang
//	@update 2026-04-25 10:00:00
func NewUpstreamCreds(baseURL, apiKey, model string) UpstreamCreds {
	return UpstreamCreds{baseURL: baseURL, apiKey: apiKey, model: model}
}

// BaseURL 返回上游基础 URL
//
//	@receiver c UpstreamCreds
//	@return string
//	@author centonhuang
//	@update 2026-04-25 10:00:00
func (c UpstreamCreds) BaseURL() string { return c.baseURL }

// APIKey 返回上游 API 密钥
//
//	@receiver c UpstreamCreds
//	@return string
//	@author centonhuang
//	@update 2026-04-25 10:00:00
func (c UpstreamCreds) APIKey() string { return c.apiKey }

// Model 返回上游真实模型名
//
//	@receiver c UpstreamCreds
//	@return string
//	@author centonhuang
//	@update 2026-04-25 10:00:00
func (c UpstreamCreds) Model() string { return c.model }

// IsValid 判断凭证是否完整
//
//	@receiver c UpstreamCreds
//	@return bool
//	@author centonhuang
//	@update 2026-04-22 16:30:00
func (c UpstreamCreds) IsValid() bool {
	return c.baseURL != "" && c.apiKey != "" && c.model != ""
}

// MaskedAPIKey 返回 apiKey 的脱敏形式用于日志输出
//
//	@receiver c UpstreamCreds
//	@return string
//	@author centonhuang
//	@update 2026-04-24 20:00:00
func (c UpstreamCreds) MaskedAPIKey() string {
	return util.MaskSecret(c.apiKey)
}
