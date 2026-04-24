// Package proxy 上游代理层，负责与上游 LLM 提供者的 HTTP/SSE 通信
package transport

// UpstreamEndpoint 上游端点信息（与数据库模型解耦）
//
//	@author centonhuang
//	@update 2026-04-05 10:00:00
type UpstreamEndpoint struct {
	Model   string
	APIKey  string
	BaseURL string
}
