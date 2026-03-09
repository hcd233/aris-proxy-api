// Package model defines the database schema for the model.
//
//	update 2024-06-22 09:33:43
package model

import "github.com/hcd233/aris-proxy-api/internal/dto"

// User 用户数据库模型
//
//	author centonhuang
//	update 2024-06-22 09:36:22
type Message struct {
	BaseModel
	ID         uint                              `json:"id" gorm:"column:id;primary_key;auto_increment;comment:用户ID"`
	APIKeyName string                            `json:"api_key_name" gorm:"column:api_key_name;not null;default:'';comment:API Key 名称"`
	Message    dto.ChatCompletionMessageResponse `json:"message" gorm:"column:message;not null;comment:消息;serializer:json"`
}
