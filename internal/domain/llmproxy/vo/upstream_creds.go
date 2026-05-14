package vo

import (
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/util"
)

// UpstreamCreds 上游接入凭证值对象
//
// 封装调用上游 LLM 所需的连接信息（BaseURL + APIKey），
// 上游真实模型名由 aggregate.Model 提供。
type UpstreamCreds struct {
	baseURL string
	apiKey  string
}

// NewUpstreamCreds 构造上游凭证值对象
func NewUpstreamCreds(baseURL, apiKey string) (UpstreamCreds, error) {
	if baseURL == "" || apiKey == "" {
		return UpstreamCreds{}, ierr.New(ierr.ErrValidation, "upstream creds must have non-empty baseURL and apiKey")
	}
	return UpstreamCreds{baseURL: baseURL, apiKey: apiKey}, nil
}

func (c UpstreamCreds) BaseURL() string      { return c.baseURL }
func (c UpstreamCreds) APIKey() string       { return c.apiKey }
func (c UpstreamCreds) IsValid() bool        { return c.baseURL != "" && c.apiKey != "" }
func (c UpstreamCreds) MaskedAPIKey() string { return util.MaskSecret(c.apiKey) }
