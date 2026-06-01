// Package enum provides common enums for the application.
package enum

// ToolChoice 工具选择模式
//
//	@author centonhuang
//	@update 2026-03-06 10:00:00
type ToolChoice = string

const (

	// ToolChoiceNone 模型不会调用任何工具，而是生成消息
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	ToolChoiceNone ToolChoice = "none"

	// ToolChoiceAuto 模型可以在生成消息和调用一个或多个工具之间选择
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	ToolChoiceAuto ToolChoice = "auto"

	// ToolChoiceRequired 模型必须调用一个或多个工具
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	ToolChoiceRequired ToolChoice = "required"
)
