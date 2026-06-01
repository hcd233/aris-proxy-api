// Package enum provides common enums for the application.
package enum

// Role 角色类型
//
//	@author centonhuang
//	@update 2026-03-06 10:00:00
type Role = string

const (

	// RoleDeveloper 开发者角色，开发者提供的指令，模型应该遵循
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	RoleDeveloper Role = "developer"

	// RoleSystem 系统角色，开发者提供的指令（在o1模型及更新版本中，使用developer角色替代）
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	RoleSystem Role = "system"

	// RoleUser 用户角色，终端用户发送的消息
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	RoleUser Role = "user"

	// RoleAssistant 助手角色，模型响应用户消息
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	RoleAssistant Role = "assistant"

	// RoleTool 工具角色，工具消息的响应
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	RoleTool Role = "tool"

	// RoleFunction 函数角色，函数消息的响应（已废弃）
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	RoleFunction Role = "function"
)
