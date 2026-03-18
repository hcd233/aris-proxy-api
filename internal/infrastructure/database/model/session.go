// Package model defines the database schema for the model.
//
//	update 2024-06-22 09:33:43
package model

// User 用户数据库模型
//
//	author centonhuang
//	update 2026-03-18 10:00:00
type Session struct {
	BaseModel
	ID         uint   `json:"id" gorm:"column:id;primary_key;auto_increment;comment:用户ID"`
	APIKeyName string `json:"api_key_name" gorm:"column:api_key_name;not null;default:'';comment:API密钥名称"`
	MessageIDs []uint `json:"message_ids" gorm:"column:message_ids;not null;comment:消息ID列表;serializer:json"`
	ToolIDs    []uint `json:"tool_ids" gorm:"column:tool_ids;not null;comment:工具ID列表;serializer:json"`
}
