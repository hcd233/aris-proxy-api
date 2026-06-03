package vo

// UnifiedTool 统一工具格式，用于跨 Provider 的工具存储
//
//	@author centonhuang
//	@update 2026-04-22 14:10:00
type UnifiedTool struct {
	Name        string              `json:"name" doc:"工具名称"`
	Description string              `json:"description" doc:"工具描述"`
	Parameters  *JSONSchemaProperty `json:"parameters" doc:"工具参数Schema"`
}
