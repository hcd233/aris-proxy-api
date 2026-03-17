// Package enum provides common enums for the application.
package enum

// ProviderType 上游 API 提供者类型
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
