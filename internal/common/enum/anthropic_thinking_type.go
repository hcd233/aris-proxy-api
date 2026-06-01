// Package enum provides common enums for the application.
package enum

// AnthropicThinkingType Anthropic thinking type values.
//
//	@author centonhuang
//	@update 2026-04-19 10:00:00
type AnthropicThinkingType = string

const (
	AnthropicThinkingTypeEnabled  AnthropicThinkingType = "enabled"
	AnthropicThinkingTypeDisabled AnthropicThinkingType = "disabled"
	AnthropicThinkingTypeAdaptive AnthropicThinkingType = "adaptive"
	AnthropicThinkingTypeLow      AnthropicThinkingType = "low"
	AnthropicThinkingTypeMedium   AnthropicThinkingType = "medium"
	AnthropicThinkingTypeHigh     AnthropicThinkingType = "high"
	AnthropicThinkingTypeMinimal  AnthropicThinkingType = "minimal"
)

// ResponseReasoningEffort OpenAI Response API reasoning effort values.
//
//	@author centonhuang
//	@update 2026-04-19 10:00:00
type ResponseReasoningEffort = string

const (
	ResponseEffortLow     ResponseReasoningEffort = "low"
	ResponseEffortMedium  ResponseReasoningEffort = "medium"
	ResponseEffortHigh    ResponseReasoningEffort = "high"
	ResponseEffortXHigh   ResponseReasoningEffort = "xhigh"
	ResponseEffortNone    ResponseReasoningEffort = "none"
	ResponseEffortMinimal ResponseReasoningEffort = "minimal"
)
