// Package model defines the database schema for the model.
//
//	update 2024-06-22 09:33:43
package model

import "time"

// Session 会话数据库模型
//
//	@author centonhuang
//	@update 2024-06-22 09:33:43
type Session struct {
	BaseModel
	ID         uint              `json:"id" gorm:"column:id;primary_key;auto_increment;comment:会话ID"`
	APIKeyName string            `json:"api_key_name" gorm:"column:api_key_name;not null;default:'';comment:API密钥名称"`
	MessageIDs []uint            `json:"message_ids" gorm:"column:message_ids;not null;comment:消息ID列表;serializer:json"`
	ToolIDs    []uint            `json:"tool_ids" gorm:"column:tool_ids;not null;comment:工具ID列表;serializer:json"`
	Questions  []uint            `json:"questions" gorm:"column:questions;comment:用户提问消息ID列表(仅role=user且tool_call_id为空);serializer:json"`
	Models     []string          `json:"models" gorm:"column:models;comment:回答模型列表;serializer:json"`
	Metadata   map[string]string `json:"metadata" gorm:"column:metadata;comment:请求元数据;serializer:json"`
	Score      *int              `json:"score" gorm:"column:score;comment:人工评分(1-5)"`
	ScoredAt   *time.Time        `json:"scored_at" gorm:"column:scored_at;comment:评分时间"`
}
