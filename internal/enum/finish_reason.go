// Package enum provides common enums for the application.
package enum

// FinishReason 完成原因
//
//	@author centonhuang
//	@update 2026-03-06 10:00:00
type FinishReason = string

const (

	// FinishReasonStop 自然停止点或提供的停止序列
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	FinishReasonStop FinishReason = "stop"

	// FinishReasonLength 达到请求中指定的最大token数
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	FinishReasonLength FinishReason = "length"

	// FinishReasonToolCalls 模型调用了工具
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	FinishReasonToolCalls FinishReason = "tool_calls"

	// FinishReasonContentFilter 内容被内容过滤器标记
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	FinishReasonContentFilter FinishReason = "content_filter"

	// FinishReasonFunctionCall 模型调用了函数（已废弃）
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	FinishReasonFunctionCall FinishReason = "function_call"
)
