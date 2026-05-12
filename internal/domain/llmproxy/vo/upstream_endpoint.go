package vo

// UpstreamEndpoint 上游端点信息（与数据库模型解耦）
// 原定义位于 infrastructure/transport，移至 domain 层以消除 application → infrastructure 依赖。
type UpstreamEndpoint struct {
	Model   string
	APIKey  string
	BaseURL string
}

// NewUpstreamEndpointFromCredential 从字段值构建 UpstreamEndpoint
func NewUpstreamEndpointFromCredential(model, apiKey, baseURL string) UpstreamEndpoint {
	return UpstreamEndpoint{
		Model:   model,
		APIKey:  apiKey,
		BaseURL: baseURL,
	}
}
