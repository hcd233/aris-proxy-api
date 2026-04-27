package dto

import (
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
)

// UnifiedTool 重新导出至 domain/conversation/vo.UnifiedTool
//
// 该类型已迁移到 internal/domain/conversation/vo 作为领域值对象，此处保留类型别名
// 避免破坏现有调用方 import。新代码应直接使用 vo.UnifiedTool。
//
// Deprecated: 请使用 internal/domain/conversation/vo.UnifiedTool
type UnifiedTool = vo.UnifiedTool

// FromOpenAITool 从 OpenAI ChatCompletionTool 转换为 UnifiedTool
//
//	@param tool *OpenAIChatCompletionTool
//	@return *UnifiedTool
//	@author centonhuang
//	@update 2026-04-22 14:10:00
func FromOpenAITool(tool *OpenAIChatCompletionTool) *UnifiedTool {
	unified := &UnifiedTool{}
	if tool.Function != nil {
		unified.Name = tool.Function.Name
		unified.Description = tool.Function.Description
		unified.Parameters = tool.Function.Parameters
	}
	return unified
}

// FromAnthropicTool 从 Anthropic AnthropicTool 转换为 UnifiedTool
//
//	@param tool *AnthropicTool
//	@return *UnifiedTool
//	@author centonhuang
//	@update 2026-04-22 14:10:00
func FromAnthropicTool(tool *AnthropicTool) *UnifiedTool {
	unified := &UnifiedTool{
		Name:        tool.Name,
		Description: tool.Description,
	}
	if tool.InputSchema != nil {
		unified.Parameters = tool.InputSchema
	}
	return unified
}
