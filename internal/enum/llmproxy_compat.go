package enum

// ProxyAPI 表示客户端请求的代理 API 类型。
type ProxyAPI int

const (
	ProxyAPIOpenAIChat ProxyAPI = iota
	ProxyAPIOpenAIResponse
	ProxyAPIAnthropicMessage
)

// CompatRoute 表示 endpoint 能力匹配后的实际上游调用路线。
type CompatRoute int

const (
	CompatRouteUnsupported CompatRoute = iota
	CompatRouteNative
	CompatRouteViaOpenAIChat
	CompatRouteViaAnthropicMessage
)
