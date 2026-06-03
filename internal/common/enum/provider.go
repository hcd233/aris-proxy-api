// Package enum provides common enums for the application.
package enum

// ProviderType 上游 API 提供者类型（保留兼容性，新代码请使用 ProtocolType）
//
//	@author centonhuang
//	@update 2026-03-17 10:00:00
type ProviderType = string

const (

	// ProviderOpenAI OpenAI 兼容 API
	//
	//	@author centonhuang
	//	@update 2026-03-17 10:00:00
	ProviderOpenAI ProviderType = "openai"

	// ProviderAnthropic Anthropic 兼容 API
	//
	//	@author centonhuang
	//	@update 2026-03-17 10:00:00
	ProviderAnthropic ProviderType = "anthropic"
)

// ProtocolType 协议类型枚举
//
//	@author centonhuang
//	@update 2026-06-04 10:00:00
type ProtocolType = string

const (

	// ProtocolOpenAIChatCompletion OpenAI Chat Completions 协议
	//
	//	@author centonhuang
	//	@update 2026-06-04 10:00:00
	ProtocolOpenAIChatCompletion ProtocolType = "openai-chat-completion"

	// ProtocolOpenAIResponse OpenAI Response API 协议
	//
	//	@author centonhuang
	//	@update 2026-06-04 10:00:00
	ProtocolOpenAIResponse ProtocolType = "openai-response"

	// ProtocolAnthropicMessage Anthropic Messages 协议
	//
	//	@author centonhuang
	//	@update 2026-06-04 10:00:00
	ProtocolAnthropicMessage ProtocolType = "anthropic-message"
)
