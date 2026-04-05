package dto

// UnifiedTool 统一工具格式，用于跨 Provider 的工具存储
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type UnifiedTool struct {
	Name        string              `json:"name" doc:"工具名称"`
	Description string              `json:"description" doc:"工具描述"`
	Parameters  *JSONSchemaProperty `json:"parameters" doc:"工具参数Schema"`
}

// FromOpenAITool 从 OpenAI OpenAIChatCompletionTool 转换为 UnifiedTool
//
//	@param tool *OpenAIChatCompletionTool
//	@return *UnifiedTool
//	@author centonhuang
//	@update 2026-03-18 10:00:00
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
//	@update 2026-03-18 10:00:00
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
