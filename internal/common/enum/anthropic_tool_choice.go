// Package enum provides common enums for the application.
package enum

// AnthropicToolChoiceType Anthropic 工具选择类型
//
//	@author centonhuang
//	@update 2026-04-09 15:00:00
type AnthropicToolChoiceType = string

const (
	// AnthropicToolChoiceTypeAuto 模型自动决定是否使用工具
	//
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	AnthropicToolChoiceTypeAuto AnthropicToolChoiceType = "auto"

	// AnthropicToolChoiceTypeAny 模型必须使用某个工具
	//
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	AnthropicToolChoiceTypeAny AnthropicToolChoiceType = "any"

	// AnthropicToolChoiceTypeNone 模型不使用工具
	//
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	AnthropicToolChoiceTypeNone AnthropicToolChoiceType = "none"

	// AnthropicToolChoiceTypeTool 模型使用指定工具
	//
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	AnthropicToolChoiceTypeTool AnthropicToolChoiceType = "tool"
)
