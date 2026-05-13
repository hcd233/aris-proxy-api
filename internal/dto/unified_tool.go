package dto

import (
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
)

// FromOpenAITool 从 OpenAI ChatCompletionTool 转换为 UnifiedTool
//
//	@param tool *OpenAIChatCompletionTool
//	@return *vo.UnifiedTool
//	@author centonhuang
//	@update 2026-04-22 14:10:00
func FromOpenAITool(tool *OpenAIChatCompletionTool) *vo.UnifiedTool {
	unified := &vo.UnifiedTool{}
	if tool.Function != nil {
		unified.Name = tool.Function.Name
		if tool.Function.Description != nil {
			unified.Description = *tool.Function.Description
		}
		unified.Parameters = &tool.Function.Parameters.JSONSchemaProperty
	}
	return unified
}

// FromAnthropicTool 从 Anthropic AnthropicTool 转换为 UnifiedTool
//
//	@param tool *AnthropicTool
//	@return *UnifiedTool
//	@author centonhuang
//	@update 2026-04-22 14:10:00
func FromAnthropicTool(tool *AnthropicTool) *vo.UnifiedTool {
	unified := &vo.UnifiedTool{}
	if tool.Name != nil {
		unified.Name = *tool.Name
	}
	if tool.Description != nil {
		unified.Description = *tool.Description
	}
	if tool.InputSchema != nil {
		unified.Parameters = &tool.InputSchema.JSONSchemaProperty
	}
	return unified
}
