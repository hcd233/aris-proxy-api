package dto

import (
	"github.com/hcd233/aris-proxy-api/internal/enum"
)

// UnifiedTool 统一工具格式，用于跨 Provider 的工具存储
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type UnifiedTool struct {
	Provider    enum.ProviderType   `json:"provider" doc:"工具来源提供者"`
	Name        string              `json:"name" doc:"工具名称"`
	Description string              `json:"description" doc:"工具描述"`
	Parameters  *JSONSchemaProperty `json:"parameters" doc:"工具参数Schema"`
}

// FromOpenAITool 从 OpenAI ChatCompletionTool 转换为 UnifiedTool
//
//	@param tool *ChatCompletionTool
//	@return *UnifiedTool
//	@author centonhuang
//	@update 2026-03-18 10:00:00
func FromOpenAITool(tool *ChatCompletionTool) *UnifiedTool {
	unified := &UnifiedTool{
		Provider: enum.ProviderOpenAI,
	}
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
//	@update 2026-03-18 10:00:00
func FromAnthropicTool(tool *AnthropicTool) *UnifiedTool {
	unified := &UnifiedTool{
		Provider:    enum.ProviderAnthropic,
		Name:        tool.Name,
		Description: tool.Description,
	}
	if tool.InputSchema != nil {
		unified.Parameters = tool.InputSchema
	}
	return unified
}
