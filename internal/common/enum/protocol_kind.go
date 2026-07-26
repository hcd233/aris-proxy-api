package enum

// ProtocolKind LLM 代理入口协议类型，供 application 层向 adapter 标识
// ProxyError 与 StreamResult 所属的协议，从而选择对应的 error envelope 与 SSE framing。
//
//	@author centonhuang
//	@update 2026-07-25 19:00:00
type ProtocolKind = uint8

const (

	// ProtocolKindOpenAI OpenAI 入口协议（Chat Completions / Responses）
	//
	//	@author centonhuang
	//	@update 2026-07-25 19:00:00
	ProtocolKindOpenAI ProtocolKind = iota

	// ProtocolKindAnthropic Anthropic 入口协议（Messages）
	//
	//	@author centonhuang
	//	@update 2026-07-25 19:00:00
	ProtocolKindAnthropic
)
