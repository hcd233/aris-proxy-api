// Package enum provides common enums for the application.
package enum

// AnthropicStopReason Anthropic 消息停止原因
//
//	@author centonhuang
//	@update 2026-04-09 15:00:00
type AnthropicStopReason = string

const (
	// AnthropicStopReasonEndTurn 模型自然结束（end_turn）
	//
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	AnthropicStopReasonEndTurn AnthropicStopReason = "end_turn"

	// AnthropicStopReasonStop 触发停止序列（stop）
	//
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	AnthropicStopReasonStop AnthropicStopReason = "stop"

	// AnthropicStopReasonMaxTokens 达到最大 token 数（max_tokens）
	//
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	AnthropicStopReasonMaxTokens AnthropicStopReason = "max_tokens"

	// AnthropicStopReasonToolUse 模型调用工具（tool_use）
	//
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	AnthropicStopReasonToolUse AnthropicStopReason = "tool_use"
)
