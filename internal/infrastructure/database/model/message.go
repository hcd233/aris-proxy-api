// Package model defines the database schema for the model.
//
//	update 2024-06-22 09:33:43
package model

import "github.com/hcd233/aris-proxy-api/internal/dto"

// Message 消息数据库模型
//
//	@author centonhuang
//	@update 2026-03-29 10:00:00
type Message struct {
	BaseModel
	ID         uint                `json:"id" gorm:"column:id;primary_key;auto_increment;comment:消息ID"`
	Model      string              `json:"model" gorm:"column:model;not null;default:'';comment:模型"`
	Message    *dto.UnifiedMessage `json:"message" gorm:"column:message;not null;comment:消息;serializer:json"`
	CheckSum   string              `json:"check_sum" gorm:"column:check_sum;not null;default:'';comment:校验和"`
	TokenCount int                 `json:"token_count" gorm:"column:token_count;not null;default:0;comment:消息token数量"`
}
