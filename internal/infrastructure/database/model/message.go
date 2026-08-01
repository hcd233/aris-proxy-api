// Package model defines the database schema for the model.
//
//	update 2024-06-22 09:33:43
package model

import "github.com/hcd233/aris-proxy-api/internal/common/vo"

// Message 消息数据库模型
//
//	@author centonhuang
//	@update 2026-04-09 10:00:00
type Message struct {
	BaseModel
	ID       uint               `json:"id" gorm:"column:id;primary_key;auto_increment;comment:消息ID"`
	ModelID  string             `json:"model_id" gorm:"column:model_id;not null;default:'';comment:业务模型ID(创建默认=alias)"`
	Message  *vo.UnifiedMessage `json:"message" gorm:"column:message;not null;comment:消息;serializer:json"`
	CheckSum string             `json:"check_sum" gorm:"column:check_sum;not null;default:'';uniqueIndex:idx_message_checksum,comment:校验和"`
}
